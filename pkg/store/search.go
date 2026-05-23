package store

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dshmyz/gracedb/pkg/index"
	"github.com/dshmyz/gracedb/pkg/types"
)

type pair struct {
	embID string
	score float32
}

// Search performs vector similarity search, FTS, or hybrid RRF-fused search.
// Context from opts is checked during long-running flat scan iterations.
func (s *BadgerStore) Search(collectionName string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error) {
	start := time.Now()

	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}
	collectionID := coll.ID

	useVector := opts.UseVectorSearch
	useText := opts.UseTextSearch

	// Default: use vector search if query vector is provided
	if !useVector && !useText {
		if len(query) > 0 {
			useVector = true
		}
	}

	ctx := opts.Context
	if ctx == nil {
		ctx = context.Background()
	}

	var vectorResults []types.ScoredEmbedding
	var ftsResults []types.ScoredEmbedding

	if useVector && len(query) > 0 {
		vectorResults, err = s.vectorSearch(ctx, collectionID, query, opts)
		if err != nil {
			return nil, err
		}
	}

	if useText && opts.Collection != "" {
		ftsResults, err = s.SearchFTSWithContent(collectionID, opts.Collection, opts.TopK)
		if err != nil {
			return nil, err
		}
	}

	// If only one search type, apply reranker and return.
	if !useVector || len(vectorResults) == 0 {
		if opts.Reranker != nil && len(ftsResults) > 0 {
			return opts.Reranker.Rerank(opts.QueryText, ftsResults)
		}
		return ftsResults, nil
	}
	if !useText || len(ftsResults) == 0 {
		if opts.Reranker != nil && len(vectorResults) > 0 {
			return opts.Reranker.Rerank(opts.QueryText, vectorResults)
		}
		return vectorResults, nil
	}

	// Hybrid: RRF fusion
	results := rrfFusion(s, collectionID, vectorResults, ftsResults, opts.TopK)

	// Apply reranker if configured.
	if opts.Reranker != nil && len(results) > 0 {
		return opts.Reranker.Rerank(opts.QueryText, results)
	}

	// Log slow queries.
	if s.config.SlowQueryThreshold > 0 {
		elapsed := time.Since(start)
		if elapsed >= s.config.SlowQueryThreshold {
			slog.Warn("slow search",
				"collection", collectionName,
				"elapsed", elapsed,
				"topK", opts.TopK,
			)
		}
	}

	return results, nil
}

// vectorSearch performs vector similarity search using in-memory index or flat scan.
// Respects ctx cancellation during flat scan iteration.
func (s *BadgerStore) vectorSearch(ctx context.Context, collectionID string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error) {
	var rawResults []types.ScoredEmbedding

	// Try in-memory index first.
	if idx, ok := s.indexes[collectionID]; ok && idx.Len() > 0 {
		searchResults, err := idx.Search(query, opts.TopK)
		if err != nil {
			return nil, err
		}
		if len(searchResults) == 0 {
			return nil, nil
		}
		// Load metadata for candidates.
		pairs := make([]pair, 0, len(searchResults))
		for _, r := range searchResults {
			pairs = append(pairs, pair{r.ID, r.Score})
		}
		meta, err := s.batchLoadEmbeddings(collectionID, pairs)
		if err != nil {
			return nil, err
		}
		for _, p := range pairs {
			emb, ok := meta[p.embID]
			if !ok {
				continue
			}
			if opts.DocID != "" && emb.DocID != opts.DocID {
				continue
			}
			if len(opts.ACL) > 0 && !hasACLMatch(emb.ACL, opts.ACL) {
				continue
			}
			if len(opts.MetadataFilter) > 0 && !hasMetadataMatch(emb.Metadata, opts.MetadataFilter, opts.MetadataExists) {
				continue
			}
			rawResults = append(rawResults, types.ScoredEmbedding{
				Embedding: *emb,
				Score:     p.score,
			})
		}
		return rawResults, nil
	}

	// Fallback: flat scan from Badger.
	vectors, err := s.ReadVectors(collectionID)
	if err != nil {
		return nil, err
	}
	if len(vectors) == 0 {
		return nil, nil
	}

	pairs := make([]pair, 0, len(vectors))
	for embID, vec := range vectors {
		// Check context cancellation periodically during flat scan.
		if len(pairs)%100 == 0 {
			if err := ctx.Err(); err != nil {
				return nil, err
			}
		}

		score := CosineSimilarity(query, vec)
		if score >= opts.Threshold {
			pairs = append(pairs, pair{embID, score})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})
	if opts.TopK > 0 && len(pairs) > opts.TopK {
		pairs = pairs[:opts.TopK]
	}

	// Batch-load all candidate metadata in a single transaction.
	meta, err := s.batchLoadEmbeddings(collectionID, pairs)
	if err != nil {
		return nil, err
	}

	for _, p := range pairs {
		emb, ok := meta[p.embID]
		if !ok {
			continue
		}
		if opts.DocID != "" && emb.DocID != opts.DocID {
			continue
		}
		if len(opts.ACL) > 0 && !hasACLMatch(emb.ACL, opts.ACL) {
			continue
		}
		if len(opts.MetadataFilter) > 0 && !hasMetadataMatch(emb.Metadata, opts.MetadataFilter, opts.MetadataExists) {
			continue
		}
		emb.Vector = vectors[p.embID]
		rawResults = append(rawResults, types.ScoredEmbedding{
			Embedding: *emb,
			Score:     p.score,
		})
	}
	return rawResults, nil
}

