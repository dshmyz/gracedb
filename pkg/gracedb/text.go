package gracedb

import (
	"context"
	"fmt"

	"github.com/dshmyz/gracedb/pkg/types"
)

// InsertText inserts text with automatic embedding generation.
func (db *DB) InsertText(collectionName, docID, content string, metadata map[string]string) (string, error) {
	if db.embedder == nil {
		return "", types.ErrEmbedderNotConfigured
	}
	if content == "" {
		return "", types.ErrEmptyText
	}

	vec, err := db.embedder.Embed(context.Background(), content)
	if err != nil {
		return "", fmt.Errorf("%w: %v", types.ErrEmbedderNotConfigured, err)
	}

	return db.Upsert(collectionName, docID, vec, content, metadata, nil)
}

// InsertTextBatch inserts multiple texts with automatic embedding generation.
func (db *DB) InsertTextBatch(collectionName string, contents []string, docIDs []string, metadata []map[string]string) ([]string, error) {
	if db.embedder == nil {
		return nil, types.ErrEmbedderNotConfigured
	}

	// Filter out empty texts.
	valid := make([]int, 0, len(contents))
	for i, c := range contents {
		if c != "" {
			valid = append(valid, i)
		}
	}
	if len(valid) == 0 {
		return nil, nil
	}

	texts := make([]string, len(valid))
	for i, idx := range valid {
		texts[i] = contents[idx]
	}

	vectors, err := db.embedder.EmbedBatch(context.Background(), texts)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrEmbedderNotConfigured, err)
	}

	// Only pass valid entries to the store.
	validVectors := make([][]float32, len(valid))
	validDocIDs := make([]string, len(valid))
	validMetadata := make([]map[string]string, len(valid))
	validContents := make([]string, len(valid))
	for i, idx := range valid {
		validVectors[i] = vectors[i]
		validDocIDs[i] = ""
		validContents[i] = contents[idx]
		if idx < len(docIDs) {
			validDocIDs[i] = docIDs[idx]
		}
		if idx < len(metadata) {
			validMetadata[i] = metadata[idx]
		}
	}

	embIDs, err := db.store_.UpsertBatch(collectionName, validVectors, validContents, validDocIDs, validMetadata)
	if err != nil {
		return nil, err
	}

	// Index content for FTS.
	coll, _ := db.store_.GetCollection(collectionName)
	if coll != nil {
		for i := range validContents {
			if validContents[i] != "" && i < len(embIDs) {
				_ = db.store_.IndexFTS(coll.ID, embIDs[i], validContents[i])
			}
		}
	}

	return embIDs, nil
}

// SearchText performs similarity search using a text query.
func (db *DB) SearchText(collectionName string, query string, topK int) ([]types.ScoredEmbedding, error) {
	if query == "" {
		return nil, types.ErrEmptyText
	}
	if db.embedder == nil {
		// Fallback to FTS if no embedder.
		return db.SearchFTSWithContent(collectionName, query, topK)
	}

	vec, err := db.embedder.Embed(context.Background(), query)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", types.ErrEmbedderNotConfigured, err)
	}

	return db.Search(collectionName, vec, types.SearchOptions{
		TopK:            topK,
		UseVectorSearch: true,
	})
}
