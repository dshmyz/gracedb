package store

import "math"

// CosineSimilarity computes the cosine similarity between two vectors.
func CosineSimilarity(a, b []float32) float32 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float32
	for i := range a {
		ai := a[i]
		bi := b[i]
		dot += ai * bi
		normA += ai * ai
		normB += bi * bi
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (sqrt(normA) * sqrt(normB))
}

// EuclideanDistance computes the L2 distance between two vectors.
func EuclideanDistance(a, b []float32) float32 {
	if len(a) != len(b) {
		return math.MaxFloat32
	}
	var sum float32
	for i := range a {
		d := a[i] - b[i]
		sum += d * d
	}
	return sqrt(sum)
}

// Score computes similarity between two vectors using the specified distance function.
func Score(a, b []float32, distance string) float32 {
	switch distance {
	case "cosine":
		return CosineSimilarity(a, b)
	case "euclidean", "l2":
		return -EuclideanDistance(a, b)
	default:
		return CosineSimilarity(a, b)
	}
}

func sqrt(x float32) float32 {
	return float32(math.Sqrt(float64(x)))
}
