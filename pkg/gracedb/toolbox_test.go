package gracedb

import (
	"context"
	"testing"
)

func TestToolbox_Definitions(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	defs := tbx.Definitions()
	if len(defs) != 7 {
		t.Fatalf("expected 7 tool definitions, got %d", len(defs))
	}

	expected := []string{
		"search_knowledge",
		"save_knowledge",
		"search_memory",
		"save_memory",
		"expand_graph",
		"recall_knowledge_memory",
		"build_context",
	}
	for i, name := range expected {
		if defs[i].Name != name {
			t.Errorf("definition[%d]: expected '%s', got '%s'", i, name, defs[i].Name)
		}
	}

	// Verify each definition has description and input schema
	for _, d := range defs {
		if d.Description == "" {
			t.Errorf("tool %s: empty description", d.Name)
		}
		if d.InputSchema == nil {
			t.Errorf("tool %s: nil input schema", d.Name)
		}
	}

	_ = ctx
}

func TestToolbox_SearchKnowledge_MissingQuery(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "search_knowledge", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolbox_SearchKnowledge_Empty(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	if _, err := db.CreateCollection("default"); err != nil {
		t.Fatalf("create: %v", err)
	}

	resp, err := tbx.Call(ctx, "search_knowledge", map[string]any{
		"query":  "hello",
		"top_k":  5,
	})
	if err != nil {
		t.Fatalf("search_knowledge: %v", err)
	}
	m := resp.(map[string]any)
	if m["query"] != "hello" {
		t.Errorf("expected query 'hello', got %v", m["query"])
	}
}

func TestToolbox_SaveKnowledge(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	if _, err := db.CreateCollection("tk_save"); err != nil {
		t.Fatalf("create: %v", err)
	}

	resp, err := tbx.Call(ctx, "save_knowledge", map[string]any{
		"knowledge_id": "k1",
		"content":      "This is important knowledge",
		"title":        "Test Doc",
		"collection":   "tk_save",
	})
	if err != nil {
		t.Fatalf("save_knowledge: %v", err)
	}
	m := resp.(map[string]any)
	if m["knowledge_id"] != "k1" {
		t.Errorf("expected knowledge_id k1, got %v", m["knowledge_id"])
	}
}

func TestToolbox_SaveKnowledge_MissingID(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "save_knowledge", map[string]any{
		"content": "no id",
	})
	if err == nil {
		t.Fatal("expected error for missing knowledge_id")
	}
}

func TestToolbox_SaveKnowledge_MissingContent(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "save_knowledge", map[string]any{
		"knowledge_id": "k1",
	})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestToolbox_SearchMemory_MissingQuery(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "search_memory", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolbox_SaveMemory(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	resp, err := tbx.Call(ctx, "save_memory", map[string]any{
		"memory_id": "mem1",
		"content":   "remember this",
		"scope":     "global",
	})
	if err != nil {
		t.Fatalf("save_memory: %v", err)
	}
	m := resp.(map[string]any)
	if m["memory_id"] != "mem1" {
		t.Errorf("expected memory_id mem1, got %v", m["memory_id"])
	}
}

func TestToolbox_SaveMemory_MissingID(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "save_memory", map[string]any{
		"content": "no id",
	})
	if err == nil {
		t.Fatal("expected error for missing memory_id")
	}
}

func TestToolbox_SaveMemory_MissingContent(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "save_memory", map[string]any{
		"memory_id": "mem1",
	})
	if err == nil {
		t.Fatal("expected error for missing content")
	}
}

func TestToolbox_ExpandGraph(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	// Expand on non-existent node should not error, just return empty
	resp, err := tbx.Call(ctx, "expand_graph", map[string]any{
		"node_ids":  []any{"non-existent-node"},
		"max_depth": 1,
	})
	if err != nil {
		t.Fatalf("expand_graph: %v", err)
	}
	m := resp.(map[string]any)
	if m["nodes"] == nil && m["edges"] == nil {
		// OK - empty result for non-existent node
	}
}

func TestToolbox_ExpandGraph_MissingNodeIDs(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "expand_graph", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing node_ids")
	}
}

func TestToolbox_RecallKnowledgeMemory(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	resp, err := tbx.Call(ctx, "recall_knowledge_memory", map[string]any{
		"query": "hello",
		"top_k": 5,
	})
	if err != nil {
		t.Fatalf("recall_knowledge_memory: %v", err)
	}
	m := resp.(map[string]any)
	if m["query"] != "hello" {
		t.Errorf("expected query 'hello', got %v", m["query"])
	}
	if _, ok := m["context_text"]; !ok {
		t.Error("expected context_text in response")
	}
}

func TestToolbox_RecallKnowledgeMemory_MissingQuery(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "recall_knowledge_memory", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolbox_BuildContext(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	resp, err := tbx.Call(ctx, "build_context", map[string]any{
		"query":     "hello",
		"max_chars": 100,
	})
	if err != nil {
		t.Fatalf("build_context: %v", err)
	}
	m := resp.(map[string]any)
	if m["query"] != "hello" {
		t.Errorf("expected query 'hello', got %v", m["query"])
	}
	text, ok := m["text"].(string)
	if !ok {
		t.Fatal("expected text field in response")
	}
	if len([]rune(text)) > 103 { // 100 + "..."
		t.Errorf("expected text <= 103 chars, got %d", len([]rune(text)))
	}
}

func TestToolbox_BuildContext_MissingQuery(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "build_context", map[string]any{})
	if err == nil {
		t.Fatal("expected error for missing query")
	}
}

func TestToolbox_UnknownTool(t *testing.T) {
	db := testDB(t)
	tbx := db.GraphRAGTools()
	ctx := context.Background()

	_, err := tbx.Call(ctx, "nonexistent_tool", map[string]any{})
	if err == nil {
		t.Fatal("expected error for unknown tool")
	}
}
