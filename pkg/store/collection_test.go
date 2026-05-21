package store

import (
	"os"
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func newTestStore(t *testing.T) *BadgerStore {
	t.Helper()
	dir, err := os.MkdirTemp("", "gracedb-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	cfg := types.DefaultConfig()
	cfg.Path = dir
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateCollection(t *testing.T) {
	s := newTestStore(t)

	coll, err := s.CreateCollection("test")
	if err != nil {
		t.Fatalf("CreateCollection failed: %v", err)
	}
	if coll.Name != "test" {
		t.Errorf("expected name 'test', got %q", coll.Name)
	}
	if coll.ID == "" {
		t.Error("expected non-empty ID")
	}

	// Duplicate should fail.
	_, err = s.CreateCollection("test")
	if err != types.ErrCollectionExists {
		t.Fatalf("expected ErrCollectionExists, got %v", err)
	}
}

func TestGetCollection(t *testing.T) {
	s := newTestStore(t)

	_, err := s.CreateCollection("test")
	if err != nil {
		t.Fatal(err)
	}

	coll, err := s.GetCollection("test")
	if err != nil {
		t.Fatalf("GetCollection failed: %v", err)
	}
	if coll.Name != "test" {
		t.Errorf("expected name 'test', got %q", coll.Name)
	}

	// Non-existent should return ErrNotFound.
	_, err = s.GetCollection("nonexistent")
	if err != types.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestListCollections(t *testing.T) {
	s := newTestStore(t)

	s.CreateCollection("a")
	s.CreateCollection("b")
	s.CreateCollection("c")

	colls, err := s.ListCollections()
	if err != nil {
		t.Fatalf("ListCollections failed: %v", err)
	}
	if len(colls) != 3 {
		t.Fatalf("expected 3 collections, got %d", len(colls))
	}
}

func TestDeleteCollection(t *testing.T) {
	s := newTestStore(t)

	s.CreateCollection("todelete")

	// Add an embedding to verify it gets cleaned up.
	coll, _ := s.GetCollection("todelete")
	s.Upsert("todelete", "doc1", []float32{1, 2, 3}, "hello", nil, nil)

	err := s.DeleteCollection("todelete")
	if err != nil {
		t.Fatalf("DeleteCollection failed: %v", err)
	}

	// Verify collection is gone.
	_, err = s.GetCollection("todelete")
	if err != types.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}

	// Verify embeddings are gone.
	count, _ := s.EmbeddingCount(coll.ID)
	if count != 0 {
		t.Errorf("expected 0 embeddings after collection delete, got %d", count)
	}
}
