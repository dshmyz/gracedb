package memoryflow

import (
	"context"
	"regexp"
	"strings"
)

// DefaultPromotionPolicy keeps high-signal categories and drops generic notes.
type DefaultPromotionPolicy struct{}

// Select filters promotion candidates, keeping decisions, preferences, milestones, and problems.
func (DefaultPromotionPolicy) Select(_ context.Context, _ Transcript, _ SessionState, candidates []PromotionCandidate) ([]PromotionCandidate, error) {
	out := make([]PromotionCandidate, 0, len(candidates))
	for _, candidate := range candidates {
		kind := normalizePromotionKind(candidate)
		switch kind {
		case PromotionKindDecision, PromotionKindPreference, PromotionKindMilestone, PromotionKindProblem:
			candidate.Kind = kind
			out = append(out, candidate)
		}
	}
	return out, nil
}

// HeuristicExtractor extracts durable facts from transcripts without an LLM.
type HeuristicExtractor struct{}

// Extract scans transcript text for preference/decision/milestone/problem signals.
func (HeuristicExtractor) Extract(_ context.Context, transcript Transcript, state SessionState) ([]PromotionCandidate, error) {
	out := make([]PromotionCandidate, 0)
	seen := make(map[string]struct{})

	for _, turn := range transcript.Turns {
		sentences := splitSentence(turn.Content)
		for _, sentence := range sentences {
			kind, ok := classifySentence(sentence)
			if !ok {
				continue
			}
			key := strings.ToLower(strings.TrimSpace(sentence)) + "|" + string(kind)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}

			entities := extractEntityNames(sentence)
			metadata := map[string]string{"kind": string(kind)}
			if state.Namespace != "" {
				metadata["namespace"] = state.Namespace
			}
			if transcript.Source != "" {
				metadata["source"] = transcript.Source
			}
			if turn.Role != "" {
				metadata["role"] = turn.Role
			}

			out = append(out, PromotionCandidate{
				Kind:     kind,
				Title:    compactTitle(sentence),
				Content:  sentence,
				Metadata: metadata,
			})
			_ = entities
		}
	}
	return out, nil
}

func normalizePromotionKind(candidate PromotionCandidate) PromotionKind {
	if candidate.Kind != "" {
		return candidate.Kind
	}
	if candidate.Metadata == nil {
		return PromotionKindNote
	}
	switch PromotionKind(strings.ToLower(strings.TrimSpace(candidate.Metadata["kind"]))) {
	case PromotionKindDecision, PromotionKindPreference, PromotionKindMilestone, PromotionKindProblem:
		return PromotionKind(candidate.Metadata["kind"])
	default:
		return PromotionKindNote
	}
}

func splitSentence(text string) []string {
	text = strings.ReplaceAll(text, "\n", " ")
	raw := strings.FieldsFunc(text, func(r rune) bool {
		return r == '.' || r == '!' || r == '?'
	})
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func compactTitle(text string) string {
	words := strings.Fields(strings.TrimSpace(text))
	if len(words) == 0 {
		return ""
	}
	if len(words) > 8 {
		words = words[:8]
	}
	return strings.Join(words, " ")
}

func classifySentence(sentence string) (PromotionKind, bool) {
	lower := strings.ToLower(strings.TrimSpace(sentence))
	switch {
	case strings.Contains(lower, "prefer"),
		strings.Contains(lower, "likes "),
		strings.Contains(lower, "dislikes "),
		strings.Contains(lower, "wants "),
		strings.Contains(lower, "prefers "):
		return PromotionKindPreference, true
	case strings.Contains(lower, "decided"),
		strings.Contains(lower, "decision"),
		strings.Contains(lower, "deadline"),
		strings.Contains(lower, "ship "),
		strings.Contains(lower, "launch "),
		strings.Contains(lower, "will "):
		return PromotionKindDecision, true
	case strings.Contains(lower, "shipped"),
		strings.Contains(lower, "launched"),
		strings.Contains(lower, "released"),
		strings.Contains(lower, "milestone"),
		strings.Contains(lower, "completed"):
		return PromotionKindMilestone, true
	case strings.Contains(lower, "bug"),
		strings.Contains(lower, "issue"),
		strings.Contains(lower, "problem"),
		strings.Contains(lower, "blocked"),
		strings.Contains(lower, "failed"),
		strings.Contains(lower, "error"):
		return PromotionKindProblem, true
	default:
		return PromotionKindNote, false
	}
}

var entityNamePattern = regexp.MustCompile(`\b[A-Z][A-Za-z0-9_-]{1,}\b`)

func extractEntityNames(text string) []string {
	matches := entityNamePattern.FindAllString(text, -1)
	seen := make(map[string]struct{})
	out := make([]string, 0, len(matches))
	for _, m := range matches {
		if _, ok := seen[m]; !ok {
			seen[m] = struct{}{}
			out = append(out, m)
		}
	}
	return out
}
