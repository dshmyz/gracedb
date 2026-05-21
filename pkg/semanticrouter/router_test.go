package semanticrouter

import (
	"context"
	"testing"
)

// mockEmbedder returns deterministic vectors for testing.
type mockEmbedder struct {
	cache map[string][]float32
}

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.cache == nil {
		m.cache = make(map[string][]float32)
	}
	if v, ok := m.cache[text]; ok {
		return v, nil
	}
	// Simple hash-based embedding for testing.
	vec := make([]float32, 8)
	for i := range vec {
		vec[i] = float32(hashText(text, i))
	}
	m.cache[text] = vec
	return vec, nil
}

func (m *mockEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	vectors := make([][]float32, len(texts))
	for i, t := range texts {
		v, err := m.Embed(ctx, t)
		if err != nil {
			return nil, err
		}
		vectors[i] = v
	}
	return vectors, nil
}

func (m *mockEmbedder) Dimension() int {
	return 8
}

func hashText(text string, pos int) float64 {
	h := 0.0
	for i, r := range text {
		h += float64(r) * float64(i+1) * float64(pos+1)
	}
	return h / (float64(len(text)) + 1)
}

func TestLexicalRouter(t *testing.T) {
	lr := NewLexicalRouter()
	lr.Add(&LexicalRoute{
		Name:       "greeting",
		Utterances: []string{"hello", "hi there", "good morning"},
		Threshold:  0.2,
	})
	lr.Add(&LexicalRoute{
		Name:       "search",
		Utterances: []string{"search for", "find", "look up"},
		Threshold:  0.2,
	})

	result, err := lr.Route(context.Background(), "hello world")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Matched || result.RouteName != "greeting" {
		t.Fatalf("expected greeting, got %s (matched=%v)", result.RouteName, result.Matched)
	}

	result2, err := lr.Route(context.Background(), "search for documents")
	if err != nil {
		t.Fatal(err)
	}
	if !result2.Matched || result2.RouteName != "search" {
		t.Fatalf("expected search, got %s", result2.RouteName)
	}
}

func TestRouterWithEmbedder(t *testing.T) {
	embedder := &mockEmbedder{}
	router := NewRouter(embedder, Config{Threshold: 0.5, TopK: 1})

	router.Add(&Route{
		Name:       "weather",
		Utterances: []string{"what is the weather", "temperature today"},
	})
	router.Add(&Route{
		Name:       "news",
		Utterances: []string{"latest news", "headlines today"},
	})

	result, err := router.Route(context.Background(), "what is the weather like")
	if err != nil {
		t.Fatal(err)
	}
	if !result.Matched || result.RouteName != "weather" {
		t.Fatalf("expected weather, got %s (matched=%v, score=%f)", result.RouteName, result.Matched, result.Score)
	}
}

func TestTokenize(t *testing.T) {
	tokens := tokenize("Hello, World! 123")
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestJaccard(t *testing.T) {
	a := []string{"hello", "world"}
	b := []string{"hello", "there"}
	score := jaccard(a, b)
	if score != 0.333 {
		t.Logf("jaccard score: %f", score)
	}

	identical := jaccard(a, a)
	if identical != 1.0 {
		t.Fatalf("expected 1.0 for identical sets, got %f", identical)
	}

	disjoint := jaccard([]string{"a"}, []string{"b"})
	if disjoint != 0 {
		t.Fatalf("expected 0 for disjoint sets, got %f", disjoint)
	}
}
