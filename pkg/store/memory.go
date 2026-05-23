package store

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
	"github.com/dshmyz/gracedb/pkg/types"
)

const (
	memPrefix = "mem:"
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
	metadata["scope"] = scope
	metadata["namespace"] = req.Namespace
	if metadata["namespace"] == "" {
		metadata["namespace"] = "default"
	}
	metadata["importance"] = req.Importance
	metadata["ttl_seconds"] = req.TTLSeconds

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

	role := req.Role
	if role == "" {
		role = "memory"
	}

	record := &types.MemoryRecord{
		ID:         req.MemoryID,
		UserID:     req.UserID,
		SessionID:  req.SessionID,
		Scope:      scope,
		Namespace:  metadata["namespace"].(string),
		Role:       role,
		Content:    req.Content,
		Metadata:   metadata,
		Importance: req.Importance,
		TTLSeconds: req.TTLSeconds,
		ExpiresAt:  expiresAt,
		CreatedAt:  now,
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return record, s.Update(func(txn *badger.Txn) error {
		memKey := []byte(fmt.Sprintf("%s%s:%s", memPrefix, bucketID, req.MemoryID))
		if err := txn.Set(memKey, metadataJSON); err != nil {
			return err
		}

		// Also store full content in a content key.
		contentKey := []byte(fmt.Sprintf("%scontent:%s:%s", memPrefix, bucketID, req.MemoryID))
		if err := txn.Set(contentKey, []byte(req.Content)); err != nil {
			return err
		}

		// Index: memoryID -> bucketID for lookup by ID alone.
		idxKey := []byte(fmt.Sprintf("mem:idx:%s", req.MemoryID))
		return txn.Set(idxKey, []byte(bucketID))
	})
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

		contentKey := []byte(fmt.Sprintf("%scontent:%s:%s", memPrefix, bucketID, req.MemoryID))
		return txn.Set(contentKey, []byte(record.Content))
	})
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
			[]byte(fmt.Sprintf("%scontent:%s:%s", memPrefix, bucketID, memoryID)),
			[]byte(fmt.Sprintf("mem:idx:%s", memoryID)),
		}
		for _, k := range keys {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

// SearchMemory searches memories in a bucket using FTS.
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

	tokens := Tokenize(req.Query)
	if len(tokens) == 0 {
		return &types.MemorySearchResponse{Query: req.Query}, nil
	}

	ctx := req.Context
	if ctx == nil {
		ctx = context.Background()
	}

	// Search content keys in this bucket.
	type scoredMem struct {
		memoryID string
		score    float64
	}
	matched := make(map[string]*scoredMem)

	prefix := []byte(fmt.Sprintf("%scontent:%s:", memPrefix, bucketID))
	err = s.View(func(txn *badger.Txn) error {
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		var count int
		for it.Rewind(); it.Valid(); it.Next() {
			// Check context periodically during scan.
			if count%100 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			count++

			key := it.Item().Key()
			memoryID := string(key[len(prefix):])

			var content []byte
			it.Item().Value(func(val []byte) error {
				content = cloneBytes(val)
				return nil
			})

			// Simple token match scoring.
			text := strings.ToLower(string(content))
			var score float64
			for _, token := range tokens {
				if strings.Contains(text, strings.ToLower(token)) {
					score += 1.0
				}
			}
			if score > 0 {
				matched[memoryID] = &scoredMem{memoryID: memoryID, score: score}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if len(matched) == 0 {
		return &types.MemorySearchResponse{Query: req.Query}, nil
	}

	// Sort by score.
	var sorted []*scoredMem
	for _, sm := range matched {
		sorted = append(sorted, sm)
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].score > sorted[j].score
	})
	if len(sorted) > req.TopK {
		sorted = sorted[:req.TopK]
	}

	// Load records.
	var results []types.MemorySearchHit
	for _, sm := range sorted {
		err := s.View(func(txn *badger.Txn) error {
			rec, err := s.loadMemory(txn, sm.memoryID)
			if err != nil {
				return err
			}
			if memoryExpired(rec) {
				return nil
			}
			results = append(results, types.MemorySearchHit{
				Memory: *rec,
				Score:  sm.score,
			})
			return nil
		})
		if err != nil {
			continue
		}
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
	contentKey := []byte(fmt.Sprintf("%scontent:%s:%s", memPrefix, bucketID, memoryID))
	var content string
	cItem, cErr := txn.Get(contentKey)
	if cErr == nil {
		cItem.Value(func(val []byte) error {
			content = string(cloneBytes(val))
			return nil
		})
	}

	record := &types.MemoryRecord{
		ID:      memoryID,
		Content: content,
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
	record.Metadata = metadata

	return record, nil
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
