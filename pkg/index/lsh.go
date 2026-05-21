package index

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"math"
	"math/rand"
	"sort"
)

// LSHIndex is a Locality-Sensitive Hashing index using random hyperplanes.
// Vectors with similar cosine angles are likely to hash to the same bucket.
type LSHIndex struct {
	numHashes int
	tables    int       // number of hash tables
	hyperplanes [][][]float32 // [tables][numHashes][dim]
	buckets   map[string]map[uint32][]string // table -> bucket -> vector IDs
	vectors   map[string][]float32
	rng       *rand.Rand
}

// NewLSHIndex creates a new LSH index.
// numHashes: number of hash functions per table
// tables: number of hash tables (more = higher recall, more memory)
func NewLSHIndex(numHashes, tables int) *LSHIndex {
	return &LSHIndex{
		numHashes: numHashes,
		tables:    tables,
		buckets:   make(map[string]map[uint32][]string),
		vectors:   make(map[string][]float32),
		rng:       rand.New(rand.NewSource(42)),
	}
}

// Build generates random hyperplanes.
func (lsh *LSHIndex) Build(dim int) {
	lsh.hyperplanes = make([][][]float32, lsh.tables)
	for t := 0; t < lsh.tables; t++ {
		lsh.hyperplanes[t] = make([][]float32, lsh.numHashes)
		for h := 0; h < lsh.numHashes; h++ {
			vec := make([]float32, dim)
			for i := 0; i < dim; i++ {
				vec[i] = float32(lsh.rng.NormFloat64())
			}
			lsh.hyperplanes[t][h] = vec
		}
	}
}

// Insert adds a vector to the index.
func (lsh *LSHIndex) Insert(vector []float32, id string) {
	lsh.vectors[id] = vector

	for t := 0; t < lsh.tables; t++ {
		bucketKey := lsh.hash(vector, t)
		tableKey := fmt.Sprintf("t%d", t)
		if _, ok := lsh.buckets[tableKey]; !ok {
			lsh.buckets[tableKey] = make(map[uint32][]string)
		}
		lsh.buckets[tableKey][bucketKey] = append(lsh.buckets[tableKey][bucketKey], id)
	}
}

// InsertBatch adds multiple vectors.
func (lsh *LSHIndex) InsertBatch(vectors [][]float32, ids []string) {
	for i, vec := range vectors {
		lsh.Insert(vec, ids[i])
	}
}

// Search finds approximate nearest neighbors using LSH.
func (lsh *LSHIndex) Search(query []float32, topK int) ([]SearchResult, error) {
	if len(lsh.vectors) == 0 {
		return nil, nil
	}

	// Collect candidate IDs from matching buckets.
	candidates := make(map[string]bool)
	for t := 0; t < lsh.tables; t++ {
		bucketKey := lsh.hash(query, t)
		tableKey := fmt.Sprintf("t%d", t)
		if bucket, ok := lsh.buckets[tableKey][bucketKey]; ok {
			for _, id := range bucket {
				candidates[id] = true
			}
		}
	}

	if len(candidates) == 0 {
		// Fallback: search all.
		candidates = make(map[string]bool, len(lsh.vectors))
		for id := range lsh.vectors {
			candidates[id] = true
		}
	}

	type pair struct {
		id    string
		score float32
	}
	pairs := make([]pair, 0, len(candidates))
	for id := range candidates {
		vec := lsh.vectors[id]
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

func (lsh *LSHIndex) hash(vector []float32, table int) uint32 {
	var hash uint32 = 0
	for h := 0; h < lsh.numHashes; h++ {
		hp := lsh.hyperplanes[table][h]
		dot := float32(0)
		for i := range vector {
			dot += vector[i] * hp[i]
		}
		bit := uint32(0)
		if dot > 0 {
			bit = 1
		}
		hash = (hash << 1) | bit
	}
	return hash
}

// Len returns the number of vectors.
func (lsh *LSHIndex) Len() int {
	return len(lsh.vectors)
}

// Save serializes the LSH index.
func (lsh *LSHIndex) Save(w io.Writer) error {
	enc := gob.NewEncoder(w)
	return enc.Encode(lsh)
}

// Load deserializes the LSH index.
func (lsh *LSHIndex) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	if err := dec.Decode(lsh); err != nil {
		return fmt.Errorf("lsh load: %w", err)
	}
	return nil
}

// RemoveVector removes a vector from the index.
func (lsh *LSHIndex) RemoveVector(id string) {
	delete(lsh.vectors, id)
	for tableKey, buckets := range lsh.buckets {
		for bucketKey, ids := range buckets {
			for i, vid := range ids {
				if vid == id {
					buckets[bucketKey] = append(ids[:i], ids[i+1:]...)
					break
				}
			}
		}
		_ = tableKey
	}
}

// Marshal serializes the index to a byte slice.
func (lsh *LSHIndex) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := lsh.Save(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal restores the index from a byte slice.
func (lsh *LSHIndex) Unmarshal(data []byte) error {
	buf := bytes.NewReader(data)
	return lsh.Load(buf)
}

// AngularDistance computes angular distance (1 - cosine similarity).
func AngularDistance(a, b []float32) float32 {
	return 1 - cosineSimilarity(a, b)
}

// HammingDistance computes the Hamming distance between two byte slices.
func HammingDistance(a, b []byte) int {
	dist := 0
	for i := 0; i < len(a) && i < len(b); i++ {
		x := a[i] ^ b[i]
		for x != 0 {
			x &= x - 1
			dist++
		}
	}
	return dist
}

// L2Distance computes the Euclidean distance between two vectors.
func L2Distance(a, b []float32) float32 {
	if len(a) != len(b) {
		return math.MaxFloat32
	}
	var sum float32
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return float32(math.Sqrt(float64(sum)))
}
