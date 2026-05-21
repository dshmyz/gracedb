package store

import (
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/dgraph-io/badger/v4"
	"github.com/go-ego/gse"
	"github.com/dshmyz/gracedb/pkg/types"
)

const ftsPrefix = "fts:"

// stopWords is a set of common English and Chinese stop words.
var stopWords = map[string]bool{
	// English
	"a": true, "an": true, "the": true, "and": true, "or": true, "but": true,
	"in": true, "on": true, "at": true, "to": true, "for": true, "of": true,
	"is": true, "are": true, "was": true, "were": true, "be": true, "been": true,
	"it": true, "this": true, "that": true, "these": true, "those": true,
	"i": true, "you": true, "he": true, "she": true, "we": true, "they": true,
	"with": true, "from": true, "by": true, "as": true, "not": true, "no": true,
	"over": true, "under": true, "again": true, "further": true, "then": true,
	"once": true, "here": true, "there": true, "when": true, "where": true,
	"why": true, "how": true, "all": true, "each": true, "few": true,
	"more": true, "most": true, "other": true, "some": true, "such": true,
	"only": true, "own": true, "same": true, "than": true, "too": true,
	"very": true, "can": true, "will": true, "just": true, "don": true,
	"should": true, "now": true,
	// Chinese
	"的": true, "了": true, "是": true, "在": true, "和": true, "就": true,
	"都": true, "而": true, "及": true, "与": true, "或": true, "一个": true,
	"没有": true, "我们": true, "你们": true, "他们": true, "什么": true,
	"这个": true, "那个": true, "因为": true, "所以": true,
}

var (
	segmenterOnce sync.Once
	segmenter     *gse.Segmenter
)

// getSegmenter returns a lazily-initialized gse segmenter.
func getSegmenter() *gse.Segmenter {
	segmenterOnce.Do(func() {
		seg := &gse.Segmenter{}
		_ = seg.LoadDict()
		seg.LoadStop()
		segmenter = seg
	})
	return segmenter
}

// Tokenize splits content into tokens using gse for Chinese/English segmentation.
// Each token is stemmed and synonym-expanded for indexing.
func Tokenize(content string) []string {
	seg := getSegmenter()
	words := seg.Cut(content, true)

	tokens := make([]string, 0, len(words))
	seen := make(map[string]bool)
	for _, w := range words {
		w = strings.TrimSpace(w)
		lower := strings.ToLower(w)
		if lower == "" || stopWords[lower] || len(lower) <= 1 {
			continue
		}

		// Apply synonym expansion: index all synonyms.
		synonyms := applySynonymIndexing(lower)
		for _, syn := range synonyms {
			if seen[syn] {
				continue
			}
			seen[syn] = true
			tokens = append(tokens, syn)
		}
	}
	return tokens
}

// FTSSearchOptions controls FTS search behavior.
type FTSSearchOptions struct {
	// Fuzzy enables fuzzy matching with max edit distance.
	FuzzyMaxDist int
	// Phrase requires all query tokens to appear adjacent.
	Phrase bool
	// Synonym enables synonym expansion for query terms.
	Synonym bool
}

// TokenizeForQuery tokenizes a query string with enhanced options.
func TokenizeForQuery(query string, opts FTSSearchOptions) []string {
	query = strings.TrimSpace(query)

	// Check for phrase mode: wrapped in double quotes.
	if strings.HasPrefix(query, "\"") && strings.HasSuffix(query, "\"") {
		query = strings.Trim(query, "\"")
		opts.Phrase = true
	}

	// Check for prefix mode: ends with *
	if strings.HasSuffix(query, "*") {
		base := strings.TrimSuffix(query, "*")
		if base == "" {
			return nil
		}
		return []string{"*:" + strings.ToLower(base)}
	}

	// Detect known canonical phrases in query and expand them.
	var tokens []string
	seen := make(map[string]bool)
	lowerQuery := strings.ToLower(query)

	for canon, syns := range canonicalTerms {
		if strings.Contains(lowerQuery, canon) {
			if !seen[canon] {
				seen[canon] = true
				tokens = append(tokens, canon)
				// Also add all synonyms.
				for _, syn := range syns {
					if !seen[syn] {
						seen[syn] = true
						tokens = append(tokens, syn)
					}
				}
			}
		}
	}

	// Also tokenize normally for other terms.
	gseTokens := tokenizeGSE(query)
	for _, t := range gseTokens {
		if !seen[t] {
			seen[t] = true
			tokens = append(tokens, t)
			// Expand synonyms for single-word tokens.
			for _, syn := range expandSynonyms(t) {
				if !seen[syn] {
					seen[syn] = true
					tokens = append(tokens, syn)
				}
			}
		}
	}

	return tokens
}

