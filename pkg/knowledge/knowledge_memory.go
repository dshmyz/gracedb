// Package knowledge provides KnowledgeMemory: fused recall, reflection,
// consolidation, and promotion from episodic memory to durable knowledge.
package knowledge

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"github.com/dshmyz/gracedb/pkg/types"
)

const (
	defaultTopKMemories    = 5
	defaultTopKKnowledge   = 4
	defaultMaxMemoryChars  = 1200
	defaultMaxFacts        = 6
	defaultMaxThemes       = 6
	defaultMaxSummaryChars = 320
)

// DBInterface abstracts the gracedb DB operations needed by KnowledgeMemory.
type DBInterface interface {
	SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error)
	GetMemory(memoryID string) (*types.MemoryRecord, error)
	SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error)
	SaveKnowledge(collectionName, knowledgeID, title, content string, req types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error)
	SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error)
	GraphBFS(startNodeID string, maxDepth int) (*GraphBFSResult, error)
	GraphGetNode(id string) (*GraphNodeView, error)
}

// GraphBFSResult holds graph expansion output.
type GraphBFSResult struct {
	Nodes []*GraphNodeView
	Edges []*GraphEdgeView
}

// GraphNodeView is a lightweight graph node snapshot for context packing.
type GraphNodeView struct {
	ID   string
	Type string
}

// GraphEdgeView is a lightweight graph edge snapshot.
type GraphEdgeView struct {
	ID         string
	FromNodeID string
	ToNodeID   string
	Type       string
}

// KnowledgeMemory is the high-level memory/knowledge facade over memory,
// knowledge, and context packing APIs.
type KnowledgeMemory struct {
	db        DBInterface
	reflector KnowledgeMemoryReflector
}

// New creates a KnowledgeMemory facade.
// If reflector is nil, a deterministic (rule-based) reflector is used.
func New(db DBInterface, reflector KnowledgeMemoryReflector) *KnowledgeMemory {
	return &KnowledgeMemory{db: db, reflector: reflector}
}

// Remember stores one episodic memory item.
func (km *KnowledgeMemory) Remember(ctx context.Context, req types.MemorySaveRequest) (*types.MemoryRecord, error) {
	return km.db.SaveMemory(req)
}

// Recall retrieves a fused memory and knowledge view plus a packed context.
func (km *KnowledgeMemory) Recall(ctx context.Context, req KnowledgeMemoryRecallRequest) (*KnowledgeMemoryRecallResponse, error) {
	req = normalizeRecallRequest(req)
	query := resolveQuery(req.Query, req.Plan, req.EntityNames)
	if strings.TrimSpace(query) == "" {
		return nil, types.ErrEmptyText
	}

	resp := &KnowledgeMemoryRecallResponse{Query: query}

	// Search memory.
	if !req.DisableMemory {
		memResp, err := km.db.SearchMemory(types.MemorySearchRequest{
			Query:     query,
			UserID:    req.UserID,
			SessionID: req.SessionID,
			Scope:     req.Scope,
			Namespace: req.Namespace,
			TopK:      req.TopKMemories,
		})
		if err == nil && memResp != nil {
			resp.Memories = memResp.Results
		}
	}

	// Search knowledge.
	var knowledgeResp *types.KnowledgeSearchResponse
	if !req.DisableKnowledge {
		collection := req.Collection
		if collection == "" {
			collection = "default"
		}
		kResp, err := km.db.SearchKnowledge(collection, query, req.TopKKnowledge)
		if err == nil && kResp != nil {
			knowledgeResp = kResp
			resp.Knowledge = kResp.Results
		}
	}

	resp.Entities = collectEntities(req.EntityNames, resp.Memories, resp.Knowledge)

	// Graph expansion: enrich context with entity neighborhood.
	if len(resp.Entities) > 0 && req.MaxHops > 0 {
		graphText := km.expandEntityGraph(ctx, resp.Entities, req.MaxHops, req.MaxTraversalNodes)
		if graphText != "" {
			resp.ContextPack = appendContextSection(resp.ContextPack, "graph", "Graph Context", graphText, nil)
		}
	}

	resp.ContextPack = buildContextPack(query, resp.Memories, knowledgeResp, resp.Entities, req.MaxMemoryItems, req.MaxMemoryChars)
	if len(resp.Entities) == 0 {
		resp.Entities = resp.ContextPack.Entities
	}
	return resp, nil
}

