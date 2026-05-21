package index

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"math"
	"sort"
)

// Flat is a linear scan (brute force) vector index.
type Flat struct {
	vectors map[string][]float32
}

// NewFlat creates a new flat index.
func NewFlat() *Flat {
	return &Flat{
		vectors: make(map[string][]float32),
	}
}

// Insert adds a single vector.
func (f *Flat) Insert(vector []float32, id string) {
	f.vectors[id] = vector
}

// InsertBatch adds multiple vectors.
func (f *Flat) InsertBatch(vectors [][]float32, ids []string) {
	for i, vec := range vectors {
		f.vectors[ids[i]] = vec
	}
}

// Search finds the topK nearest neighbors via linear scan.
func (f *Flat) Search(query []float32, topK int) ([]SearchResult, error) {
	if len(f.vectors) == 0 {
		return nil, nil
	}

	type pair struct {
		id    string
		score float32
	}
	pairs := make([]pair, 0, len(f.vectors))
	for id, vec := range f.vectors {
		score := cosineSimilarity(query, vec)
		pairs = append(pairs, pair{id, score})
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})
	if len(pairs) > topK {
		pairs = pairs[:topK]
	}

	results := make([]SearchResult, len(pairs))
	for i, p := range pairs {
		results[i] = SearchResult{ID: p.id, Score: p.score}
	}
	return results, nil
}

// Len returns the number of vectors.
func (f *Flat) Len() int {
	return len(f.vectors)
}

// Save serializes the flat index.
func (f *Flat) Save(w io.Writer) error {
	enc := gob.NewEncoder(w)
	return enc.Encode(f.vectors)
}

// Load deserializes the flat index.
func (f *Flat) Load(r io.Reader) error {
	f.vectors = make(map[string][]float32)
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&f.vectors); err != nil {
		return fmt.Errorf("flat load: %w", err)
	}
	return nil
}

func cosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(normA))) * float32(math.Sqrt(float64(normB))))
}

// RemoveVector removes a vector from the index.
func (f *Flat) RemoveVector(id string) {
	delete(f.vectors, id)
}

// Marshal serializes the index to a byte slice.
func (f *Flat) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := f.Save(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal restores the index from a byte slice.
func (f *Flat) Unmarshal(data []byte) error {
	buf := bytes.NewReader(data)
	return f.Load(buf)
}
