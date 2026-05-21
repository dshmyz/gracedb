package index

import (
	"testing"
)

func TestIVFIndex(t *testing.T) {
	idx := NewIVFIndex(3, 2)

	// Insert vectors.
	vectors := [][]float32{
		{1.0, 0.0, 0.0},
		{0.9, 0.1, 0.0},
		{0.0, 1.0, 0.0},
		{0.0, 0.9, 0.1},
		{0.0, 0.0, 1.0},
		{0.1, 0.0, 0.9},
	}
	ids := []string{"v1", "v2", "v3", "v4", "v5", "v6"}

	for i, v := range vectors {
		idx.Insert(v, ids[i])
	}

	if err := idx.Train(); err != nil {
		t.Fatal(err)
	}

	results, err := idx.Search([]float32{1.0, 0.0, 0.0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}
}

func TestLSHIndex(t *testing.T) {
	idx := NewLSHIndex(8, 4)
	idx.Build(4)

	vectors := [][]float32{
		{1.0, 0.0, 0.0, 0.0},
		{0.9, 0.1, 0.0, 0.0},
		{0.0, 1.0, 0.0, 0.0},
		{0.0, 0.0, 1.0, 0.0},
	}
	ids := []string{"v1", "v2", "v3", "v4"}

	for i, v := range vectors {
		idx.Insert(v, ids[i])
	}

	results, err := idx.Search([]float32{0.95, 0.05, 0.0, 0.0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected LSH search results")
	}
}

func TestAngularDistance(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{1.0, 0.0, 0.0}
	c := []float32{0.0, 1.0, 0.0}

	d1 := AngularDistance(a, b)
	if d1 > 0.001 {
		t.Fatalf("expected ~0 distance for identical vectors, got %f", d1)
	}

	d2 := AngularDistance(a, c)
	if d2 < 0.9 {
		t.Fatalf("expected ~1 distance for orthogonal vectors, got %f", d2)
	}
}

func TestL2Distance(t *testing.T) {
	a := []float32{0.0, 0.0}
	b := []float32{3.0, 4.0}

	d := L2Distance(a, b)
	if d < 4.9 || d > 5.1 {
		t.Fatalf("expected distance ~5.0, got %f", d)
	}
}
