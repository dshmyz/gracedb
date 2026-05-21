// Package semanticrouter provides semantic routing based on vector similarity
// and lexical keyword matching to classify user queries before invoking LLM calls.
package semanticrouter

import (
	"context"
	"math"
	"sort"
	"sync"
)

// Route represents a semantic route.
type Route struct {
	Name       string
	Utterances []string
	Handler    RouteHandler
	Metadata   map[string]string
}

// RouteHandler handles a matched route.
type RouteHandler func(ctx context.Context, query string, score float64) (string, error)

// RouteResult is a routing decision.
type RouteResult struct {
	RouteName string  `json:"route_name"`
	Score     float64 `json:"score"`
	Matched   bool    `json:"matched"`
}

// Config holds router configuration.
type Config struct {
	Threshold float64
	TopK      int
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{Threshold: 0.75, TopK: 1}
}

// Router performs semantic routing using vector similarity.
type Router struct {
	routes   []*Route
	cfg      Config
	embedder Embedder
	mu       sync.RWMutex

	// Cached embeddings for utterances.
	cache map[string][][]float32
}

// Embedder computes text embeddings.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimension() int
}

// NewRouter creates a semantic router.
func NewRouter(embedder Embedder, cfg Config) *Router {
	if cfg.Threshold == 0 {
		cfg.Threshold = 0.75
	}
	if cfg.TopK == 0 {
		cfg.TopK = 1
	}
	return &Router{
		cfg:      cfg,
		embedder: embedder,
		cache:    make(map[string][][]float32),
	}
}

// Add adds a route.
func (r *Router) Add(route *Route) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.routes = append(r.routes, route)
	delete(r.cache, route.Name)
	return nil
}

// Route performs semantic routing.
func (r *Router) Route(ctx context.Context, text string) (*RouteResult, error) {
	if r.embedder == nil {
		return nil, nil
	}

	queryVec, err := r.embedder.Embed(ctx, text)
	if err != nil {
		return nil, err
	}

	r.mu.RLock()
	routes := r.routes
	r.mu.RUnlock()

	type scored struct {
		name  string
		score float64
		route *Route
	}
	var all []scored

	for _, route := range routes {
		vectors := r.getVectors(ctx, route)
		best := 0.0
		for _, v := range vectors {
			s := cosineSim(queryVec, v)
			if s > best {
				best = s
			}
		}
		all = append(all, scored{route.Name, best, route})
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].score > all[j].score
	})

	if len(all) == 0 {
		return &RouteResult{}, nil
	}

	top := all[0]
	return &RouteResult{
		RouteName: top.name,
		Score:     top.score,
		Matched:   top.score >= r.cfg.Threshold,
	}, nil
}

func (r *Router) getVectors(ctx context.Context, route *Route) [][]float32 {
	r.mu.Lock()
	defer r.mu.Unlock()

	if vectors, ok := r.cache[route.Name]; ok {
		return vectors
	}

	vectors := make([][]float32, len(route.Utterances))
	for i, u := range route.Utterances {
		v, err := r.embedder.Embed(ctx, u)
		if err != nil {
			v = nil
		}
		vectors[i] = v
	}

	r.cache[route.Name] = vectors
	return vectors
}

func cosineSim(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		na += float64(a[i]) * float64(a[i])
		nb += float64(b[i]) * float64(b[i])
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (math.Sqrt(na) * math.Sqrt(nb))
}
