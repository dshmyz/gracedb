package gracedb

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/dshmyz/gracedb/pkg/types"
)

type topicEmbedder struct{}

func (topicEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	switch text {
	case "User loves feline companions.", "cat question":
		return []float32{1, 0, 0}, nil
	case "User prefers green tea.", "tea question":
		return []float32{0, 1, 0}, nil
	default:
		return []float32{0, 0, 1}, nil
	}
}

func (e topicEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, text := range texts {
		vec, err := e.Embed(ctx, text)
		if err != nil {
			return nil, err
		}
		vectors[i] = vec
	}
	return vectors, nil
}

func (topicEmbedder) Dimension() int { return 3 }

func TestDB_SearchMemory_UsesSemanticVectorSearch(t *testing.T) {
	db := testDB(t, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))

	_, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-tea",
		Content:   "User prefers green tea.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Memory.ID != "mem-cat" {
		t.Fatalf("expected semantic result mem-cat, got %+v", resp.Results)
	}
	if resp.Results[0].SemanticScore <= 0 {
		t.Fatalf("expected semantic score explanation, got %+v", resp.Results[0])
	}
	if resp.Results[0].Score != resp.Results[0].FinalScore {
		t.Fatalf("expected Score to match FinalScore, got score=%f final=%f", resp.Results[0].Score, resp.Results[0].FinalScore)
	}
}

func TestDB_SearchMemory_ExplainsLexicalScore(t *testing.T) {
	db := testDB(t, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))

	_, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "feline companions",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 {
		t.Fatalf("expected lexical result, got %+v", resp.Results)
	}
	if resp.Results[0].LexicalScore <= 0 {
		t.Fatalf("expected lexical score explanation, got %+v", resp.Results[0])
	}
	if resp.Results[0].FinalScore <= 0 || resp.Results[0].Score != resp.Results[0].FinalScore {
		t.Fatalf("expected final score explanation, got %+v", resp.Results[0])
	}
}

func TestDB_SearchMemory_RanksImportanceAndRecency(t *testing.T) {
	db := testDB(t, WithIndexType("flat"))

	_, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID:   "mem-low",
		Content:    "Shared retrieval phrase.",
		Scope:      types.MemoryScopeUser,
		UserID:     "user-1",
		Namespace:  "prefs",
		Importance: 0.1,
	})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(10 * time.Millisecond)
	_, err = db.SaveMemory(types.MemorySaveRequest{
		MemoryID:   "mem-high",
		Content:    "Shared retrieval phrase.",
		Scope:      types.MemoryScopeUser,
		UserID:     "user-1",
		Namespace:  "prefs",
		Importance: 1.0,
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "shared retrieval phrase",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      2,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 2 {
		t.Fatalf("expected two results, got %+v", resp.Results)
	}
	if resp.Results[0].Memory.ID != "mem-high" {
		t.Fatalf("expected important recent memory first, got %+v", resp.Results)
	}
	if resp.Results[0].ImportanceScore <= resp.Results[1].ImportanceScore {
		t.Fatalf("expected higher importance explanation on first result, got %+v", resp.Results)
	}
	if resp.Results[0].RecencyScore <= resp.Results[1].RecencyScore {
		t.Fatalf("expected higher recency explanation on first result, got %+v", resp.Results)
	}
}

func TestDB_SearchMemory_SemanticSearchRespectsScope(t *testing.T) {
	db := testDB(t, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))

	_, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-user-1-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-user-2-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-2",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Memory.ID != "mem-user-1-cat" {
		t.Fatalf("expected only user-1 memory, got %+v", resp.Results)
	}
}

func TestDB_UpdateMemory_ReindexesSemanticVector(t *testing.T) {
	db := testDB(t, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))

	_, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-topic",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}

	updated := "User prefers green tea."
	if _, err := db.UpdateMemory(types.MemoryUpdateRequest{
		MemoryID: "mem-topic",
		Content:  &updated,
	}); err != nil {
		t.Fatal(err)
	}

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "tea question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Memory.ID != "mem-topic" {
		t.Fatalf("expected updated semantic result, got %+v", resp.Results)
	}

	resp, err = db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected old semantic vector to be removed, got %+v", resp.Results)
	}
}

func TestDB_DeleteMemory_RemovesSemanticVector(t *testing.T) {
	db := testDB(t, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))

	_, err := db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteMemory("mem-cat"); err != nil {
		t.Fatal(err)
	}

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected deleted memory to be absent, got %+v", resp.Results)
	}
}

func TestDB_SearchMemory_SemanticSearchSurvivesReopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "gracedb-memory-reopen-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := Open(dir, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dir, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Memory.ID != "mem-cat" {
		t.Fatalf("expected reopened semantic result mem-cat, got %+v", resp.Results)
	}
}

func TestDB_UpdateMemory_SemanticIndexSurvivesReopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "gracedb-memory-update-reopen-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := Open(dir, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-topic",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	updated := "User prefers green tea."
	if _, err := db.UpdateMemory(types.MemoryUpdateRequest{
		MemoryID: "mem-topic",
		Content:  &updated,
	}); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dir, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected old semantic vector to stay absent after reopen, got %+v", resp.Results)
	}

	resp, err = db.SearchMemory(types.MemorySearchRequest{
		Query:     "tea question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Memory.ID != "mem-topic" {
		t.Fatalf("expected updated semantic vector after reopen, got %+v", resp.Results)
	}
}

func TestDB_DeleteMemory_SemanticIndexSurvivesReopen(t *testing.T) {
	dir, err := os.MkdirTemp("", "gracedb-memory-delete-reopen-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	db, err := Open(dir, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.SaveMemory(types.MemorySaveRequest{
		MemoryID:  "mem-cat",
		Content:   "User loves feline companions.",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.DeleteMemory("mem-cat"); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	db, err = Open(dir, WithEmbedder(topicEmbedder{}), WithIndexType("flat"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	resp, err := db.SearchMemory(types.MemorySearchRequest{
		Query:     "cat question",
		Scope:     types.MemoryScopeUser,
		UserID:    "user-1",
		Namespace: "prefs",
		TopK:      5,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 0 {
		t.Fatalf("expected deleted semantic memory to stay absent after reopen, got %+v", resp.Results)
	}
}

var _ types.Embedder = topicEmbedder{}
