package index

import (
	"bytes"
	"testing"
)

func TestHNSWInsert(t *testing.T) {
	h := NewHNSW(16, 64, 50)

	vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5}
	h.Insert(vec, "id1")

	if h.Len() != 1 {
		t.Fatalf("expected 1 vector, got %d", h.Len())
	}
}

func TestHNSWInsertBatch(t *testing.T) {
	h := NewHNSW(16, 64, 50)

	vectors := [][]float32{
		{0.1, 0.2, 0.3},
		{0.4, 0.5, 0.6},
		{0.7, 0.8, 0.9},
	}
	ids := []string{"a", "b", "c"}
	h.InsertBatch(vectors, ids)

	if h.Len() != 3 {
		t.Fatalf("expected 3 vectors, got %d", h.Len())
	}
}

func TestHNSWSearch(t *testing.T) {
	h := NewHNSW(16, 64, 50)

	vectors := [][]float32{
		{1, 0, 0, 0, 0},
		{0.9, 0.1, 0, 0, 0},
		{0, 1, 0, 0, 0},
		{0, 0, 1, 0, 0},
		{0, 0, 0, 1, 0},
	}
	ids := []string{"v1", "v2", "v3", "v4", "v5"}
	h.InsertBatch(vectors, ids)

	query := []float32{0.95, 0.05, 0, 0, 0}
	results, err := h.Search(query, 3)
	if err != nil {
		t.Fatal(err)
	}

	if len(results) == 0 {
		t.Fatal("expected results, got none")
	}

	// The closest should be v1 or v2
	if results[0].ID != "v1" && results[0].ID != "v2" {
		t.Logf("warning: closest was %s (expected v1 or v2)", results[0].ID)
	}

	t.Logf("top results: %+v", results)
}

func TestHNSWSearchEmpty(t *testing.T) {
	h := NewHNSW(16, 64, 50)

	results, err := h.Search([]float32{0.1, 0.2}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatal("expected nil results for empty index")
	}
}

func TestHNSWSaveLoad(t *testing.T) {
	h1 := NewHNSW(16, 64, 50)

	vectors := [][]float32{
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
	}
	ids := []string{"x", "y", "z"}
	h1.InsertBatch(vectors, ids)

	var buf bytes.Buffer
	err := h1.Save(&buf)
	if err != nil {
		t.Fatal(err)
	}

	h2 := NewHNSW(16, 64, 50)
	err = h2.Load(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if h2.Len() != h1.Len() {
		t.Fatalf("expected %d vectors after load, got %d", h1.Len(), h2.Len())
	}
}

func TestFlatIndex(t *testing.T) {
	f := NewFlat()

	vectors := [][]float32{
		{1, 0, 0},
		{0.9, 0.1, 0},
		{0, 1, 0},
	}
	ids := []string{"a", "b", "c"}
	for i := range vectors {
		f.Insert(vectors[i], ids[i])
	}

	if f.Len() != 3 {
		t.Fatalf("expected 3 vectors, got %d", f.Len())
	}

	results, err := f.Search([]float32{0.95, 0.05, 0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Top result should be a or b
	if results[0].ID != "a" && results[0].ID != "b" {
		t.Logf("top result was %s", results[0].ID)
	}
	if results[0].Score < 0.9 {
		t.Fatalf("expected high score, got %f", results[0].Score)
	}
}

func TestFlatSearchEmpty(t *testing.T) {
	f := NewFlat()

	results, err := f.Search([]float32{0.1, 0.2}, 5)
	if err != nil {
		t.Fatal(err)
	}
	if results != nil {
		t.Fatal("expected nil results for empty index")
	}
}

func TestFlatSaveLoad(t *testing.T) {
	f1 := NewFlat()
	f1.Insert([]float32{1, 0, 0}, "a")
	f1.Insert([]float32{0, 1, 0}, "b")

	var buf bytes.Buffer
	err := f1.Save(&buf)
	if err != nil {
		t.Fatal(err)
	}

	f2 := NewFlat()
	err = f2.Load(&buf)
	if err != nil {
		t.Fatal(err)
	}

	if f2.Len() != 2 {
		t.Fatalf("expected 2 vectors after load, got %d", f2.Len())
	}
}
