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

// IVFIndex is an Inverted File Index for vectors.
// Vectors are partitioned into clusters, and search is performed
// only in the nearest nprobe clusters.
type IVFIndex struct {
	nclusters  int
	nprobe     int
	centroids  [][]float32         // cluster centroids
	clusters   map[int][]string    // cluster_id -> vector IDs
	vectors    map[string][]float32
	rng        *rand.Rand
}

// NewIVFIndex creates a new IVF index.
func NewIVFIndex(nclusters, nprobe int) *IVFIndex {
	return &IVFIndex{
		nclusters: nclusters,
		nprobe:    nprobe,
		clusters:  make(map[int][]string),
		vectors:   make(map[string][]float32),
		rng:       rand.New(rand.NewSource(42)),
	}
}

// Insert adds a single vector.
func (ivf *IVFIndex) Insert(vector []float32, id string) {
	ivf.vectors[id] = vector
	clusterID := ivf.assignCluster(vector)
	ivf.clusters[clusterID] = append(ivf.clusters[clusterID], id)
}

// InsertBatch adds multiple vectors.
func (ivf *IVFIndex) InsertBatch(vectors [][]float32, ids []string) {
	for i, vec := range vectors {
		ivf.Insert(vec, ids[i])
	}
}

// Search finds the topK nearest neighbors.
func (ivf *IVFIndex) Search(query []float32, topK int) ([]SearchResult, error) {
	if len(ivf.vectors) == 0 {
		return nil, nil
	}

	// Find nearest clusters.
	nearestClusters := ivf.nearestCentroids(query, ivf.nprobe)

	type pair struct {
		id    string
		score float32
	}
	var pairs []pair

	for _, cID := range nearestClusters {
		for _, vecID := range ivf.clusters[cID] {
			vec := ivf.vectors[vecID]
			score := cosineSimilarity(query, vec)
			pairs = append(pairs, pair{vecID, score})
		}
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

// Train initializes centroids using k-means++ style initialization.
func (ivf *IVFIndex) Train() error {
	if len(ivf.vectors) < ivf.nclusters {
		return fmt.Errorf("ivf: need at least %d vectors for %d clusters", ivf.nclusters, ivf.nclusters)
	}

	// Collect all vectors.
	allIDs := make([]string, 0, len(ivf.vectors))
	for id := range ivf.vectors {
		allIDs = append(allIDs, id)
	}

	// Initialize centroids with k-means++.
	ivf.centroids = make([][]float32, ivf.nclusters)
	ivf.centroids[0] = ivf.vectors[allIDs[ivf.rng.Intn(len(allIDs))]]

	for c := 1; c < ivf.nclusters; c++ {
		// Compute distances to nearest centroid.
		distances := make([]float64, len(allIDs))
		for i, id := range allIDs {
			minDist := math.MaxFloat64
			for j := 0; j < c; j++ {
				d := ivf.distance(ivf.vectors[id], ivf.centroids[j])
				if d < minDist {
					minDist = d
				}
			}
			distances[i] = minDist
		}

		// Choose next centroid with probability proportional to distance squared.
		total := 0.0
		for _, d := range distances {
			total += d
		}
		r := ivf.rng.Float64() * total
		acc := 0.0
		for i, d := range distances {
			acc += d
			if acc >= r {
				ivf.centroids[c] = make([]float32, len(ivf.vectors[allIDs[i]]))
				copy(ivf.centroids[c], ivf.vectors[allIDs[i]])
				break
			}
		}
	}

	// Reassign vectors to clusters.
	ivf.clusters = make(map[int][]string)
	for id, vec := range ivf.vectors {
		cID := ivf.assignCluster(vec)
		ivf.clusters[cID] = append(ivf.clusters[cID], id)
	}

	return nil
}

func (ivf *IVFIndex) assignCluster(vector []float32) int {
	if len(ivf.centroids) == 0 {
		return 0
	}
	bestCluster := 0
	bestDist := math.MaxFloat64
	for i, centroid := range ivf.centroids {
		d := ivf.distance(vector, centroid)
		if d < bestDist {
			bestDist = d
			bestCluster = i
		}
	}
	return bestCluster
}

func (ivf *IVFIndex) nearestCentroids(query []float32, n int) []int {
	if len(ivf.centroids) == 0 {
		return nil
	}

	type scored struct {
		id    int
		score float64
	}
	scores := make([]scored, len(ivf.centroids))
	for i, c := range ivf.centroids {
		scores[i] = scored{i, ivf.distance(query, c)}
	}
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score < scores[j].score
	})
	if n > len(scores) {
		n = len(scores)
	}

	result := make([]int, n)
	for i := 0; i < n; i++ {
		result[i] = scores[i].id
	}
	return result
}

func (ivf *IVFIndex) distance(a, b []float32) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}
	var sum float64
	for i := range a {
		d := float64(a[i] - b[i])
		sum += d * d
	}
	return math.Sqrt(sum)
}

// Len returns the number of vectors.
func (ivf *IVFIndex) Len() int {
	return len(ivf.vectors)
}

// Save serializes the IVF index.
func (ivf *IVFIndex) Save(w io.Writer) error {
	enc := gob.NewEncoder(w)
	return enc.Encode(ivf)
}

// Load deserializes the IVF index.
func (ivf *IVFIndex) Load(r io.Reader) error {
	dec := gob.NewDecoder(r)
	if err := dec.Decode(ivf); err != nil {
		return fmt.Errorf("ivf load: %w", err)
	}
	return nil
}

// RemoveVector removes a vector from the index.
func (ivf *IVFIndex) RemoveVector(id string) {
	delete(ivf.vectors, id)
	for cID, ids := range ivf.clusters {
		for i, vid := range ids {
			if vid == id {
				ivf.clusters[cID] = append(ids[:i], ids[i+1:]...)
				break
			}
		}
	}
}

// Marshal serializes the index to a byte slice.
func (ivf *IVFIndex) Marshal() ([]byte, error) {
	var buf bytes.Buffer
	if err := ivf.Save(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unmarshal restores the index from a byte slice.
func (ivf *IVFIndex) Unmarshal(data []byte) error {
	buf := bytes.NewReader(data)
	return ivf.Load(buf)
}
