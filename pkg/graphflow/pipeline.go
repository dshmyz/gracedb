// Package graphflow provides corpus-to-graph workflow: extraction, build,
// analyze, and export.
package graphflow

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

// ExtractionNode is an entity extracted from text.
type ExtractionNode struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Type        string   `json:"type"`
	Description string   `json:"description,omitempty"`
	SourceDocID string   `json:"source_doc_id,omitempty"`
	ChunkIDs    []string `json:"chunk_ids,omitempty"`
	Mentions    int      `json:"mentions"`
}

// ExtractionEdge is a relation extracted from text.
type ExtractionEdge struct {
	ID       string  `json:"id"`
	FromNode string  `json:"from_node"`
	ToNode   string  `json:"to_node"`
	Type     string  `json:"type"`
	Weight   float64 `json:"weight"`
}

// ExtractionResult holds all extractions from a document.
type ExtractionResult struct {
	DocumentID string            `json:"document_id"`
	Nodes      []ExtractionNode  `json:"nodes"`
	Edges      []ExtractionEdge  `json:"edges"`
	Metadata   map[string]string `json:"metadata,omitempty"`
}

// BuildOptions controls the build pipeline.
type BuildOptions struct {
	Deduplicate bool
	MinWeight   float64
}

// AnalyzeRequest controls graph analysis.
type AnalyzeRequest struct {
	TopN       int
	MinDegree  int
	IncludeIsolated bool
}

// GraphReport is the analysis output.
type GraphReport struct {
	TotalNodes    int              `json:"total_nodes"`
	TotalEdges    int              `json:"total_edges"`
	TopNodes      []NodeStats      `json:"top_nodes"`
	TypeCounts    map[string]int   `json:"type_counts"`
	EdgeTypeCounts map[string]int  `json:"edge_type_counts"`
	IsolatedNodes []string         `json:"isolated_nodes"`
	Density       float64          `json:"density"`
}

// NodeStats holds per-node statistics.
type NodeStats struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Degree      int    `json:"degree"`
	InDegree    int    `json:"in_degree"`
	OutDegree   int    `json:"out_degree"`
}

// ExportRequest controls export behavior.
type ExportRequest struct {
	OutputDir   string
	Analysis    *GraphReport
	Format      string // "json", "markdown", "html"
}

// ExportResult holds export output.
type ExportResult struct {
	Files []string `json:"files"`
}

// GraphStore abstracts the graph operations.
type GraphStore interface {
	UpsertNode(node *GraphNode) error
	GetNode(id string) (*GraphNode, error)
	UpsertEdge(edge *GraphEdge) error
}

// GraphNode represents a node to store.
type GraphNode struct {
	ID         string
	Type       string
	Properties map[string]string
}

// GraphEdge represents an edge to store.
type GraphEdge struct {
	ID         string
	FromNodeID string
	ToNodeID   string
	Type       string
	Weight     float64
	Properties map[string]string
}

// Build adds extraction results to the graph.
func Build(ctx context.Context, store GraphStore, results []ExtractionResult, opts BuildOptions) error {
	seenNodes := make(map[string]bool)
	seenEdges := make(map[string]bool)

	for _, result := range results {
		for _, node := range result.Nodes {
			if opts.Deduplicate && seenNodes[node.ID] {
				continue
			}
			seenNodes[node.ID] = true

			err := store.UpsertNode(&GraphNode{
				ID:   node.ID,
				Type: node.Type,
				Properties: map[string]string{
					"name":        node.Name,
					"description": node.Description,
					"source_doc":  result.DocumentID,
					"mentions":    fmt.Sprintf("%d", node.Mentions),
				},
			})
			if err != nil {
				return fmt.Errorf("upsert node %s: %w", node.ID, err)
			}
		}

		for _, edge := range result.Edges {
			if opts.Deduplicate && seenEdges[edge.ID] {
				continue
			}
			if opts.MinWeight > 0 && edge.Weight < opts.MinWeight {
				continue
			}
			seenEdges[edge.ID] = true

			err := store.UpsertEdge(&GraphEdge{
				ID:         edge.ID,
				FromNodeID: edge.FromNode,
				ToNodeID:   edge.ToNode,
				Type:       edge.Type,
				Weight:     edge.Weight,
				Properties: map[string]string{
					"source_doc": result.DocumentID,
				},
			})
			if err != nil {
				return fmt.Errorf("upsert edge %s: %w", edge.ID, err)
			}
		}
	}

	return nil
}

