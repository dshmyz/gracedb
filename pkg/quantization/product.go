package quantization

import (
	"math"
	"math/rand"
)

// ProductQuantizer implements Product Quantization (PQ).
// Vectors are split into sub-vectors, each quantized with a separate codebook.
type ProductQuantizer struct {
	NumSubVectors int
	NumCentroids  int
	SubDim        int
	Codebooks     [][][]float32 // [subvector][centroid][dim]
}

// NewProductQuantizer creates a new PQ quantizer.
func NewProductQuantizer(numSubVectors, numCentroids int) *ProductQuantizer {
	return &ProductQuantizer{
		NumSubVectors: numSubVectors,
		NumCentroids:  numCentroids,
	}
}

// Train builds codebooks using k-means on training vectors.
func (pq *ProductQuantizer) Train(vectors [][]float32) error {
	if len(vectors) == 0 {
		return nil
	}

	dim := len(vectors[0])
	if dim%pq.NumSubVectors != 0 {
		return nil
	}
	pq.SubDim = dim / pq.NumSubVectors

	pq.Codebooks = make([][][]float32, pq.NumSubVectors)
	rng := rand.New(rand.NewSource(42))

	for s := 0; s < pq.NumSubVectors; s++ {
		// Extract sub-vectors.
		subVectors := make([][]float32, len(vectors))
		for i, v := range vectors {
			subVectors[i] = v[s*pq.SubDim : (s+1)*pq.SubDim]
		}

		// K-means clustering.
		codebook, err := kmeans(subVectors, pq.NumCentroids, rng)
		if err != nil {
			return err
		}
		pq.Codebooks[s] = codebook
	}

	return nil
}

// Encode quantizes a vector to PQ codes (byte array).
func (pq *ProductQuantizer) Encode(vector []float32) []byte {
	if len(pq.Codebooks) == 0 {
		return nil
	}

	codes := make([]byte, pq.NumSubVectors)
	for s := 0; s < pq.NumSubVectors; s++ {
		subVec := vector[s*pq.SubDim : (s+1)*pq.SubDim]
		codes[s] = byte(findNearestCentroid(subVec, pq.Codebooks[s]))
	}
	return codes
}

// Decode reconstructs an approximate vector from PQ codes.
func (pq *ProductQuantizer) Decode(codes []byte) []float32 {
	if len(pq.Codebooks) == 0 {
		return nil
	}

	result := make([]float32, 0, pq.NumSubVectors*pq.SubDim)
	for s := 0; s < pq.NumSubVectors; s++ {
		centroid := pq.Codebooks[s][codes[s]]
		result = append(result, centroid...)
	}
	return result
}

// Distance computes the asymmetric distance between a raw vector and a PQ code.
func (pq *ProductQuantizer) Distance(vector []float32, codes []byte) float32 {
	if len(pq.Codebooks) == 0 {
		return math.MaxFloat32
	}

	var dist float32
	for s := 0; s < pq.NumSubVectors; s++ {
		subVec := vector[s*pq.SubDim : (s+1)*pq.SubDim]
		centroid := pq.Codebooks[s][codes[s]]
		for i := range subVec {
			d := subVec[i] - centroid[i]
			dist += d * d
		}
	}
	return dist
}

func findNearestCentroid(vector []float32, codebook [][]float32) int {
	bestIdx := 0
	bestDist := float32(math.MaxFloat32)
	for i, c := range codebook {
		d := squaredDistance(vector, c)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

func squaredDistance(a, b []float32) float32 {
	var sum float32
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sum
}

func kmeans(vectors [][]float32, k int, rng *rand.Rand) ([][]float32, error) {
	if len(vectors) < k {
		k = len(vectors)
	}
	if k == 0 {
		return nil, nil
	}

	dim := len(vectors[0])

	// Initialize centroids with random vectors.
	centroids := make([][]float32, k)
	for i := 0; i < k; i++ {
		centroids[i] = make([]float32, dim)
		copy(centroids[i], vectors[rng.Intn(len(vectors))])
	}

	// K-means iterations.
	assignments := make([]int, len(vectors))
	for iter := 0; iter < 20; iter++ {
		// Assign.
		changed := false
		for i, v := range vectors {
			best := findNearestCentroid(v, centroids)
			if best != assignments[i] {
				changed = true
			}
			assignments[i] = best
		}
		if !changed {
			break
		}

		// Update centroids.
		counts := make([]int, k)
		sums := make([][]float32, k)
		for i := range sums {
			sums[i] = make([]float32, dim)
		}
		for i, v := range vectors {
			c := assignments[i]
			counts[c]++
			for j := range v {
				sums[c][j] += v[j]
			}
		}
		for i := 0; i < k; i++ {
			if counts[i] > 0 {
				for j := range centroids[i] {
					centroids[i][j] = sums[i][j] / float32(counts[i])
				}
			}
		}
	}

	return centroids, nil
}
