package store

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/dshmyz/gracedb/pkg/types"
	"github.com/google/uuid"
)

const (
	memPrefix     = "mem:"
	memVecPrefix  = "mem:vec:"
	memFTSPrefix  = "mem:fts:"
	memContPrefix = "mem:content:"
)

// resolveMemoryBucket determines the bucket ID for a memory.
func resolveMemoryBucket(req *types.MemorySaveRequest) (scope, bucketID string, err error) {
	namespace := req.Namespace
	if namespace == "" {
		namespace = "default"
	}
	scope = strings.ToLower(strings.TrimSpace(req.Scope))

	switch scope {
	case "":
		if strings.TrimSpace(req.SessionID) != "" {
			scope = types.MemoryScopeSession
		} else if strings.TrimSpace(req.UserID) != "" {
			scope = types.MemoryScopeUser
		} else {
			scope = types.MemoryScopeGlobal
		}
	case types.MemoryScopeGlobal, types.MemoryScopeUser, types.MemoryScopeSession:
	default:
		return "", "", fmt.Errorf("unsupported memory scope: %s", scope)
	}

	switch scope {
	case types.MemoryScopeGlobal:
		return scope, fmt.Sprintf("memory:%s:%s", scope, namespace), nil
	case types.MemoryScopeUser:
		if strings.TrimSpace(req.UserID) == "" {
			return "", "", fmt.Errorf("user_id is required for %s scope", scope)
		}
		return scope, fmt.Sprintf("memory:%s:%s:%s", scope, req.UserID, namespace), nil
	case types.MemoryScopeSession:
		if strings.TrimSpace(req.SessionID) == "" {
			return "", "", fmt.Errorf("session_id is required for %s scope", scope)
		}
		return scope, fmt.Sprintf("memory:%s:%s:%s", scope, req.SessionID, namespace), nil
	}
	return "", "", fmt.Errorf("unsupported scope: %s", scope)
}

