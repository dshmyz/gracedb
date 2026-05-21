package graph

import (
	"os"
	"testing"

	"github.com/dgraph-io/badger/v4"
)

func newTestGraphStore(t *testing.T) *GraphStore {
	dir, err := os.MkdirTemp("", "graph-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	db, err := badger.Open(badger.DefaultOptions(dir).WithInMemory(false))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	return NewGraphStore(db)
}

func TestUpsertAndGetNode(t *testing.T) {
	g := newTestGraphStore(t)

	node := &GraphNode{
		ID:   "node-1",
		Type: "person",
		Properties: map[string]string{
			"name": "Alice",
		},
	}

	if err := g.UpsertNode(node); err != nil {
		t.Fatal(err)
	}

	got, err := g.GetNode("node-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.Type != "person" {
		t.Fatalf("expected type 'person', got %q", got.Type)
	}
	if got.Properties["name"] != "Alice" {
		t.Fatalf("expected name 'Alice', got %q", got.Properties["name"])
	}
}

func TestUpsertAndGetEdge(t *testing.T) {
	g := newTestGraphStore(t)

	_ = g.UpsertNode(&GraphNode{ID: "a", Type: "person"})
	_ = g.UpsertNode(&GraphNode{ID: "b", Type: "person"})

	edge := &GraphEdge{
		ID:         "edge-1",
		FromNodeID: "a",
		ToNodeID:   "b",
		Type:       "knows",
		Weight:     1.0,
	}

	if err := g.UpsertEdge(edge); err != nil {
		t.Fatal(err)
	}

	got, err := g.GetEdge("edge-1")
	if err != nil {
		t.Fatal(err)
	}
	if got.FromNodeID != "a" {
		t.Fatalf("expected from 'a', got %q", got.FromNodeID)
	}
	if got.ToNodeID != "b" {
		t.Fatalf("expected to 'b', got %q", got.ToNodeID)
	}
}

func TestGetNeighbors(t *testing.T) {
	g := newTestGraphStore(t)

	_ = g.UpsertNode(&GraphNode{ID: "center", Type: "hub"})
	_ = g.UpsertNode(&GraphNode{ID: "n1", Type: "leaf"})
	_ = g.UpsertNode(&GraphNode{ID: "n2", Type: "leaf"})

	_ = g.UpsertEdge(&GraphEdge{ID: "e1", FromNodeID: "center", ToNodeID: "n1", Type: "connected"})
	_ = g.UpsertEdge(&GraphEdge{ID: "e2", FromNodeID: "center", ToNodeID: "n2", Type: "connected"})

	nodes, edges, err := g.GetNeighbors("center", NeighborOptions{Direction: "out"})
	if err != nil {
		t.Fatal(err)
	}
	if len(nodes) != 2 {
		t.Fatalf("expected 2 neighbors, got %d", len(nodes))
	}
	if len(edges) != 2 {
		t.Fatalf("expected 2 edges, got %d", len(edges))
	}
}

func TestDeleteNode(t *testing.T) {
	g := newTestGraphStore(t)

	_ = g.UpsertNode(&GraphNode{ID: "to-del", Type: "temp"})
	_ = g.UpsertNode(&GraphNode{ID: "other", Type: "temp"})
	_ = g.UpsertEdge(&GraphEdge{ID: "e-del", FromNodeID: "to-del", ToNodeID: "other", Type: "x"})

	if err := g.DeleteNode("to-del"); err != nil {
		t.Fatal(err)
	}

	_, err := g.GetNode("to-del")
	if err == nil {
		t.Fatal("expected node to be deleted")
	}

	// Edge should be deleted (from-prefix cleanup).
	_, err = g.GetEdge("e-del")
	if err == nil {
		t.Fatal("expected edge to be deleted with node")
	}
}

func TestBFS(t *testing.T) {
	g := newTestGraphStore(t)

	// Create a chain: A -> B -> C -> D
	_ = g.UpsertNode(&GraphNode{ID: "A"})
	_ = g.UpsertNode(&GraphNode{ID: "B"})
	_ = g.UpsertNode(&GraphNode{ID: "C"})
	_ = g.UpsertNode(&GraphNode{ID: "D"})

	_ = g.UpsertEdge(&GraphEdge{ID: "ab", FromNodeID: "A", ToNodeID: "B", Type: "next"})
	_ = g.UpsertEdge(&GraphEdge{ID: "bc", FromNodeID: "B", ToNodeID: "C", Type: "next"})
	_ = g.UpsertEdge(&GraphEdge{ID: "cd", FromNodeID: "C", ToNodeID: "D", Type: "next"})

	result, err := g.BFS("A", NeighborOptions{MaxDepth: 2})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Nodes) < 2 {
		t.Fatalf("expected at least 2 nodes from BFS depth=2, got %d", len(result.Nodes))
	}
}

func TestShortestPath(t *testing.T) {
	g := newTestGraphStore(t)

	_ = g.UpsertNode(&GraphNode{ID: "start"})
	_ = g.UpsertNode(&GraphNode{ID: "mid"})
	_ = g.UpsertNode(&GraphNode{ID: "end"})

	_ = g.UpsertEdge(&GraphEdge{ID: "s-m", FromNodeID: "start", ToNodeID: "mid", Type: "x"})
	_ = g.UpsertEdge(&GraphEdge{ID: "m-e", FromNodeID: "mid", ToNodeID: "end", Type: "x"})

	path, err := g.ShortestPath("start", "end")
	if err != nil {
		t.Fatal(err)
	}
	if path == nil {
		t.Fatal("expected a path")
	}
	if path.Length != 2 {
		t.Fatalf("expected path length 2, got %d", path.Length)
	}
	if len(path.NodeIDs) != 3 {
		t.Fatalf("expected 3 nodes in path, got %d", len(path.NodeIDs))
	}
}
