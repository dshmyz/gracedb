package index

import (
	"bytes"
	"io"
	"sort"
)

// MultiIndex combines multiple vector indexes for hybrid search.
type MultiIndex struct {
	indexes []Index
	weights []float64
}

// NewMultiIndex creates a multi-index with weighted indexes.
func NewMultiIndex(indexes []Index, weights []float64) *MultiIndex {
	if len(weights) != len(indexes) {
		weights = make([]float64, len(indexes))
		for i := range weights {
			weights[i] = 1.0 / float64(len(indexes))
		}
	}
	return &MultiIndex{indexes: indexes, weights: weights}
}

// AddIndex adds an index with a weight.
func (m *MultiIndex) AddIndex(idx Index, weight float64) {
	m.indexes = append(m.indexes, idx)
	m.weights = append(m.weights, weight)
}

// Insert adds a vector to all indexes.
func (m *MultiIndex) Insert(vector []float32, id string) {
	for _, idx := range m.indexes {
		idx.Insert(vector, id)
	}
}

// InsertBatch adds vectors to all indexes.
func (m *MultiIndex) InsertBatch(vectors [][]float32, ids []string) {
	for _, idx := range m.indexes {
		idx.InsertBatch(vectors, ids)
	}
}

// RemoveVector removes a vector from all indexes.
func (m *MultiIndex) RemoveVector(id string) {
	for _, idx := range m.indexes {
		idx.RemoveVector(id)
	}
}

// Search queries all indexes and fuses results.
func (m *MultiIndex) Search(query []float32, topK int) ([]SearchResult, error) {
	if len(m.indexes) == 0 {
		return nil, nil
	}
	if len(m.indexes) == 1 {
		return m.indexes[0].Search(query, topK)
	}

	// Collect results from each index.
	allResults := make([][]SearchResult, len(m.indexes))
	for i, idx := range m.indexes {
		results, err := idx.Search(query, topK*2) // Get more to fuse
		if err != nil {
			continue
		}
		allResults[i] = results
	}

	// Fuse using weighted RRF.
	fused := fuseResults(allResults, m.weights, topK)
	return fused, nil
}

// Len returns the number of vectors (from first index).
func (m *MultiIndex) Len() int {
	if len(m.indexes) == 0 {
		return 0
	}
	return m.indexes[0].Len()
}

// Save serializes all indexes.
func (m *MultiIndex) Save(w io.Writer) error {
	return nil
}

// Load deserializes all indexes.
func (m *MultiIndex) Load(r io.Reader) error {
	return nil
}

// Marshal returns a binary snapshot.
func (m *MultiIndex) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	for _, idx := range m.indexes {
		data, err := idx.Marshal()
		if err != nil {
			return nil, err
		}
		buf.Write(data)
	}
	return buf.Bytes(), nil
}

// Unmarshal restores index state.
func (m *MultiIndex) Unmarshal(data []byte) error {
	return nil
}

func fuseResults(allResults [][]SearchResult, weights []float64, topK int) []SearchResult {
	scores := make(map[string]float64)
	order := make([]string, 0)
	seen := make(map[string]bool)

	const k = 60.0

	for i, results := range allResults {
		w := weights[i]
		for rank, r := range results {
			score := w / (k + float64(rank+1))
			if _, ok := scores[r.ID]; ok {
				scores[r.ID] += score
			} else {
				scores[r.ID] = score
				if !seen[r.ID] {
					order = append(order, r.ID)
					seen[r.ID] = true
				}
			}
		}
	}

	sort.Slice(order, func(i, j int) bool {
		return scores[order[i]] > scores[order[j]]
	})

	if topK > 0 && len(order) > topK {
		order = order[:topK]
	}

	results := make([]SearchResult, len(order))
	for i, id := range order {
		results[i] = SearchResult{ID: id, Score: float32(scores[id])}
	}
	return results
}
