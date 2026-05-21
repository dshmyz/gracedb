package store

import (
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func TestVectorSearch(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	vectors := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{0.9, 0.1, 0},
	}
	contents := []string{"alpha", "beta", "gamma", "alpha-variant"}

	_, err := s.UpsertBatch("test", vectors, contents, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// Search for a vector similar to [1, 0, 0].
	results, err := s.Search("test", []float32{1, 0, 0}, types.SearchOptions{
		TopK:            2,
		UseVectorSearch: true,
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	// Top result should be the exact match.
	if results[0].Score < 0.99 {
		t.Errorf("expected top score ~1.0, got %.4f", results[0].Score)
	}
}

func TestFTS(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	embID, err := s.Upsert("test", "doc1", []float32{1, 2, 3}, "the quick brown fox jumps over the lazy dog", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	coll, _ := s.GetCollection("test")

	// Index for FTS.
	err = s.IndexFTS(coll.ID, embID, "the quick brown fox jumps over the lazy dog")
	if err != nil {
		t.Fatalf("IndexFTS failed: %v", err)
	}

	// Search for "quick fox".
	results, err := s.SearchFTSWithContent(coll.ID, "quick fox", 5)
	if err != nil {
		t.Fatalf("SearchFTSWithContent failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 FTS result")
	}
}

func TestHybridSearch(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	vectors := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
	}
	contents := []string{"hello world greeting", "goodbye farewell exit"}

	_, err := s.UpsertBatch("test", vectors, contents, nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	coll, _ := s.GetCollection("test")
	ids, _ := s.ListEmbeddingIDs(coll.ID)
	for i, id := range ids {
		if i < len(contents) {
			_ = s.IndexFTS(coll.ID, id, contents[i])
		}
	}

	// Hybrid search.
	results, err := s.Search("test", []float32{1, 0, 0}, types.SearchOptions{
		TopK:            2,
		UseVectorSearch: true,
		UseTextSearch:   true,
		Collection:      "hello",
	})
	if err != nil {
		t.Fatalf("Hybrid search failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 hybrid result")
	}
}

func TestUnindexFTS(t *testing.T) {
	s := newTestStore(t)
	s.CreateCollection("test")

	embID, _ := s.Upsert("test", "doc1", []float32{1, 2, 3}, "hello world", nil, nil)
	coll, _ := s.GetCollection("test")

	s.IndexFTS(coll.ID, embID, "hello world")

	// Verify FTS works.
	results, _ := s.SearchFTS(coll.ID, "hello")
	if len(results) == 0 {
		t.Fatal("expected FTS match before unindex")
	}

	// Unindex.
	err := s.UnindexFTS(coll.ID, embID)
	if err != nil {
		t.Fatalf("UnindexFTS failed: %v", err)
	}

	// Verify FTS no longer finds it.
	results, _ = s.SearchFTS(coll.ID, "hello")
	if len(results) != 0 {
		t.Error("expected no FTS match after unindex")
	}
}
