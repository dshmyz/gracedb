package types

import "time"

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
	Metadata   map[string]any `json:"metadata,omitempty"`
	Importance float64        `json:"importance,omitempty"`
	TTLSeconds int            `json:"ttl_seconds,omitempty"`
	ExpiresAt  *time.Time     `json:"expires_at,omitempty"`
	CreatedAt  time.Time      `json:"created_at,omitempty"`
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
	Metadata   map[string]any
	Importance float64
	TTLSeconds int
}

// MemoryUpdateRequest updates a memory item.
type MemoryUpdateRequest struct {
	MemoryID   string
	Content    *string
	Metadata   map[string]any
	Importance *float64
	TTLSeconds *int
}

// MemorySearchRequest searches memories in a resolved bucket.
type MemorySearchRequest struct {
	Query            string
	UserID           string
	SessionID        string
	Scope            string
	Namespace        string
	TopK             int
}

// MemorySearchHit is one scored memory result.
type MemorySearchHit struct {
	Memory MemoryRecord `json:"memory"`
	Score  float64      `json:"score"`
}

// MemorySearchResponse contains retrieved memories.
type MemorySearchResponse struct {
	Query   string             `json:"query"`
	Results []MemorySearchHit  `json:"results"`
}
