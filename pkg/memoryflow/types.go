// Package memoryflow provides agent memory workflow: transcript ingest, recall,
// wake-up layers, diary, and knowledge promotion.
package memoryflow

import (
	"context"
	"time"

	"github.com/dshmyz/gracedb/pkg/types"
)

// PromotionKind classifies durable knowledge candidates.
type PromotionKind string

const (
	PromotionKindDecision   PromotionKind = "decision"
	PromotionKindPreference PromotionKind = "preference"
	PromotionKindMilestone  PromotionKind = "milestone"
	PromotionKindProblem    PromotionKind = "problem"
	PromotionKindNote       PromotionKind = "note"
)

// WakeUpLevel identifies context density tiers.
type WakeUpLevel string

const (
	WakeUpLevelL0 WakeUpLevel = "L0"
	WakeUpLevelL1 WakeUpLevel = "L1"
	WakeUpLevelL2 WakeUpLevel = "L2"
	WakeUpLevelL3 WakeUpLevel = "L3"
)

// TranscriptTurn is one conversation turn.
type TranscriptTurn struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp,omitempty"`
	SourceID  string    `json:"source_id,omitempty"`
}

// Transcript is the input to the ingest path.
type Transcript struct {
	SessionID string           `json:"session_id"`
	UserID    string           `json:"user_id"`
	Source    string           `json:"source"`
	Title     string           `json:"title"`
	Turns     []TranscriptTurn `json:"turns"`
}

// SessionState provides planner context.
type SessionState struct {
	UserID    string         `json:"user_id"`
	SessionID string         `json:"session_id"`
	Namespace string         `json:"namespace"`
	Tags      []string       `json:"tags"`
	Metadata  map[string]any `json:"metadata"`
}

// PromotionCandidate is proposed durable knowledge.
type PromotionCandidate struct {
	KnowledgeID string              `json:"knowledge_id"`
	Kind        PromotionKind       `json:"kind"`
	Title       string              `json:"title"`
	Content     string              `json:"content"`
	Collection  string              `json:"collection"`
	Metadata    map[string]string   `json:"metadata"`
}

// QueryPlanner produces retrieval plans.
type QueryPlanner interface {
	Plan(ctx context.Context, query string, state SessionState) (*RetrievalPlan, error)
}

// SessionExtractor proposes knowledge from a transcript.
type SessionExtractor interface {
	Extract(ctx context.Context, transcript Transcript, state SessionState) ([]PromotionCandidate, error)
}

// PromotionPolicy filters promotion candidates.
type PromotionPolicy interface {
	Select(ctx context.Context, candidates []PromotionCandidate) ([]PromotionCandidate, error)
}

// IngestTranscriptRequest stores a transcript as episodic memory.
type IngestTranscriptRequest struct {
	Transcript Transcript
	Scope      string
	Namespace  string
	Tags       []string
	Metadata   map[string]any
}

// IngestTranscriptResponse summarizes stored memories.
type IngestTranscriptResponse struct {
	MemoryIDs []string `json:"memory_ids"`
	Count     int      `json:"count"`
}

// Episode is one stored exchange record.
type Episode struct {
	Memory  types.MemoryRecord `json:"memory"`
	Index   int                `json:"exchange_index"`
	Turns   []TranscriptTurn   `json:"turns"`
}

// RecallRequest returns fused memory/knowledge pack.
type RecallRequest struct {
	Query            string
	UserID           string
	SessionID        string
	Scope            string
	Namespace        string
	Collection       string
	TopKMemories     int
	TopKKnowledge    int
	RetrievalMode    string
	DisableMemory    bool
	DisableKnowledge bool
	MaxContextChars  int
	MaxContextChunks int
	Plan             *RetrievalPlan
	State            SessionState
}

// RecallResponse contains fused results.
type RecallResponse struct {
	Plan     RetrievalPlan                    `json:"plan"`
	Decision RetrievalDecision                `json:"decision"`
	Memories []types.MemorySearchHit          `json:"memories"`
	Knowledge []types.KnowledgeSearchHit      `json:"knowledge"`
	Context  string                           `json:"context"`
}

// WakeUpLayer is one wake-up context tier.
type WakeUpLayer struct {
	Level WakeUpLevel `json:"level"`
	Title string      `json:"title"`
	Text  string      `json:"text"`
}

// WakeUpLayersRequest requests all context tiers.
type WakeUpLayersRequest struct {
	Identity string
	Recall   RecallRequest
}

// WakeUpLayersResponse returns all tiers.
type WakeUpLayersResponse struct {
	Layers []WakeUpLayer   `json:"layers"`
	Recall RecallResponse  `json:"recall"`
}

// CloseSessionRequest promotes knowledge at session end.
type CloseSessionRequest struct {
	Transcript Transcript
	Scope      string
	Namespace  string
	Promote    bool
	Collection string
	Author     string
}

// CloseSessionResponse reports promotions.
type CloseSessionResponse struct {
	Promotions []types.KnowledgeRecord `json:"promotions"`
	Count      int                     `json:"count"`
}

// DiaryEntryRequest appends one diary entry.
type DiaryEntryRequest struct {
	EntryID    string
	UserID     string
	SessionID  string
	Scope      string
	Namespace  string
	Content    string
	Metadata   map[string]any
	Importance float64
}

// DiaryListRequest lists diary entries.
type DiaryListRequest struct {
	UserID    string
	SessionID string
	Scope     string
	Namespace string
	Limit     int
}

// RetrievalPlan is the structured search plan.
type RetrievalPlan struct {
	Query         string            `json:"query"`
	Keywords      []string          `json:"keywords"`
	EntityNames   []string          `json:"entity_names"`
	RetrievalMode string            `json:"retrieval_mode"`
	Filters       *RetrievalFilters `json:"filters"`
}

// RetrievalFilters captures structured constraints.
type RetrievalFilters struct {
	Collection  string
	UserID      string
	SessionID   string
	Scope       string
	Namespace   string
}

// RetrievalDecision explains how retrieval was resolved.
type RetrievalDecision struct {
	RequestedMode string `json:"requested_mode"`
	EffectiveMode string `json:"effective_mode"`
	Reason        string `json:"reason"`
}
