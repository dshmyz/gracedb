package gracedb

import (
	"context"
	"time"

	"github.com/dshmyz/gracedb/pkg/store"
	"github.com/dshmyz/gracedb/pkg/types"
)

// Upsert inserts or updates a single embedding.
func (db *DB) Upsert(collectionName string, docID string, vector []float32, content string, metadata map[string]string, acl []string) (string, error) {
	ctx := spanWithCollection(context.Background(), "Upsert", collectionName)
	defer func() {
		endSpan(ctx, nil)
		recordUpsert(ctx, 1)
	}()

	embID, err := db.store_.Upsert(collectionName, docID, vector, content, metadata, acl)
	if err != nil {
		return "", err
	}
	// Index content for FTS if provided.
	if content != "" {
		coll, _ := db.store_.GetCollection(collectionName)
		if coll != nil {
			_ = db.store_.IndexFTS(coll.ID, embID, content)
		}
	}
	return embID, nil
}

// UpsertBatch inserts or updates multiple embeddings.
func (db *DB) UpsertBatch(collectionName string, vectors [][]float32, contents []string, docIDs []string, metadata []map[string]string) error {
	ctx := spanWithCollection(context.Background(), "UpsertBatch", collectionName)
	defer func() {
		endSpan(ctx, nil)
		recordUpsert(ctx, len(vectors))
	}()

	embIDs, err := db.store_.UpsertBatch(collectionName, vectors, contents, docIDs, metadata)
	if err != nil {
		return err
	}
	// Index content for FTS using the returned embedding IDs.
	coll, _ := db.store_.GetCollection(collectionName)
	if coll != nil {
		for i, content := range contents {
			if content != "" && i < len(embIDs) {
				_ = db.store_.IndexFTS(coll.ID, embIDs[i], content)
			}
		}
	}
	return nil
}

// GetEmbedding retrieves an embedding by ID.
func (db *DB) GetEmbedding(collectionName, embID string, includeVector bool) (*types.Embedding, error) {
	ctx := spanWithCollection(context.Background(), "GetEmbedding", collectionName)
	defer endSpan(ctx, nil)

	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}
	return db.store_.GetEmbedding(coll.ID, embID, includeVector)
}

// DeleteEmbedding deletes a single embedding.
func (db *DB) DeleteEmbedding(collectionName, embID string) error {
	ctx := spanWithCollection(context.Background(), "DeleteEmbedding", collectionName)
	defer endSpan(ctx, nil)

	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return err
	}
	return db.store_.DeleteEmbedding(coll.ID, embID)
}

// DeleteByDocID deletes all embeddings for a document.
func (db *DB) DeleteByDocID(collectionName, docID string) error {
	ctx := spanWithCollection(context.Background(), "DeleteByDocID", collectionName)
	defer endSpan(ctx, nil)

	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return err
	}
	return db.store_.DeleteByDocID(coll.ID, docID)
}

// DeleteEmbeddingBatch deletes multiple embeddings.
func (db *DB) DeleteEmbeddingBatch(collectionName string, ids []string) error {
	ctx := spanWithCollection(context.Background(), "DeleteBatch", collectionName)
	defer endSpan(ctx, nil)

	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return err
	}
	return db.store_.DeleteBatch(coll.ID, ids)
}

// Search performs similarity and/or full-text search.
func (db *DB) Search(collectionName string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error) {
	start := time.Now()
	ctx := spanWithCollection(context.Background(), "Search", collectionName)
	defer endSpan(ctx, nil)

	results, err := db.store_.Search(collectionName, query, opts)
	if err != nil {
		return nil, err
	}
	recordSearchDuration(ctx, start)
	return results, nil
}

// SearchFTS performs full-text search only.
func (db *DB) SearchFTS(collectionName string, query string) ([]string, error) {
	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}
	return db.store_.SearchFTS(coll.ID, query)
}

// SearchFTSWithContent performs FTS and returns scored embeddings.
func (db *DB) SearchFTSWithContent(collectionName string, query string, topK int) ([]types.ScoredEmbedding, error) {
	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}
	return db.store_.SearchFTSWithContent(coll.ID, query, topK)
}

// EmbeddingCount returns the number of embeddings in a collection.
func (db *DB) EmbeddingCount(collectionName string) (int, error) {
	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return 0, err
	}
	return db.store_.EmbeddingCount(coll.ID)
}

// ListEmbeddingIDs returns all embedding IDs for a collection.
func (db *DB) ListEmbeddingIDs(collectionName string) ([]string, error) {
	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}
	return db.store_.ListEmbeddingIDs(coll.ID)
}

// LoadIndex prepares the collection for vector search.
func (db *DB) LoadIndex(collectionName string) error {
	return db.store_.LoadIndex(collectionName)
}

// SaveIndex persists index snapshots.
func (db *DB) SaveIndex(collectionName string) error {
	return db.store_.SaveIndex(collectionName)
}

// RebuildIndex rebuilds the search index for all embeddings in a collection.
func (db *DB) RebuildIndex(collectionName string) error {
	coll, err := db.store_.GetCollection(collectionName)
	if err != nil {
		return err
	}

	ids, err := db.store_.ListEmbeddingIDs(coll.ID)
	if err != nil {
		return err
	}

	for _, embID := range ids {
		// Clear existing FTS entries first to avoid duplicates.
		if err := db.store_.UnindexFTS(coll.ID, embID); err != nil {
			return err
		}
		emb, err := db.store_.GetEmbedding(coll.ID, embID, false)
		if err != nil {
			continue
		}
		if emb.Content != "" {
			if err := db.store_.IndexFTS(coll.ID, embID, emb.Content); err != nil {
				return err
			}
		}
	}
	return nil
}

// Compile-time check that BadgerStore implements Store.
var _ store.Store = (*store.BadgerStore)(nil)
