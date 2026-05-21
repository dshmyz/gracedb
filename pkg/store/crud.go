package store

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/dshmyz/gracedb/pkg/types"
)

const (
	embPrefix     = "emb:"
	embVecPrefix  = "emb:vec:"
	embContPrefix = "emb:content:"
)

// Upsert inserts or updates a single embedding in a collection.
func (s *BadgerStore) Upsert(collectionName string, docID string, vector []float32, content string, metadata map[string]string, acl []string) (string, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return "", err
	}

	embID := uuid.New().String()
	emb := &types.Embedding{
		ID:           embID,
		CollectionID: coll.ID,
		Collection:   collectionName,
		DocID:        docID,
		Vector:       vector,
		Content:      content,
		Metadata:     metadata,
		ACL:          acl,
		CreatedAt:    time.Now(),
	}

	if err := s.writeEmbedding(emb); err != nil {
		return "", err
	}
	return embID, nil
}

// UpsertBatch inserts or updates multiple embeddings and returns the created embedding IDs.
func (s *BadgerStore) UpsertBatch(collectionName string, vectors [][]float32, contents []string, docIDs []string, metadata []map[string]string) ([]string, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	n := len(vectors)
	s.mu.Lock()
	defer s.mu.Unlock()

	embIDs := make([]string, n)
	err = s.Update(func(txn *badger.Txn) error {
		for i := 0; i < n; i++ {
			embID := uuid.New().String()
			embIDs[i] = embID
			emb := &types.Embedding{
				ID:           embID,
				CollectionID: coll.ID,
				Collection:   collectionName,
				Vector:       vectors[i],
				CreatedAt:    time.Now(),
			}
			if i < len(contents) {
				emb.Content = contents[i]
			}
			if i < len(docIDs) {
				emb.DocID = docIDs[i]
			}
			if i < len(metadata) {
				emb.Metadata = metadata[i]
			}
			if err := writeEmbeddingTxn(txn, emb); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Sync in-memory index.
	if idx, ok := s.indexes[coll.ID]; ok {
		vectorsToIndex := make([][]float32, 0, n)
		idsToIndex := make([]string, 0, n)
		for i := 0; i < n; i++ {
			if len(vectors[i]) > 0 {
				vectorsToIndex = append(vectorsToIndex, vectors[i])
				idsToIndex = append(idsToIndex, embIDs[i])
			}
		}
		if len(vectorsToIndex) > 0 {
			idx.InsertBatch(vectorsToIndex, idsToIndex)
		}
	}

	return embIDs, nil
}

func (s *BadgerStore) writeEmbedding(emb *types.Embedding) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.Update(func(txn *badger.Txn) error {
		return writeEmbeddingTxn(txn, emb)
	})
	if err != nil {
		return err
	}
	// Sync in-memory index.
	if idx, ok := s.indexes[emb.CollectionID]; ok {
		idx.Insert(emb.Vector, emb.ID)
	}
	return nil
}

func writeEmbeddingTxn(txn *badger.Txn, emb *types.Embedding) error {
	if emb.Vector == nil || len(emb.Vector) == 0 {
		return types.ErrInvalidVector
	}
	if emb.ID == "" {
		emb.ID = uuid.New().String()
	}
	if emb.CreatedAt.IsZero() {
		emb.CreatedAt = time.Now()
	}

	// emb:{collectionID}:{embID} → Embedding JSON (without vector)
	embData := types.Embedding{
		ID:           emb.ID,
		CollectionID: emb.CollectionID,
		Collection:   emb.Collection,
		DocID:        emb.DocID,
		Content:      emb.Content,
		Metadata:     emb.Metadata,
		ACL:          emb.ACL,
		CreatedAt:    emb.CreatedAt,
	}
	data, err := marshal(embData)
	if err != nil {
		return err
	}
	if err := txn.Set([]byte(fmt.Sprintf("%s%s:%s", embPrefix, emb.CollectionID, emb.ID)), data); err != nil {
		return err
	}

	// emb:vec:{collectionID}:{embID} → binary vector
	vecData := vectorToBytes(emb.Vector)
	if err := txn.Set([]byte(fmt.Sprintf("%s%s:%s", embVecPrefix, emb.CollectionID, emb.ID)), vecData); err != nil {
		return err
	}

	// emb:content:{collectionID}:{embID} → content text (for FTS)
	if emb.Content != "" {
		if err := txn.Set([]byte(fmt.Sprintf("%s%s:%s", embContPrefix, emb.CollectionID, emb.ID)), []byte(emb.Content)); err != nil {
			return err
		}
	}

	return nil
}

// vectorToBytes encodes a float32 slice: [4-byte length][float32s as little-endian].
func vectorToBytes(v []float32) []byte {
	buf := make([]byte, 4+len(v)*4)
	binary.LittleEndian.PutUint32(buf[:4], uint32(len(v)))
	for i, f := range v {
		binary.LittleEndian.PutUint32(buf[4+i*4:], math.Float32bits(f))
	}
	return buf
}

// bytesToVector decodes a float32 slice from binary format.
func bytesToVector(data []byte) []float32 {
	if len(data) < 4 {
		return nil
	}
	n := int(binary.LittleEndian.Uint32(data[:4]))
	vec := make([]float32, n)
	for i := 0; i < n; i++ {
		vec[i] = math.Float32frombits(binary.LittleEndian.Uint32(data[4+i*4:]))
	}
	return vec
}

// GetEmbedding retrieves an embedding by ID.
func (s *BadgerStore) GetEmbedding(collectionID, embID string, includeVector bool) (*types.Embedding, error) {
	var emb *types.Embedding
	var vec []float32

	err := s.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(fmt.Sprintf("%s%s:%s", embPrefix, collectionID, embID)))
		if err != nil {
			return toNotFound(err)
		}
		if err := item.Value(func(val []byte) error {
			emb = &types.Embedding{}
			return unmarshal(cloneBytes(val), emb)
		}); err != nil {
			return err
		}

		if includeVector {
			item, err := txn.Get([]byte(fmt.Sprintf("%s%s:%s", embVecPrefix, collectionID, embID)))
			if err != nil && err != badger.ErrKeyNotFound {
				return err
			}
			if err == nil {
				item.Value(func(val []byte) error {
					vec = bytesToVector(cloneBytes(val))
					return nil
				})
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	if vec != nil {
		emb.Vector = vec
	}
	return emb, nil
}

// DeleteEmbedding deletes a single embedding by ID, including FTS index entries.
func (s *BadgerStore) DeleteEmbedding(collectionID, embID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from in-memory index first.
	if idx, ok := s.indexes[collectionID]; ok {
		idx.RemoveVector(embID)
	}

	return s.Update(func(txn *badger.Txn) error {
		// Clean FTS entries within the same transaction.
		if err := deleteFTSEntries(txn, collectionID, embID); err != nil {
			return err
		}

		keys := [][]byte{
			[]byte(fmt.Sprintf("%s%s:%s", embPrefix, collectionID, embID)),
			[]byte(fmt.Sprintf("%s%s:%s", embVecPrefix, collectionID, embID)),
			[]byte(fmt.Sprintf("%s%s:%s", embContPrefix, collectionID, embID)),
		}
		for _, k := range keys {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

// DeleteByDocID deletes all embeddings associated with a document ID, including FTS entries.
func (s *BadgerStore) DeleteByDocID(collectionID, docID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(fmt.Sprintf("%s%s:", embPrefix, collectionID))
		it := txn.NewIterator(opts)
		defer it.Close()

		var embIDs []string
		var toDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			var embData *types.Embedding
			item.Value(func(val []byte) error {
				e := &types.Embedding{}
				if unmarshal(cloneBytes(val), e) == nil {
					embData = e
				}
				return nil
			})
			if embData != nil && embData.DocID == docID {
				id := embData.ID
				embIDs = append(embIDs, id)
				toDelete = append(toDelete,
					[]byte(fmt.Sprintf("%s%s:%s", embPrefix, collectionID, id)),
					[]byte(fmt.Sprintf("%s%s:%s", embVecPrefix, collectionID, id)),
					[]byte(fmt.Sprintf("%s%s:%s", embContPrefix, collectionID, id)),
				)
			}
		}

		for _, k := range toDelete {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}

		// Clean FTS entries for all deleted embeddings.
		for _, id := range embIDs {
			if err := deleteFTSEntries(txn, collectionID, id); err != nil {
				return err
			}
		}

		// Remove from in-memory index.
		if idx, ok := s.indexes[collectionID]; ok {
			for _, id := range embIDs {
				idx.RemoveVector(id)
			}
		}

		return nil
	})
}

// ListEmbeddingIDs returns all embedding IDs for a collection.
func (s *BadgerStore) ListEmbeddingIDs(collectionID string) ([]string, error) {
	var ids []string
	prefix := []byte(fmt.Sprintf("%s%s:", embPrefix, collectionID))
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			embID := string(key[len(prefix):])
			ids = append(ids, embID)
		}
		return nil
	})
	return ids, err
}

// ReadVectors loads all vectors for a collection into a map[embID]vector.
func (s *BadgerStore) ReadVectors(collectionID string) (map[string][]float32, error) {
	vectors := make(map[string][]float32)
	prefix := []byte(fmt.Sprintf("%s%s:", embVecPrefix, collectionID))
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			key := item.Key()
			embID := string(key[len(prefix):])
			item.Value(func(val []byte) error {
				vectors[embID] = bytesToVector(cloneBytes(val))
				return nil
			})
		}
		return nil
	})
	return vectors, err
}

// EmbeddingCount returns the number of embeddings in a collection.
func (s *BadgerStore) EmbeddingCount(collectionID string) (int, error) {
	count := 0
	prefix := []byte(fmt.Sprintf("%s%s:", embPrefix, collectionID))
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			count++
		}
		return nil
	})
	return count, err
}

// deleteFTSEntries removes all FTS inverted index entries for an embedding within the current transaction.
func deleteFTSEntries(txn *badger.Txn, collectionID, embID string) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(ftsPrefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	suffix := ":" + collectionID + ":" + embID
	for it.Rewind(); it.Valid(); it.Next() {
		key := it.Item().Key()
		if len(key) > len(ftsPrefix) && hasSuffix(key, suffix) {
			k := make([]byte, len(key))
			copy(k, key)
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	return nil
}

// hasSuffix checks if byte slice ends with the given string suffix.
func hasSuffix(b []byte, suffix string) bool {
	n := len(b)
	if n < len(suffix) {
		return false
	}
	return string(b[n-len(suffix):]) == suffix
}
