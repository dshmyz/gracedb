package types

// Reranker re-ranks search results based on a secondary scoring method.
type Reranker interface {
	Rerank(query string, candidates []ScoredEmbedding) ([]ScoredEmbedding, error)
}
