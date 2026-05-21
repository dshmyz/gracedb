package gracedb

import (
	"github.com/dshmyz/gracedb/pkg/types"
)

// SaveKnowledge stores or replaces a durable knowledge item.
func (db *DB) SaveKnowledge(collectionName, knowledgeID, title, content string, opts types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error) {
	opts.KnowledgeID = knowledgeID
	opts.Title = title
	opts.Content = content
	opts.Collection = collectionName
	return db.store_.SaveKnowledge(collectionName, knowledgeID, title, content, opts)
}

// GetKnowledge fetches a knowledge item by ID.
func (db *DB) GetKnowledge(collectionName, knowledgeID string) (*types.KnowledgeRecord, error) {
	return db.store_.GetKnowledge(collectionName, knowledgeID)
}

// UpdateKnowledge updates a knowledge item.
func (db *DB) UpdateKnowledge(collectionName, knowledgeID string, req types.KnowledgeUpdateRequest) (*types.KnowledgeRecord, error) {
	return db.store_.UpdateKnowledge(collectionName, knowledgeID, req)
}

// DeleteKnowledge removes a knowledge item and its chunks.
func (db *DB) DeleteKnowledge(collectionName, knowledgeID string) error {
	return db.store_.DeleteKnowledge(collectionName, knowledgeID)
}

// SearchKnowledge searches durable knowledge with FTS and aggregates by document.
func (db *DB) SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error) {
	return db.store_.SearchKnowledge(collectionName, query, topK)
}
