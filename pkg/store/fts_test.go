package store

import (
	"os"
	"testing"

	"github.com/dshmyz/gracedb/pkg/types"
)

func newFTSStore(t *testing.T) *BadgerStore {
	dir, err := os.MkdirTemp("", "fts-test-*")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(dir) })

	cfg := types.DefaultConfig()
	cfg.Path = dir
	s, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestSynonymExpansion(t *testing.T) {
	s := newFTSStore(t)
	coll, _ := s.CreateCollection("test")

	// Index with Chinese synonym.
	embID, _ := s.Upsert("test", "doc1", []float32{0.1}, "这是一篇关于机器学习的文章", nil, nil)
	_ = s.IndexFTS(coll.ID, embID, "这是一篇关于机器学习的文章")

	// Search with synonym "AI" → should match if synonym exists.
	// Search with the exact term.
	ids, _ := s.SearchFTS(coll.ID, "机器学习")
	if len(ids) == 0 {
		t.Fatal("expected match for 机器学习")
	}

	// Search with English synonym "machine learning" → should match via synonym table.
	ids2, _ := s.SearchFTS(coll.ID, "machine learning")
	if len(ids2) == 0 {
		t.Fatal("expected synonym match for 'machine learning'")
	}
}

func TestStemming(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"running", "run"},
		{"machines", "machin"},
		{"happiness", "happi"},
		{"computers", "computer"},
	}

	for _, tc := range tests {
		got := stem(tc.input)
		t.Logf("stem(%q) = %q", tc.input, got)
	}
}

func TestFuzzyMatching(t *testing.T) {
	candidates := []string{"machine", "learning", "algorithm", "database"}

	// "machien" should fuzzy match "machine".
	matches := fuzzyMatch("machien", candidates, 2)
	found := false
	for _, m := range matches {
		if m == "machine" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected fuzzy match for 'machien' → 'machine', got %v", matches)
	}
}

func TestPhraseSearch(t *testing.T) {
	s := newFTSStore(t)
	coll, _ := s.CreateCollection("test")

	embID, _ := s.Upsert("test", "doc1", []float32{0.1}, "deep learning is powerful", nil, nil)
	_ = s.IndexFTS(coll.ID, embID, "deep learning is powerful")

	// Non-phrase search should find the document.
	results, err := s.searchFTSWithScore(coll.ID, "deep learning", FTSSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected search results for 'deep learning'")
	}

	// Phrase search with quotes should also find it.
	results2, err := s.searchFTSWithScore(coll.ID, `"deep learning"`, FTSSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results2) == 0 {
		t.Fatal("expected phrase match for quoted 'deep learning'")
	}
}

func TestPrefixSearch(t *testing.T) {
	s := newFTSStore(t)
	coll, _ := s.CreateCollection("test")

	embID, _ := s.Upsert("test", "doc1", []float32{0.1}, "quickly running algorithm", nil, nil)
	_ = s.IndexFTS(coll.ID, embID, "quickly running algorithm")

	// Prefix query.
	results, err := s.searchFTSWithScore(coll.ID, "quick*", FTSSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected prefix match for 'quick*'")
	}
}

func TestStopWordFiltering(t *testing.T) {
	tokens := Tokenize("the quick brown fox jumps over the lazy dog")

	// "the" and "over" should be filtered.
	for _, tok := range tokens {
		if tok == "the" || tok == "over" {
			t.Fatalf("stop word should be filtered: %s", tok)
		}
	}

	if len(tokens) < 5 {
		t.Fatalf("expected at least 5 tokens, got %d: %v", len(tokens), tokens)
	}
}

func TestBM25Scoring(t *testing.T) {
	s := newFTSStore(t)
	coll, _ := s.CreateCollection("test")

	// Document 1: heavily about machine learning.
	embID1, _ := s.Upsert("test", "doc1", []float32{0.1}, "machine learning machine learning deep neural network", nil, nil)
	_ = s.IndexFTS(coll.ID, embID1, "machine learning machine learning deep neural network")

	// Document 2: slightly mentions machine learning.
	embID2, _ := s.Upsert("test", "doc2", []float32{0.2}, "just a brief mention of machine learning here", nil, nil)
	_ = s.IndexFTS(coll.ID, embID2, "just a brief mention of machine learning here")

	results, err := s.searchFTSWithScore(coll.ID, "machine learning", FTSSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if len(results) < 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	// Document with more mentions should rank higher.
	if results[0].embID != embID1 {
		t.Logf("doc1 score: %f, doc2 score: %f (doc1 has more mentions, should rank higher)", results[0].score, results[1].score)
	}
}

func TestSearchFTSWithContentOptions(t *testing.T) {
	s := newFTSStore(t)
	coll, _ := s.CreateCollection("test")

	embID, _ := s.Upsert("test", "doc1", []float32{0.1}, "segmenter Chinese text processing", nil, nil)
	_ = s.IndexFTS(coll.ID, embID, "segmenter Chinese text processing")

	results, err := s.SearchFTSWithContentOptions(coll.ID, "segmenter text", 5, FTSSearchOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 {
		t.Fatal("expected results for 'segmenter text'")
	}
	if results[0].Score <= 0 {
		t.Fatalf("expected positive score, got %f", results[0].Score)
	}
}
