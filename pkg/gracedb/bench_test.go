package gracedb

import (
	"context"
	"os"
	"testing"

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
