package types

import "time"

// KnowledgeRecord is the high-level durable knowledge object.
type KnowledgeRecord struct {
	ID         string            `json:"id"`
	Title      string            `json:"title,omitempty"`
	Content    string            `json:"content,omitempty"`
	SourceURL  string            `json:"source_url,omitempty"`
	Author     string            `json:"author,omitempty"`
	Collection string            `json:"collection,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	ChunkIDs   []string          `json:"chunk_ids,omitempty"`
	Entities   []string          `json:"entities,omitempty"`
	Version    int               `json:"version"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
	UpdatedAt  time.Time         `json:"updated_at,omitempty"`
}

// KnowledgeSaveRequest stores or replaces a durable knowledge item.
type KnowledgeSaveRequest struct {
	KnowledgeID  string
	Title        string
	Content      string
	SourceURL    string
	Author       string
	Collection   string
	ChunkSize    int
	ChunkOverlap int
	Metadata     map[string]string
	Entities     []string
}

// KnowledgeUpdateRequest updates fields of an existing knowledge item.
type KnowledgeUpdateRequest struct {
	KnowledgeID  string
	Title        *string
	Content      *string
	SourceURL    *string
	Author       *string
	Metadata     map[string]string
	ChunkSize    *int
	ChunkOverlap *int
}

// KnowledgeSearchRequest searches durable knowledge.
type KnowledgeSearchRequest struct {
	Query      string
	Collection string
	TopK       int
}

// KnowledgeSearchHit is a document-shaped search result.
type KnowledgeSearchHit struct {
	KnowledgeID string            `json:"knowledge_id"`
	Title       string            `json:"title,omitempty"`
	SourceURL   string            `json:"source_url,omitempty"`
	Author      string            `json:"author,omitempty"`
	Snippet     string            `json:"snippet,omitempty"`
	Score       float64           `json:"score"`
	ChunkIDs    []string          `json:"chunk_ids,omitempty"`
	Entities    []string          `json:"entities,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// KnowledgeSearchResponse contains grouped knowledge hits.
type KnowledgeSearchResponse struct {
	Query   string               `json:"query"`
	Results []KnowledgeSearchHit `json:"results"`
}

// Chunk represents a text chunk derived from a knowledge item.
type Chunk struct {
	ID      string
	Content string
	Start   int
	End     int
}
