package gracedb

import (
	"context"
	"fmt"
	"strings"

	"github.com/dshmyz/gracedb/pkg/graph"
	"github.com/dshmyz/gracedb/pkg/mcp"
	"github.com/dshmyz/gracedb/pkg/types"
)

// ToolChunk is a chunk-shaped response used by tool APIs.
type ToolChunk struct {
	ID         string            `json:"id"`
	DocumentID string            `json:"document_id,omitempty"`
	Content    string            `json:"content"`
	Score      float64           `json:"score,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// GraphRAGToolbox exposes functions for external LLM orchestration.
type GraphRAGToolbox struct {
	db *DB
}

// GraphRAGTools returns the toolbox for external LLM orchestration.
func (db *DB) GraphRAGTools() *GraphRAGToolbox {
	return &GraphRAGToolbox{db: db}
}

// Definitions returns all available tool definitions.
func (t *GraphRAGToolbox) Definitions() []mcp.ToolDef {
	return []mcp.ToolDef{
		{Name: "search_knowledge", Description: "Search durable knowledge by query", InputSchema: map[string]any{"query": "string", "top_k": "int"}},
		{Name: "save_knowledge", Description: "Save a knowledge item with content", InputSchema: map[string]any{"knowledge_id": "string", "content": "string"}},
		{Name: "search_memory", Description: "Search memories in a scope bucket", InputSchema: map[string]any{"query": "string", "scope": "string"}},
		{Name: "save_memory", Description: "Save a memory with scope", InputSchema: map[string]any{"memory_id": "string", "content": "string", "scope": "string"}},
		{Name: "expand_graph", Description: "Expand a graph neighborhood around nodes", InputSchema: map[string]any{"node_ids": "[]string", "max_depth": "int"}},
		{Name: "recall_knowledge_memory", Description: "Fused recall across memory and knowledge", InputSchema: map[string]any{"query": "string", "top_k": "int"}},
		{Name: "build_context", Description: "Assemble chunk text into a prompt context pack", InputSchema: map[string]any{"query": "string", "max_chars": "int"}},
	}
}

// Call invokes a tool by name with the given payload.
func (t *GraphRAGToolbox) Call(ctx context.Context, name string, payload map[string]any) (any, error) {
	switch name {
	case "search_knowledge":
		return t.callSearchKnowledge(payload)
	case "save_knowledge":
		return t.callSaveKnowledge(payload)
	case "search_memory":
		return t.callSearchMemory(payload)
	case "save_memory":
		return t.callSaveMemory(payload)
	case "expand_graph":
		return t.callExpandGraph(payload)
	case "recall_knowledge_memory":
		return t.callRecallKnowledgeMemory(payload)
	case "build_context":
		return t.callBuildContext(payload)
	default:
		return nil, fmt.Errorf("unknown tool: %s", name)
	}
}

func (t *GraphRAGToolbox) callSearchKnowledge(payload map[string]any) (any, error) {
	query, _ := payload["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	collection, _ := payload["collection"].(string)
	if collection == "" {
		collection = "default"
	}
	topK, _ := payload["top_k"].(int)
	if topK <= 0 {
		topK = 10
	}

	resp, err := t.db.SearchKnowledge(collection, query, topK)
	if err != nil {
		return nil, err
	}

	var chunks []ToolChunk
	for _, hit := range resp.Results {
		chunks = append(chunks, ToolChunk{ID: hit.KnowledgeID, DocumentID: hit.KnowledgeID, Content: hit.Snippet, Score: hit.Score})
	}
	return map[string]any{"chunks": chunks, "query": query}, nil
}

func (t *GraphRAGToolbox) callSaveKnowledge(payload map[string]any) (any, error) {
	knowledgeID, _ := payload["knowledge_id"].(string)
	if knowledgeID == "" {
		return nil, fmt.Errorf("knowledge_id is required")
	}
	content, _ := payload["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	title, _ := payload["title"].(string)
	collection, _ := payload["collection"].(string)
	if collection == "" {
		collection = "default"
	}

	record, err := t.db.SaveKnowledge(collection, knowledgeID, title, content, types.KnowledgeSaveRequest{Content: content, Title: title})
	if err != nil {
		return nil, err
	}
	return map[string]any{"knowledge_id": record.ID, "chunk_count": len(record.ChunkIDs)}, nil
}

func (t *GraphRAGToolbox) callSearchMemory(payload map[string]any) (any, error) {
	query, _ := payload["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	scope, _ := payload["scope"].(string)
	topK, _ := payload["top_k"].(int)
	if topK <= 0 {
		topK = 5
	}

	resp, err := t.db.SearchMemory(types.MemorySearchRequest{Query: query, Scope: scope, TopK: topK})
	if err != nil {
		return nil, err
	}

	var results []map[string]any
	for _, hit := range resp.Results {
		results = append(results, map[string]any{"id": hit.Memory.ID, "content": hit.Memory.Content, "score": hit.Score})
	}
	return map[string]any{"results": results, "query": query}, nil
}

func (t *GraphRAGToolbox) callSaveMemory(payload map[string]any) (any, error) {
	memoryID, _ := payload["memory_id"].(string)
	if memoryID == "" {
		return nil, fmt.Errorf("memory_id is required")
	}
	content, _ := payload["content"].(string)
	if content == "" {
		return nil, fmt.Errorf("content is required")
	}
	scope, _ := payload["scope"].(string)
	namespace, _ := payload["namespace"].(string)

	record, err := t.db.SaveMemory(types.MemorySaveRequest{MemoryID: memoryID, Content: content, Scope: scope, Namespace: namespace})
	if err != nil {
		return nil, err
	}
	return map[string]any{"memory_id": record.ID, "scope": record.Scope}, nil
}

func (t *GraphRAGToolbox) callExpandGraph(payload map[string]any) (any, error) {
	rawIDs, _ := payload["node_ids"].([]any)
	if len(rawIDs) == 0 {
		return nil, fmt.Errorf("node_ids is required")
	}
	maxDepth, _ := payload["max_depth"].(int)
	if maxDepth <= 0 {
		maxDepth = 2
	}

	var allNodes []*graph.GraphNode
	var allEdges []*graph.GraphEdge

	for _, rawID := range rawIDs {
		nodeID, ok := rawID.(string)
		if !ok {
			continue
		}
		result, err := t.db.Graph().BFS(nodeID, graph.NeighborOptions{MaxDepth: maxDepth})
		if err != nil {
			continue
		}
		allNodes = append(allNodes, result.Nodes...)
		allEdges = append(allEdges, result.Edges...)
	}

	nodes := make([]map[string]any, len(allNodes))
	for i, n := range allNodes {
		nodes[i] = map[string]any{"id": n.ID, "type": n.Type}
	}
	edges := make([]map[string]any, len(allEdges))
	for i, e := range allEdges {
		edges[i] = map[string]any{"id": e.ID, "from": e.FromNodeID, "to": e.ToNodeID, "type": e.Type}
	}
	return map[string]any{"nodes": nodes, "edges": edges}, nil
}

func (t *GraphRAGToolbox) callRecallKnowledgeMemory(payload map[string]any) (any, error) {
	query, _ := payload["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	topK, _ := payload["top_k"].(int)
	if topK <= 0 {
		topK = 5
	}

	knowledgeResp, _ := t.db.SearchKnowledge("default", query, topK)
	memoryResp, _ := t.db.SearchMemory(types.MemorySearchRequest{Query: query, TopK: topK})

	var contextParts []string
	if knowledgeResp != nil {
		for _, hit := range knowledgeResp.Results {
			contextParts = append(contextParts, fmt.Sprintf("[Knowledge: %s] %s", hit.Title, hit.Snippet))
		}
	}
	if memoryResp != nil {
		for _, hit := range memoryResp.Results {
			contextParts = append(contextParts, fmt.Sprintf("[Memory] %s", hit.Memory.Content))
		}
	}

	return map[string]any{"query": query, "knowledge": knowledgeResp, "memories": memoryResp, "context_text": strings.Join(contextParts, "\n")}, nil
}

func (t *GraphRAGToolbox) callBuildContext(payload map[string]any) (any, error) {
	query, _ := payload["query"].(string)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	maxChars, _ := payload["max_chars"].(int)
	if maxChars <= 0 {
		maxChars = 4000
	}

	recall, err := t.callRecallKnowledgeMemory(map[string]any{"query": query, "top_k": 10})
	if err != nil {
		return nil, err
	}

	result := recall.(map[string]any)
	contextText, _ := result["context_text"].(string)

	runes := []rune(contextText)
	if len(runes) > maxChars {
		contextText = string(runes[:maxChars]) + "..."
	}

	return map[string]any{"query": query, "text": contextText, "char_count": len(contextText)}, nil
}
