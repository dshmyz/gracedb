package gracedb

import (
	"context"
	"os"
	"testing"

	"github.com/dshmyz/gracedb/pkg/knowledge"
	"github.com/dshmyz/gracedb/pkg/store"
	"github.com/dshmyz/gracedb/pkg/types"
)

func benchDB(b *testing.B) *DB {
	dir, err := os.MkdirTemp("", "gracedb-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })

	db, err := Open(dir)
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { db.Close() })
	return db
}

func BenchmarkDBUpsert(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.Upsert("bench", "doc", vec, "content", nil, nil)
	}
}

func BenchmarkDBSearch(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < 1000; i++ {
		_, _ = db.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	query := make([]float32, 128)
	for i := range query {
		query[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.Search("bench", query, types.SearchOptions{TopK: 10, UseVectorSearch: true})
	}
}

func BenchmarkToolboxCall(b *testing.B) {
	db := benchDB(b)
	tbx := db.GraphRAGTools()
	_, _ = db.CreateCollection("default")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tbx.Call(context.Background(), "search_knowledge", map[string]any{
			"query": "test",
			"top_k": 5,
		})
	}
}

func BenchmarkQuickSearch(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")
	q := db.Quick()

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < 1000; i++ {
		_, _ = q.AddToCollection(context.Background(), "bench", vec, "content")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = q.SearchInCollection(context.Background(), "bench", vec, 10)
	}
}

func BenchmarkDBBackup(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < 100; i++ {
		_, _ = db.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.Backup(b.TempDir() + "/backup.db")
	}
}

func BenchmarkDBUpsertBatch(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vectors := make([][]float32, 100)
	contents := make([]string, 100)
	for i := range vectors {
		vectors[i] = make([]float32, 128)
		for j := range vectors[i] {
			vectors[i][j] = float32(i*100+j) * 0.001
		}
		contents[i] = "content"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.UpsertBatch("bench", vectors, contents, nil, nil)
	}
}

func BenchmarkDBUpsertBatchLarge(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vectors := make([][]float32, 1000)
	contents := make([]string, 1000)
	for i := range vectors {
		vectors[i] = make([]float32, 128)
		for j := range vectors[i] {
			vectors[i][j] = float32(i*1000+j) * 0.001
		}
		contents[i] = "content"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = db.UpsertBatch("bench", vectors, contents, nil, nil)
	}
}

func BenchmarkDBSearchFTS(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < 1000; i++ {
		_, _ = db.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.SearchFTSWithContent("bench", "content", 10)
	}
}

func BenchmarkDBSearchHybrid(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < 1000; i++ {
		_, _ = db.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	query := make([]float32, 128)
	for i := range query {
		query[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.Search("bench", query, types.SearchOptions{
			TopK:            10,
			UseVectorSearch: true,
			UseTextSearch:   true,
			Collection:      "bench",
		})
	}
}

func BenchmarkDBKnowledgeSave(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	content := "This is a knowledge article about Go programming language. " +
		"Go is a statically typed, compiled programming language designed at Google. " +
		"It is syntactically similar to C, but with memory safety, garbage collection, " +
		"structural typing, and CSP-style concurrency."

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.SaveKnowledge("bench", "wiki", "Go Language", content,
			types.KnowledgeSaveRequest{ChunkSize: 100, ChunkOverlap: 20})
	}
}

func BenchmarkDBMemory(b *testing.B) {
	db := benchDB(b)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.SaveMemory(types.MemorySaveRequest{
			MemoryID:  "mem",
			Content:   "User likes Go",
			Scope:     "user",
			UserID:    "user-123",
			Namespace: "preferences",
		})
	}
}

func BenchmarkDBSearchMemory(b *testing.B) {
	db := benchDB(b)

	for i := 0; i < 100; i++ {
		_, _ = db.SaveMemory(types.MemorySaveRequest{
			MemoryID:  "mem",
			Content:   "User likes Go",
			Scope:     "user",
			UserID:    "user-123",
			Namespace: "preferences",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.SearchMemory(types.MemorySearchRequest{
			Query: "likes",
			Scope: "user",
			TopK:  10,
		})
	}
}

func BenchmarkDBAggregate(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := 0; i < 1000; i++ {
		meta := map[string]string{
			"category": "cat",
			"score":    "0.5",
		}
		_, _ = db.Upsert("bench", "doc", vec, "content", meta, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.Aggregate("bench", "score", store.AggAvg)
	}
}

func BenchmarkDBGroupAggregate(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := 0; i < 1000; i++ {
		meta := map[string]string{
			"category": "cat",
			"score":    "0.5",
		}
		_, _ = db.Upsert("bench", "doc", vec, "content", meta, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = db.GroupAggregate("bench", "category", "score", store.AggAvg)
	}
}

func BenchmarkDBKnowledgeMemoryRecall(b *testing.B) {
	db := benchDB(b)
	_, _ = db.CreateCollection("default")

	for i := 0; i < 10; i++ {
		_, _ = db.SaveMemory(types.MemorySaveRequest{
			MemoryID:  "mem",
			Content:   "User likes Go",
			Scope:     "user",
			UserID:    "user-123",
			Namespace: "preferences",
		})
	}

	km := db.KnowledgeMemory(nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = km.Recall(context.Background(), knowledge.KnowledgeMemoryRecallRequest{
			Query:         "user preferences",
			TopKMemories:  5,
			TopKKnowledge: 4,
		})
	}
}
