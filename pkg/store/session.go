package store

import (
	"fmt"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/dshmyz/gracedb/pkg/types"
)

const (
	sesPrefix = "ses:"
	msgPrefix = "msg:"
	docPrefix = "doc:"
)

// CreateSession creates a new session.
func (s *BadgerStore) CreateSession(name string) (*types.Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	sess := &types.Session{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}

	err := s.Update(func(txn *badger.Txn) error {
		data, err := marshal(sess)
		if err != nil {
			return err
		}
		return txn.Set([]byte(sesPrefix+sess.ID), data)
	})
	if err != nil {
		return nil, err
	}
	return sess, nil
}

// GetSession retrieves a session by ID.
func (s *BadgerStore) GetSession(id string) (*types.Session, error) {
	var sess *types.Session
	err := s.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(sesPrefix + id))
		if err != nil {
			return toNotFound(err)
		}
		return item.Value(func(val []byte) error {
			sess = &types.Session{}
			return unmarshal(cloneBytes(val), sess)
		})
	})
	return sess, err
}

// ListSessions returns all sessions.
func (s *BadgerStore) ListSessions() ([]*types.Session, error) {
	var result []*types.Session
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(sesPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				sess := &types.Session{}
				if err := unmarshal(cloneBytes(val), sess); err != nil {
					return err
				}
				result = append(result, sess)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

// DeleteSession deletes a session and all its messages.
func (s *BadgerStore) DeleteSession(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		if err := txn.Delete([]byte(sesPrefix + id)); err != nil && err != badger.ErrKeyNotFound {
			return err
		}
		prefix := []byte(fmt.Sprintf("%s%s:", msgPrefix, id))
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		for it.Rewind(); it.Valid(); it.Next() {
			key := make([]byte, len(it.Item().Key()))
			copy(key, it.Item().Key())
			if err := txn.Delete(key); err != nil {
				it.Close()
				return err
			}
		}
		it.Close()
		return nil
	})
}

// AddMessage adds a message to a session.
func (s *BadgerStore) AddMessage(msg *types.Message) error {
	if msg.ID == "" {
		msg.ID = uuid.New().String()
	}
	if msg.CreatedAt.IsZero() {
		msg.CreatedAt = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// Verify session exists.
	if _, err := s.GetSession(msg.SessionID); err != nil {
		return err
	}

	return s.Update(func(txn *badger.Txn) error {
		data, err := marshal(msg)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(fmt.Sprintf("%s%s:%s", msgPrefix, msg.SessionID, msg.ID)), data); err != nil {
			return err
		}
		// Update session's UpdatedAt.
		sessItem, err := txn.Get([]byte(sesPrefix + msg.SessionID))
		if err != nil {
			return err
		}
		return sessItem.Value(func(val []byte) error {
			sess := &types.Session{}
			if err := unmarshal(cloneBytes(val), sess); err != nil {
				return err
			}
			sess.UpdatedAt = time.Now()
			data, err := marshal(sess)
			if err != nil {
				return err
			}
			return txn.Set([]byte(sesPrefix+msg.SessionID), data)
		})
	})
}

// GetSessionHistory returns messages for a session ordered by creation time.
func (s *BadgerStore) GetSessionHistory(sessionID string, limit int) ([]*types.Message, error) {
	msgs, err := s.GetMessages(sessionID)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(msgs) > limit {
		msgs = msgs[len(msgs)-limit:]
	}
	return msgs, nil
}

// GetMessages returns all messages for a session, ordered by creation time.
func (s *BadgerStore) GetMessages(sessionID string) ([]*types.Message, error) {
	var msgs []*types.Message
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(fmt.Sprintf("%s%s:", msgPrefix, sessionID))
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				msg := &types.Message{}
				if err := unmarshal(cloneBytes(val), msg); err != nil {
					return err
				}
				msgs = append(msgs, msg)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	sort.Slice(msgs, func(i, j int) bool {
		return msgs[i].CreatedAt.Before(msgs[j].CreatedAt)
	})
	return msgs, err
}

// DeleteMessage deletes a single message.
func (s *BadgerStore) DeleteMessage(sessionID, messageID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(fmt.Sprintf("%s%s:%s", msgPrefix, sessionID, messageID)))
	})
}

// CreateDocument creates a new document.
func (s *BadgerStore) CreateDocument(doc *types.Document) error {
	if doc.ID == "" {
		doc.ID = uuid.New().String()
	}
	if doc.CreatedAt.IsZero() {
		doc.CreatedAt = time.Now()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		data, err := marshal(doc)
		if err != nil {
			return err
		}
		return txn.Set([]byte(docPrefix+doc.ID), data)
	})
}

// GetDocument retrieves a document by ID.
func (s *BadgerStore) GetDocument(id string) (*types.Document, error) {
	var doc *types.Document
	err := s.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(docPrefix + id))
		if err != nil {
			return toNotFound(err)
		}
		return item.Value(func(val []byte) error {
			doc = &types.Document{}
			return unmarshal(cloneBytes(val), doc)
		})
	})
	return doc, err
}

// ListDocuments returns all documents.
func (s *BadgerStore) ListDocuments() ([]*types.Document, error) {
	var result []*types.Document
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(docPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			err := item.Value(func(val []byte) error {
				doc := &types.Document{}
				if err := unmarshal(cloneBytes(val), doc); err != nil {
					return err
				}
				result = append(result, doc)
				return nil
			})
			if err != nil {
				return err
			}
		}
		return nil
	})
	return result, err
}

// DeleteDocument deletes a document by ID.
func (s *BadgerStore) DeleteDocument(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(docPrefix + id))
	})
}

// Stats returns database statistics.
func (s *BadgerStore) Stats() (types.StoreStats, error) {
	var stats types.StoreStats

	cols, err := s.ListCollections()
	if err == nil {
		stats.CollectionCount = len(cols)
	}

	sessions, err := s.ListSessions()
	if err == nil {
		stats.SessionCount = len(sessions)
	}

	docs, err := s.ListDocuments()
	if err == nil {
		stats.DocumentCount = len(docs)
	}

	for _, c := range cols {
		n, _ := s.EmbeddingCount(c.ID)
		stats.EmbeddingCount += n
	}

	for _, sess := range sessions {
		msgs, _ := s.GetMessages(sess.ID)
		stats.MessageCount += len(msgs)
	}

	return stats, nil
}
