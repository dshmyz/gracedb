package graphflow

import (
	"context"
	"testing"
)

func TestBuildAndAnalyze(t *testing.T) {
	results := []ExtractionResult{
		{
			DocumentID: "doc-1",
			Nodes: []ExtractionNode{
				{ID: "node:alice", Name: "Alice", Type: "person", Mentions: 3},
				{ID: "node:bob", Name: "Bob", Type: "person", Mentions: 2},
				{ID: "node:project", Name: "Project", Type: "project", Mentions: 1},
			},
			Edges: []ExtractionEdge{
				{ID: "edge:1", FromNode: "node:alice", ToNode: "node:bob", Type: "knows", Weight: 2.0},
				{ID: "edge:2", FromNode: "node:alice", ToNode: "node:project", Type: "owns", Weight: 1.0},
			},
		},
		{
			DocumentID: "doc-2",
			Nodes: []ExtractionNode{
				{ID: "node:alice", Name: "Alice", Type: "person", Mentions: 1},
				{ID: "node:carol", Name: "Carol", Type: "person", Mentions: 1},
			},
			Edges: []ExtractionEdge{
				{ID: "edge:3", FromNode: "node:alice", ToNode: "node:carol", Type: "knows", Weight: 1.0},
			},
		},
	}

	report, err := Analyze(context.Background(), results, AnalyzeRequest{TopN: 3})
	if err != nil {
		t.Fatal(err)
	}

	if report.TotalNodes != 4 {
		t.Fatalf("expected 4 nodes, got %d", report.TotalNodes)
	}
	if report.TotalEdges != 3 {
		t.Fatalf("expected 3 edges, got %d", report.TotalEdges)
	}

	// Alice should be the top node (degree 3: knows Bob, owns Project, knows Carol).
	if len(report.TopNodes) == 0 {
		t.Fatal("expected top nodes")
	}
	topName := report.TopNodes[0].Name
	if topName != "Alice" {
		t.Fatalf("expected Alice as top node, got %s", topName)
	}
}

func TestHeuristicExtractor(t *testing.T) {
	extractor := &HeuristicExtractor{}
	text := "Alice and Bob work on the Apollo project at Google."
	result := extractor.Extract(text, "doc-1")

	if len(result.Nodes) == 0 {
		t.Fatal("expected extracted nodes")
	}

	// Should have found capitalized words as entities.
	found := false
	for _, n := range result.Nodes {
		if n.Name == "Alice" || n.Name == "Bob" || n.Name == "Google" || n.Name == "Apollo" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find at least one entity, got %v", result.Nodes)
	}
}

func TestRenderReport(t *testing.T) {
	report := &GraphReport{
		TotalNodes: 10,
		TotalEdges: 15,
		Density:    0.33,
		TopNodes: []NodeStats{
			{ID: "n1", Name: "Alice", Degree: 5, InDegree: 3, OutDegree: 2},
		},
		TypeCounts:     map[string]int{"person": 5, "project": 5},
		EdgeTypeCounts: map[string]int{"knows": 10, "owns": 5},
	}

	// Test all formats.
	for _, format := range []string{"text", "markdown", "html"} {
		out := RenderReport(report, format)
		if len(out) == 0 {
			t.Fatalf("empty output for format %s", format)
		}
	}
}

func TestBuildDeduplication(t *testing.T) {
	store := &mockGraphStore{}
	results := []ExtractionResult{
		{
			DocumentID: "doc-1",
			Nodes: []ExtractionNode{
				{ID: "n1", Name: "Alice", Type: "person"},
			},
			Edges: []ExtractionEdge{
				{ID: "e1", FromNode: "n1", ToNode: "n2", Type: "knows", Weight: 1.0},
			},
		},
		{
			DocumentID: "doc-2",
			Nodes: []ExtractionNode{
				{ID: "n1", Name: "Alice", Type: "person"}, // duplicate
			},
			Edges: []ExtractionEdge{
				{ID: "e1", FromNode: "n1", ToNode: "n2", Type: "knows", Weight: 2.0}, // duplicate
			},
		},
	}

	err := Build(context.Background(), store, results, BuildOptions{Deduplicate: true})
	if err != nil {
		t.Fatal(err)
	}

	// With dedup, should only have 1 node and 1 edge.
	if store.nodeCount != 1 {
		t.Fatalf("expected 1 node with dedup, got %d", store.nodeCount)
	}
	if store.edgeCount != 1 {
		t.Fatalf("expected 1 edge with dedup, got %d", store.edgeCount)
	}
}

type mockGraphStore struct {
	nodeCount int
	edgeCount int
}

func (m *mockGraphStore) UpsertNode(node *GraphNode) error {
	m.nodeCount++
	return nil
}

func (m *mockGraphStore) GetNode(id string) (*GraphNode, error) {
	return nil, nil
}

func (m *mockGraphStore) UpsertEdge(edge *GraphEdge) error {
	m.edgeCount++
	return nil
}
