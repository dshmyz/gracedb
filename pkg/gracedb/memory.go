package gracedb

import (
	"github.com/dshmyz/gracedb/pkg/types"
)

// SaveMemory stores a memory record with scope/namespace bucketing.
func (db *DB) SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error) {
	return db.store_.SaveMemory(req)
}

// GetMemory fetches a memory record by ID.
func (db *DB) GetMemory(memoryID string) (*types.MemoryRecord, error) {
	return db.store_.GetMemory(memoryID)
}

// UpdateMemory updates a memory record.
func (db *DB) UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error) {
	return db.store_.UpdateMemory(req)
}

// DeleteMemory removes a memory record by ID.
func (db *DB) DeleteMemory(memoryID string) error {
	return db.store_.DeleteMemory(memoryID)
}

// SearchMemory searches memories in a resolved bucket.
func (db *DB) SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error) {
	return db.store_.SearchMemory(req)
}