// SaveMemory stores a memory record.
func (s *BadgerStore) SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error) {
	if req.MemoryID == "" {
		req.MemoryID = uuid.New().String()
	}
	if strings.TrimSpace(req.Content) == "" {
		return nil, fmt.Errorf("memory content cannot be empty")
	}

	scope, bucketID, err := resolveMemoryBucket(&req)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	metadata := cloneAnyMap(req.Metadata)
	metadata["kind"] = "memory"
	metadata["user_id"] = req.UserID
	metadata["session_id"] = req.SessionID
	metadata["scope"] = scope
	metadata["namespace"] = req.Namespace
	if metadata["namespace"] == "" {
		metadata["namespace"] = "default"
	}
	role := req.Role
	if role == "" {
		role = "memory"
	}
	metadata["role"] = role
	metadata["importance"] = req.Importance
	metadata["ttl_seconds"] = req.TTLSeconds
	metadata["created_at"] = now.Format(time.RFC3339Nano)
	metadata["updated_at"] = now.Format(time.RFC3339Nano)

	var expiresAt *time.Time
	if req.TTLSeconds > 0 {
		exp := now.Add(time.Duration(req.TTLSeconds) * time.Second)
		expiresAt = &exp
		metadata["expires_at"] = exp.Format(time.RFC3339)
	}

	metadataJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	record := &types.MemoryRecord{
		ID:         req.MemoryID,
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		Scope:      scope,
		Namespace:  metadata["namespace"].(string),
		Role:       role,
		Content:    req.Content,
		Vector:     req.Vector,
		Metadata:   metadata,
		Importance: req.Importance,
		TTLSeconds: req.TTLSeconds,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	err = s.Update(func(txn *badger.Txn) error {
		memKey := []byte(fmt.Sprintf("%s%s:%s", memPrefix, bucketID, req.MemoryID))
		if err := txn.Set(memKey, metadataJSON); err != nil {
			return err
		}

		// Also store full content in a content key.
		contentKey := []byte(fmt.Sprintf("%s%s:%s", memContPrefix, bucketID, req.MemoryID))
		if err := txn.Set(contentKey, []byte(req.Content)); err != nil {
			return err
		}
		if err := deleteMemoryFTSEntries(txn, bucketID, req.MemoryID); err != nil {
			return err
		}
		if err := indexMemoryFTSTxn(txn, bucketID, req.MemoryID, req.Content); err != nil {
			return err
		}
		if len(req.Vector) > 0 {
			vecKey := []byte(fmt.Sprintf("%s%s:%s", memVecPrefix, bucketID, req.MemoryID))
			if err := txn.Set(vecKey, vectorToBytes(req.Vector)); err != nil {
				return err
			}
		}

		// Index: memoryID -> bucketID for lookup by ID alone.
		idxKey := []byte(fmt.Sprintf("mem:idx:%s", req.MemoryID))
		return txn.Set(idxKey, []byte(bucketID))
	})
	if err != nil {
		return nil, err
	}
	if len(req.Vector) > 0 {
		s.upsertMemoryIndex(bucketID, req.MemoryID, req.Vector)
	}
	return record, err
}

// GetMemory fetches a memory record by ID.
func (s *BadgerStore) GetMemory(memoryID string) (*types.MemoryRecord, error) {
	var record *types.MemoryRecord
	err := s.View(func(txn *badger.Txn) error {
		var err error
		record, err = s.loadMemory(txn, memoryID)
		return err
	})
	return record, err
}

// UpdateMemory updates a memory record.
func (s *BadgerStore) UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var record *types.MemoryRecord
	err := s.Update(func(txn *badger.Txn) error {
		var err error
		record, err = s.loadMemory(txn, req.MemoryID)
		if err != nil {
			return err
		}

		if req.Content != nil {
			record.Content = *req.Content
		}
		if req.Vector != nil {
			record.Vector = req.Vector
		}
		if req.Importance != nil {
			record.Importance = *req.Importance
		}
		if req.TTLSeconds != nil {
			record.TTLSeconds = *req.TTLSeconds
		}
		if req.Metadata != nil {
			for k, v := range req.Metadata {
				record.Metadata[k] = v
			}
		}

		// Recalculate expires_at.
		if record.TTLSeconds > 0 {
			exp := time.Now().Add(time.Duration(record.TTLSeconds) * time.Second)
			record.ExpiresAt = &exp
			record.Metadata["expires_at"] = exp.Format(time.RFC3339)
		}

		record.Metadata["importance"] = record.Importance
		record.Metadata["ttl_seconds"] = record.TTLSeconds
		record.UpdatedAt = time.Now()
		record.Metadata["updated_at"] = record.UpdatedAt.Format(time.RFC3339Nano)

		metadataJSON, err := json.Marshal(record.Metadata)
		if err != nil {
			return err
		}

		bucketID, err := extractBucketIDFromMemoryID(txn, req.MemoryID)
		if err != nil {
			return err
		}
		memKey := []byte(fmt.Sprintf("%s%s:%s", memPrefix, bucketID, req.MemoryID))
		if err := txn.Set(memKey, metadataJSON); err != nil {
			return err
		}

		contentKey := []byte(fmt.Sprintf("%s%s:%s", memContPrefix, bucketID, req.MemoryID))
		if err := txn.Set(contentKey, []byte(record.Content)); err != nil {
			return err
		}
		if req.Content != nil {
			if err := deleteMemoryFTSEntries(txn, bucketID, req.MemoryID); err != nil {
				return err
			}
			if err := indexMemoryFTSTxn(txn, bucketID, req.MemoryID, record.Content); err != nil {
				return err
			}
		}
		vecKey := []byte(fmt.Sprintf("%s%s:%s", memVecPrefix, bucketID, req.MemoryID))
		if req.Vector != nil {
			return txn.Set(vecKey, vectorToBytes(req.Vector))
		}
		return nil
	})
	if err == nil && req.Vector != nil && record != nil {
		bucketID, bucketErr := s.memoryBucketID(record.ID)
		if bucketErr == nil {
			s.upsertMemoryIndex(bucketID, record.ID, req.Vector)
		}
	}
	return record, err
}

