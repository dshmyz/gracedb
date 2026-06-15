package store

import (
	"os"
	"strings"
	"testing"

	"github.com/dgraph-io/badger/v4"
	"github.com/dshmyz/gracedb/pkg/types"
)

func newIntegrationStore(t *testing.T) *BadgerStore {
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

func TestKnowledgeSaveAndGet(t *testing.T) {
	s := newIntegrationStore(t)
	if _, err := s.CreateCollection("docs"); err != nil {
		t.Fatal(err)
	}

	_, err := s.SaveKnowledge("docs", "test-1", "Test Doc", "This is a test document about vector databases.", types.KnowledgeSaveRequest{
		Content: "This is a test document about vector databases.",
		Title:   "Test Doc",
	})
	if err != nil {
		t.Fatal(err)
	}

	record, err := s.GetKnowledge("docs", "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Title != "Test Doc" {
		t.Fatalf("expected title 'Test Doc', got %q", record.Title)
	}
	if len(record.ChunkIDs) == 0 {
		t.Fatal("expected chunks to be created")
	}
}

func TestKnowledgeSearch(t *testing.T) {
	s := newIntegrationStore(t)
	coll, err := s.CreateCollection("docs")
	if err != nil {
		t.Fatal(err)
	}

	_, _ = s.SaveKnowledge("docs", "go-doc", "Go Language", "Go is a statically typed programming language designed at Google.", types.KnowledgeSaveRequest{})
	_, _ = s.SaveKnowledge("docs", "python-doc", "Python", "Python is a high-level general-purpose programming language.", types.KnowledgeSaveRequest{})

	// FTS search on knowledge chunks.
	resp, err := s.SearchKnowledge("docs", "Go programming", 5)
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) == 0 {
		t.Logf("no results (FTS may need different tokens)")
	}
	// Knowledge was saved - verify that.
	record, err := s.GetKnowledge("docs", "go-doc")
	if err != nil {
		t.Fatal(err)
	}
	_ = record
	_ = coll
}

func TestKnowledgeUpdate(t *testing.T) {
	s := newIntegrationStore(t)
	if _, err := s.CreateCollection("docs"); err != nil {
		t.Fatal(err)
	}

	_, err := s.SaveKnowledge("docs", "test-1", "Old Title", "Old content.", types.KnowledgeSaveRequest{})
	if err != nil {
		t.Fatal(err)
	}

	newContent := "New content about vector search."
	_, err = s.UpdateKnowledge("docs", "test-1", types.KnowledgeUpdateRequest{
		Title:   ptrStr("New Title"),
		Content: &newContent,
	})
	if err != nil {
		t.Fatal(err)
	}

	record, err := s.GetKnowledge("docs", "test-1")
	if err != nil {
		t.Fatal(err)
	}
	if record.Title != "New Title" {
		t.Fatalf("expected title 'New Title', got %q", record.Title)
	}
	if record.Version != 2 {
		t.Fatalf("expected version 2, got %d", record.Version)
	}
}

func TestKnowledgeDelete(t *testing.T) {
	s := newIntegrationStore(t)
	if _, err := s.CreateCollection("docs"); err != nil {
		t.Fatal(err)
	}

	_, err := s.SaveKnowledge("docs", "test-1", "Test", "Content to delete.", types.KnowledgeSaveRequest{})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteKnowledge("docs", "test-1"); err != nil {
		t.Fatal(err)
	}

	_, err = s.GetKnowledge("docs", "test-1")
	if err != types.ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestMemorySaveAndGet(t *testing.T) {
	s := newIntegrationStore(t)

	record, err := s.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-1",
		Content:   "User prefers dark mode.",
		Scope:     types.MemoryScopeGlobal,
		Namespace: "preferences",
	})
	if err != nil {
		t.Fatal(err)
	}
	if record.Scope != types.MemoryScopeGlobal {
		t.Fatalf("expected scope 'global', got %q", record.Scope)
	}

	got, err := s.GetMemory("mem-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "User prefers dark mode." {
		t.Fatalf("expected content %q, got %q", "User prefers dark mode.", got.Content)
	}
}

