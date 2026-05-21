package gracedb

import (
	"context"
	"testing"
)

func TestQuick_AddAndSearch(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("quick_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	id, err := q.AddToCollection(ctx, "quick_test", vec, "hello")
	if err != nil {
		t.Fatalf("quick add: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}

	results, err := q.SearchInCollection(ctx, "quick_test", vec, 5)
	if err != nil {
		t.Fatalf("quick search: %v", err)
	}
	if len(results) < 1 {
		t.Fatal("expected at least 1 result")
	}
}

func TestQuick_SearchInCollection(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("coll_a"); err != nil {
		t.Fatalf("create coll_a: %v", err)
	}
	if _, err := db.CreateCollection("coll_b"); err != nil {
		t.Fatalf("create coll_b: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	vecA := []float32{1.0, 0.0, 0.0, 0.0}
	vecB := []float32{0.0, 1.0, 0.0, 0.0}

	if _, err := q.AddToCollection(ctx, "coll_a", vecA, "in a"); err != nil {
		t.Fatalf("add to coll_a: %v", err)
	}
	if _, err := q.AddToCollection(ctx, "coll_b", vecB, "in b"); err != nil {
		t.Fatalf("add to coll_b: %v", err)
	}

	results, err := q.SearchInCollection(ctx, "coll_a", vecA, 5)
	if err != nil {
		t.Fatalf("search in coll_a: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result in coll_a, got %d", len(results))
	}

	resultsB, err := q.SearchInCollection(ctx, "coll_a", vecB, 5)
	if err != nil {
		t.Fatalf("search coll_a with vecB: %v", err)
	}
	// Should return 1 result (vecA) with low similarity score (orthogonal vectors)
	if len(resultsB) != 1 {
		t.Errorf("expected 1 result (low score), got %d", len(resultsB))
	}
}

func TestQuick_AddText_WithEmbedder(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("quick_text_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	id, err := q.AddTextToCollection(ctx, "quick_text_test", "hello world", nil)
	if err != nil {
		t.Fatalf("quick add text: %v", err)
	}
	if id == "" {
		t.Fatal("expected non-empty ID")
	}
}

func TestQuick_AddText_NoEmbedder(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("quick_no_emb_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	_, err := q.AddTextToCollection(ctx, "quick_no_emb_test", "hello", nil)
	if err == nil {
		t.Fatal("expected error without embedder")
	}
}

func TestQuick_SearchText_WithEmbedder(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("quick_search_text_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	if _, err := q.AddTextToCollection(ctx, "quick_search_text_test", "hello world", nil); err != nil {
		t.Fatalf("add text: %v", err)
	}
	if _, err := q.AddTextToCollection(ctx, "quick_search_text_test", "foo bar", nil); err != nil {
		t.Fatalf("add text2: %v", err)
	}

	results, err := q.SearchTextInCollection(ctx, "quick_search_text_test", "hello", 5)
	if err != nil {
		t.Fatalf("quick search text: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results")
	}
}

func TestQuick_SearchTextOnly_FTS(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("quick_fts_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("quick_fts_test", "ft1", vec, "hello world", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	results, err := q.SearchTextOnlyInCollection(ctx, "quick_fts_test", "hello", 5)
	if err != nil {
		t.Fatalf("quick search text only: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected FTS results")
	}
}

func TestQuick_EmptyCollectionSearch(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("empty_quick_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	q := db.Quick()
	ctx := context.Background()

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	results, err := q.SearchInCollection(ctx, "empty_quick_test", vec, 5)
	if err != nil {
		t.Fatalf("search empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}
