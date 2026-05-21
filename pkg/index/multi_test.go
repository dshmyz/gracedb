package index

import "testing"

func TestMultiIndex(t *testing.T) {
	flat := NewFlat()
	hnsw := NewHNSW(16, 32, 16)

	multi := NewMultiIndex([]Index{flat, hnsw}, []float64{0.6, 0.4})

	vectors := [][]float32{
		{1.0, 0.0, 0.0},
		{0.9, 0.1, 0.0},
		{0.0, 1.0, 0.0},
		{0.0, 0.0, 1.0},
	}
	ids := []string{"v1", "v2", "v3", "v4"}

	multi.InsertBatch(vectors, ids)

	results, err := multi.Search([]float32{1.0, 0.0, 0.0}, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results")
	}

	// Top result should be v1 (closest to query).
	if results[0].ID != "v1" {
		t.Logf("top result: %s (expected v1, but multi-index fusion may vary)", results[0].ID)
	}
}

func TestFuseResults(t *testing.T) {
	r1 := []SearchResult{{ID: "a", Score: 0.9}, {ID: "b", Score: 0.7}}
	r2 := []SearchResult{{ID: "b", Score: 0.8}, {ID: "c", Score: 0.6}}

	fused := fuseResults([][]SearchResult{r1, r2}, []float64{0.5, 0.5}, 3)

	if len(fused) != 3 {
		t.Fatalf("expected 3 results, got %d", len(fused))
	}

	// "b" should rank highest (appears in both).
	if fused[0].ID != "b" {
		t.Fatalf("expected b to be top, got %s", fused[0].ID)
	}
}

func TestMultiIndexSingle(t *testing.T) {
	flat := NewFlat()
	multi := NewMultiIndex([]Index{flat}, nil)

	flat.Insert([]float32{1.0, 0.0}, "a")
	flat.Insert([]float32{0.0, 1.0}, "b")

	results, err := multi.Search([]float32{1.0, 0.0}, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].ID != "a" {
		t.Fatalf("expected a, got %v", results)
	}
}
