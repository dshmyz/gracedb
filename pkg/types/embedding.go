package types

import (
	"context"
	"time"
)

// Embedding represents a vector embedding with optional metadata.
type Embedding struct {
	ID           string            `json:"id"`
	CollectionID string            `json:"collection_id"`
	Collection   string            `json:"collection"`
	Vector       []float32         `json:"vector"`
	Content      string            `json:"content,omitempty"`
	DocID        string            `json:"doc_id,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	ACL          []string          `json:"acl,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
}

// ScoredEmbedding is an embedding with a similarity score.
type ScoredEmbedding struct {
	Embedding
	Score float32 `json:"score"`
}

// SearchOptions controls how similarity and full-text searches behave.
type SearchOptions struct {
	TopK             int
	Threshold        float32
	Filter           map[string]string
	Collection       string
	UseVectorSearch  bool
	UseTextSearch    bool
	Metadata         []string
	// MetadataFilter requires exact key=value matches on embedding metadata.
	MetadataFilter map[string]string
	// ACL filters results to embeddings where at least one ACL entry matches.
	ACL []string
	// MetadataExists requires the given metadata keys to be present.
	MetadataExists []string
	// DocID filters results to a specific document ID.
	DocID string
	// QueryText is the original text query, used by Reranker for cross-encoding.
	QueryText string
	// Reranker applies secondary re-ranking after initial retrieval. Nil = skip.
	Reranker Reranker
	// Context carries a deadline/cancellation for search. If nil,
	// context.Background() is used. Pass a context with timeout to prevent
	// vector searches from blocking indefinitely.
	Context context.Context
}

// Collection represents a namespace for embeddings.
type Collection struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	CreatedAt      time.Time         `json:"created_at"`
	DocumentCount  int               `json:"document_count"`
	EmbeddingCount int               `json:"embedding_count"`
}

// Document is a generic document store entry.
type Document struct {
	ID        string            `json:"id"`
	Source    string            `json:"source,omitempty"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Session represents a conversational session.
type Session struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Message represents a single message in a session.
type Message struct {
	ID        string            `json:"id"`
	SessionID string            `json:"session_id"`
	Role      string            `json:"role"`
	Content   string            `json:"content"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// Config holds database configuration.
type Config struct {
	Path             string
	VectorDim        int
	SimilarityFn     string
	IndexType        string   // "hnsw" (default) / "ivf" / "flat" / "lsh"
	IndexTypes       []string // multi-index: use >1 types for hybrid search
	HNSWConfig       *HNSWConfig
	Logger           func(msg string, args ...any)
	AutoSave         bool
	ValueLogFileSize int64
	Embedder         Embedder
	// TTLInterval controls how often the background memory cleanup runs.
	// Zero means no automatic cleanup.
	TTLInterval time.Duration
	// SlowQueryThreshold logs a warning when a search takes longer than this.
	// Zero disables slow query detection.
	SlowQueryThreshold time.Duration
}

// HNSWConfig holds HNSW index parameters.
type HNSWConfig struct {
	M              int
	EfConstruction int
	EfSearch       int
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Path:         "",
		VectorDim:    0,
		SimilarityFn: "cosine",
		IndexType:    "hnsw",
		HNSWConfig: &HNSWConfig{
			M:              16,
			EfConstruction: 64,
			EfSearch:       50,
		},
		AutoSave:             true,
		TTLInterval:          5 * time.Minute,
		SlowQueryThreshold:   time.Second,
	}
}

// Option is a functional option for Config.
type Option func(*Config)

// StoreStats holds database statistics.
type StoreStats struct {
	CollectionCount int
	EmbeddingCount  int
	SessionCount    int
	MessageCount    int
	DocumentCount   int
	IndexSize       int
}
