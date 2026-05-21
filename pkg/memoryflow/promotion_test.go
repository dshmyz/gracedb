package memoryflow

import (
	"context"
	"testing"
)

func TestHeuristicExtractor(t *testing.T) {
	extractor := HeuristicExtractor{}

	transcript := Transcript{
		SessionID: "s1",
		Turns: []TranscriptTurn{
			{Role: "user", Content: "I prefer dark mode for the interface."},
			{Role: "assistant", Content: "Got it. What else?"},
			{Role: "user", Content: "I decided to launch the project on Friday."},
			{Role: "user", Content: "There is a bug in the authentication module."},
		},
	}

	candidates, err := extractor.Extract(context.Background(), transcript, SessionState{})
	if err != nil {
		t.Fatal(err)
	}

	if len(candidates) < 3 {
		t.Fatalf("expected at least 3 candidates, got %d", len(candidates))
	}

	// Check kinds.
	kinds := make(map[PromotionKind]bool)
	for _, c := range candidates {
		kinds[c.Kind] = true
	}

	if !kinds[PromotionKindPreference] {
		t.Fatal("expected preference candidate")
	}
	if !kinds[PromotionKindDecision] {
		t.Fatal("expected decision candidate")
	}
	if !kinds[PromotionKindProblem] {
		t.Fatal("expected problem candidate")
	}
}

func TestDefaultPromotionPolicy(t *testing.T) {
	policy := DefaultPromotionPolicy{}

	candidates := []PromotionCandidate{
		{Kind: PromotionKindDecision, Content: "Decided to ship on Friday."},
		{Kind: PromotionKindPreference, Content: "Prefers dark mode."},
		{Kind: PromotionKindNote, Content: "Just a note."},
	}

	selected, err := policy.Select(context.Background(), Transcript{}, SessionState{}, candidates)
	if err != nil {
		t.Fatal(err)
	}

	// Should filter out notes.
	if len(selected) != 2 {
		t.Fatalf("expected 2 candidates (decision + preference), got %d", len(selected))
	}
}

func TestClassifySentence(t *testing.T) {
	tests := []struct {
		sentence string
		expected PromotionKind
		matched  bool
	}{
		{"I prefer dark mode", PromotionKindPreference, true},
		{"I decided to launch on Friday", PromotionKindDecision, true},
		{"We shipped the new feature", PromotionKindMilestone, true},
		{"There is a bug in auth", PromotionKindProblem, true},
		{"The weather is nice today", PromotionKindNote, false},
	}

	for _, tc := range tests {
		kind, ok := classifySentence(tc.sentence)
		if ok != tc.matched {
			t.Errorf("classifySentence(%q) matched=%v, want %v", tc.sentence, ok, tc.matched)
		}
		if ok && kind != tc.expected {
			t.Errorf("classifySentence(%q) = %s, want %s", tc.sentence, kind, tc.expected)
		}
	}
}

func TestCompactTitle(t *testing.T) {
	title := compactTitle("This is a very long sentence that should be truncated")
	if len(title) > 50 {
		t.Fatalf("title too long: %q", title)
	}
}

func TestPassthroughRecallStrategy(t *testing.T) {
	strategy := PassthroughRecallStrategy{}

	req := RecallRequest{
		Query: "test query",
	}

	called := false
	next := func(ctx context.Context, r RecallRequest) (*RecallResponse, error) {
		called = true
		if r.Query != req.Query {
			t.Fatalf("query mismatch: got %q, want %q", r.Query, req.Query)
		}
		return &RecallResponse{}, nil
	}

	_, err := strategy.Recall(context.Background(), req, next)
	if err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("next function was not called")
	}
}
