package store

import (
	"fmt"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/dshmyz/gracedb/pkg/types"
)

const (
	colPrefix   = "col:"
	colIdxPrefix = "col:_idx:"
)

// CreateCollection creates a new collection.
func (s *BadgerStore) CreateCollection(name string) (*types.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Check if already exists.
	existing, _ := s.GetCollection(name)
	if existing != nil {
		return nil, types.ErrCollectionExists
	}

	now := time.Now()
	coll := &types.Collection{
		ID:        uuid.New().String(),
		Name:      name,
		CreatedAt: now,
	}

	err := s.Update(func(txn *badger.Txn) error {
		data, err := marshal(coll)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(colPrefix+name), data); err != nil {
			return err
		}
		return txn.Set([]byte(colIdxPrefix+coll.ID), []byte(name))
	})
	if err != nil {
		return nil, err
	}
	return coll, nil
}

// GetCollection retrieves a collection by name.
func (s *BadgerStore) GetCollection(name string) (*types.Collection, error) {
	var coll *types.Collection
	err := s.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(colPrefix + name))
		if err != nil {
			return toNotFound(err)
		}
		return item.Value(func(val []byte) error {
			coll = &types.Collection{}
			return unmarshal(cloneBytes(val), coll)
		})
	})
	return coll, err
}

// GetCollectionByID retrieves a collection by its ID.
func (s *BadgerStore) GetCollectionByID(id string) (*types.Collection, error) {
	var name string
	err := s.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(colIdxPrefix + id))
		if err != nil {
			return toNotFound(err)
		}
		return item.Value(func(val []byte) error {
			name = string(cloneBytes(val))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return s.GetCollection(name)
}

// ListCollections returns all collections.
func (s *BadgerStore) ListCollections() ([]*types.Collection, error) {
	var result []*types.Collection
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(colPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		idxPrefix := []byte(colIdxPrefix)
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			// Skip col:_idx:* entries.
			if len(key) >= len(idxPrefix) && string(key[:len(idxPrefix)]) == colIdxPrefix {
				continue
			}
			err := item.Value(func(val []byte) error {
				coll := &types.Collection{}
				if err := unmarshal(cloneBytes(val), coll); err != nil {
					return err
				}
				result = append(result, coll)
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

// DeleteCollection deletes a collection and all associated data.
func (s *BadgerStore) DeleteCollection(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	coll, err := s.GetCollection(name)
	if err != nil {
		return err
	}

	// Remove in-memory index.
	delete(s.indexes, coll.ID)

	return s.Update(func(txn *badger.Txn) error {
		prefixes := [][]byte{
			[]byte(fmt.Sprintf("emb:%s:", coll.ID)),
			[]byte(fmt.Sprintf("emb:vec:%s:", coll.ID)),
			[]byte(fmt.Sprintf("emb:content:%s:", coll.ID)),
			[]byte(fmt.Sprintf("idx:snapshot:%s", coll.ID)),
		}

		for _, prefix := range prefixes {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = prefix
			opts.AllVersions = false
			it := txn.NewIterator(opts)
			for it.Rewind(); it.Valid(); it.Next() {
				item := it.Item()
				key := make([]byte, len(item.Key()))
				copy(key, item.Key())
				if err := txn.Delete(key); err != nil {
					it.Close()
					return err
				}
			}
			it.Close()
		}

		// FTS keys are fts:{token}:{collectionID}:{embID}, scan fts: prefix and filter by collectionID.
		if err := deleteFTSCollection(txn, coll.ID); err != nil {
			return err
		}

		if err := txn.Delete([]byte(colPrefix + name)); err != nil {
			return err
		}
		return txn.Delete([]byte(colIdxPrefix + coll.ID))
	})
}

func toNotFound(err error) error {
	if err == badger.ErrKeyNotFound {
		return types.ErrNotFound
	}
	return err
}

// deleteFTSCollection removes all FTS entries for a given collectionID.
// FTS keys are fts:{token}:{collectionID}:{embID}, so we scan fts: prefix and match.
func deleteFTSCollection(txn *badger.Txn, collectionID string) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(ftsPrefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	collector := ":" + collectionID + ":"
	for it.Rewind(); it.Valid(); it.Next() {
		key := it.Item().Key()
		// Key format: fts:{token}:{collectionID}:{embID}
		// Check if key contains :collectionID: after the fts:{token} part.
		if idx := indexAfterPrefix(key, collector); idx >= 0 {
			k := make([]byte, len(key))
			copy(k, key)
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	return nil
}

// indexAfterPrefix checks if the key contains collector pattern after ftsPrefix.
func indexAfterPrefix(key []byte, collector string) int {
	if len(key) <= len(ftsPrefix) {
		return -1
	}
	// Skip ftsPrefix, find collector in remaining bytes.
	remaining := key[len(ftsPrefix):]
	idx := findBytes(remaining, collector)
	if idx < 0 {
		return -1
	}
	return idx + len(ftsPrefix)
}

func findBytes(haystack []byte, needle string) int {
	n := len(needle)
	for i := 0; i <= len(haystack)-n; i++ {
		if string(haystack[i:i+n]) == needle {
			return i
		}
	}
	return -1
}
