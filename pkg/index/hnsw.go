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

// HNSW is a simplified Hierarchical Navigable Small World index.
type HNSW struct {
	m              int
	efConstruction int
	efSearch       int
	maxLevel       int
	// level -> list of node IDs
	layers map[int][]string
	// id -> vector
	vectors map[string][]float32
	// id -> max level for this node
	nodeLevels map[string]int
	// level -> src -> dst -> score (neighbors)
	neighbors map[int]map[string]map[string]float32
	enter     string
	rng       *rand.Rand
}

// NewHNSW creates a new HNSW index with given parameters.
func NewHNSW(m, efConstruction, efSearch int) *HNSW {
	return &HNSW{
		m:              m,
		efConstruction: efConstruction,
		efSearch:       efSearch,
		layers:         make(map[int][]string),
		vectors:        make(map[string][]float32),
		nodeLevels:     make(map[string]int),
		neighbors:      make(map[int]map[string]map[string]float32),
		rng:            rand.New(rand.NewSource(42)),
	}
}

func (h *HNSW) randomLevel() int {
	level := 0
	for float64(h.rng.Float32()) < math.Pow(math.E, -1.0/float64(h.m)) {
		level++
	}
	return level
}

// Insert adds a single vector to the index.
func (h *HNSW) Insert(vector []float32, id string) {
	h.InsertBatch([][]float32{vector}, []string{id})
}

// InsertBatch adds multiple vectors to the index.
func (h *HNSW) InsertBatch(vectors [][]float32, ids []string) {
	for i, vec := range vectors {
		id := ids[i]
		h.vectors[id] = vec
		level := h.randomLevel()
		h.nodeLevels[id] = level

		if level > h.maxLevel {
			h.maxLevel = level
		}

		for l := 0; l <= level; l++ {
			h.layers[l] = append(h.layers[l], id)
		}

		// For each layer this node participates in, find neighbors
		for l := level; l >= 0; l-- {
			if h.enter == "" {
				h.enter = id
				continue
			}
			if _, ok := h.neighbors[l]; !ok {
				h.neighbors[l] = make(map[string]map[string]float32)
			}

			ef := h.efConstruction
			if l == 0 {
				ef = h.efSearch
			}
			candidates := h.searchLayer(id, l, ef)

			// Keep top-M neighbors
			kept := h.selectNeighbors(candidates, h.m)
			if _, ok := h.neighbors[l][id]; !ok {
				h.neighbors[l][id] = make(map[string]float32)
			}
			for nid, score := range kept {
				h.neighbors[l][id][nid] = score
				// Add reverse link
				if _, ok := h.neighbors[l][nid]; !ok {
					h.neighbors[l][nid] = make(map[string]float32)
				}
				h.neighbors[l][nid][id] = score
			}

			// Prune neighbors that exceed M
			for nid, nbrs := range h.neighbors[l] {
				if len(nbrs) > h.m {
					pruned := h.selectNeighbors(nbrs, h.m)
					h.neighbors[l][nid] = pruned
				}
			}
		}
	}
}

