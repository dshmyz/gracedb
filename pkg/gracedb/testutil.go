package gracedb

import (
	"context"
	"os"
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

// mockEmbedder is a test embedder that returns deterministic vectors.
type mockEmbedder struct {
	dim int
}

func newMockEmbedder(dim int) *mockEmbedder {
	return &mockEmbedder{dim: dim}
}

func (m *mockEmbedder) Embed(_ context.Context, text string) ([]float32, error) {
	vec := make([]float32, m.dim)
	for i := 0; i < m.dim && i < len(text); i++ {
		vec[i] = float32(text[i]) / 255.0
	}
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	vecs := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(context.Background(), t)
		if err != nil {
			return nil, err
		}
		vecs[i] = v
	}
	return vecs, nil
}

func (m *mockEmbedder) Dimension() int {
	return m.dim
}

// testDB creates a temporary in-memory or on-disk DB for testing.
// The DB is automatically closed via t.Cleanup.
func testDB(t *testing.T, opts ...Option) *DB {
	t.Helper()
	dir, err := os.MkdirTemp("", "gracedb-test-*")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}

	var dbOpts []Option
	dbOpts = append(dbOpts, WithPath(dir))
	dbOpts = append(dbOpts, opts...)

	db, err := Open(dir, dbOpts...)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatalf("open db: %v", err)
	}

	t.Cleanup(func() {
		db.Close()
		os.RemoveAll(dir)
	})
	return db
}

// testDBWithEmbedder returns a DB configured with a mock embedder.
func testDBWithEmbedder(t *testing.T, dim int) *DB {
	t.Helper()
	return testDB(t, WithEmbedder(newMockEmbedder(dim)))
}

// seedCollection inserts N test embeddings into a named collection.
// Content is "doc-0", "doc-1", etc. with a 16-dim deterministic vector.
func seedCollection(t *testing.T, db *DB, name string, n int) []string {
	t.Helper()

	_, err := db.CreateCollection(name)
	if err != nil {
		t.Fatalf("create collection %s: %v", name, err)
	}

	ids := make([]string, 0, n)
	vectors := make([][]float32, 0, n)
	contents := make([]string, 0, n)
	docIDs := make([]string, 0, n)

	for i := 0; i < n; i++ {
		vec := make([]float32, 16)
		for j := 0; j < 16; j++ {
			vec[j] = float32(i*16+j) / 100.0
		}
		vectors = append(vectors, vec)
		contents = append(contents, "doc-"+string(rune('0'+i)))
		docIDs = append(docIDs, "doc-"+string(rune('0'+i)))
	}

	err = db.UpsertBatch(name, vectors, contents, docIDs, nil)
	if err != nil {
		t.Fatalf("upsert batch: %v", err)
	}

	for _, id := range docIDs {
		ids = append(ids, id)
	}
	return ids
}

var _ types.Embedder = (*mockEmbedder)(nil)