// Analyze computes graph statistics.
func Analyze(ctx context.Context, results []ExtractionResult, req AnalyzeRequest) (*GraphReport, error) {
	if req.TopN <= 0 {
		req.TopN = 10
	}

	nodeDegree := make(map[string]int)
	nodeName := make(map[string]string)
	inDegree := make(map[string]int)
	outDegree := make(map[string]int)
	typeCounts := make(map[string]int)
	edgeTypeCounts := make(map[string]int)
	edgeCount := 0

	for _, result := range results {
		for _, node := range result.Nodes {
			nodeName[node.ID] = node.Name
			typeCounts[node.Type]++
			if _, ok := nodeDegree[node.ID]; !ok {
				nodeDegree[node.ID] = 0
			}
		}

		for _, edge := range result.Edges {
			nodeDegree[edge.FromNode]++
			nodeDegree[edge.ToNode]++
			outDegree[edge.FromNode]++
			inDegree[edge.ToNode]++
			edgeTypeCounts[edge.Type]++
			edgeCount++
		}
	}

	// Top nodes by degree.
	type scoredNode struct {
		id   string
		name string
		deg  int
	}
	var scored []scoredNode
	for id, deg := range nodeDegree {
		scored = append(scored, scoredNode{id, nodeName[id], deg})
	}
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].deg > scored[j].deg
	})
	if len(scored) > req.TopN {
		scored = scored[:req.TopN]
	}

	topNodes := make([]NodeStats, len(scored))
	for i, sn := range scored {
		topNodes[i] = NodeStats{
			ID:        sn.id,
			Name:      sn.name,
			Degree:    sn.deg,
			InDegree:  inDegree[sn.id],
			OutDegree: outDegree[sn.id],
		}
	}

	// Find isolated nodes.
	var isolated []string
	if req.IncludeIsolated {
		for id, deg := range nodeDegree {
			if deg == 0 {
				isolated = append(isolated, id)
			}
		}
	}

	// Density = 2E / (N * (N-1))
	n := len(nodeDegree)
	density := 0.0
	if n > 1 {
		density = float64(2*edgeCount) / float64(n*(n-1))
	}

	return &GraphReport{
		TotalNodes:     n,
		TotalEdges:     edgeCount,
		TopNodes:       topNodes,
		TypeCounts:     typeCounts,
		EdgeTypeCounts: edgeTypeCounts,
		IsolatedNodes:  isolated,
		Density:        density,
	}, nil
}

// RenderReport formats the analysis as text.
func RenderReport(report *GraphReport, format string) string {
	switch format {
	case "markdown":
		return renderMarkdown(report)
	case "html":
		return renderHTML(report)
	default:
		return renderText(report)
	}
}

func renderText(r *GraphReport) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("Graph Analysis Report\n"))
	b.WriteString(fmt.Sprintf("========================\n"))
	b.WriteString(fmt.Sprintf("Nodes: %d, Edges: %d\n", r.TotalNodes, r.TotalEdges))
	b.WriteString(fmt.Sprintf("Density: %.4f\n\n", r.Density))

	b.WriteString("Top Nodes:\n")
	for _, n := range r.TopNodes {
		b.WriteString(fmt.Sprintf("  %s (%s): degree=%d, in=%d, out=%d\n", n.Name, n.ID, n.Degree, n.InDegree, n.OutDegree))
	}

	b.WriteString("\nNode Types:\n")
	for t, c := range r.TypeCounts {
		b.WriteString(fmt.Sprintf("  %s: %d\n", t, c))
	}

	b.WriteString("\nEdge Types:\n")
	for t, c := range r.EdgeTypeCounts {
		b.WriteString(fmt.Sprintf("  %s: %d\n", t, c))
	}

	return b.String()
}