// DeleteMemory removes a memory record by ID.
func (s *BadgerStore) DeleteMemory(memoryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		bucketID, err := extractBucketIDFromMemoryID(txn, memoryID)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return types.ErrNotFound
			}
			return err
		}
		keys := [][]byte{
			[]byte(fmt.Sprintf("%s%s:%s", memPrefix, bucketID, memoryID)),
			[]byte(fmt.Sprintf("%s%s:%s", memContPrefix, bucketID, memoryID)),
			[]byte(fmt.Sprintf("%s%s:%s", memVecPrefix, bucketID, memoryID)),
			[]byte(fmt.Sprintf("mem:idx:%s", memoryID)),
		}
		for _, k := range keys {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		if err := deleteMemoryFTSEntries(txn, bucketID, memoryID); err != nil {
			return err
		}
		if idx, ok := s.memoryIndexes[bucketID]; ok {
			idx.RemoveVector(memoryID)
		}
		return nil
	})
}

// SearchMemory searches memories in a bucket using semantic and lexical signals.
func (s *BadgerStore) SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error) {
	scope, bucketID, err := resolveMemoryBucket(&types.MemorySaveRequest{
		UserID:    req.UserID,
		SessionID: req.SessionID,
		Scope:     req.Scope,
		Namespace: req.Namespace,
	})
	if err != nil {
		return nil, err
	}
	_ = scope

	if req.TopK <= 0 {
		req.TopK = 5
	}

	hasLexicalQuery := strings.TrimSpace(req.Query) != ""
	if !hasLexicalQuery && len(req.QueryVector) == 0 {
		return &types.MemorySearchResponse{Query: req.Query}, nil
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}
	weights := memorySearchWeights(req)

	type memoryScore struct {
		memoryID        string
		finalScore      float64
		semanticScore   float64
		lexicalScore    float64
		importanceScore float64
		recencyScore    float64
	}
	matched := make(map[string]*memoryScore)
	ensureScore := func(memoryID string) *memoryScore {
		score, ok := matched[memoryID]
		if !ok {
			score = &memoryScore{memoryID: memoryID}
			matched[memoryID] = score
		}
		return score
	}

	if len(req.QueryVector) > 0 {
		vectorScores, err := s.searchMemoryVectors(ctx, bucketID, req.QueryVector, req.TopK*4)
		if err != nil {
			return nil, err
		}
		for memoryID, score := range vectorScores {
			ms := ensureScore(memoryID)
			ms.semanticScore = float64(score)
		}
	}

	if hasLexicalQuery {
		lexicalScores, err := s.searchMemoryFTS(ctx, bucketID, req.Query)
		if err != nil {
			return nil, err
		}
		for memoryID, score := range lexicalScores {
			ms := ensureScore(memoryID)
			ms.lexicalScore = score
		}
	}

	if len(matched) == 0 {
		return &types.MemorySearchResponse{Query: req.Query}, nil
	}

	candidates := make([]*memoryScore, 0, len(matched))
	records := make(map[string]*types.MemoryRecord, len(matched))
	now := time.Now()
	for _, score := range matched {
		var rec *types.MemoryRecord
		err := s.View(func(txn *badger.Txn) error {
			var err error
			rec, err = s.loadMemory(txn, score.memoryID)
			return err
		})
		if err != nil || rec == nil || memoryExpired(rec) {
			continue
		}
		score.importanceScore = clamp01(rec.Importance)
		score.recencyScore = memoryRecencyScore(rec, now, memoryRecencyHalfLife(req))
		score.finalScore = score.semanticScore*weights.semantic +
			score.lexicalScore*weights.lexical +
			score.importanceScore*weights.importance +
			score.recencyScore*weights.recency
		records[score.memoryID] = rec
		candidates = append(candidates, score)
	}
	if len(candidates) == 0 {
		return &types.MemorySearchResponse{Query: req.Query}, nil
	}

	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].finalScore > candidates[j].finalScore
	})
	if len(candidates) > req.TopK {
		candidates = candidates[:req.TopK]
	}

	var results []types.MemorySearchHit
	for _, sm := range candidates {
		rec := records[sm.memoryID]
		if rec == nil {
			continue
		}
		results = append(results, types.MemorySearchHit{
			Memory:          *rec,
			Score:           sm.finalScore,
			FinalScore:      sm.finalScore,
			SemanticScore:   sm.semanticScore,
			LexicalScore:    sm.lexicalScore,
			ImportanceScore: sm.importanceScore,
			RecencyScore:    sm.recencyScore,
		})
	}

	return &types.MemorySearchResponse{
		Query:   req.Query,
		Results: results,
	}, nil
}

