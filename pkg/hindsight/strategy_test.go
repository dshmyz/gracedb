package hindsight

import (
	"context"
	"testing"

	"github.com/dshmyz/gracedb/pkg/memoryflow"
)

func TestStrategyEnrichesRequest(t *testing.T) {
	strategy := NewStrategy(StrategyOptions{
		BankID:      "my-project",
		Keywords:    []string{"deadline", "release"},
		EntityNames: []string{"Apollo"},
		UseKG:       true,
	})

	req := memoryflow.RecallRequest{
		Query: "What is the status?",
		State: memoryflow.SessionState{
			Tags:      []string{"urgent"},
			Namespace: "assistant",
		},
	}

	called := false
	next := func(ctx context.Context, r memoryflow.RecallRequest) (*memoryflow.RecallResponse, error) {
		called = true

		// Check keywords were enriched.
		if r.Plan == nil {
			t.Fatal("expected plan to be set")
		}

		// Check entity names were added.
		found := false
		for _, e := range r.Plan.EntityNames {
			if e == "Apollo" {
				found = true
				break
			}
		}
		if !found {
			t.Fatal("expected Apollo entity in plan")
		}

		// Check retrieval mode was set to graph.
		if r.Plan.RetrievalMode != "graph" {
			t.Fatalf("expected retrieval mode 'graph', got %q", r.Plan.RetrievalMode)
		}

		return &memoryflow.RecallResponse{}, nil
	}

	_, err := strategy.Recall(context.Background(), req, next)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("next function was not called")
	}
}

func TestSanitizeBankID(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"my-project", "my-project"},
		{"My Project", "my-project"},
		{"", "default"},
	}

	for _, tc := range tests {
		got := sanitizeBankID(tc.input)
		if got != tc.expected {
			t.Errorf("sanitizeBankID(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestMergeStrings(t *testing.T) {
	a := []string{"a", "b", "c"}
	b := []string{"b", "c", "d"}

	merged := mergeStrings(a, b)
	if len(merged) != 4 {
		t.Fatalf("expected 4 unique strings, got %d: %v", len(merged), merged)
	}

	// Check order preserved from a, then b.
	if merged[0] != "a" || merged[1] != "b" || merged[2] != "c" || merged[3] != "d" {
		t.Fatalf("unexpected order: %v", merged)
	}
}

func TestHindsightMemoryNamespace(t *testing.T) {
	ns := hindsightMemoryNamespace("my-bank")
	if ns != "hindsight:my-bank" {
		t.Fatalf("expected 'hindsight:my-bank', got %q", ns)
	}
}