// tokenizeGSE segments text using gse, filtering stop words.
func tokenizeGSE(content string) []string {
	seg := getSegmenter()
	words := seg.Cut(content, true)

	tokens := make([]string, 0, len(words))
	for _, w := range words {
		w = strings.TrimSpace(w)
		lower := strings.ToLower(w)
		if lower == "" || stopWords[lower] || len(lower) <= 1 {
			continue
		}
		tokens = append(tokens, lower)
	}
	return tokens
}

// IndexFTS writes inverted index entries for an embedding's content.
// Stores token count as value for BM25 TF scoring.
func (s *BadgerStore) IndexFTS(collectionID, embID, content string) error {
	return s.Update(func(txn *badger.Txn) error {
		// Count from raw gse output before deduplication, then expand synonyms.
		rawCounts := countRawTokens(content)

		// Expand: for each raw token, add its synonym count to the canonical form.
		counts := make(map[string]int)
		for token, count := range rawCounts {
			syns := expandSynonyms(token)
			for _, syn := range syns {
				counts[syn] += count
			}
		}

		for term, count := range counts {
			key := []byte(ftsPrefix + term + ":" + collectionID + ":" + embID)
			if count > 255 {
				count = 255
			}
			if err := txn.Set(key, []byte{byte(count)}); err != nil {
				return err
			}
		}
		return nil
	})
}

// countRawTokens counts token occurrences from gse output (no dedup, no synonym).
func countRawTokens(content string) map[string]int {
	seg := getSegmenter()
	words := seg.Cut(content, true)
	counts := make(map[string]int)
	for _, w := range words {
		w = strings.TrimSpace(w)
		lower := strings.ToLower(w)
		if lower == "" || stopWords[lower] || len(lower) <= 1 {
			continue
		}
		counts[lower]++
	}
	return counts
}