func (s *BadgerStore) loadMemory(txn *badger.Txn, memoryID string) (*types.MemoryRecord, error) {
	// Look up bucket ID via index.
	idxKey := []byte(fmt.Sprintf("mem:idx:%s", memoryID))
	idxItem, err := txn.Get(idxKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, types.ErrNotFound
		}
		return nil, err
	}

	var bucketID string
	idxItem.Value(func(val []byte) error {
		bucketID = string(cloneBytes(val))
		return nil
	})

	memKey := []byte(fmt.Sprintf("%s%s:%s", memPrefix, bucketID, memoryID))
	item, err := txn.Get(memKey)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, types.ErrNotFound
		}
		return nil, err
	}

	var metadata map[string]any
	err = item.Value(func(val []byte) error {
		return json.Unmarshal(val, &metadata)
	})
	if err != nil {
		return nil, err
	}

	// Load content.
	contentKey := []byte(fmt.Sprintf("%s%s:%s", memContPrefix, bucketID, memoryID))
	var content string
	cItem, cErr := txn.Get(contentKey)
	if cErr == nil {
		cItem.Value(func(val []byte) error {
			content = string(cloneBytes(val))
			return nil
		})
	}
	var vector []float32
	vecKey := []byte(fmt.Sprintf("%s%s:%s", memVecPrefix, bucketID, memoryID))
	if vItem, vErr := txn.Get(vecKey); vErr == nil {
		vItem.Value(func(val []byte) error {
			vector = bytesToVector(cloneBytes(val))
			return nil
		})
	}

	record := &types.MemoryRecord{
		ID:      memoryID,
		Content: content,
		Vector:  vector,
	}

	if v, ok := metadata["user_id"]; ok {
		if s, ok := v.(string); ok {
			record.UserID = s
		}
	}
	if v, ok := metadata["session_id"]; ok {
		if s, ok := v.(string); ok {
			record.SessionID = s
		}
	}
	if v, ok := metadata["scope"]; ok {
		if s, ok := v.(string); ok {
			record.Scope = s
		}
	}
	if v, ok := metadata["namespace"]; ok {
		if s, ok := v.(string); ok {
			record.Namespace = s
		}
	}
	if v, ok := metadata["role"]; ok {
		if s, ok := v.(string); ok {
			record.Role = s
		}
	}
	if v, ok := metadata["importance"]; ok {
		switch f := v.(type) {
		case float64:
			record.Importance = f
		case int:
			record.Importance = float64(f)
		}
	}
	if v, ok := metadata["ttl_seconds"]; ok {
		switch n := v.(type) {
		case float64:
			record.TTLSeconds = int(n)
		case int:
			record.TTLSeconds = n
		}
	}
	if v, ok := metadata["expires_at"]; ok {
		if s, ok := v.(string); ok && s != "" {
			if t, err := time.Parse(time.RFC3339, s); err == nil {
				record.ExpiresAt = &t
			}
		}
	}
	if v, ok := metadata["created_at"]; ok {
		if s, ok := v.(string); ok && s != "" {
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
				record.CreatedAt = t
			}
		}
	}
	if v, ok := metadata["updated_at"]; ok {
		if s, ok := v.(string); ok && s != "" {
			if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
				record.UpdatedAt = t
			}
		}
	}
	record.Metadata = metadata

	return record, nil
}

func (s *BadgerStore) upsertMemoryIndex(bucketID, memoryID string, vector []float32) {
	if len(vector) == 0 {
		return
	}
	idx, ok := s.memoryIndexes[bucketID]
	if !ok {
		idx = s.newIndex()
		s.memoryIndexes[bucketID] = idx
	}
	if idx == nil {
		return
	}
	idx.RemoveVector(memoryID)
	idx.Insert(vector, memoryID)
}

