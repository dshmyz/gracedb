package gracedb

import (
	"context"
	"fmt"

	"github.com/dshmyz/gracedb/pkg/types"
)

// SaveMemory stores a memory record with scope/namespace bucketing.
func (db *DB) SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error) {
	if db.embedder != nil {
		vec, err := db.embedder.Embed(context.Background(), req.Content)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", types.ErrEmbedderNotConfigured, err)
		}
		req.Vector = vec
	}
	return db.store_.SaveMemory(req)
}

// GetMemory fetches a memory record by ID.
func (db *DB) GetMemory(memoryID string) (*types.MemoryRecord, error) {
	return db.store_.GetMemory(memoryID)
}

// UpdateMemory updates a memory record.
func (db *DB) UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error) {
	if db.embedder != nil && req.Content != nil {
		vec, err := db.embedder.Embed(context.Background(), *req.Content)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", types.ErrEmbedderNotConfigured, err)
		}
		req.Vector = vec
	}
	return db.store_.UpdateMemory(req)
}

// DeleteMemory removes a memory record by ID.
func (db *DB) DeleteMemory(memoryID string) error {
	return db.store_.DeleteMemory(memoryID)
}

// SearchMemory searches memories in a resolved bucket.
func (db *DB) SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error) {
	if db.embedder != nil && req.Query != "" {
		vec, err := db.embedder.Embed(context.Background(), req.Query)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", types.ErrEmbedderNotConfigured, err)
		}
		req.QueryVector = vec
	}
	return db.store_.SearchMemory(req)
}