// UnindexFTS removes all FTS entries for an embedding.
func (s *BadgerStore) UnindexFTS(collectionID, embID string) error {
	return s.Update(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(ftsPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		suffix := ":" + collectionID + ":" + embID
		var toDelete [][]byte
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			if strings.HasSuffix(string(key), suffix) {
				k := make([]byte, len(key))
				copy(k, key)
				toDelete = append(toDelete, k)
			}
		}
		for _, k := range toDelete {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

// SearchFTS searches for embeddings containing the given terms.
func (s *BadgerStore) SearchFTS(collectionID string, query string) ([]string, error) {
	return s.SearchFTSWithOptions(collectionID, query, FTSSearchOptions{
		FuzzyMaxDist: 0,
		Synonym:      true,
	})
}

// SearchFTSWithOptions searches with configurable options.
func (s *BadgerStore) SearchFTSWithOptions(collectionID string, query string, opts FTSSearchOptions) ([]string, error) {
	results, err := s.searchFTSWithScore(collectionID, query, opts)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(results))
	for i, r := range results {
		ids[i] = r.embID
	}
	return ids, nil
}

type ftsResult struct {
	embID string
	score float64
}

func (s *BadgerStore) searchFTSWithScore(collectionID string, query string, opts FTSSearchOptions) ([]ftsResult, error) {
	tokens := TokenizeForQuery(query, opts)
	if len(tokens) == 0 {
		return nil, nil
	}

	// Check for phrase mode.
	if opts.Phrase {
		return s.searchPhrase(collectionID, tokens)
	}

	// Collect all indexed terms in this collection for fuzzy matching.
	var allTerms []string
	if opts.FuzzyMaxDist > 0 {
		allTerms = s.collectTerms(collectionID)
	}

	totalDocs, _ := s.EmbeddingCount(collectionID)
	if totalDocs == 0 {
		totalDocs = 1
	}
	avgDocLen := 10.0

	type termMatches struct {
		term      string
		isPrefix  bool
		docFreq   int
		docScores map[string]float64
	}

	var termResults []termMatches

	for _, token := range tokens {
		isPrefix := strings.HasPrefix(token, "*:")
		searchTerm := token
		if isPrefix {
			searchTerm = strings.TrimPrefix(token, "*:")
		}

		docScores := make(map[string]float64)

		// Fuzzy matching: find similar terms.
		var searchTerms []string
		if opts.FuzzyMaxDist > 0 && !isPrefix {
			matches := fuzzyMatch(searchTerm, allTerms, opts.FuzzyMaxDist)
			searchTerms = append(searchTerms, searchTerm) // exact first
			for _, m := range matches {
				if m != searchTerm {
					searchTerms = append(searchTerms, m)
				}
			}
		} else {
			searchTerms = []string{searchTerm}
		}

		for _, st := range searchTerms {
			err := s.View(func(txn *badger.Txn) error {
				if isPrefix {
					opts2 := badger.DefaultIteratorOptions
					opts2.Prefix = []byte(ftsPrefix)
					it := txn.NewIterator(opts2)
					defer it.Close()

					collectionMarker := ":" + collectionID + ":"
					for it.Rewind(); it.Valid(); it.Next() {
						key := it.Item().Key()
						keyStr := string(key)
						if !strings.HasPrefix(keyStr, ftsPrefix) {
							continue
						}
						rest := keyStr[len(ftsPrefix):]
						idx := strings.Index(rest, collectionMarker)
						if idx < 0 {
							continue
						}
						term := rest[:idx]
						if !strings.HasPrefix(term, st) {
							continue
						}
						embID := rest[idx+len(collectionMarker):]
						it.Item().Value(func(val []byte) error {
							if len(val) > 0 {
								docScores[embID] += float64(val[0])
							} else {
								docScores[embID]++
							}
							return nil
						})
					}
				} else {
					prefix := []byte(ftsPrefix + st + ":" + collectionID + ":")
					opts2 := badger.DefaultIteratorOptions
					opts2.Prefix = prefix
					it := txn.NewIterator(opts2)
					defer it.Close()

					suffix := len(prefix)
					for it.Rewind(); it.Valid(); it.Next() {
						key := it.Item().Key()
						embID := string(key[suffix:])
						// Read stored TF count from value.
						it.Item().Value(func(val []byte) error {
							if len(val) > 0 {
								docScores[embID] += float64(val[0])
							} else {
								docScores[embID]++
							}
							return nil
						})
					}
				}
				return nil
			})
			if err != nil {
				return nil, err
			}
		}

		if len(docScores) > 0 {
			termResults = append(termResults, termMatches{
				term:      searchTerm,
				isPrefix:  isPrefix,
				docFreq:   len(docScores),
				docScores: docScores,
			})
		}
	}

	if len(termResults) == 0 {
		return nil, nil
	}

	// BM25 scoring.
	const k1 = 1.2
	const b = 0.75

	scores := make(map[string]float64)
	for _, tr := range termResults {
		// BM25 IDF: log((N - df + 0.5) / (df + 0.5) + 1)
		num := float64(totalDocs) - float64(tr.docFreq) + 0.5
		denom := float64(tr.docFreq) + 0.5
		idf := math.Log(num/denom + 1.0)
		for embID, tf := range tr.docScores {
			docLen := avgDocLen
			tfComponent := tf * (k1 + 1) / (tf + k1*(1-b+b*docLen/avgDocLen))
			scores[embID] += idf * tfComponent
		}
	}

	results := make([]ftsResult, 0, len(scores))
	for embID, score := range scores {
		results = append(results, ftsResult{embID, score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	return results, nil
}

// searchPhrase requires all tokens to appear in the same document.
func (s *BadgerStore) searchPhrase(collectionID string, tokens []string) ([]ftsResult, error) {
	if len(tokens) == 0 {
		return nil, nil
	}

	// Get docs for first token.
	firstScores, err := s.getDocScoresForTerm(collectionID, tokens[0])
	if err != nil || len(firstScores) == 0 {
		return nil, nil
	}

	// Intersect with remaining tokens.
	for _, token := range tokens[1:] {
		otherScores, err := s.getDocScoresForTerm(collectionID, token)
		if err != nil || len(otherScores) == 0 {
			return nil, nil
		}
		for embID := range firstScores {
			if _, ok := otherScores[embID]; !ok {
				delete(firstScores, embID)
			} else {
				firstScores[embID] += otherScores[embID]
			}
		}
	}

	if len(firstScores) == 0 {
		return nil, nil
	}

	results := make([]ftsResult, 0, len(firstScores))
	for embID, score := range firstScores {
		results = append(results, ftsResult{embID, score})
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})
	return results, nil
}

func (s *BadgerStore) getDocScoresForTerm(collectionID, term string) (map[string]float64, error) {
	scores := make(map[string]float64)
	prefix := []byte(ftsPrefix + term + ":" + collectionID + ":")
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		suffix := len(prefix)
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			embID := string(key[suffix:])
			it.Item().Value(func(val []byte) error {
				if len(val) > 0 {
					scores[embID] += float64(val[0])
				} else {
					scores[embID]++
				}
				return nil
			})
		}
		return nil
	})
	return scores, err
}

// collectTerms returns all unique terms indexed in a collection.
func (s *BadgerStore) collectTerms(collectionID string) []string {
	terms := make(map[string]bool)
	collectionMarker := ":" + collectionID + ":"
	s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(ftsPrefix)
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			keyStr := string(key)
			if !strings.HasPrefix(keyStr, ftsPrefix) {
				continue
			}
			rest := keyStr[len(ftsPrefix):]
			idx := strings.Index(rest, collectionMarker)
			if idx < 0 {
				continue
			}
			term := rest[:idx]
			terms[term] = true
		}
		return nil
	})
	out := make([]string, 0, len(terms))
	for t := range terms {
		out = append(out, t)
	}
	return out
}