// batchLoadEmbeddings loads multiple embeddings in a single DB transaction.
func (s *BadgerStore) batchLoadEmbeddings(collectionID string, pairs []pair) (map[string]*types.Embedding, error) {
	meta := make(map[string]*types.Embedding)
	err := s.View(func(txn *badger.Txn) error {
		for _, p := range pairs {
			item, err := txn.Get([]byte(fmt.Sprintf("%s%s:%s", embPrefix, collectionID, p.embID)))
			if err != nil {
				continue
			}
			emb := &types.Embedding{}
			if err := item.Value(func(val []byte) error {
				return unmarshal(cloneBytes(val), emb)
			}); err != nil {
				continue
			}
			meta[p.embID] = emb
		}
		return nil
	})
	return meta, err
}

func hasACLMatch(embACL, filterACL []string) bool {
	if len(embACL) == 0 {
		return false
	}
	set := make(map[string]struct{}, len(embACL))
	for _, a := range embACL {
		set[a] = struct{}{}
	}
	for _, a := range filterACL {
		if _, ok := set[a]; ok {
			return true
		}
	}
	return false
}

func hasMetadataMatch(meta map[string]string, filter map[string]string, exists []string) bool {
	for k, v := range filter {
		if meta[k] != v {
			return false
		}
	}
	for _, k := range exists {
		if _, ok := meta[k]; !ok {
			return false
		}
	}
	return true
}

// rrfFusion combines vector and FTS results using Reciprocal Rank Fusion.
func rrfFusion(s *BadgerStore, collectionID string, vectorResults, ftsResults []types.ScoredEmbedding, topK int) []types.ScoredEmbedding {
	const k = 60.0

	scores := make(map[string]float32)
	order := make([]string, 0)
	seen := make(map[string]bool)

	for i, r := range vectorResults {
		score := float32(1.0 / (k + float64(i+1)))
		if _, ok := scores[r.ID]; ok {
			scores[r.ID] += score
		} else {
			scores[r.ID] = score
			order = append(order, r.ID)
			seen[r.ID] = true
		}
	}

	for i, r := range ftsResults {
		score := float32(1.0 / (k + float64(i+1)))
		if _, ok := scores[r.ID]; ok {
			scores[r.ID] += score
		} else {
			scores[r.ID] = score
			if !seen[r.ID] {
				order = append(order, r.ID)
				seen[r.ID] = true
			}
		}
	}

	sort.Slice(order, func(i, j int) bool {
		return scores[order[i]] > scores[order[j]]
	})

	if topK > 0 && len(order) > topK {
		order = order[:topK]
	}

	var results []types.ScoredEmbedding
	for _, id := range order {
		var found bool
		// Try to get from existing results first
		for _, r := range vectorResults {
			if r.ID == id {
				r.Score = scores[id]
				results = append(results, r)
				found = true
				break
			}
		}
		if found {
			continue
		}
		for _, r := range ftsResults {
			if r.ID == id {
				r.Score = scores[id]
				results = append(results, r)
				found = true
				break
			}
		}
		if found {
			continue
		}
		// Fallback: fetch from store
		emb, err := s.GetEmbedding(collectionID, id, false)
		if err == nil {
			results = append(results, types.ScoredEmbedding{
				Embedding: *emb,
				Score:     scores[id],
			})
		}
	}
	return results
}

