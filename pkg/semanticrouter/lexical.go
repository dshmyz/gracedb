package semanticrouter

import (
	"context"
	"strings"
)

// LexicalRoute is a route for lexical matching.
type LexicalRoute struct {
	Name       string
	Utterances []string
	Threshold  float64
}

// LexicalRouter performs keyword-based routing.
type LexicalRouter struct {
	routes []*LexicalRoute
}

// NewLexicalRouter creates a lexical router.
func NewLexicalRouter() *LexicalRouter {
	return &LexicalRouter{}
}

// Add adds a lexical route.
func (lr *LexicalRouter) Add(route *LexicalRoute) {
	if route.Threshold == 0 {
		route.Threshold = 0.3
	}
	lr.routes = append(lr.routes, route)
}

// Route performs lexical matching.
func (lr *LexicalRouter) Route(ctx context.Context, text string) (*RouteResult, error) {
	text = strings.ToLower(text)
	tokens := tokenize(text)

	type scored struct {
		name  string
		score float64
	}
	var all []scored

	for _, route := range lr.routes {
		best := 0.0
		for _, utterance := range route.Utterances {
			ut := strings.ToLower(utterance)
			uTokens := tokenize(ut)
			score := jaccard(tokens, uTokens)
			if score > best {
				best = score
			}
		}
		all = append(all, scored{route.Name, best})
	}

	if len(all) == 0 {
		return &RouteResult{}, nil
	}

	best := all[0]
	for _, s := range all[1:] {
		if s.score > best.score {
			best = s
		}
	}

	return &RouteResult{
		RouteName: best.name,
		Score:     best.score,
		Matched:   best.score >= lr.routes[findRoute(lr.routes, best.name)].Threshold,
	}, nil
}

func tokenize(s string) []string {
	s = strings.ToLower(s)
	var tokens []string
	var cur strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z' && r <= 0x7F) {
			cur.WriteRune(r)
		} else {
			if cur.Len() > 0 {
				tokens = append(tokens, cur.String())
				cur.Reset()
			}
		}
	}
	if cur.Len() > 0 {
		tokens = append(tokens, cur.String())
	}
	return tokens
}

func jaccard(a, b []string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 0
	}
	setA := make(map[string]bool)
	for _, t := range a {
		setA[t] = true
	}
	setB := make(map[string]bool)
	for _, t := range b {
		setB[t] = true
	}

	intersection := 0
	for t := range setA {
		if setB[t] {
			intersection++
		}
	}
	union := len(setA) + len(setB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}

func findRoute(routes []*LexicalRoute, name string) int {
	for i, r := range routes {
		if r.Name == name {
			return i
		}
	}
	return 0
}
