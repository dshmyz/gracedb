package store

import (
	"os"
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func BenchmarkUpsert(b *testing.B) {
	s := benchStore(b)
	defer s.Close()
	_, _ = s.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Upsert("bench", "doc", vec, "content", nil, nil)
	}
}

func BenchmarkUpsertBatch100(b *testing.B) {
	benchmarkUpsertBatch(b, 100)
}

func BenchmarkUpsertBatch1000(b *testing.B) {
	benchmarkUpsertBatch(b, 1000)
}

func benchmarkUpsertBatch(b *testing.B, batchSize int) {
	s := benchStore(b)
	defer s.Close()
	_, _ = s.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	vectors := make([][]float32, batchSize)
	contents := make([]string, batchSize)
	for i := 0; i < batchSize; i++ {
		vectors[i] = vec
		contents[i] = "content"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.UpsertBatch("bench", vectors, contents, nil, nil)
	}
}

func BenchmarkVectorSearchFlat(b *testing.B) {
	benchmarkVectorSearch(b, "flat")
}

func BenchmarkVectorSearch100(b *testing.B) {
	benchmarkVectorSearchWithData(b, 100)
}

func BenchmarkVectorSearch1000(b *testing.B) {
	benchmarkVectorSearchWithData(b, 1000)
}

func BenchmarkVectorSearch10000(b *testing.B) {
	benchmarkVectorSearchWithData(b, 10000)
}

func benchmarkVectorSearch(b *testing.B, indexType string) {
	s := benchStoreWithIndex(b, indexType)
	defer s.Close()
	_, _ = s.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	// Insert 1000 vectors.
	for i := 0; i < 1000; i++ {
		_, _ = s.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	query := make([]float32, 128)
	for i := range query {
		query[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Search("bench", query, types.SearchOptions{TopK: 10})
	}
}

func benchmarkVectorSearchWithData(b *testing.B, n int) {
	s := benchStore(b)
	defer s.Close()
	_, _ = s.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < n; i++ {
		_, _ = s.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	query := make([]float32, 128)
	for i := range query {
		query[i] = float32(i) * 0.01
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.Search("bench", query, types.SearchOptions{TopK: 10})
	}
}

func BenchmarkFTSSearch100(b *testing.B) {
	benchmarkFTSSearchWithCount(b, 100)
}

func BenchmarkFTSSearch1000(b *testing.B) {
	benchmarkFTSSearchWithCount(b, 1000)
}

func benchmarkFTSSearchWithCount(b *testing.B, n int) {
	s := benchStore(b)
	defer s.Close()
	coll, _ := s.CreateCollection("bench")

	for i := 0; i < n; i++ {
		embID, _ := s.Upsert("bench", "doc", []float32{0.1}, "the quick brown fox jumps over the lazy dog", nil, nil)
		_ = s.IndexFTS(coll.ID, embID, "the quick brown fox jumps over the lazy dog")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = s.SearchFTS(coll.ID, "quick brown fox")
	}
}

func BenchmarkLoadIndex(b *testing.B) {
	s := benchStore(b)
	defer s.Close()
	_, _ = s.CreateCollection("bench")

	vec := make([]float32, 128)
	for i := range vec {
		vec[i] = float32(i) * 0.01
	}

	for i := 0; i < 1000; i++ {
		_, _ = s.Upsert("bench", "doc", vec, "content", nil, nil)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = s.LoadIndex("bench")
	}
}

func benchStore(b *testing.B) *BadgerStore {
	return benchStoreWithIndex(b, "flat")
}

func benchStoreWithIndex(b *testing.B, indexType string) *BadgerStore {
	dir, err := os.MkdirTemp("", "gracedb-bench-*")
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { os.RemoveAll(dir) })

	cfg := types.DefaultConfig()
	cfg.Path = dir
	cfg.IndexType = indexType
	s, err := New(cfg)
	if err != nil {
		b.Fatal(err)
	}
	return s
}
