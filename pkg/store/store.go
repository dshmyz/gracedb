package store

import (
	"encoding/json"
	"sync"

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
	idxType string                 // "hnsw" / "ivf" / "flat" / "lsh"
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
		db:      db,
		config:  cfg,
		indexes: make(map[string]index.Index),
		idxType: idxType,
	}, nil
}

// Close closes the underlying Badger database.
func (s *BadgerStore) Close() error {
	return s.db.Close()
}

// DB returns the raw Badger DB for advanced operations.
func (s *BadgerStore) DB() *badger.DB {
	return s.db
}

// View wraps db.View.
func (s *BadgerStore) View(fn func(txn *badger.Txn) error) error {
	return s.db.View(fn)
}

// Update wraps db.Update.
func (s *BadgerStore) Update(fn func(txn *badger.Txn) error) error {
	return s.db.Update(fn)
}

// RunValueLogGC runs Badger's value log garbage collection.
func (s *BadgerStore) RunValueLogGC(discardRatio float64) error {
	return s.db.RunValueLogGC(discardRatio)
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
