package types

import (
	"math"
	"testing"
)

func TestCosineSimilarity(t *testing.T) {
	// Identical vectors should have similarity 1.
	a := []float32{1, 0, 0}
	b := []float32{1, 0, 0}
	sim := CosineSimilarity(a, b)
	if sim != 1.0 {
		t.Errorf("expected 1.0, got %f", sim)
	}

	// Orthogonal vectors should have similarity 0.
	c := []float32{0, 1, 0}
	sim = CosineSimilarity(a, c)
	if sim != 0.0 {
		t.Errorf("expected 0.0, got %f", sim)
	}

	// Opposite vectors should have similarity -1.
	d := []float32{-1, 0, 0}
	sim = CosineSimilarity(a, d)
	if sim != -1.0 {
		t.Errorf("expected -1.0, got %f", sim)
	}
}

func TestDotProduct(t *testing.T) {
	a := []float32{1, 2, 3}
	b := []float32{4, 5, 6}
	dot := DotProduct(a, b)
	expected := float32(1*4 + 2*5 + 3*6) // 32
	if dot != expected {
		t.Errorf("expected %f, got %f", expected, dot)
	}

	// Mismatched lengths should return 0.
	c := []float32{1, 2}
	dot = DotProduct(a, c)
	if dot != 0 {
		t.Errorf("expected 0 for mismatched lengths, got %f", dot)
	}
}

func TestEuclideanDistance(t *testing.T) {
	a := []float32{0, 0, 0}
	b := []float32{3, 4, 0}
	dist := EuclideanDistance(a, b)
	expected := float32(5.0) // sqrt(9+16)
	if math.Abs(float64(dist-expected)) > 0.001 {
		t.Errorf("expected %f, got %f", expected, dist)
	}

	// Identical vectors should have distance 0.
	c := []float32{1, 2, 3}
	dist = EuclideanDistance(c, c)
	if dist != 0.0 {
		t.Errorf("expected 0.0, got %f", dist)
	}

	// Mismatched lengths should return MaxFloat32.
	d := []float32{1}
	dist = EuclideanDistance(a, d)
	if dist != math.MaxFloat32 {
		t.Errorf("expected MaxFloat32 for mismatched lengths, got %f", dist)
	}
}
