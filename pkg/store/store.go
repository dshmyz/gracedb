package store

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dshmyz/gracedb/pkg/index"
	"github.com/dshmyz/gracedb/pkg/types"
)

// BadgerStore is the Badger-backed storage layer for gracedb.
type BadgerStore struct {
	db      *badger.DB
	config  *types.Config
	mu      sync.RWMutex
	indexes map[string]index.Index // collectionID → in-memory vector index
	// bucketID -> in-memory semantic memory index
	memoryIndexes map[string]index.Index
	idxType       string   // "hnsw" / "ivf" / "flat" / "lsh"
	idxTypes      []string // multi-index types

	// Graceful shutdown
	closeMu sync.RWMutex
	closed  bool

	// TTL cleanup
	cleanupCancel context.CancelFunc
	cleanupDone   chan struct{}
}

// New creates and opens a BadgerStore.
func New(cfg *types.Config) (*BadgerStore, error) {
	opts := badger.DefaultOptions(cfg.Path).
		WithInMemory(cfg.Path == "").
		WithValueLogFileSize(100 * 1024 * 1024)

	if cfg.Path == "" {
		opts = opts.WithInMemory(true)
	}

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	idxType := cfg.IndexType
	if idxType == "" {
		idxType = "flat"
	}

	return &BadgerStore{
		db:            db,
		config:        cfg,
		indexes:       make(map[string]index.Index),
		memoryIndexes: make(map[string]index.Index),
		idxType:       idxType,
		idxTypes:      cfg.IndexTypes,
	}, nil
}

// Close drains in-flight operations and closes the Badger database.
func (s *BadgerStore) Close() error {
	s.closeMu.Lock()
	defer s.closeMu.Unlock()

	if s.closed {
		return nil
	}
	s.closed = true

	// Stop background cleanup.
	if s.cleanupCancel != nil {
		s.cleanupCancel()
	}
	if s.cleanupDone != nil {
		<-s.cleanupDone
	}

	return s.db.Close()
}

// Ready reports whether the store is accessible and not closing.
func (s *BadgerStore) Ready() bool {
	s.closeMu.RLock()
	if s.closed {
		s.closeMu.RUnlock()
		return false
	}
	s.closeMu.RUnlock()

	err := s.db.View(func(txn *badger.Txn) error {
		return nil
	})
	return err == nil
}

// DB returns the raw Badger DB for advanced operations.
func (s *BadgerStore) DB() *badger.DB {
	return s.db
}

// View wraps db.View with close protection.
func (s *BadgerStore) View(fn func(txn *badger.Txn) error) error {
	s.closeMu.RLock()
	if s.closed {
		s.closeMu.RUnlock()
		return types.ErrClosed
	}
	defer s.closeMu.RUnlock()
	return s.db.View(fn)
}

// Update wraps db.Update with close protection.
func (s *BadgerStore) Update(fn func(txn *badger.Txn) error) error {
	s.closeMu.RLock()
	if s.closed {
		s.closeMu.RUnlock()
		return types.ErrClosed
	}
	defer s.closeMu.RUnlock()
	return s.db.Update(fn)
}

// StartMemoryCleanup launches a background goroutine that periodically removes
// expired memory records. Stops when ctx is cancelled or Close() is called.
// Safe to call multiple times — subsequent calls are no-ops.
func (s *BadgerStore) StartMemoryCleanup(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cleanupCancel != nil {
		return // already running
	}

	interval := s.config.TTLInterval
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	cleanupCtx, cancel := context.WithCancel(ctx)
	s.cleanupCancel = cancel
	s.cleanupDone = make(chan struct{})

	go func() {
		defer close(s.cleanupDone)
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-cleanupCtx.Done():
				return
			case <-ticker.C:
				count, err := s.deleteExpiredMemories()
				if err != nil {
					slog.Warn("memory cleanup failed", "error", err)
				} else if count > 0 {
					slog.Info("memory cleanup removed expired entries",
						"count", count)
				}
			}
		}
	}()
}

// deleteExpiredMemories scans all memory index entries and removes expired ones.
func (s *BadgerStore) deleteExpiredMemories() (int, error) {
	var expired []string

	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte("mem:idx:")
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			memoryID := string(key[len("mem:idx:"):])

			// Load metadata to check TTL.
			idxItem := it.Item()
			var bucketID string
			if err := idxItem.Value(func(val []byte) error {
				bucketID = string(cloneBytes(val))
				return nil
			}); err != nil {
				continue
			}

			memKey := []byte("mem:" + bucketID + ":" + memoryID)
			metaItem, err := txn.Get(memKey)
			if err != nil {
				continue
			}

			var metadata map[string]any
			if err := metaItem.Value(func(val []byte) error {
				return json.Unmarshal(cloneBytes(val), &metadata)
			}); err != nil {
				continue
			}

			if isMetadataExpired(metadata) {
				expired = append(expired, memoryID)
			}
		}
		return nil
	})
	if err != nil {
		return 0, err
	}

	if len(expired) == 0 {
		return 0, nil
	}

	// Delete expired memories in batches.
	for _, memoryID := range expired {
		if err := s.DeleteMemoryDirect(memoryID); err != nil {
			slog.Warn("failed to delete expired memory", "id", memoryID, "error", err)
		}
	}

	return len(expired), nil
}

// isMetadataExpired checks if the expires_at field in metadata is in the past.
func isMetadataExpired(metadata map[string]any) bool {
	v, ok := metadata["expires_at"]
	if !ok {
		return false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return false
	}
	return t.Before(time.Now().UTC())
}

// DeleteMemoryDirect deletes a memory by ID for the cleanup goroutine.
func (s *BadgerStore) DeleteMemoryDirect(memoryID string) error {
	var bucketID string
	err := s.Update(func(txn *badger.Txn) error {
		// Look up bucket ID via index.
		idxKey := []byte("mem:idx:" + memoryID)
		idxItem, err := txn.Get(idxKey)
		if err != nil {
			return err
		}
		idxItem.Value(func(val []byte) error {
			bucketID = string(cloneBytes(val))
			return nil
		})

		keys := [][]byte{
			[]byte("mem:" + bucketID + ":" + memoryID),
			[]byte("mem:content:" + bucketID + ":" + memoryID),
			[]byte("mem:vec:" + bucketID + ":" + memoryID),
			idxKey,
		}
		for _, k := range keys {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		if err := deleteMemoryFTSEntries(txn, bucketID, memoryID); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}
	s.mu.Lock()
	if idx, ok := s.memoryIndexes[bucketID]; ok {
		idx.RemoveVector(memoryID)
	}
	s.mu.Unlock()
	return nil
}

func marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

func cloneBytes(b []byte) []byte {
	out := make([]byte, len(b))
	copy(out, b)
	return out
}
