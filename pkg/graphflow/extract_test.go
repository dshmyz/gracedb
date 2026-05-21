package graphflow

import (
	"context"
	"testing"
)

func TestLLMExtractor(t *testing.T) {
	// Test with mock generator.
	extractor := &LLMExtractor{
		Generator: &mockGenerator{
			output: `{
				"entities": [
					{"name": "Alice", "type": "person"},
					{"name": "CortexDB", "type": "project"}
				],
				"relations": [
					{"from": "Alice", "to": "CortexDB", "type": "created"}
				]
			}`,
		},
	}

	doc := SourceDocument{
		ID:      "test-1",
		Title:   "Test Document",
		Content: "Alice created the CortexDB project.",
	}

	result, err := extractor.Extract(context.Background(), doc)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Nodes) < 3 {
		t.Fatalf("expected at least 3 nodes (doc + 2 entities), got %d", len(result.Nodes))
	}

	// Check entity nodes exist.
	found := false
	for _, n := range result.Nodes {
		if n.Name == "Alice" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Alice entity node")
	}

	// Check relation edge exists.
	if len(result.Edges) < 1 {
		t.Fatal("expected at least 1 relation edge")
	}
}

func TestExtractEntities(t *testing.T) {
	text := "Alice works on the CortexDB project at Google. `OpenAI` is also involved."
	entities := extractEntities(text)

	// Should find capitalized words and backtick entities.
	found := false
	for _, e := range entities {
		if e == "Alice" || e == "CortexDB" || e == "Google" || e == "OpenAI" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected to find at least one entity, got %v", entities)
	}
}

func TestClassifyDocument(t *testing.T) {
	tests := []struct {
		path     string
		expected SourceDocumentType
	}{
		{"doc.txt", DocTypeText},
		{"readme.md", DocTypeMarkdown},
		{"main.go", DocTypeCode},
		{"script.py", DocTypeCode},
	}

	for _, tc := range tests {
		got := ClassifyDocument(tc.path)
		if got != tc.expected {
			t.Errorf("ClassifyDocument(%q) = %s, want %s", tc.path, got, tc.expected)
		}
	}
}

func TestDedupeEdges(t *testing.T) {
	edges := []ExtractionEdge{
		{FromNode: "a", ToNode: "b", Type: "knows"},
		{FromNode: "a", ToNode: "b", Type: "knows"}, // duplicate
		{FromNode: "b", ToNode: "c", Type: "knows"},
	}

	deduped := dedupeEdges(edges)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 edges after dedup, got %d", len(deduped))
	}
}

func TestValidateExtraction(t *testing.T) {
	// Valid extraction.
	valid := &ExtractionResult{
		DocumentID: "doc-1",
		Nodes:      []ExtractionNode{{ID: "n1"}},
	}
	if err := ValidateExtraction(valid); err != nil {
		t.Fatalf("valid extraction should pass: %v", err)
	}

	// Missing document ID.
	invalid := &ExtractionResult{}
	if err := ValidateExtraction(invalid); err == nil {
		t.Fatal("invalid extraction should fail")
	}
}

type mockGenerator struct {
	output string
}

func (m *mockGenerator) GenerateJSON(ctx context.Context, systemPrompt string, userPrompt string) ([]byte, error) {
	return []byte(m.output), nil
}
