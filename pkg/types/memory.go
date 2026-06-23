package types

import (
	"context"
	"time"
)

const (
	MemoryScopeGlobal  = "global"
	MemoryScopeUser    = "user"
	MemoryScopeSession = "session"
)

// MemoryRecord is a high-level memory object stored in a memory bucket.
type MemoryRecord struct {
	ID         string         `json:"id"`
	UserID     string         `json:"user_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	Namespace  string         `json:"namespace,omitempty"`
	Role       string         `json:"role,omitempty"`
	Content    string         `json:"content"`
	Vector     []float32      `json:"vector,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	Importance float64        `json:"importance,omitempty"`
	TTLSeconds int            `json:"ttl_seconds,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
	UpdatedAt  time.Time      `json:"updated_at,omitempty"`
}

// MemorySaveRequest stores a memory in a memory bucket.
type MemorySaveRequest struct {
	MemoryID   string
	UserID     string
	SessionID  string
	Scope      string
	Namespace  string
	Role       string
	Content    string
	Vector     []float32
	Metadata   map[string]any
	Importance float64
	TTLSeconds int
}

// MemoryUpdateRequest updates a memory item.
type MemoryUpdateRequest struct {
	MemoryID   string
	Content    *string
	Vector     []float32
	Metadata   map[string]any
	Importance *float64
	TTLSeconds *int
}

// MemorySearchRequest searches memories in a resolved bucket.
type MemorySearchRequest struct {
	Query       string
	UserID      string
	SessionID   string
	Scope       string
	Namespace   string
	TopK        int
	QueryVector []float32
	// Ranking weights. When all four are zero, gracedb uses the default
	// semantic/lexical/importance/recency weights.
	SemanticWeight   float64
	LexicalWeight    float64
	ImportanceWeight float64
	RecencyWeight    float64
	// RecencyHalfLife controls how quickly recency score decays. Zero uses the
	// store default.
	RecencyHalfLife time.Duration
	// Context carries a deadline/cancellation for memory search. If nil,
	// context.Background() is used.
	Context context.Context
}

// MemorySearchHit is one scored memory result.
type MemorySearchHit struct {
	Memory          MemoryRecord `json:"memory"`
	Score           float64      `json:"score"`
	FinalScore      float64      `json:"final_score"`
	SemanticScore   float64      `json:"semantic_score,omitempty"`
	LexicalScore    float64      `json:"lexical_score,omitempty"`
	ImportanceScore float64      `json:"importance_score,omitempty"`
	RecencyScore    float64      `json:"recency_score,omitempty"`
}

// MemorySearchResponse contains retrieved memories.
type MemorySearchResponse struct {
	Query   string            `json:"query"`
	Results []MemorySearchHit `json:"results"`
}
