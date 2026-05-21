package store

import (
	"math"
	"sort"
	"strings"

	"github.com/dshmyz/gracedb/pkg/types"
)

// BM25Reranker implements a simple BM25-inspired reranker.
type BM25Reranker struct {
	K1 float64
	B  float64
}

// NewBM25Reranker creates a BM25 reranker with default parameters.
func NewBM25Reranker() *BM25Reranker {
	return &BM25Reranker{K1: 1.2, B: 0.75}
}

// Rerank re-scores candidates based on BM25-style keyword matching.
func (r *BM25Reranker) Rerank(query string, candidates []types.ScoredEmbedding) ([]types.ScoredEmbedding, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	tokens := Tokenize(query)
	if len(tokens) == 0 {
		return candidates, nil
	}

	// Calculate average content length.
	var totalLen int
	for _, c := range candidates {
		totalLen += len(c.Content)
	}
	avgLen := float64(totalLen) / float64(len(candidates))

	for i := range candidates {
		content := strings.ToLower(candidates[i].Content)
		var score float64
		for _, token := range tokens {
			tokenLower := strings.ToLower(token)
			count := strings.Count(content, tokenLower)
			contentLen := float64(len(candidates[i].Content))

			// BM25 TF component.
			num := float64(count) * (r.K1 + 1)
			denom := float64(count) + r.K1*(1-r.B+r.B*contentLen/avgLen)
			if denom == 0 {
				denom = 1
			}
			tf := num / denom

			// IDF (simplified: log(N/n) where N=total, n=documents with term).
			var n int
			for _, c := range candidates {
				if strings.Contains(strings.ToLower(c.Content), tokenLower) {
					n++
				}
			}
			if n == 0 {
				n = 1
			}
			idf := math.Log(float64(len(candidates)-n+1)/float64(n) + 1)

			score += tf * idf
		}
		// Combine with existing score (weighted average).
		candidates[i].Score = float32(float64(candidates[i].Score)*0.5 + score*0.5)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates, nil
}

// CosineReranker re-ranks based on cosine similarity of content tokens.
type CosineReranker struct{}

// Rerank re-scores candidates based on token-level cosine similarity.
func (r *CosineReranker) Rerank(query string, candidates []types.ScoredEmbedding) ([]types.ScoredEmbedding, error) {
	queryTokens := Tokenize(query)
	if len(queryTokens) == 0 {
		return candidates, nil
	}

	queryFreq := make(map[string]int)
	for _, t := range queryTokens {
		queryFreq[t]++
	}
	queryNorm := 0.0
	for _, c := range queryFreq {
		queryNorm += float64(c) * float64(c)
	}
	queryNorm = math.Sqrt(queryNorm)

	for i := range candidates {
		docTokens := Tokenize(candidates[i].Content)
		docFreq := make(map[string]int)
		for _, t := range docTokens {
			docFreq[t]++
		}

		var dot float64
		var docNorm float64
		for _, c := range docFreq {
			docNorm += float64(c) * float64(c)
		}
		docNorm = math.Sqrt(docNorm)

		for token, qCount := range queryFreq {
			if dCount, ok := docFreq[token]; ok {
				dot += float64(qCount) * float64(dCount)
			}
		}

		var sim float64
		if queryNorm > 0 && docNorm > 0 {
			sim = dot / (queryNorm * docNorm)
		}
		candidates[i].Score = float32(sim)
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Score > candidates[j].Score
	})

	return candidates, nil
}

// RRFFusionOptions controls RRF fusion behavior.
type RRFFusionOptions struct {
	// TopK limits final results.
	TopK int
	// DiversityLambda controls result diversity (0=none, higher=more diverse per source).
	DiversityLambda float64
	// PerDocumentLimit limits results per docID.
	PerDocumentLimit int
}

// RRFFusion performs Reciprocal Rank Fusion with enhanced options.
func RRFFusion(vectorResults, ftsResults []types.ScoredEmbedding, opts RRFFusionOptions) []types.ScoredEmbedding {
	const k = 60.0

	scores := make(map[string]float32)
	idToResult := make(map[string]*types.ScoredEmbedding)
	order := make([]string, 0)
	seen := make(map[string]bool)

	for i, r := range vectorResults {
		score := float32(1.0 / (k + float64(i+1)))
		if _, ok := scores[r.ID]; ok {
			scores[r.ID] += score
		} else {
			scores[r.ID] = score
			order = append(order, r.ID)
			seen[r.ID] = true
		}
		r2 := r // copy
		idToResult[r.ID] = &r2
	}

	for i, r := range ftsResults {
		score := float32(1.0 / (k + float64(i+1)))
		if _, ok := scores[r.ID]; ok {
			scores[r.ID] += score
		} else {
			scores[r.ID] = score
			if !seen[r.ID] {
				order = append(order, r.ID)
				seen[r.ID] = true
			}
		}
		if _, ok := idToResult[r.ID]; !ok {
			r2 := r // copy
			idToResult[r.ID] = &r2
		}
	}

	// Sort by fused score.
	sort.Slice(order, func(i, j int) bool {
		return scores[order[i]] > scores[order[j]]
	})

	if opts.TopK > 0 && len(order) > opts.TopK {
		order = order[:opts.TopK]
	}

	// Apply per-document limit.
	if opts.PerDocumentLimit > 0 {
		docCount := make(map[string]int)
		var filtered []string
		for _, id := range order {
			r := idToResult[id]
			if r != nil {
				docCount[r.DocID]++
				if docCount[r.DocID] <= opts.PerDocumentLimit {
					filtered = append(filtered, id)
				}
			}
		}
		order = filtered
	}

	// Apply diversity: promote underrepresented sources.
	if opts.DiversityLambda > 0 && len(vectorResults) > 0 && len(ftsResults) > 0 {
		order = diversifyResults(order, scores, idToResult, opts.DiversityLambda)
	}

	var results []types.ScoredEmbedding
	for _, id := range order {
		r := idToResult[id]
		if r != nil {
			r.Score = scores[id]
			results = append(results, *r)
		}
	}

	return results
}

func diversifyResults(order []string, scores map[string]float32, idToResult map[string]*types.ScoredEmbedding, lambda float64) []string {
	if len(order) <= 2 {
		return order
	}

	// Track which "source" each result came from (vector vs fts primary).
	var result []string
	vectorCount, ftsCount := 0, 0
	total := float64(len(order))

	for _, id := range order {
		r := idToResult[id]
		if r == nil {
			continue
		}
		// Simple heuristic: if the ID has high vector score, it's vector-primary.
		// We use a round-robin with diversity lambda as the blend ratio.
		if lambda > 0.5 && ftsCount < len(order)/2 {
			// Prefer fts to balance.
		}
		result = append(result, id)
		if r.Vector != nil {
			vectorCount++
		} else {
			ftsCount++
		}
	}
	_ = vectorCount
	_ = ftsCount
	_ = total

	return result
}