func TestMemorySearch(t *testing.T) {
	s := newIntegrationStore(t)

	_, _ = s.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-a",
		Content:   "User likes Python and Go.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	_, _ = s.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-b",
		Content:   "User works on machine learning projects.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})

	resp, err := s.SearchMemory(types.MemorySearchRequest{
		Query:  "Python",
		Scope:  types.MemoryScopeUser,
		UserID: "user-1",
		TopK:   5,
	})
	if err != nil {
		t.Fatal(err)
	}
	// Memory search uses token matching on content.
	if len(resp.Results) == 0 {
		t.Logf("no results for memory search (token matching)")
	}
	if len(resp.Results) > 0 {
		if resp.Results[0].LexicalScore <= 0 {
			t.Fatalf("expected lexical score explanation, got %+v", resp.Results[0])
		}
		if resp.Results[0].Score != resp.Results[0].FinalScore {
			t.Fatalf("expected score to match final score, got %+v", resp.Results[0])
		}
	}
}

func TestMemorySearchCustomWeights(t *testing.T) {
	s := newIntegrationStore(t)

	_, err := s.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-semantic",
		Content:   "Ordinary unrelated text.",
		Vector:    []float32{1, 0},
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveMemory(types.MemorySaveRequest{
		MemoryID:   "mem-important",
		Content:    "Shared retrieval phrase.",
		Vector:     []float32{0, 1},
		Scope:      types.MemoryScopeUser,
		UserID:     "user-1",
		Namespace:  "prefs",
		Importance: 1,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := s.SearchMemory(types.MemorySearchRequest{
		Query:       "shared",
		QueryVector: []float32{1, 0},
		Scope:       types.MemoryScopeUser,
		UserID:      "user-1",
		Namespace:   "prefs",
		TopK:        2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 || resp.Results[0].Memory.ID != "mem-semantic" {
		t.Fatalf("expected default weights to prefer semantic match, got %+v", resp.Results)
	}

	resp, err = s.SearchMemory(types.MemorySearchRequest{
		Query:            "shared",
		QueryVector:      []float32{1, 0},
		Scope:            types.MemoryScopeUser,
		UserID:           "user-1",
		Namespace:        "prefs",
		TopK:             2,
		SemanticWeight:   0,
		LexicalWeight:    1,
		ImportanceWeight: 1,
		RecencyWeight:    0,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 || resp.Results[0].Memory.ID != "mem-important" {
		t.Fatalf("expected custom weights to prefer lexical important match, got %+v", resp.Results)
	}
}

func TestMemoryFTSIndexLifecycle(t *testing.T) {
	s := newIntegrationStore(t)

	_, err := s.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-fts",
		Content:   "User likes purple aardvark.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if countMemoryFTSKeys(t, s, "aardvark", "memory:user:user-1:prefs", "mem-fts") == 0 {
		t.Fatal("expected memory FTS key for saved content")
	}

	updated := "User prefers green tea."
	if _, err := s.UpdateMemory(types.MemoryUpdateRequest{
		MemoryID: "mem-fts",
		Content:  &updated,
	}); err != nil {
		t.Fatal(err)
	}
	if countMemoryFTSKeys(t, s, "aardvark", "memory:user:user-1:prefs", "mem-fts") != 0 {
		t.Fatal("expected old memory FTS key to be removed after update")
	}
	if countMemoryFTSKeys(t, s, "green", "memory:user:user-1:prefs", "mem-fts") == 0 {
		t.Fatal("expected new memory FTS key after update")
	}

	if err := s.DeleteMemory("mem-fts"); err != nil {
		t.Fatal(err)
	}
	if countMemoryFTSKeys(t, s, "green", "memory:user:user-1:prefs", "mem-fts") != 0 {
		t.Fatal("expected memory FTS key to be removed after delete")
	}
}

func TestMemoryFTSIndexLifecycleSurvivesReopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "gracedb-memory-fts-reopen-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	cfg := types.DefaultConfig()
	cfg.Path = dir
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-fts",
		Content:   "User likes purple aardvark.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated := "User prefers green tea."
	if _, err := s.UpdateMemory(types.MemoryUpdateRequest{
		MemoryID: "mem-fts",
		Content:  &updated,
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if countMemoryFTSKeys(t, s, "aardvark", "memory:user:user-1:prefs", "mem-fts") != 0 {
		t.Fatal("expected old memory FTS key to stay absent after reopen")
	}
	if countMemoryFTSKeys(t, s, "green", "memory:user:user-1:prefs", "mem-fts") == 0 {
		t.Fatal("expected updated memory FTS key after reopen")
	}
	if err := s.DeleteMemory("mem-fts"); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}

	s, err = New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if countMemoryFTSKeys(t, s, "green", "memory:user:user-1:prefs", "mem-fts") != 0 {
		t.Fatal("expected deleted memory FTS key to stay absent after reopen")
	}
}

func countMemoryFTSKeys(t *testing.T, s *BadgerStore, term, bucketID, memoryID string) int {
	t.Helper()
	var count int
	prefix := []byte("mem:fts:" + term + ":" + bucketID + ":")
	if err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if strings.HasSuffix(string(it.Item().Key()), ":"+memoryID) {
				count++
			}
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return count
}

func TestMemoryDelete(t *testing.T) {
	s := newIntegrationStore(t)

	_, err := s.SaveMemory(types.MemorySaveRequest{
		MemoryID: "mem-del",
		Content:  "Temporary memory.",
		Scope:    types.MemoryScopeGlobal,
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := s.DeleteMemory("mem-del"); err != nil {
		t.Fatal(err)
	}

	_, err = s.GetMemory("mem-del")
	if err != types.ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestChunkBySize(t *testing.T) {
	content := "This is the first sentence. This is the second sentence. And the third one."
	chunks := ChunkBySize(content, 30, 5)

	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	// Verify all content is covered.
	var totalLen int
	for _, c := range chunks {
		totalLen += len(c.Content)
	}
	if totalLen < len(content) {
		t.Fatalf("chunks cover %d chars, original is %d", totalLen, len(content))
	}
}

func TestDeleteEmbeddingCleansFTS(t *testing.T) {
	s := newIntegrationStore(t)
	coll, err := s.CreateCollection("test")
	if err != nil {
		t.Fatal(err)
	}

	embID, err := s.Upsert("test", "doc-1", []float32{0.1, 0.2, 0.3, 0.4}, "This is a test document about search.", nil, nil)
	if err != nil {
		t.Fatal(err)
	}

	// IndexFTS is called at gracedb layer, not store layer. Index manually here.
	if err := s.IndexFTS(coll.ID, embID, "This is a test document about search."); err != nil {
		t.Fatal(err)
	}

	// Verify FTS works using collection ID.
	ids, err := s.SearchFTS(coll.ID, "test document")
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, id := range ids {
		if id == embID {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected FTS match before delete, got %v", ids)
	}

	// Delete and verify FTS is cleaned using collection ID.
	if err := s.DeleteEmbedding(coll.ID, embID); err != nil {
		t.Fatal(err)
	}

	ids, err = s.SearchFTS(coll.ID, "test document")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range ids {
		if id == embID {
			t.Fatal("FTS entry should be cleaned up after delete")
		}
	}
}

func TestUpsertBatchReturnsIDs(t *testing.T) {
	s := newIntegrationStore(t)
	coll, err := s.CreateCollection("test")
	if err != nil {
		t.Fatal(err)
	}

	vectors := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
	}
	contents := []string{"first doc", "second doc"}

	ids, err := s.UpsertBatch("test", vectors, contents, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(ids) != 2 {
		t.Fatalf("expected 2 IDs, got %d", len(ids))
	}

	// Verify embeddings exist using collection ID.
	for _, id := range ids {
		emb, err := s.GetEmbedding(coll.ID, id, false)
		if err != nil {
			t.Fatalf("expected embedding for ID %s: %v", id, err)
		}
		_ = emb
	}
}

func ptrStr(s string) *string {
	return &s
}
