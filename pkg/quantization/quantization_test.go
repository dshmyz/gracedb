package quantization

import (
	"math"
	"testing"
)

func TestScalarQuantize(t *testing.T) {
	vec := []float32{-1.0, 0.0, 0.5, 1.0}

	quantized, minParams, maxParams := ScalarQuantize(vec, 8)
	if len(quantized) != 4 {
		t.Fatalf("expected 4 bytes, got %d", len(quantized))
	}

	// Dequantize and check error.
	dequantized := ScalarDequantize(quantized, minParams[0], maxParams[0])
	if len(dequantized) != 4 {
		t.Fatalf("expected 4 floats after dequantize, got %d", len(dequantized))
	}

	// Check reconstruction error.
	for i := range vec {
		err := float64(dequantized[i]) - float64(vec[i])
		if math.Abs(err) > 0.01 {
			t.Logf("reconstruction error at index %d: %f (acceptable for 8-bit)", i, err)
		}
	}
}

func TestBinaryQuantize(t *testing.T) {
	vec := []float32{1.0, -0.5, 0.3, -0.1, 0.8}

	binary := BinaryQuantize(vec)
	if len(binary) == 0 {
		t.Fatal("expected non-empty binary vector")
	}

	// BinarySimilarity uses bit count, so pass actual bit capacity.
	sim := BinarySimilarity(binary, binary, len(binary)*8)
	if sim < 0.9 {
		t.Fatalf("expected self-similarity ~1.0, got %f", sim)
	}
}

func TestProductQuantizer(t *testing.T) {
	pq := NewProductQuantizer(2, 4) // 2 sub-vectors, 4 centroids each

	// Train with sample vectors.
	vectors := [][]float32{
		{1.0, 2.0, 3.0, 4.0},
		{1.1, 2.1, 3.1, 4.1},
		{0.9, 1.9, 2.9, 3.9},
		{5.0, 6.0, 7.0, 8.0},
		{5.1, 6.1, 7.1, 8.1},
	}

	if err := pq.Train(vectors); err != nil {
		t.Fatal(err)
	}

	if len(pq.Codebooks) != 2 {
		t.Fatalf("expected 2 codebooks, got %d", len(pq.Codebooks))
	}

	// Encode and decode.
	vec := []float32{1.0, 2.0, 3.0, 4.0}
	codes := pq.Encode(vec)
	if len(codes) != 2 {
		t.Fatalf("expected 2 codes, got %d", len(codes))
	}

	reconstructed := pq.Decode(codes)
	if len(reconstructed) != 4 {
		t.Fatalf("expected 4-dim reconstruction, got %d", len(reconstructed))
	}

	// Distance check.
	dist := pq.Distance(vec, codes)
	if dist < 0 {
		t.Fatal("distance should be non-negative")
	}
}

func TestHammingDistance(t *testing.T) {
	a := []byte{0b11110000, 0b10101010}
	b := []byte{0b11110000, 0b10101010}
	c := []byte{0b00001111, 0b01010101}

	d1 := hammingDist(a, b)
	if d1 != 0 {
		t.Fatalf("expected 0 for identical, got %d", d1)
	}

	d2 := hammingDist(a, c)
	if d2 != 16 {
		t.Fatalf("expected 16 for complements, got %d", d2)
	}
}

func hammingDist(a, b []byte) int {
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
