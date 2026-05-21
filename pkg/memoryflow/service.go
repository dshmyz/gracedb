package memoryflow

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/dshmyz/gracedb/pkg/types"
)

// DBInterface abstracts the gracedb DB for memoryflow.
type DBInterface interface {
	SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error)
	GetMemory(memoryID string) (*types.MemoryRecord, error)
	UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error)
	DeleteMemory(memoryID string) error
	SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error)
	SaveKnowledge(collectionName, knowledgeID, title, content string, req types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error)
	GetKnowledge(collectionName, knowledgeID string) (*types.KnowledgeRecord, error)
	SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error)
	DeleteKnowledge(collectionName, knowledgeID string) error
}

// Service orchestrates memory workflow operations.
type Service struct {
	db        DBInterface
	planner   QueryPlanner
	extractor SessionExtractor
	policy    PromotionPolicy
	strategy  RecallStrategy
}

// New creates a memoryflow service.
func New(db DBInterface, planner QueryPlanner, extractor SessionExtractor, opts ...Option) *Service {
	svc := &Service{
		db:        db,
		planner:   planner,
		extractor: extractor,
		policy:    &defaultPolicy{},
		strategy:  &PassthroughRecallStrategy{},
	}
	for _, opt := range opts {
		opt(svc)
	}
	return svc
}

// Option configures the Service.
type Option func(*Service)

// WithPromotionPolicy sets the promotion policy.
func WithPromotionPolicy(p PromotionPolicy) Option {
	return func(s *Service) {
		s.policy = p
	}
}

// WithRecallStrategy sets the recall strategy.
func WithRecallStrategy(s RecallStrategy) Option {
	return func(svc *Service) {
		svc.strategy = s
	}
}

// IngestTranscript stores a transcript as episodic memory exchanges.
func (s *Service) IngestTranscript(ctx context.Context, req IngestTranscriptRequest) (*IngestTranscriptResponse, error) {
	if len(req.Transcript.Turns) == 0 {
		return &IngestTranscriptResponse{}, nil
	}

	scope := req.Scope
	if scope == "" {
		scope = types.MemoryScopeSession
	}
	namespace := req.Namespace
	if namespace == "" {
		namespace = "assistant"
	}

	// Group turns into exchanges (user+assistant pairs).
	var memoryIDs []string
	var exchange []TranscriptTurn

	for i, turn := range req.Transcript.Turns {
		exchange = append(exchange, turn)

		// Flush exchange at pair boundary or end.
		isPairEnd := (turn.Role == "assistant" && len(exchange) >= 2) || i == len(req.Transcript.Turns)-1
		if isPairEnd && len(exchange) > 0 {
			content := buildExchangeContent(exchange)
			meta := cloneAnyMap(req.Metadata)
			meta["tags"] = req.Tags
			meta["exchange_index"] = len(memoryIDs)
			meta["taxonomy"] = map[string]string{
				"source": req.Transcript.Source,
			}

			record, err := s.db.SaveMemory(types.MemorySaveRequest{
				MemoryID:  fmt.Sprintf("episode:%s:%d", req.Transcript.SessionID, len(memoryIDs)),
				UserID:    req.Transcript.UserID,
				SessionID: req.Transcript.SessionID,
				Scope:     scope,
				Namespace: namespace,
				Role:      "exchange",
				Content:   content,
				Metadata:  meta,
			})
			if err != nil {
				return nil, fmt.Errorf("save episode: %w", err)
			}
			memoryIDs = append(memoryIDs, record.ID)
			exchange = nil
		}
	}

	return &IngestTranscriptResponse{
		MemoryIDs: memoryIDs,
		Count:     len(memoryIDs),
	}, nil
}