func (s *BadgerStore) searchMemoryVectors(ctx context.Context, bucketID string, query []float32, topK int) (map[string]float32, error) {
	if topK <= 0 {
		topK = 5
	}

	s.mu.RLock()
	idx := s.memoryIndexes[bucketID]
	if idx != nil && idx.Len() > 0 {
		raw, err := idx.Search(query, topK)
		s.mu.RUnlock()
		if err != nil {
			return nil, err
		}
		out := make(map[string]float32, len(raw))
		for _, r := range raw {
			if r.Score > 0 {
				out[r.ID] = r.Score
			}
		}
		return out, nil
	}
	s.mu.RUnlock()

	vectors, err := s.readMemoryVectors(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	pairs := make([]pair, 0, len(vectors))
	for memoryID, vector := range vectors {
		score := CosineSimilarity(query, vector)
		if score > 0 {
			pairs = append(pairs, pair{embID: memoryID, score: score})
		}
	}
	sort.Slice(pairs, func(i, j int) bool {
		return pairs[i].score > pairs[j].score
	})
	if len(pairs) > topK {
		pairs = pairs[:topK]
	}
	out := make(map[string]float32, len(pairs))
	for _, p := range pairs {
		out[p.embID] = p.score
	}
	return out, nil
}

func (s *BadgerStore) readMemoryVectors(ctx context.Context, bucketID string) (map[string][]float32, error) {
	vectors := make(map[string][]float32)
	prefix := []byte(fmt.Sprintf("%s%s:", memVecPrefix, bucketID))
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		var count int
		for it.Rewind(); it.Valid(); it.Next() {
			if count%100 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			count++
			key := it.Item().Key()
			memoryID := string(key[len(prefix):])
			if err := it.Item().Value(func(val []byte) error {
				vectors[memoryID] = bytesToVector(cloneBytes(val))
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return vectors, err
}

func indexMemoryFTSTxn(txn *badger.Txn, bucketID, memoryID, content string) error {
	rawCounts := countRawTokens(content)
	counts := make(map[string]int)
	for token, count := range rawCounts {
		for _, syn := range expandSynonyms(token) {
			counts[syn] += count
		}
	}
	for term, count := range counts {
		if count > 255 {
			count = 255
		}
		key := []byte(fmt.Sprintf("%s%s:%s:%s", memFTSPrefix, term, bucketID, memoryID))
		if err := txn.Set(key, []byte{byte(count)}); err != nil {
			return err
		}
	}
	return nil
}

func deleteMemoryFTSEntries(txn *badger.Txn, bucketID, memoryID string) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(memFTSPrefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	suffix := ":" + bucketID + ":" + memoryID
	var toDelete [][]byte
	for it.Rewind(); it.Valid(); it.Next() {
		key := it.Item().Key()
		if strings.HasSuffix(string(key), suffix) {
			toDelete = append(toDelete, cloneBytes(key))
		}
	}
	for _, key := range toDelete {
		if err := txn.Delete(key); err != nil && err != badger.ErrKeyNotFound {
			return err
		}
	}
	return nil
}

func (s *BadgerStore) searchMemoryFTS(ctx context.Context, bucketID, query string) (map[string]float64, error) {
	tokens := TokenizeForQuery(query, FTSSearchOptions{Synonym: true})
	if len(tokens) == 0 {
		return nil, nil
	}

	termResults := make([]map[string]float64, 0, len(tokens))
	for _, token := range tokens {
		docScores, err := s.searchMemoryFTSTerm(ctx, bucketID, token)
		if err != nil {
			return nil, err
		}
		if len(docScores) > 0 {
			termResults = append(termResults, docScores)
		}
	}
	if len(termResults) == 0 {
		return nil, nil
	}

	totalDocs, err := s.memoryDocumentCount(ctx, bucketID)
	if err != nil {
		return nil, err
	}
	if totalDocs == 0 {
		totalDocs = 1
	}

	const k1 = 1.2
	const b = 0.75
	const avgDocLen = 10.0

	scores := make(map[string]float64)
	for _, docScores := range termResults {
		df := len(docScores)
		idf := 0.0
		if df > 0 {
			num := float64(totalDocs) - float64(df) + 0.5
			denom := float64(df) + 0.5
			idf = math.Log(num/denom + 1.0)
		}
		for memoryID, tf := range docScores {
			docLen := avgDocLen
			tfComponent := tf * (k1 + 1) / (tf + k1*(1-b+b*docLen/avgDocLen))
			scores[memoryID] += idf * tfComponent
		}
	}

	var maxScore float64
	for _, score := range scores {
		if score > maxScore {
			maxScore = score
		}
	}
	if maxScore > 0 {
		for memoryID, score := range scores {
			scores[memoryID] = score / maxScore
		}
	}
	return scores, nil
}

func (s *BadgerStore) searchMemoryFTSTerm(ctx context.Context, bucketID, token string) (map[string]float64, error) {
	isPrefix := strings.HasPrefix(token, "*:")
	searchTerm := strings.TrimPrefix(token, "*:")
	docScores := make(map[string]float64)

	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		if isPrefix {
			opts.Prefix = []byte(memFTSPrefix)
		} else {
			opts.Prefix = []byte(fmt.Sprintf("%s%s:%s:", memFTSPrefix, searchTerm, bucketID))
		}
		it := txn.NewIterator(opts)
		defer it.Close()

		bucketMarker := ":" + bucketID + ":"
		var count int
		for it.Rewind(); it.Valid(); it.Next() {
			if count%100 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			count++

			keyStr := string(it.Item().Key())
			var memoryID string
			if isPrefix {
				rest := keyStr[len(memFTSPrefix):]
				idx := strings.Index(rest, bucketMarker)
				if idx < 0 {
					continue
				}
				term := rest[:idx]
				if !strings.HasPrefix(term, searchTerm) {
					continue
				}
				memoryID = rest[idx+len(bucketMarker):]
			} else {
				memoryID = keyStr[len(opts.Prefix):]
			}

			if err := it.Item().Value(func(val []byte) error {
				if len(val) > 0 {
					docScores[memoryID] += float64(val[0])
				} else {
					docScores[memoryID]++
				}
				return nil
			}); err != nil {
				return err
			}
		}
		return nil
	})
	return docScores, err
}

func (s *BadgerStore) memoryDocumentCount(ctx context.Context, bucketID string) (int, error) {
	prefix := []byte(fmt.Sprintf("%s%s:", memContPrefix, bucketID))
	var count int
	err := s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			if count%100 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			count++
		}
		return nil
	})
	return count, err
}