func renderMarkdown(r *GraphReport) string {
	var b strings.Builder
	b.WriteString("# Graph Analysis Report\n\n")
	b.WriteString(fmt.Sprintf("| Metric | Value |\n|---|---|\n"))
	b.WriteString(fmt.Sprintf("| Nodes | %d |\n", r.TotalNodes))
	b.WriteString(fmt.Sprintf("| Edges | %d |\n", r.TotalEdges))
	b.WriteString(fmt.Sprintf("| Density | %.4f |\n\n", r.Density))

	b.WriteString("## Top Nodes\n\n")
	b.WriteString("| Name | ID | Degree | In | Out |\n|---|---|---|---|---|\n")
	for _, n := range r.TopNodes {
		b.WriteString(fmt.Sprintf("| %s | %s | %d | %d | %d |\n", n.Name, n.ID, n.Degree, n.InDegree, n.OutDegree))
	}

	return b.String()
}

func renderHTML(r *GraphReport) string {
	var b strings.Builder
	b.WriteString("<html><head><title>Graph Analysis Report</title></head><body>\n")
	b.WriteString("<h1>Graph Analysis Report</h1>\n")
	b.WriteString(fmt.Sprintf("<p>Nodes: %d, Edges: %d, Density: %.4f</p>\n", r.TotalNodes, r.TotalEdges, r.Density))

	b.WriteString("<h2>Top Nodes</h2>\n<table><tr><th>Name</th><th>ID</th><th>Degree</th></tr>\n")
	for _, n := range r.TopNodes {
		b.WriteString(fmt.Sprintf("<tr><td>%s</td><td>%s</td><td>%d</td></tr>\n", n.Name, n.ID, n.Degree))
	}
	b.WriteString("</table>\n</body></html>\n")

	return b.String()
}

// HeuristicExtractor is a simple rule-based extractor.
type HeuristicExtractor struct{}

// Extract extracts entities and relations using simple heuristics.
func (e *HeuristicExtractor) Extract(text, docID string) ExtractionResult {
	words := strings.Fields(text)
	nodeMap := make(map[string]*ExtractionNode)
	edgeMap := make(map[string]*ExtractionEdge)

	// Simple: treat capitalized words as entities.
	for _, w := range words {
		if len(w) > 2 && w[0] >= 'A' && w[0] <= 'Z' {
			clean := strings.Trim(w, ".,!?;:\"'()[]{}")
			if len(clean) < 2 {
				continue
			}
			if _, ok := nodeMap[clean]; !ok {
				nodeMap[clean] = &ExtractionNode{
					ID:   "node:" + strings.ToLower(clean),
					Name: clean,
					Type: "entity",
				}
			}
			nodeMap[clean].Mentions++
		}
	}

	// Simple: create edges between consecutive entities.
	var prevNode *ExtractionNode
	for _, w := range words {
		clean := strings.Trim(w, ".,!?;:\"'()[]{}")
		if node, ok := nodeMap[clean]; ok {
			if prevNode != nil && prevNode.ID != node.ID {
				edgeKey := prevNode.ID + "->" + node.ID
				if _, ok := edgeMap[edgeKey]; !ok {
					edgeMap[edgeKey] = &ExtractionEdge{
						ID:       "edge:" + edgeKey,
						FromNode: prevNode.ID,
						ToNode:   node.ID,
						Type:     "co-occurrence",
						Weight:   1.0,
					}
				} else {
					edgeMap[edgeKey].Weight++
				}
			}
			prevNode = node
		} else {
			prevNode = nil
		}
	}

	nodes := make([]ExtractionNode, 0, len(nodeMap))
	for _, n := range nodeMap {
		n.SourceDocID = docID
		nodes = append(nodes, *n)
	}

	edges := make([]ExtractionEdge, 0, len(edgeMap))
	for _, e := range edgeMap {
		edges = append(edges, *e)
	}

	return ExtractionResult{
		DocumentID: docID,
		Nodes:      nodes,
		Edges:      edges,
	}
}