// Recall performs fused memory + knowledge recall.
func (s *Service) Recall(ctx context.Context, req RecallRequest) (*RecallResponse, error) {
	// Apply recall strategy.
	resp, err := s.strategy.Recall(ctx, req, func(ctx context.Context, r RecallRequest) (*RecallResponse, error) {
		// Resolve retrieval plan.
		plan := resolvePlan(r)

		var memories []types.MemorySearchHit
		var knowledge []types.KnowledgeSearchHit
		var contextParts []string

		// Search memory.
		if !r.DisableMemory {
			memResp, err := s.db.SearchMemory(types.MemorySearchRequest{
				Query:     plan.Query,
				UserID:    r.UserID,
				SessionID: r.SessionID,
				Scope:     r.Scope,
				Namespace: r.Namespace,
				TopK:      r.TopKMemories,
			})
			if err == nil && memResp != nil {
				memories = memResp.Results
				for _, hit := range memories {
					contextParts = append(contextParts, fmt.Sprintf("[Memory: %s] %s", hit.Memory.Scope, hit.Memory.Content))
				}
			}
		}

		// Search knowledge.
		if !r.DisableKnowledge {
			collection := r.Collection
			if collection == "" {
				collection = "default"
			}
			knownResp, err := s.db.SearchKnowledge(collection, plan.Query, r.TopKKnowledge)
			if err == nil && knownResp != nil {
				knowledge = knownResp.Results
				for _, hit := range knowledge {
					contextParts = append(contextParts, fmt.Sprintf("[Knowledge: %s] %s", hit.Title, hit.Snippet))
				}
			}
		}

		// Build context text.
		contextText := strings.Join(contextParts, "\n")
		if r.MaxContextChars > 0 && len(contextText) > r.MaxContextChars {
			runes := []rune(contextText)
			contextText = string(runes[:r.MaxContextChars]) + "..."
		}
		if r.MaxContextChunks > 0 {
			totalChunks := len(memories) + len(knowledge)
			if totalChunks > r.MaxContextChunks {
				if len(memories) > r.MaxContextChunks/2 {
					memories = memories[:r.MaxContextChunks/2]
				}
				remaining := r.MaxContextChunks - len(memories)
				if len(knowledge) > remaining {
					knowledge = knowledge[:remaining]
				}
			}
		}

		return &RecallResponse{
			Plan:      plan,
			Decision:  resolveDecision(r.RetrievalMode, plan),
			Memories:  memories,
			Knowledge: knowledge,
			Context:   contextText,
		}, nil
	})
	return resp, err
}

// WakeUpLayers assembles all wake-up context tiers.
func (s *Service) WakeUpLayers(ctx context.Context, req WakeUpLayersRequest) (*WakeUpLayersResponse, error) {
	recallResp, err := s.Recall(ctx, req.Recall)
	if err != nil {
		return nil, err
	}

	var layers []WakeUpLayer

	// L0: Identity
	if req.Identity != "" {
		layers = append(layers, WakeUpLayer{
			Level: WakeUpLevelL0,
			Title: "Identity",
			Text:  req.Identity,
		})
	}

	// L1: Recent memories
	if len(recallResp.Memories) > 0 {
		var text string
		for _, m := range recallResp.Memories {
			text += fmt.Sprintf("- %s\n", m.Memory.Content)
		}
		layers = append(layers, WakeUpLayer{
			Level: WakeUpLevelL1,
			Title: "Recent Memories",
			Text:  text,
		})
	}

	// L2: Knowledge
	if len(recallResp.Knowledge) > 0 {
		var text string
		for _, k := range recallResp.Knowledge {
			text += fmt.Sprintf("- [%s] %s\n", k.Title, k.Snippet)
		}
		layers = append(layers, WakeUpLayer{
			Level: WakeUpLevelL2,
			Title: "Knowledge",
			Text:  text,
		})
	}

	// L3: Full context
	if recallResp.Context != "" {
		layers = append(layers, WakeUpLayer{
			Level: WakeUpLevelL3,
			Title: "Full Context",
			Text:  recallResp.Context,
		})
	}

	return &WakeUpLayersResponse{
		Layers: layers,
		Recall: *recallResp,
	}, nil
}

// CloseSession ingests transcript and optionally promotes knowledge.
func (s *Service) CloseSession(ctx context.Context, req CloseSessionRequest) (*CloseSessionResponse, error) {
	resp := &CloseSessionResponse{}

	// Ingest transcript.
	ingestResp, err := s.IngestTranscript(ctx, IngestTranscriptRequest{
		Transcript: req.Transcript,
		Scope:      req.Scope,
		Namespace:  req.Namespace,
	})
	if err != nil {
		return nil, err
	}

	// Promote knowledge if requested.
	if req.Promote && s.extractor != nil {
		state := SessionState{
			UserID:    req.Transcript.UserID,
			SessionID: req.Transcript.SessionID,
		}
		candidates, err := s.extractor.Extract(ctx, req.Transcript, state)
		if err != nil {
			return nil, fmt.Errorf("extract knowledge: %w", err)
		}

		if s.policy != nil {
			candidates, err = s.policy.Select(ctx, candidates)
			if err != nil {
				return nil, err
			}
		}

		collection := req.Collection
		if collection == "" {
			collection = "default"
		}

		for _, c := range candidates {
			if c.Kind == "" {
				c.Kind = PromotionKindNote
			}
			record, err := s.db.SaveKnowledge(collection, c.KnowledgeID, c.Title, c.Content, types.KnowledgeSaveRequest{
				Content:    c.Content,
				Title:      c.Title,
				Collection: c.Collection,
				Metadata:   c.Metadata,
			})
			if err != nil {
				continue
			}
			resp.Promotions = append(resp.Promotions, *record)
		}
	}

	resp.Count = len(resp.Promotions) + ingestResp.Count
	return resp, nil
}