// searchLayer finds the nearest neighbors of queryID in a given layer.
func (h *HNSW) searchLayer(queryID string, layer, ef int) map[string]float32 {
	visited := make(map[string]bool)
	// candidates ordered by score ascending (best first via heap-like behavior)
	type entry struct {
		id    string
		score float32
	}

	var candidates []entry
	added := make(map[string]bool)

	// Start from entry point
	if h.enter == "" {
		return nil
	}

	current := h.enter
	currentScore := h.similarity(queryID, current)
	visited[current] = true
	candidates = append(candidates, entry{current, currentScore})
	added[current] = true

	// Greedy search to find the closest entry point
	for {
		best := current
		bestScore := currentScore
		for _, e := range candidates {
			if visited[e.id] {
				continue
			}
			for nb := range h.neighborsForLayer(e.id, layer) {
				if visited[nb] {
					continue
				}
				s := h.similarity(queryID, nb)
				if s > bestScore {
					best = nb
					bestScore = s
				}
			}
		}
		if best == current {
			break
		}
		current = best
		currentScore = bestScore
	}

	// Now expand to ef candidates
	// Sort candidates by score descending
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})

	// Expand using a priority queue approach
	lowest := currentScore
	expanded := 0
	for expanded < ef {
		bestCandidate := ""
		bestScore := float32(-1)
		for _, e := range candidates {
			if visited[e.id] {
				continue
			}
			for nb := range h.neighborsForLayer(e.id, layer) {
				if visited[nb] || added[nb] {
					continue
				}
				s := h.similarity(queryID, nb)
				if s > lowest {
					if s > bestScore {
						bestCandidate = nb
						bestScore = s
					}
				}
			}
		}
		if bestCandidate == "" {
			break
		}
		visited[bestCandidate] = true
		added[bestCandidate] = true
		candidates = append(candidates, entry{bestCandidate, bestScore})
		if bestScore < lowest {
			lowest = bestScore
		}
		expanded++
	}

	// Return top results as map
	result := make(map[string]float32)
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].score > candidates[j].score
	})
	for _, e := range candidates {
		result[e.id] = e.score
	}
	return result
}

func (h *HNSW) neighborsForLayer(id string, layer int) map[string]float32 {
	if nbrs, ok := h.neighbors[layer]; ok {
		if m, ok := nbrs[id]; ok {
			return m
		}
	}
	return nil
}

