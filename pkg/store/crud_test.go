package store

import (
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func TestUpsert(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	embID, err := s.Upsert("test", "doc1", []float32{0.1, 0.2, 0.3}, "hello world", nil, nil)
	if err != nil {
		t.Fatalf("Upsert failed: %v", err)
	}
	if embID == "" {
		t.Fatal("expected non-empty embID")
	}

	// Verify retrieval.
	coll, _ := s.GetCollection("test")
	emb, err := s.GetEmbedding(coll.ID, embID, false)
	if err != nil {
		t.Fatalf("GetEmbedding failed: %v", err)
	}
	if emb.Content != "hello world" {
		t.Errorf("expected content 'hello world', got %q", emb.Content)
	}
	if emb.DocID != "doc1" {
		t.Errorf("expected docID 'doc1', got %q", emb.DocID)
	}
}

func TestUpsertInvalidVector(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	_, err := s.Upsert("test", "doc1", nil, "hello", nil, nil)
	if err != types.ErrInvalidVector {
		t.Fatalf("expected ErrInvalidVector, got %v", err)
	}
}

func TestUpsertBatch(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	vectors := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
		{0.7, 0.8, 0.9},
	}
	contents := []string{"first", "second", "third"}
	docIDs := []string{"d1", "d2", "d3"}

	_, err := s.UpsertBatch("test", vectors, contents, docIDs, nil)
	if err != nil {
		t.Fatalf("UpsertBatch failed: %v", err)
	}

	coll, _ := s.GetCollection("test")
	count, err := s.EmbeddingCount(coll.ID)
	if err != nil {
		t.Fatalf("EmbeddingCount failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 embeddings, got %d", count)
	}
}

func TestDeleteEmbedding(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	embID, _ := s.Upsert("test", "doc1", []float32{1, 2, 3}, "test content", nil, nil)

	coll, _ := s.GetCollection("test")
	err := s.DeleteEmbedding(coll.ID, embID)
	if err != nil {
		t.Fatalf("DeleteEmbedding failed: %v", err)
	}

	count, _ := s.EmbeddingCount(coll.ID)
	if count != 0 {
		t.Errorf("expected 0 embeddings after delete, got %d", count)
	}
}

func TestDeleteByDocID(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	s.Upsert("test", "doc1", []float32{1, 2, 3}, "first", nil, nil)
	s.Upsert("test", "doc1", []float32{4, 5, 6}, "second", nil, nil)
	s.Upsert("test", "doc2", []float32{7, 8, 9}, "third", nil, nil)

	coll, _ := s.GetCollection("test")
	countBefore, _ := s.EmbeddingCount(coll.ID)
	if countBefore != 3 {
		t.Fatalf("expected 3 embeddings before delete, got %d", countBefore)
	}

	err := s.DeleteByDocID(coll.ID, "doc1")
	if err != nil {
		t.Fatalf("DeleteByDocID failed: %v", err)
	}

	countAfter, _ := s.EmbeddingCount(coll.ID)
	if countAfter != 1 {
		t.Errorf("expected 1 embedding after delete, got %d", countAfter)
	}
}

func TestDeleteBatch(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	embID1, _ := s.Upsert("test", "d1", []float32{1, 2, 3}, "a", nil, nil)
	embID2, _ := s.Upsert("test", "d2", []float32{4, 5, 6}, "b", nil, nil)

	coll, _ := s.GetCollection("test")
	err := s.DeleteBatch(coll.ID, []string{embID1, embID2})
	if err != nil {
		t.Fatalf("DeleteBatch failed: %v", err)
	}

	count, _ := s.EmbeddingCount(coll.ID)
	if count != 0 {
		t.Errorf("expected 0 embeddings after batch delete, got %d", count)
	}
}

func TestReadVectors(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	s.Upsert("test", "d1", []float32{1, 2, 3}, "hello", nil, nil)
	s.Upsert("test", "d2", []float32{4, 5, 6}, "world", nil, nil)

	coll, _ := s.GetCollection("test")
	vectors, err := s.ReadVectors(coll.ID)
	if err != nil {
		t.Fatalf("ReadVectors failed: %v", err)
	}
	if len(vectors) != 2 {
		t.Errorf("expected 2 vectors, got %d", len(vectors))
	}
}