// LoadIndex builds an in-memory index for a collection, loading from snapshot or rebuilding.
func (s *BadgerStore) LoadIndex(collectionName string) error {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return err
	}

	idx := s.newIndex()
	if idx == nil {
		return nil
	}

	// Try loading snapshot from Badger.
	if data, err := s.loadIndexSnapshot(coll.ID); err == nil && len(data) > 0 {
		if err := idx.Unmarshal(data); err == nil {
			s.mu.Lock()
			s.indexes[coll.ID] = idx
			s.mu.Unlock()
			return nil
		}
	}

	// Rebuild from stored vectors.
	vectors, err := s.ReadVectors(coll.ID)
	if err != nil {
		return err
	}
	for embID, vec := range vectors {
		idx.Insert(vec, embID)
	}
	s.mu.Lock()
	s.indexes[coll.ID] = idx
	s.mu.Unlock()
	return nil
}

// SaveIndex persists an in-memory index snapshot to Badger.
func (s *BadgerStore) SaveIndex(collectionName string) error {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return err
	}

	idx, ok := s.indexes[coll.ID]
	if !ok {
		return nil
	}

	data, err := idx.Marshal()
	if err != nil {
		return err
	}

	return s.Update(func(txn *badger.Txn) error {
		key := []byte("idx:snapshot:" + coll.ID)
		return txn.Set(key, data)
	})
}

// RebuildVectorIndex drops the in-memory vector index for a collection and
// rebuilds it from all stored vectors. Useful after restoring from backup or
// recovering from index corruption.
func (s *BadgerStore) RebuildVectorIndex(collectionName string) error {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return err
	}

	// Remove existing index.
	s.mu.Lock()
	delete(s.indexes, coll.ID)
	s.mu.Unlock()

	// Create fresh index.
	idx := s.newIndex()
	if idx == nil {
		return nil
	}

	// Reload all vectors.
	vectors, err := s.ReadVectors(coll.ID)
	if err != nil {
		return err
	}

	for embID, vec := range vectors {
		idx.Insert(vec, embID)
	}

	s.mu.Lock()
	s.indexes[coll.ID] = idx
	s.mu.Unlock()

	slog.Info("vector index rebuilt",
		"collection", collectionName,
		"vectors", len(vectors),
	)
	return nil
}

func (s *BadgerStore) newIndex() index.Index {
	// Multi-index mode: combine multiple index types.
	if len(s.idxTypes) > 1 {
		indexes := make([]index.Index, len(s.idxTypes))
		for i, t := range s.idxTypes {
			indexes[i] = s.makeIndex(t)
		}
		return index.NewMultiIndex(indexes, nil)
	}
	return s.makeIndex(s.idxType)
}

func (s *BadgerStore) makeIndex(idxType string) index.Index {
	switch idxType {
	case "hnsw":
		cfg := s.config.HNSWConfig
		if cfg == nil {
			cfg = &types.HNSWConfig{M: 16, EfConstruction: 64, EfSearch: 50}
		}
		return index.NewHNSW(cfg.M, cfg.EfConstruction, cfg.EfSearch)
	case "ivf":
		return index.NewIVFIndex(10, 3)
	case "lsh":
		return index.NewLSHIndex(16, 4)
	default:
		return index.NewFlat()
	}
}

const idxSnapshotPrefix = "idx:snapshot:"

func (s *BadgerStore) loadIndexSnapshot(collectionID string) ([]byte, error) {
	var data []byte
	err := s.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(idxSnapshotPrefix + collectionID))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			data = cloneBytes(val)
			return nil
		})
	})
	return data, err
}

// DeleteBatch deletes multiple embeddings by ID, including FTS index entries.
func (s *BadgerStore) DeleteBatch(collectionID string, ids []string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Remove from in-memory index.
	if idx, ok := s.indexes[collectionID]; ok {
		for _, id := range ids {
			idx.RemoveVector(id)
		}
	}

	return s.Update(func(txn *badger.Txn) error {
		for _, id := range ids {
			keys := [][]byte{
				[]byte(embPrefix + collectionID + ":" + id),
				[]byte(embVecPrefix + collectionID + ":" + id),
				[]byte(embContPrefix + collectionID + ":" + id),
			}
			for _, k := range keys {
				if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
					return err
				}
			}
			// Clean FTS entries within the same transaction.
			if err := deleteFTSEntries(txn, collectionID, id); err != nil {
				return err
			}
		}
		return nil
	})
}