func (h *HNSW) similarity(aID, bID string) float32 {
	a, okA := h.vectors[aID]
	b, okB := h.vectors[bID]
	if !okA || !okB {
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

func (h *HNSW) selectNeighbors(candidates map[string]float32, m int) map[string]float32 {
	type pair struct {
		id    string
		score float32
	}
	pairs := make([]pair, 0, len(candidates))
	for id, score := range candidates {
		pairs = append(pairs, pair{id, score})
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})
	if len(pairs) > m {
		pairs = pairs[:m]
	}
	result := make(map[string]float32, len(pairs))
	for _, p := range pairs {
		result[p.id] = p.score
	}
	return result
}

// Search queries the index for the topK nearest neighbors.
func (h *HNSW) Search(query []float32, topK int) ([]SearchResult, error) {
	if h.enter == "" || len(h.vectors) == 0 {
		return nil, nil
	}

	// Navigate down layers to find entry point for layer 0
	current := h.enter
	for l := h.maxLevel; l > 0; l-- {
		if nbrs := h.neighborsForLayer(current, l); nbrs != nil {
			best := current
			bestScore := h.similarityVec(query, h.vectors[current])
			for nb := range nbrs {
				s := h.similarityVec(query, h.vectors[nb])
				if s > bestScore {
					best = nb
					bestScore = s
				}
			}
			current = best
		}
	}

	// Search layer 0 with ef
	candidates := h.searchLayerVec(query, current, h.efSearch)

	// Return topK
	type pair struct {
		id    string
		score float32
	}
	pairs := make([]pair, 0, len(candidates))
	for id, score := range candidates {
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

func (h *HNSW) searchLayerVec(query []float32, start string, ef int) map[string]float32 {
	visited := make(map[string]bool)
	type entry struct {
		id    string
		score float32
	}

	var candidates []entry
	added := make(map[string]bool)

	current := start
	currentScore := h.similarityVec(query, h.vectors[current])
	visited[current] = true
	candidates = append(candidates, entry{current, currentScore})
	added[current] = true

	// Greedy search
	for {
		best := current
		bestScore := currentScore
		for _, e := range candidates {
			if visited[e.id] {
				continue
			}
			for nb := range h.neighborsForLayer(e.id, 0) {
				if visited[nb] {
					continue
				}
				s := h.similarityVec(query, h.vectors[nb])
				if s > bestScore {
					best = nb
					bestScore = s
				}
			}
		}
		if best == current {
			break
		}
		current = best
		currentScore = bestScore
	}

	// Expand
	lowest := currentScore
	expanded := 0
	for expanded < ef {
		bestCandidate := ""
		bestScore := float32(-1)
		for _, e := range candidates {
			if visited[e.id] {
				continue
			}
			for nb := range h.neighborsForLayer(e.id, 0) {
				if visited[nb] || added[nb] {
					continue
				}
				s := h.similarityVec(query, h.vectors[nb])
				if s > lowest {
					if s > bestScore {
						bestCandidate = nb
						bestScore = s
					}
				}
			}
		}
		if bestCandidate == "" {
			break
		}
		visited[bestCandidate] = true
		added[bestCandidate] = true
		candidates = append(candidates, entry{bestCandidate, bestScore})
		if bestScore < lowest {
			lowest = bestScore
		}
		expanded++
	}

	result := make(map[string]float32)
	for _, e := range candidates {
		result[e.id] = e.score
	}
	return result
}

func (h *HNSW) similarityVec(a, b []float32) float32 {
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

// Len returns the number of vectors in the index.
func (h *HNSW) Len() int {
	return len(h.vectors)
}

// SerializableHNSW is the gob-serializable form.
type SerializableHNSW struct {
	M              int
	EfConstruction int
	EfSearch       int
	MaxLevel       int
	Layers         map[int][]string
	Vectors        map[string][]float32
	NodeLevels     map[string]int
	Neighbors      map[int]map[string]map[string]float32
	Enter          string
}

// Save serializes the HNSW index.
func (h *HNSW) Save(w io.Writer) error {
	s := SerializableHNSW{
		M:              h.m,
		EfConstruction: h.efConstruction,
		EfSearch:       h.efSearch,
		MaxLevel:       h.maxLevel,
		Layers:         h.layers,
		Vectors:        h.vectors,
		NodeLevels:     h.nodeLevels,
		Neighbors:      h.neighbors,
		Enter:          h.enter,
	}
	enc := gob.NewEncoder(w)
	return enc.Encode(s)
}

// Load deserializes the HNSW index.
func (h *HNSW) Load(r io.Reader) error {
	var s SerializableHNSW
	dec := gob.NewDecoder(r)
	if err := dec.Decode(&s); err != nil {
		return fmt.Errorf("hnsw load: %w", err)
	}
	h.m = s.M
	h.efConstruction = s.EfConstruction
	h.efSearch = s.EfSearch
	h.maxLevel = s.MaxLevel
	h.layers = s.Layers
	h.vectors = s.Vectors
	h.nodeLevels = s.NodeLevels
	h.neighbors = s.Neighbors
	h.enter = s.Enter
	return nil
}

func init() {
	gob.Register(map[string]float32(nil))
	gob.Register(map[string]map[string]float32(nil))
	gob.Register(map[int]map[string]map[string]float32(nil))
	gob.Register(map[int][]string(nil))
}

// RemoveVector removes a vector from the index.
func (h *HNSW) RemoveVector(id string) {
	delete(h.vectors, id)
	delete(h.nodeLevels, id)
	for l := range h.layers {
		for i, nid := range h.layers[l] {
			if nid == id {
				h.layers[l] = append(h.layers[l][:i], h.layers[l][i+1:]...)
				break
			}
		}
	}
	for l, nbrs := range h.neighbors {
		delete(nbrs, id)
		for nid := range nbrs {
			delete(nbrs[nid], id)
		}
		if len(nbrs) == 0 {
			delete(h.neighbors, l)
		}
	}
	if h.enter == id {
		h.enter = ""
		for _, id2 := range h.layers[0] {
			if _, ok := h.vectors[id2]; ok {
				h.enter = id2
				break
			}
		}
	}
}

// Marshal serializes the index to a byte slice.
func (h *HNSW) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := h.Save(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal restores the index from a byte slice.
func (h *HNSW) Unmarshal(data []byte) error {
	buf := bytes.NewReader(data)
	return h.Load(buf)
}