func (s *BadgerStore) memoryBucketID(memoryID string) (string, error) {
	var bucketID string
	err := s.View(func(txn *badger.Txn) error {
		var err error
		bucketID, err = extractBucketIDFromMemoryID(txn, memoryID)
		return err
	})
	return bucketID, err
}

func extractBucketIDFromMemoryID(txn *badger.Txn, memoryID string) (string, error) {
	idxKey := []byte(fmt.Sprintf("mem:idx:%s", memoryID))
	idxItem, err := txn.Get(idxKey)
	if err != nil {
		return "", err
	}
	var bucketID string
	idxItem.Value(func(val []byte) error {
		bucketID = string(cloneBytes(val))
		return nil
	})
	return bucketID, nil
}

func memoryExpired(record *types.MemoryRecord) bool {
	if record.ExpiresAt == nil {
		return false
	}
	return record.ExpiresAt.Before(time.Now().UTC())
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

type memoryWeights struct {
	semantic   float64
	lexical    float64
	importance float64
	recency    float64
}

func memorySearchWeights(req types.MemorySearchRequest) memoryWeights {
	if req.SemanticWeight == 0 &&
		req.LexicalWeight == 0 &&
		req.ImportanceWeight == 0 &&
		req.RecencyWeight == 0 {
		return memoryWeights{
			semantic:   0.60,
			lexical:    0.25,
			importance: 0.10,
			recency:    0.05,
		}
	}
	return memoryWeights{
		semantic:   req.SemanticWeight,
		lexical:    req.LexicalWeight,
		importance: req.ImportanceWeight,
		recency:    req.RecencyWeight,
	}
}

func memoryRecencyHalfLife(req types.MemorySearchRequest) time.Duration {
	if req.RecencyHalfLife > 0 {
		return req.RecencyHalfLife
	}
	return 7 * 24 * time.Hour
}

func memoryRecencyScore(record *types.MemoryRecord, now time.Time, halfLife time.Duration) float64 {
	t := record.UpdatedAt
	if t.IsZero() {
		t = record.CreatedAt
	}
	if t.IsZero() {
		return 0
	}
	age := now.Sub(t)
	if age < 0 {
		age = 0
	}
	if halfLife <= 0 {
		halfLife = 7 * 24 * time.Hour
	}
	return 1 / (1 + age.Hours()/halfLife.Hours())
}

func cloneAnyMap(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