// AddDiaryEntry appends a diary entry.
func (s *Service) AddDiaryEntry(ctx context.Context, req DiaryEntryRequest) (*types.MemoryRecord, error) {
	scope := req.Scope
	if scope == "" {
		scope = types.MemoryScopeSession
	}
	namespace := req.Namespace
	if namespace == "" {
		namespace = "diary"
	}

	entryID := req.EntryID
	if entryID == "" {
		entryID = fmt.Sprintf("diary:%s:%d", req.SessionID, time.Now().UnixNano())
	}

	return s.db.SaveMemory(types.MemorySaveRequest{
		MemoryID:   entryID,
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		Scope:      scope,
		Namespace:  namespace,
		Role:       "diary",
		Content:    req.Content,
		Metadata:   req.Metadata,
		Importance: req.Importance,
	})
}

// ListDiaryEntries lists diary entries.
func (s *Service) ListDiaryEntries(ctx context.Context, req DiaryListRequest) ([]types.MemoryRecord, error) {
	topK := req.Limit
	if topK <= 0 {
		topK = 50
	}

	resp, err := s.db.SearchMemory(types.MemorySearchRequest{
		Query:     "",
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Scope:     req.Scope,
		Namespace: "diary",
		TopK:      topK,
	})
	if err != nil {
		return nil, err
	}

	entries := make([]types.MemoryRecord, len(resp.Results))
	for i, hit := range resp.Results {
		entries[i] = hit.Memory
	}
	return entries, nil
}

// DefaultQueryPlanner is a simple planner that tokenizes the query.
type DefaultQueryPlanner struct{}

// Plan tokenizes the query into a retrieval plan.
func (p *DefaultQueryPlanner) Plan(ctx context.Context, query string, state SessionState) (*RetrievalPlan, error) {
	keywords := tokenize(query)
	return &RetrievalPlan{
		Query:         query,
		Keywords:      keywords,
		RetrievalMode: "hybrid",
		Filters: &RetrievalFilters{
			UserID:    state.UserID,
			SessionID: state.SessionID,
			Scope:     state.SessionID,
		},
	}, nil
}

// DefaultSessionExtractor is a simple extractor that doesn't propose knowledge.
type DefaultSessionExtractor struct{}

// Extract returns empty candidates (no LLM available).
func (e *DefaultSessionExtractor) Extract(ctx context.Context, transcript Transcript, state SessionState) ([]PromotionCandidate, error) {
	return nil, nil
}

// defaultPolicy accepts all candidates.
type defaultPolicy struct{}

func (p *defaultPolicy) Select(ctx context.Context, candidates []PromotionCandidate) ([]PromotionCandidate, error) {
	return candidates, nil
}

func buildExchangeContent(turns []TranscriptTurn) string {
	var parts []string
	for _, t := range turns {
		parts = append(parts, fmt.Sprintf("[%s] %s", t.Role, t.Content))
	}
	return strings.Join(parts, "\n")
}

func resolvePlan(req RecallRequest) RetrievalPlan {
	mode := req.RetrievalMode
	if mode == "" {
		mode = "hybrid"
	}
	return RetrievalPlan{
		Query:         req.Query,
		RetrievalMode: mode,
		Filters: &RetrievalFilters{
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Scope:     req.Scope,
			Namespace: req.Namespace,
		},
	}
}

func resolveDecision(requested string, plan RetrievalPlan) RetrievalDecision {
	effective := plan.RetrievalMode
	if effective == "" {
		effective = "hybrid"
	}
	return RetrievalDecision{
		RequestedMode: requested,
		EffectiveMode: effective,
		Reason:        "auto-resolved from request parameters",
	}
}

func tokenize(text string) []string {
	text = strings.ToLower(text)
	var tokens []string
	var current strings.Builder
	for _, r := range text {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func cloneAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
