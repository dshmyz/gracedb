package gracedb

import (
	"context"
	"sync"

	"github.com/dshmyz/gracedb/pkg/graph"
	"github.com/dshmyz/gracedb/pkg/knowledge"
	"github.com/dshmyz/gracedb/pkg/types"
)

// ---------------------------------------------------------------------------
// KnowledgeMemory facade
// ---------------------------------------------------------------------------

// KnowledgeMemory returns the high-level memory/knowledge facade.
func (db *DB) KnowledgeMemory(reflector knowledge.KnowledgeMemoryReflector) *knowledge.KnowledgeMemory {
	return knowledge.New(db, reflector)
}

// ---------------------------------------------------------------------------
// AutoRetain – automatic fact extraction on AddMessage
// ---------------------------------------------------------------------------

// AutoRetainConfig controls automatic retain behaviour triggered by AddMessage.
type AutoRetainConfig struct {
	Enabled bool
	WindowSize   int
	TriggerEvery int
	RoleFilter   []string
}

// FactExtractor extracts structured facts from conversation messages.
type FactExtractor func(ctx context.Context, messages []*types.Message) ([]ExtractedFact, error)

// ExtractedFact is a single structured fact produced by a FactExtractor.
type ExtractedFact struct {
	ID       string
	Type     string
	Content  string
	Vector   []float32
	Entities []string
	Metadata map[string]any
}

// db-level auto-retain state.
var (
	globalAutoRetainCfg   AutoRetainConfig
	globalAutoRetainMu    sync.RWMutex
	globalFactExtractor   FactExtractor
	globalFactExtractorMu sync.RWMutex
	sessionCounters       = make(map[string]*int64)
	sessionCountersMu     sync.Mutex
)

// SetAutoRetain enables automatic fact extraction on AddMessage.
func (db *DB) SetAutoRetain(cfg AutoRetainConfig) {
	globalAutoRetainMu.Lock()
	defer globalAutoRetainMu.Unlock()
	if cfg.WindowSize <= 0 {
		cfg.WindowSize = 6
	}
	if cfg.TriggerEvery <= 0 {
		cfg.TriggerEvery = 2
	}
	globalAutoRetainCfg = cfg
}

// SetFactExtractor registers a FactExtractor for use by auto-retain.
func (db *DB) SetFactExtractor(fn FactExtractor) {
	globalFactExtractorMu.Lock()
	defer globalFactExtractorMu.Unlock()
	globalFactExtractor = fn
}

// fireAutoRetain is called by AddMessage (in session.go) when the counter fires.
func (db *DB) fireAutoRetain(sessionID string) {
	globalAutoRetainMu.RLock()
	cfg := globalAutoRetainCfg
	globalAutoRetainMu.RUnlock()

	globalFactExtractorMu.RLock()
	extractor := globalFactExtractor
	globalFactExtractorMu.RUnlock()

	if extractor == nil {
		return
	}

	go db.triggerExtraction(sessionID, cfg.WindowSize, extractor)
}

func (db *DB) triggerExtraction(sessionID string, windowSize int, extractor FactExtractor) {
	ctx := context.Background()

	msgs, err := db.GetSessionHistory(sessionID, windowSize)
	if err != nil || len(msgs) == 0 {
		return
	}

	facts, err := extractor(ctx, msgs)
	if err != nil || len(facts) == 0 {
		return
	}

	for _, f := range facts {
		if f.ID == "" || f.Content == "" {
			continue
		}
		meta := cloneAny(f.Metadata)
		meta["auto_retained"] = true
		if len(f.Entities) > 0 {
			meta["entities"] = f.Entities
		}
		_, _ = db.SaveMemory(types.MemorySaveRequest{
			MemoryID:  f.ID,
			SessionID: sessionID,
			Scope:     types.MemoryScopeSession,
			Namespace: "auto-retained",
			Role:      firstNonEmptyStr(f.Type, "fact"),
			Content:   f.Content,
			Metadata:  meta,
		})
	}
}

func getOrCreateCounter(sessionID string) *int64 {
	sessionCountersMu.Lock()
	defer sessionCountersMu.Unlock()
	if c, ok := sessionCounters[sessionID]; ok {
		return c
	}
	var n int64
	c := &n
	sessionCounters[sessionID] = c
	return c
}

// Compile-time adapter check: DB implements knowledge.DBInterface.
var _ knowledge.DBInterface = (*DB)(nil)

func (db *DB) GraphBFS(startNodeID string, maxDepth int) (*knowledge.GraphBFSResult, error) {
	result, err := db.graph_.BFS(startNodeID, graph.NeighborOptions{MaxDepth: maxDepth})
	if err != nil {
		return nil, err
	}
	out := &knowledge.GraphBFSResult{}
	for _, n := range result.Nodes {
		out.Nodes = append(out.Nodes, &knowledge.GraphNodeView{ID: n.ID, Type: n.Type})
	}
	for _, e := range result.Edges {
		out.Edges = append(out.Edges, &knowledge.GraphEdgeView{
			ID: e.ID, FromNodeID: e.FromNodeID, ToNodeID: e.ToNodeID, Type: e.Type,
		})
	}
	return out, nil
}

func (db *DB) GraphGetNode(id string) (*knowledge.GraphNodeView, error) {
	node, err := db.graph_.GetNode(id)
	if err != nil {
		return nil, err
	}
	return &knowledge.GraphNodeView{ID: node.ID, Type: node.Type}, nil
}
