package index

import "io"

// SearchResult holds a single search result with its score.
type SearchResult struct {
	ID    string
	Score float32
}

// Index defines the interface for a vector index.
type Index interface {
	Insert(vector []float32, id string)
	InsertBatch(vectors [][]float32, ids []string)
	RemoveVector(id string)
	Search(query []float32, topK int) ([]SearchResult, error)
	Len() int
	Save(w io.Writer) error
	Load(r io.Reader) error
	// Marshal returns a binary snapshot of the index state.
	Marshal() ([]byte, error)
	// Unmarshal restores index state from a binary snapshot.
	Unmarshal(data []byte) error
}
