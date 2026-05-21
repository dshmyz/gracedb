package graph

import (
	"encoding/json"
	"time"
)

// GraphNode represents a node in the property graph.
type GraphNode struct {
	ID         string            `json:"id"`
	Type       string            `json:"type"`
	Labels     []string          `json:"labels,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
}

// GraphEdge represents a directed edge between two nodes.
type GraphEdge struct {
	ID         string            `json:"id"`
	FromNodeID string            `json:"from_node_id"`
	ToNodeID   string            `json:"to_node_id"`
	Type       string            `json:"type"`
	Weight     float64           `json:"weight,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
	CreatedAt  time.Time         `json:"created_at,omitempty"`
}

// PathResult represents a path between two nodes.
type PathResult struct {
	FromNodeID string
	ToNodeID   string
	NodeIDs    []string
	EdgeIDs    []string
	Length     int
}

// TraversalResult represents the result of a graph traversal.
type TraversalResult struct {
	Nodes []*GraphNode
	Edges []*GraphEdge
}

// NeighborOptions controls neighbor queries.
type NeighborOptions struct {
	Direction string // "out", "in", "both"
	EdgeTypes []string
	NodeTypes []string
	Limit     int
	MaxDepth  int
}

// Marshal serializes a node to JSON.
func (n *GraphNode) Marshal() ([]byte, error) {
	return json.Marshal(n)
}

// UnmarshalNode deserializes a node from JSON.
func UnmarshalNode(data []byte) (*GraphNode, error) {
	var n GraphNode
	err := json.Unmarshal(data, &n)
	return &n, err
}

// Marshal serializes an edge to JSON.
func (e *GraphEdge) Marshal() ([]byte, error) {
	return json.Marshal(e)
}

// UnmarshalEdge deserializes an edge from JSON.
func UnmarshalEdge(data []byte) (*GraphEdge, error) {
	var e GraphEdge
	err := json.Unmarshal(data, &e)
	return &e, err
}
