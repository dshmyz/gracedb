package graphflow

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

// Confidence level for extraction edges.
const (
	ConfidenceExtracted = "extracted"
	ConfidenceInferred  = "inferred"
)

// SourceDocument is the input to the extraction pipeline.
type SourceDocument struct {
	ID       string
	Title    string
	Content  string
	Path     string
	Type     string
	Metadata map[string]string
}

// JSONGenerator generates structured JSON from text via an LLM.
type JSONGenerator interface {
	GenerateJSON(ctx context.Context, systemPrompt string, userPrompt string) ([]byte, error)
}

// LLMExtractor uses an LLM to extract entities and relations.
type LLMExtractor struct {
	Generator JSONGenerator
}

// Extract uses the LLM to extract entities and relations from a document.
func (e *LLMExtractor) Extract(ctx context.Context, doc SourceDocument) (*ExtractionResult, error) {
	if strings.TrimSpace(doc.ID) == "" {
		return nil, fmt.Errorf("source document id is required")
	}

	systemPrompt := `You are an entity and relation extraction system. Analyze the given text and extract:
1. Named entities (people, organizations, products, projects, locations)
2. Relations between entities (owns, works_for, created, manages, etc.)

Return valid JSON in this exact format:
{
  "entities": [{"name": "...", "type": "person|org|product|project|location"}],
  "relations": [{"from": "...", "to": "...", "type": "owns|works_for|created|manages"}]
}`

	userPrompt := fmt.Sprintf("Extract entities and relations from this document:\n\n%s", doc.Content)

	data, err := e.Generator.GenerateJSON(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, fmt.Errorf("LLM extraction failed: %w", err)
	}

	var llmResult struct {
		Entities  []struct{ Name, Type string } `json:"entities"`
		Relations []struct{ From, To, Type string } `json:"relations"`
	}
	if err := json.Unmarshal(data, &llmResult); err != nil {
		return nil, fmt.Errorf("parse LLM output: %w", err)
	}

	docNodeID := "doc:" + doc.ID
	var nodes []ExtractionNode
	var edges []ExtractionEdge

	// Document node.
	nodes = append(nodes, ExtractionNode{
		ID:          docNodeID,
		Name:        firstNonEmpty(doc.Title, doc.ID),
		Type:        "document",
		Description: summarize(doc.Content, 140),
		SourceDocID: doc.ID,
	})

	// Entity nodes and mention edges.
	for _, ent := range llmResult.Entities {
		if ent.Name == "" {
			continue
		}
		entityID := "entity:" + normalizeLabel(ent.Name)
		entType := "entity"
		if ent.Type != "" {
			entType = ent.Type
		}
		nodes = append(nodes, ExtractionNode{
			ID:          entityID,
			Name:        ent.Name,
			Type:        entType,
			Description: ent.Type,
			SourceDocID: doc.ID,
			Mentions:    1,
		})
		edges = append(edges, ExtractionEdge{
			ID:       "edge:" + docNodeID + "->" + entityID,
			FromNode: docNodeID,
			ToNode:   entityID,
			Type:     "mentions",
			Weight:   1.0,
		})
	}

	// Relation edges.
	for _, rel := range llmResult.Relations {
		if rel.From == "" || rel.To == "" {
			continue
		}
		edges = append(edges, ExtractionEdge{
			ID:       "edge:" + normalizeLabel(rel.From) + "->" + normalizeLabel(rel.To),
			FromNode: "entity:" + normalizeLabel(rel.From),
			ToNode:   "entity:" + normalizeLabel(rel.To),
			Type:     firstNonEmpty(rel.Type, "related_to"),
			Weight:   1.0,
		})
	}

	result := &ExtractionResult{
		DocumentID: doc.ID,
		Nodes:      nodes,
		Edges:      dedupeEdges(edges),
	}
	if err := ValidateExtraction(result); err != nil {
		return nil, err
	}
	return result, nil
}

// ValidateExtraction validates an extraction result.
func ValidateExtraction(result *ExtractionResult) error {
	if result.DocumentID == "" {
		return fmt.Errorf("document_id is required")
	}
	if len(result.Nodes) == 0 {
		return fmt.Errorf("at least one node is required")
	}
	return nil
}

// DetectSourceFiles scans a directory for extractable documents.
func DetectSourceFiles(dir string) ([]SourceDocument, error) {
	// Simple: scan for .txt, .md files.
	var docs []SourceDocument
	// Implementation would use os.ReadDir.
	_ = dir
	return docs, nil
}

// SourceDocumentType classifies document types.
type SourceDocumentType string

const (
	DocTypeText     SourceDocumentType = "text"
	DocTypeMarkdown SourceDocumentType = "markdown"
	DocTypeCode     SourceDocumentType = "code"
)

// ClassifyDocument determines the document type from its path.
func ClassifyDocument(path string) SourceDocumentType {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".md") || strings.HasSuffix(lower, ".markdown"):
		return DocTypeMarkdown
	case strings.HasSuffix(lower, ".go") || strings.HasSuffix(lower, ".py") || strings.HasSuffix(lower, ".js"):
		return DocTypeCode
	default:
		return DocTypeText
	}
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func summarize(text string, max int) string {
	text = strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
	if len(text) <= max {
		return text
	}
	return text[:max] + "..."
}

func normalizeLabel(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	value = strings.ReplaceAll(value, " ", "_")
	value = strings.ReplaceAll(value, "/", "_")
	value = strings.ReplaceAll(value, ":", "_")
	return value
}

var entityPattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9_]{1,}\b`)
var backtickPattern = regexp.MustCompile("`([^`]+)`")

func extractEntities(text string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0)
	for _, match := range entityPattern.FindAllString(text, -1) {
		if _, ok := seen[match]; ok {
			continue
		}
		seen[match] = struct{}{}
		out = append(out, match)
	}
	for _, groups := range backtickPattern.FindAllStringSubmatch(text, -1) {
		if len(groups) < 2 {
			continue
		}
		value := strings.TrimSpace(groups[1])
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func dedupeEdges(edges []ExtractionEdge) []ExtractionEdge {
	seen := make(map[string]struct{}, len(edges))
	out := make([]ExtractionEdge, 0, len(edges))
	for _, edge := range edges {
		key := edge.FromNode + "|" + edge.Type + "|" + edge.ToNode
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, edge)
	}
	return out
}
