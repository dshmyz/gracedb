// Package knowledge provides Reflect and Consolidate types for gracedb.
package knowledge

import (
	"context"

	"github.com/dshmyz/gracedb/pkg/types"
)

// KnowledgeMemoryReflector synthesizes raw facts into structured reflections.
// Implementations may use LLMs or deterministic heuristics.
type KnowledgeMemoryReflector interface {
	Reflect(ctx context.Context, req KnowledgeMemoryReflectRequest, input KnowledgeMemoryReflectInput) (*KnowledgeMemoryReflection, error)
}

// ContextSection is one labeled section in a context pack.
type ContextSection struct {
	Kind      string   `json:"kind"`
	Title     string   `json:"title"`
	Text      string   `json:"text"`
	SourceIDs []string `json:"source_ids,omitempty"`
}

// ContextPack is an assembled prompt context with provenance tracking.
type ContextPack struct {
	Query        string           `json:"query"`
	Text         string           `json:"text"`
	Sections     []ContextSection `json:"sections,omitempty"`
	MemoryIDs    []string         `json:"memory_ids,omitempty"`
	KnowledgeIDs []string         `json:"knowledge_ids,omitempty"`
	ChunkIDs     []string         `json:"chunk_ids,omitempty"`
	Entities     []string         `json:"entities,omitempty"`
}

// KnowledgeMemoryReflectRequest controls a reflection step.
type KnowledgeMemoryReflectRequest struct {
	Query         string
	UserID        string
	SessionID     string
	Scope         string
	Namespace     string
	Collection    string
	TopKMemories  int
	TopKKnowledge int
	MaxFacts      int
	MaxThemes     int
	MaxSummaryChars int
	Instructions  string // hints for the reflector (e.g., "focus on user preferences")
}

// KnowledgeMemoryReflectInput is the raw material for reflection.
type KnowledgeMemoryReflectInput struct {
	Recall KnowledgeMemoryRecallResponse
}

// KnowledgeMemoryReflection is the structured output of a reflection.
type KnowledgeMemoryReflection struct {
	Summary            string   `json:"summary"`
	Themes             []string `json:"themes"`
	Entities           []string `json:"entities"`
	Facts              []string `json:"facts"`
	SourceMemoryIDs    []string `json:"source_memory_ids"`
	SourceKnowledgeIDs []string `json:"source_knowledge_ids"`
	SourceChunkIDs     []string `json:"source_chunk_ids"`
	ContextPack        ContextPack `json:"context_pack"`
}

// KnowledgeMemoryRecallRequest retrieves fused memory and knowledge.
type KnowledgeMemoryRecallRequest struct {
	Query            string
	UserID           string
	SessionID        string
	Scope            string
	Namespace        string
	Collection       string
	TopKMemories     int
	TopKKnowledge    int
	MaxMemoryItems   int
	MaxMemoryChars   int
	Keywords         []string
	AlternateQueries []string
	RetrievalMode    string
	DisableMemory    bool
	DisableKnowledge bool
	MaxContextChunks int
	MaxContextChars  int
	Plan             *RetrievalPlan
	EntityNames      []string
	// MaxHops controls graph expansion depth. 0 = disabled.
	MaxHops int
	// MaxTraversalNodes caps the number of graph nodes to traverse.
	MaxTraversalNodes int
}

// KnowledgeMemoryRecallResponse contains fused results with context pack.
type KnowledgeMemoryRecallResponse struct {
	Query            string                    `json:"query"`
	Memories         []types.MemorySearchHit   `json:"memories"`
	Knowledge        []types.KnowledgeSearchHit `json:"knowledge"`
	Entities         []string                  `json:"entities"`
	ContextPack      ContextPack               `json:"context_pack"`
	MemoryPlan       RetrievalPlan             `json:"memory_plan"`
	MemoryDecision   RetrievalDecision         `json:"memory_decision"`
	KnowledgePlan    RetrievalPlan             `json:"knowledge_plan"`
	KnowledgeDecision RetrievalDecision        `json:"knowledge_decision"`
}

// KnowledgeMemoryConsolidateRequest reflects, stores summary, optionally promotes.
type KnowledgeMemoryConsolidateRequest struct {
	Reflect         KnowledgeMemoryReflectRequest
	MemoryID        string
	UserID          string
	SessionID       string
	Scope           string
	Namespace       string
	Role            string
	Importance      float64
	TTLSeconds      int
	Metadata        map[string]any
	PromoteToKnowledge bool
	Promotion       *PromoteToKnowledgeRequest
}

// KnowledgeMemoryConsolidateResponse reports the full consolidation outcome.
type KnowledgeMemoryConsolidateResponse struct {
	Reflection KnowledgeMemoryReflection     `json:"reflection"`
	Memory     *types.MemoryRecord           `json:"memory,omitempty"`
	Knowledge  *types.KnowledgeRecord        `json:"knowledge,omitempty"`
}

// PromoteToKnowledgeRequest promotes memories into durable knowledge.
type PromoteToKnowledgeRequest struct {
	MemoryIDs    []string
	KnowledgeID  string
	Title        string
	Content      string
	Collection   string
	ChunkSize    int
	ChunkOverlap int
	Metadata     map[string]string
	Entities     []string
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
