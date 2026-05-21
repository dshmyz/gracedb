package gracedb

import (
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func TestDB_InsertText_WithEmbedder(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("text_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	embID, err := db.InsertText("text_test", "txt-1", "hello world", nil)
	if err != nil {
		t.Fatalf("insert text: %v", err)
	}
	if embID == "" {
		t.Fatal("expected non-empty embID")
	}

	emb, err := db.GetEmbedding("text_test", embID, true)
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}
	if emb.Content != "hello world" {
		t.Errorf("expected 'hello world', got '%s'", emb.Content)
	}
	if len(emb.Vector) != 16 {
		t.Errorf("expected 16-dim vector, got %d", len(emb.Vector))
	}
}

func TestDB_InsertText_NoEmbedder(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("no_emb_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := db.InsertText("no_emb_test", "x", "hello", nil)
	if err == nil {
		t.Fatal("expected error without embedder")
	}
	if err != types.ErrEmbedderNotConfigured {
		t.Errorf("expected ErrEmbedderNotConfigured, got %v", err)
	}
}

func TestDB_InsertText_EmptyContent(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("empty_text_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := db.InsertText("empty_text_test", "x", "", nil)
	if err == nil {
		t.Fatal("expected error for empty content")
	}
	if err != types.ErrEmptyText {
		t.Errorf("expected ErrEmptyText, got %v", err)
	}
}

func TestDB_InsertTextBatch_WithEmbedder(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("batch_text_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	texts := []string{"hello", "world", "foo"}
	docIDs := []string{"h", "w", "f"}
	metadata := []map[string]string{{"src": "a"}, {"src": "b"}, {"src": "c"}}

	embIDs, err := db.InsertTextBatch("batch_text_test", texts, docIDs, metadata)
	if err != nil {
		t.Fatalf("insert text batch: %v", err)
	}
	if len(embIDs) != 3 {
		t.Fatalf("expected 3 embIDs, got %d", len(embIDs))
	}
}

func TestDB_InsertTextBatch_SkipsEmptyTexts(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("skip_empty_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	texts := []string{"hello", "", "world"}
	docIDs := []string{"h", "empty", "w"}

	embIDs, err := db.InsertTextBatch("skip_empty_test", texts, docIDs, nil)
	if err != nil {
		t.Fatalf("insert text batch: %v", err)
	}
	if len(embIDs) != 2 {
		t.Fatalf("expected 2 embIDs (skipping empty), got %d", len(embIDs))
	}
}

func TestDB_InsertTextBatch_AllEmpty(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("all_empty_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	embIDs, err := db.InsertTextBatch("all_empty_test", []string{"", "", ""}, nil, nil)
	if err != nil {
		t.Fatalf("insert text batch all empty: %v", err)
	}
	if embIDs != nil {
		t.Error("expected nil result for all-empty batch")
	}
}

func TestDB_InsertTextBatch_NoEmbedder(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("batch_no_emb_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := db.InsertTextBatch("batch_no_emb_test", []string{"hello"}, []string{"h"}, nil)
	if err == nil {
		t.Fatal("expected error without embedder")
	}
}

func TestDB_SearchText_WithEmbedder(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("search_text_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	if _, err := db.InsertText("search_text_test", "d1", "hello world", nil); err != nil {
		t.Fatalf("insert d1: %v", err)
	}
	if _, err := db.InsertText("search_text_test", "d2", "foo bar", nil); err != nil {
		t.Fatalf("insert d2: %v", err)
	}

	results, err := db.SearchText("search_text_test", "hello", 5)
	if err != nil {
		t.Fatalf("search text: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'hello'")
	}
}

func TestDB_SearchText_NoEmbedder_Fallback(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("search_fts_fallback"); err != nil {
		t.Fatalf("create: %v", err)
	}

	// Insert with explicit vector (no embedder)
	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("search_fts_fallback", "f1", vec, "hello world", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	results, err := db.SearchText("search_fts_fallback", "hello", 5)
	if err != nil {
		t.Fatalf("search text fallback: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected FTS fallback results")
	}
}

func TestDB_SearchText_EmptyQuery(t *testing.T) {
	db := testDBWithEmbedder(t, 16)

	if _, err := db.CreateCollection("empty_query_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err := db.SearchText("empty_query_test", "", 5)
	if err == nil {
		t.Fatal("expected error for empty query")
	}
}