func (km *KnowledgeMemory) expandEntityGraph(ctx context.Context, entities []string, maxHops, maxNodes int) string {
	if maxNodes <= 0 {
		maxNodes = 50
	}
	var parts []string
	nodeCount := 0
	seen := make(map[string]bool)
	for _, name := range entities {
		if nodeCount >= maxNodes {
			break
		}
		// Try resolving entity name to node ID (heuristic: try as-is, then with prefixes).
		nodeID := resolveEntityNodeID(name)
		result, err := km.db.GraphBFS(nodeID, maxHops)
		if err != nil {
			continue
		}
		for _, n := range result.Nodes {
			if seen[n.ID] {
				continue
			}
			seen[n.ID] = true
			parts = append(parts, fmt.Sprintf("[Node: %s (%s)]", n.ID, n.Type))
			nodeCount++
			if nodeCount >= maxNodes {
				break
			}
		}
		for _, e := range result.Edges {
			parts = append(parts, fmt.Sprintf("[Edge: %s ->%s-> %s]", e.FromNodeID, e.Type, e.ToNodeID))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	return strings.Join(parts, "\n")
}

func resolveEntityNodeID(name string) string {
	// Try common entity ID patterns. Users may store entities with various prefixes.
	return "entity:" + name
}

func appendContextSection(pack ContextPack, kind, title, text string, sourceIDs []string) ContextPack {
	pack.Sections = append(pack.Sections, ContextSection{
		Kind: kind, Title: title, Text: text, SourceIDs: sourceIDs,
	})
	parts := []string{pack.Text}
	if text != "" {
		parts = append(parts, fmt.Sprintf("[%s]\n%s", strings.ToUpper(kind), text))
	}
	pack.Text = strings.Join(parts, "\n\n")
	return pack
}

// Reflect retrieves relevant context and synthesizes a structured reflection.
// If no reflector is set, a deterministic (rule-based) reflection is produced.
func (km *KnowledgeMemory) Reflect(ctx context.Context, req KnowledgeMemoryReflectRequest) (*KnowledgeMemoryReflection, error) {
	recallResp, err := km.Recall(ctx, KnowledgeMemoryRecallRequest{
		Query:           req.Query,
		UserID:          req.UserID,
		SessionID:       req.SessionID,
		Scope:           req.Scope,
		Namespace:       req.Namespace,
		Collection:      req.Collection,
		TopKMemories:    req.TopKMemories,
		TopKKnowledge:   req.TopKKnowledge,
	})
	if err != nil {
		return nil, err
	}

	input := KnowledgeMemoryReflectInput{Recall: *recallResp}

	var reflection *KnowledgeMemoryReflection
	if km.reflector != nil {
		reflection, err = km.reflector.Reflect(ctx, req, input)
		if err != nil {
			return nil, err
		}
	} else {
		reflection = deterministicReflect(req, input)
	}
	if reflection == nil {
		return nil, fmt.Errorf("knowledge: reflector returned nil reflection")
	}

	reflection.SourceMemoryIDs = uniqueSortedStrings(append(reflection.SourceMemoryIDs, recallResp.ContextPack.MemoryIDs...))
	reflection.SourceKnowledgeIDs = uniqueSortedStrings(append(reflection.SourceKnowledgeIDs, recallResp.ContextPack.KnowledgeIDs...))
	reflection.SourceChunkIDs = uniqueSortedStrings(append(reflection.SourceChunkIDs, recallResp.ContextPack.ChunkIDs...))
	reflection.Entities = uniqueSortedStrings(append(reflection.Entities, recallResp.Entities...))
	if reflection.ContextPack.Text == "" {
		reflection.ContextPack = recallResp.ContextPack
	}
	return reflection, nil
}

// Consolidate reflects over relevant context, stores a summary memory, and can
// optionally promote it to durable knowledge.
func (km *KnowledgeMemory) Consolidate(ctx context.Context, req KnowledgeMemoryConsolidateRequest) (*KnowledgeMemoryConsolidateResponse, error) {
	reflectResp, err := km.Reflect(ctx, req.Reflect)
	if err != nil {
		return nil, err
	}

	summary := strings.TrimSpace(reflectResp.Summary)
	if summary == "" {
		return nil, fmt.Errorf("knowledge: reflection summary is empty")
	}

	memoryID := strings.TrimSpace(req.MemoryID)
	if memoryID == "" {
		memoryID = fmt.Sprintf("summary:%s", req.SessionID)
	}

	metadata := cloneAnyMap(req.Metadata)
	metadata["knowledge_memory_summary"] = true
	metadata["knowledge_memory_themes"] = reflectResp.Themes
	metadata["knowledge_memory_entities"] = reflectResp.Entities
	metadata["knowledge_memory_source_memory_ids"] = reflectResp.SourceMemoryIDs
	metadata["knowledge_memory_source_knowledge_ids"] = reflectResp.SourceKnowledgeIDs
	metadata["knowledge_memory_source_chunk_ids"] = reflectResp.SourceChunkIDs

	memRecord, err := km.db.SaveMemory(types.MemorySaveRequest{
		MemoryID:   memoryID,
		UserID:     firstNonEmpty(req.UserID, req.Reflect.UserID),
		SessionID:  firstNonEmpty(req.SessionID, req.Reflect.SessionID),
		Scope:      firstNonEmpty(req.Scope, req.Reflect.Scope),
		Namespace:  firstNonEmpty(req.Namespace, req.Reflect.Namespace),
		Role:       firstNonEmpty(req.Role, "summary"),
		Content:    summary,
		Metadata:   metadata,
		Importance: req.Importance,
		TTLSeconds: req.TTLSeconds,
	})
	if err != nil {
		return nil, err
	}

	var knowledge *types.KnowledgeRecord
	if req.PromoteToKnowledge || req.Promotion != nil {
		promotionReq := types.KnowledgeSaveRequest{}
		if req.Promotion != nil {
			promotionReq.ChunkSize = req.Promotion.ChunkSize
			promotionReq.ChunkOverlap = req.Promotion.ChunkOverlap
			promotionReq.Metadata = req.Promotion.Metadata
			promotionReq.Entities = req.Promotion.Entities
		}
		if strings.TrimSpace(promotionReq.Title) == "" {
			promotionReq.Title = compactTitle(summary)
		}
		if strings.TrimSpace(promotionReq.Content) == "" {
			promotionReq.Content = summary
		}

		knowledgeID := fmt.Sprintf("knowledge:%s", req.SessionID)
		if req.Promotion != nil && req.Promotion.KnowledgeID != "" {
			knowledgeID = req.Promotion.KnowledgeID
		}
		collection := "default"
		if req.Promotion != nil && req.Promotion.Collection != "" {
			collection = req.Promotion.Collection
		}
		knowledge, err = km.db.SaveKnowledge(collection, knowledgeID, promotionReq.Title, summary, promotionReq)
		if err != nil {
			return nil, err
		}
	}

	return &KnowledgeMemoryConsolidateResponse{
		Reflection: *reflectResp,
		Memory:     memRecord,
		Knowledge:  knowledge,
	}, nil
}

// --- internal helpers ---

func normalizeRecallRequest(req KnowledgeMemoryRecallRequest) KnowledgeMemoryRecallRequest {
	if req.TopKMemories <= 0 {
		req.TopKMemories = defaultTopKMemories
	}
	if req.TopKKnowledge <= 0 {
		req.TopKKnowledge = defaultTopKKnowledge
	}
	if req.MaxMemoryItems <= 0 {
		req.MaxMemoryItems = req.TopKMemories
	}
	if req.MaxMemoryChars <= 0 {
		req.MaxMemoryChars = defaultMaxMemoryChars
	}
	return req
}

func resolveQuery(query string, plan *RetrievalPlan, entityNames []string) string {
	query = strings.TrimSpace(query)
	if query != "" {
		return query
	}
	if plan != nil && strings.TrimSpace(plan.Query) != "" {
		return strings.TrimSpace(plan.Query)
	}
	if len(entityNames) > 0 {
		return strings.Join(entityNames, " ")
	}
	return ""
}

func buildContextPack(query string, memories []types.MemorySearchHit, knowledgeResp *types.KnowledgeSearchResponse, entities []string, maxItems, maxChars int) ContextPack {
	var sections []ContextSection
	var memoryIDs []string
	var knowledgeIDs []string
	var chunkIDs []string

	memoryText := buildMemoryContext(memories, maxItems, maxChars)
	for _, hit := range memories {
		memoryIDs = append(memoryIDs, hit.Memory.ID)
	}
	if memoryText != "" {
		sections = append(sections, ContextSection{
			Kind:      "memories",
			Title:     "Memories",
			Text:      memoryText,
			SourceIDs: memoryIDs,
		})
	}

	if knowledgeResp != nil {
		for _, hit := range knowledgeResp.Results {
			knowledgeIDs = append(knowledgeIDs, hit.KnowledgeID)
			chunkIDs = append(chunkIDs, hit.ChunkIDs...)
		}
		if len(chunkIDs) > 0 {
			// Build knowledge context from snippet text.
			var kbParts []string
			for _, hit := range knowledgeResp.Results {
				text := firstNonEmpty(hit.Snippet, hit.Title)
				if text != "" {
					kbParts = append(kbParts, fmt.Sprintf("[%s] %s", hit.KnowledgeID, text))
				}
			}
			sections = append(sections, ContextSection{
				Kind:      "knowledge",
				Title:     "Knowledge",
				Text:      strings.Join(kbParts, "\n"),
				SourceIDs: chunkIDs,
			})
		}
	}

	return contextPackFromSections(query, sections, memoryIDs, knowledgeIDs, chunkIDs, entities)
}

func contextPackFromSections(query string, sections []ContextSection, memoryIDs, knowledgeIDs, chunkIDs, entities []string) ContextPack {
	var filtered []ContextSection
	var parts []string
	for _, section := range sections {
		if strings.TrimSpace(section.Text) == "" {
			continue
		}
		section.SourceIDs = uniqueStrings(section.SourceIDs)
		filtered = append(filtered, section)
		parts = append(parts, fmt.Sprintf("[%s]\n%s", strings.ToUpper(section.Kind), strings.TrimSpace(section.Text)))
	}

	return ContextPack{
		Query:        query,
		Text:         strings.TrimSpace(strings.Join(parts, "\n\n")),
		Sections:     filtered,
		MemoryIDs:    uniqueStrings(memoryIDs),
		KnowledgeIDs: uniqueStrings(knowledgeIDs),
		ChunkIDs:     uniqueStrings(chunkIDs),
		Entities:     uniqueStrings(entities),
	}
}

func buildMemoryContext(memories []types.MemorySearchHit, maxItems, maxChars int) string {
	if len(memories) == 0 {
		return ""
	}
	if maxItems <= 0 || maxItems > len(memories) {
		maxItems = len(memories)
	}
	var lines []string
	charsUsed := 0
	for _, hit := range memories[:maxItems] {
		line := fmt.Sprintf("[%s] %s", hit.Memory.ID, strings.TrimSpace(hit.Memory.Content))
		if maxChars > 0 && charsUsed+len(line) > maxChars {
			remaining := maxChars - charsUsed
			if remaining <= 0 {
				break
			}
			line = clipString(line, remaining)
		}
		lines = append(lines, line)
		charsUsed += len(line)
		if maxChars > 0 && charsUsed >= maxChars {
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func collectEntities(seed []string, memories []types.MemorySearchHit, knowledge []types.KnowledgeSearchHit) []string {
	entities := append([]string(nil), seed...)
	for _, hit := range memories {
		entities = append(entities, extractEntityNames(hit.Memory.Content)...)
	}
	for _, hit := range knowledge {
		entities = append(entities, hit.Entities...)
		entities = append(entities, extractEntityNames(hit.Title)...)
		entities = append(entities, extractEntityNames(hit.Snippet)...)
	}
	return uniqueStrings(entities)
}

// deterministicReflect produces a reflection without LLM calls.
// It extracts facts from retrieved memories/knowledge, builds a summary,
// and identifies themes from entities and query keywords.
func deterministicReflect(req KnowledgeMemoryReflectRequest, input KnowledgeMemoryReflectInput) *KnowledgeMemoryReflection {
	maxFacts := req.MaxFacts
	if maxFacts <= 0 {
		maxFacts = defaultMaxFacts
	}
	maxThemes := req.MaxThemes
	if maxThemes <= 0 {
		maxThemes = defaultMaxThemes
	}
	maxSummaryChars := req.MaxSummaryChars
	if maxSummaryChars <= 0 {
		maxSummaryChars = defaultMaxSummaryChars
	}

	var facts []string
	sourceMemoryIDs := make([]string, 0, len(input.Recall.Memories))
	for _, hit := range input.Recall.Memories {
		if len(facts) >= maxFacts {
			break
		}
		facts = append(facts, compactSnippet(hit.Memory.Content))
		sourceMemoryIDs = append(sourceMemoryIDs, hit.Memory.ID)
	}

	sourceKnowledgeIDs := make([]string, 0, len(input.Recall.Knowledge))
	for _, hit := range input.Recall.Knowledge {
		if len(facts) >= maxFacts {
			break
		}
		fact := firstNonEmpty(hit.Snippet, hit.Title)
		if strings.TrimSpace(fact) != "" {
			facts = append(facts, compactSnippet(fact))
		}
		sourceKnowledgeIDs = append(sourceKnowledgeIDs, hit.KnowledgeID)
	}

	if len(facts) == 0 && strings.TrimSpace(input.Recall.ContextPack.Text) != "" {
		facts = append(facts, compactSnippet(input.Recall.ContextPack.Text))
	}

	summary := strings.Join(facts, " ")
	summary = clipString(strings.TrimSpace(summary), maxSummaryChars)
	themes := extractThemes(req, input.Recall, maxThemes)

	return &KnowledgeMemoryReflection{
		Summary:            summary,
		Themes:             themes,
		Entities:           uniqueStrings(input.Recall.Entities),
		Facts:              facts,
		SourceMemoryIDs:    uniqueStrings(sourceMemoryIDs),
		SourceKnowledgeIDs: uniqueStrings(sourceKnowledgeIDs),
		SourceChunkIDs:     uniqueStrings(input.Recall.ContextPack.ChunkIDs),
		ContextPack:        input.Recall.ContextPack,
	}
}

func extractThemes(req KnowledgeMemoryReflectRequest, recall KnowledgeMemoryRecallResponse, maxThemes int) []string {
	themes := append([]string(nil), recall.Entities...)
	themes = append(themes, queryKeywords(req.Query)...)
	if req.Instructions != "" {
		themes = append(themes, queryKeywords(req.Instructions)...)
	}
	themes = uniqueStrings(themes)
	if len(themes) > maxThemes {
		themes = themes[:maxThemes]
	}
	return themes
}

func queryKeywords(query string) []string {
	query = strings.ToLower(query)
	var tokens []string
	var current strings.Builder
	for _, r := range query {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
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

func extractEntityNames(text string) []string {
	// Simple heuristic: extract capitalized multi-word phrases.
	var entities []string
	var current strings.Builder
	inEntity := false
	for _, r := range text {
		if unicode.IsUpper(r) || unicode.IsLetter(r) && inEntity {
			current.WriteRune(r)
			inEntity = true
		} else {
			if inEntity && current.Len() > 2 {
				entities = append(entities, current.String())
			}
			current.Reset()
			inEntity = false
		}
	}
	if inEntity && current.Len() > 2 {
		entities = append(entities, current.String())
	}
	return entities
}

func uniqueStrings(s []string) []string {
	seen := make(map[string]struct{}, len(s))
	var out []string
	for _, v := range s {
		v = strings.TrimSpace(v)
		if v == "" {
			continue
		}
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func uniqueSortedStrings(s []string) []string {
	return uniqueStrings(s) // TODO: add sort if needed
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func clipString(text string, max int) string {
	text = strings.TrimSpace(text)
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return strings.TrimSpace(text[:max-3]) + "..."
}

func compactSnippet(text string) string {
	s := strings.TrimSpace(text)
	if len(s) > 200 {
		s = s[:197] + "..."
	}
	return s
}

func compactTitle(text string) string {
	title := compactSnippet(text)
	title = clipString(title, 96)
	title = strings.TrimSpace(title)
	if title == "" {
		return "Knowledge"
	}
	return title
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