// SearchFTSWithContent searches and returns scored embedding objects.
func (s *BadgerStore) SearchFTSWithContent(collectionID string, query string, topK int) ([]types.ScoredEmbedding, error) {
	return s.SearchFTSWithContentOptions(collectionID, query, topK, FTSSearchOptions{
		FuzzyMaxDist: 0,
		Synonym:      true,
	})
}

// SearchFTSWithContentOptions searches with configurable options.
func (s *BadgerStore) SearchFTSWithContentOptions(collectionID string, query string, topK int, opts FTSSearchOptions) ([]types.ScoredEmbedding, error) {
	results, err := s.searchFTSWithScore(collectionID, query, opts)
	if err != nil {
		return nil, err
	}

	if topK > 0 && len(results) > topK {
		results = results[:topK]
	}

	pairs := make([]pair, len(results))
	for i, r := range results {
		pairs[i] = pair{r.embID, 0}
	}
	meta, err := s.batchLoadEmbeddings(collectionID, pairs)
	if err != nil {
		return nil, err
	}

	var out []types.ScoredEmbedding
	for _, r := range results {
		emb, ok := meta[r.embID]
		if !ok {
			continue
		}
		out = append(out, types.ScoredEmbedding{
			Embedding: *emb,
			Score:     float32(r.score),
		})
	}
	return out, nil
}

func intersect(sets [][]string) []string {
	if len(sets) == 0 {
		return nil
	}
	if len(sets) == 1 {
		return sets[0]
	}

	base := make(map[string]struct{})
	for _, id := range sets[0] {
		base[id] = struct{}{}
	}

	for _, set := range sets[1:] {
		next := make(map[string]struct{})
		for _, id := range set {
			if _, ok := base[id]; ok {
				next[id] = struct{}{}
			}
		}
		base = next
		if len(base) == 0 {
			return nil
		}
	}

	result := make([]string, 0, len(base))
	for id := range base {
		result = append(result, id)
	}
	sort.Strings(result)
	return result
}
