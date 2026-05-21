package gracedb

import (
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func TestDB_UpsertAndGetEmbedding(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("vec_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
	embID, err := db.Upsert("vec_test", "doc-1", vec, "hello world", nil, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if embID == "" {
		t.Fatal("expected non-empty embID")
	}

	emb, err := db.GetEmbedding("vec_test", embID, false)
	if err != nil {
		t.Fatalf("get embedding: %v", err)
	}
	if emb.Content != "hello world" {
		t.Errorf("expected content 'hello world', got '%s'", emb.Content)
	}
	if emb.DocID != "doc-1" {
		t.Errorf("expected doc_id 'doc-1', got '%s'", emb.DocID)
	}
}

func TestDB_UpsertBatch(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("batch_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vectors := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
		{0.9, 1.0, 1.1, 1.2},
	}
	contents := []string{"doc a", "doc b", "doc c"}
	docIDs := []string{"a", "b", "c"}

	err := db.UpsertBatch("batch_test", vectors, contents, docIDs, nil)
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	count, err := db.EmbeddingCount("batch_test")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 embeddings, got %d", count)
	}
}

func TestDB_Search_VectorOnly(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("search_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
	if _, err := db.Upsert("search_test", "d1", vec, "hello", nil, nil); err != nil {
		t.Fatalf("upsert d1: %v", err)
	}
	vec2 := []float32{0.9, 0.8, 0.7, 0.6, 0.5, 0.4, 0.3, 0.2}
	if _, err := db.Upsert("search_test", "d2", vec2, "world", nil, nil); err != nil {
		t.Fatalf("upsert d2: %v", err)
	}

	results, err := db.Search("search_test", vec, types.SearchOptions{TopK: 2, UseVectorSearch: true})
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}
	if results[0].Score <= results[1].Score {
		t.Errorf("expected results sorted by score, got %v, %v", results[0].Score, results[1].Score)
	}
}

func TestDB_Search_EmptyCollection(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("empty_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	results, err := db.Search("empty_test", []float32{0.1}, types.SearchOptions{TopK: 5, UseVectorSearch: true})
	if err != nil {
		t.Fatalf("search empty: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestDB_SearchFTS(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("fts_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("fts_test", "f1", vec, "hello world foo", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if _, err := db.Upsert("fts_test", "f2", vec, "bar baz qux", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// FTS returns matching embedding IDs
	ids, err := db.SearchFTS("fts_test", "hello")
	if err != nil {
		t.Fatalf("fts search: %v", err)
	}
	if len(ids) == 0 {
		t.Fatal("expected at least 1 FTS match for 'hello'")
	}

	results, err := db.SearchFTSWithContent("fts_test", "hello", 5)
	if err != nil {
		t.Fatalf("fts with content: %v", err)
	}
	if len(results) == 0 {
		t.Fatal("expected at least 1 result with content")
	}
	if results[0].Content == "" {
		t.Error("expected non-empty content in FTS result")
	}
}

func TestDB_DeleteEmbedding(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("del_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	embID, err := db.Upsert("del_test", "to_del", vec, "delete me", nil, nil)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}

	err = db.DeleteEmbedding("del_test", embID)
	if err != nil {
		t.Fatalf("delete embedding: %v", err)
	}

	_, err = db.GetEmbedding("del_test", embID, false)
	if err == nil {
		t.Error("expected error getting deleted embedding")
	}
}

func TestDB_DeleteByDocID(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("del_doc_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("del_doc_test", "multi-doc", vec, "doc content", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	err := db.DeleteByDocID("del_doc_test", "multi-doc")
	if err != nil {
		t.Fatalf("delete by docID: %v", err)
	}
}

func TestDB_DeleteBatch(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("batch_del_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vectors := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
		{0.9, 1.0, 1.1, 1.2},
	}
	contents := []string{"a", "b", "c"}
	docIDs := []string{"a", "b", "c"}

	if err := db.UpsertBatch("batch_del_test", vectors, contents, docIDs, nil); err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	// Get actual embedding IDs to delete
	ids, err := db.ListEmbeddingIDs("batch_del_test")
	if err != nil {
		t.Fatalf("list ids: %v", err)
	}
	if len(ids) != 3 {
		t.Fatalf("expected 3 embeddings before delete, got %d", len(ids))
	}

	// Delete first two
	err = db.DeleteEmbeddingBatch("batch_del_test", []string{ids[0], ids[1]})
	if err != nil {
		t.Fatalf("delete batch: %v", err)
	}

	count, err := db.EmbeddingCount("batch_del_test")
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 remaining, got %d", count)
	}
}

func TestDB_ListEmbeddingIDs(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("list_ids_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vectors := [][]float32{
		{0.1, 0.2, 0.3, 0.4},
		{0.5, 0.6, 0.7, 0.8},
	}
	contents := []string{"x", "y"}
	docIDs := []string{"x", "y"}

	if err := db.UpsertBatch("list_ids_test", vectors, contents, docIDs, nil); err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	ids, err := db.ListEmbeddingIDs("list_ids_test")
	if err != nil {
		t.Fatalf("list ids: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 IDs, got %d", len(ids))
	}
}

func TestDB_Search_MetadataFilter(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("meta_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("meta_test", "m1", vec, "hello", map[string]string{"color": "red"}, nil); err != nil {
		t.Fatalf("upsert m1: %v", err)
	}
	if _, err := db.Upsert("meta_test", "m2", vec, "hello", map[string]string{"color": "blue"}, nil); err != nil {
		t.Fatalf("upsert m2: %v", err)
	}

	results, err := db.Search("meta_test", vec, types.SearchOptions{
		TopK:            10,
		UseVectorSearch: true,
		MetadataFilter:  map[string]string{"color": "red"},
	})
	if err != nil {
		t.Fatalf("search with filter: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (color=red), got %d", len(results))
	}
	if results[0].Metadata["color"] != "red" {
		t.Errorf("expected color=red, got %s", results[0].Metadata["color"])
	}
}

func TestDB_Search_ACLFilter(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("acl_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("acl_test", "a1", vec, "public", nil, []string{"admin", "user"}); err != nil {
		t.Fatalf("upsert a1: %v", err)
	}
	if _, err := db.Upsert("acl_test", "a2", vec, "secret", nil, []string{"admin"}); err != nil {
		t.Fatalf("upsert a2: %v", err)
	}

	results, err := db.Search("acl_test", vec, types.SearchOptions{
		TopK:            10,
		UseVectorSearch: true,
		ACL:             []string{"user"},
	})
	if err != nil {
		t.Fatalf("search with ACL: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result (ACL=user), got %d", len(results))
	}
	if results[0].DocID != "a1" {
		t.Errorf("expected doc_id a1, got %s", results[0].DocID)
	}
}

func TestDB_RebuildIndex(t *testing.T) {
	db := testDB(t)

	if _, err := db.CreateCollection("rebuild_test"); err != nil {
		t.Fatalf("create: %v", err)
	}

	vec := []float32{0.1, 0.2, 0.3, 0.4}
	if _, err := db.Upsert("rebuild_test", "r1", vec, "hello world", nil, nil); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	err := db.RebuildIndex("rebuild_test")
	if err != nil {
		t.Fatalf("rebuild index: %v", err)
	}

	// After rebuild, FTS should still work
	ids, err := db.SearchFTS("rebuild_test", "hello")
	if err != nil {
		t.Fatalf("fts after rebuild: %v", err)
	}
	if len(ids) == 0 {
		t.Error("expected FTS match after rebuild")
	}
}
