package gracedb

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/dshmyz/gracedb/pkg/types"
)

// Quick provides a simplified interface for common operations.
type Quick struct {
	db *DB
}

// Quick returns a Quick helper for simple operations.
func (db *DB) Quick() *Quick {
	return &Quick{db: db}
}

// Add adds a vector with automatic ID generation.
func (q *Quick) Add(ctx context.Context, vector []float32, content string) (string, error) {
	return q.AddToCollection(ctx, "", vector, content)
}

// AddToCollection adds a vector to a specific collection.
func (q *Quick) AddToCollection(ctx context.Context, collection string, vector []float32, content string) (string, error) {
	id := uuid.New().String()
	return q.db.Upsert(collection, id, vector, content, nil, nil)
}

// Search performs similarity search.
func (q *Quick) Search(ctx context.Context, query []float32, topK int) ([]types.ScoredEmbedding, error) {
	return q.SearchInCollection(ctx, "", query, topK)
}

// SearchInCollection performs similarity search within a collection.
func (q *Quick) SearchInCollection(ctx context.Context, collection string, query []float32, topK int) ([]types.ScoredEmbedding, error) {
	return q.db.Search(collection, query, types.SearchOptions{
		TopK:            topK,
		UseVectorSearch: true,
	})
}

// AddText adds text with automatic embedding.
func (q *Quick) AddText(ctx context.Context, text string, metadata map[string]string) (string, error) {
	return q.AddTextToCollection(ctx, "", text, metadata)
}

// AddTextToCollection adds text to a specific collection with automatic embedding.
func (q *Quick) AddTextToCollection(ctx context.Context, collection string, text string, metadata map[string]string) (string, error) {
	if q.db.embedder == nil {
		return "", types.ErrEmbedderNotConfigured
	}

	vec, err := q.db.embedder.Embed(ctx, text)
	if err != nil {
		return "", fmt.Errorf("embed failed: %v", err)
	}

	id := uuid.New().String()
	return q.db.Upsert(collection, id, vec, text, metadata, nil)
}

// SearchText performs similarity search using a text query.
func (q *Quick) SearchText(ctx context.Context, query string, topK int) ([]types.ScoredEmbedding, error) {
	return q.SearchTextInCollection(ctx, "", query, topK)
}

// SearchTextInCollection performs text search within a collection.
func (q *Quick) SearchTextInCollection(ctx context.Context, collection string, query string, topK int) ([]types.ScoredEmbedding, error) {
	return q.db.SearchText(collection, query, topK)
}

// SearchTextOnly performs pure FTS search without embeddings.
func (q *Quick) SearchTextOnly(ctx context.Context, query string, topK int) ([]types.ScoredEmbedding, error) {
	return q.db.SearchFTSWithContent("", query, topK)
}

// SearchTextOnlyInCollection performs FTS search within a collection.
func (q *Quick) SearchTextOnlyInCollection(ctx context.Context, collection string, query string, topK int) ([]types.ScoredEmbedding, error) {
	return q.db.SearchFTSWithContent(collection, query, topK)
}
