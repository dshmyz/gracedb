package store

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dgraph-io/badger/v4"
	"github.com/dshmyz/gracedb/pkg/types"
)

const (
	knowPrefix    = "know:"
	knowChunkPrefix = "know:chunk:"
)

// SaveKnowledge stores or replaces a knowledge item and its chunks.
func (s *BadgerStore) SaveKnowledge(collectionName, knowledgeID, title, content string, req types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	chunkSize := req.ChunkSize
	if chunkSize <= 0 {
		chunkSize = 500
	}
	chunkOverlap := req.ChunkOverlap
	if chunkOverlap <= 0 {
		chunkOverlap = chunkSize / 4
	}

	// Check existing version.
	existing, _ := s.loadKnowledgeRecordTxn(coll.ID, knowledgeID)
	version := 1
	if existing != nil {
		version = existing.Version + 1
	}

	chunks := ChunkBySize(content, chunkSize, chunkOverlap)

	s.mu.Lock()
	defer s.mu.Unlock()

	var chunkIDs []string
	now := time.Now()

	err = s.Update(func(txn *badger.Txn) error {
		// Save knowledge record.
		record := &types.KnowledgeRecord{
			ID:         knowledgeID,
			Title:      title,
			Content:    content,
			SourceURL:  req.SourceURL,
			Author:     req.Author,
			Collection: collectionName,
			Metadata:   req.Metadata,
			Entities:   req.Entities,
			Version:    version,
			CreatedAt:  now,
			UpdatedAt:  now,
		}
		if existing != nil {
			record.CreatedAt = existing.CreatedAt
		}

		recordData, err := marshal(record)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(fmt.Sprintf("%s%s:%s", knowPrefix, coll.ID, knowledgeID)), recordData); err != nil {
			return err
		}

		// Save each chunk as an embedding-like entry and index for FTS.
		for _, chunk := range chunks {
			chunkKey := []byte(fmt.Sprintf("%s%s:%s:%s", knowChunkPrefix, coll.ID, knowledgeID, chunk.ID))

			// Store chunk data.
			chunkData, err := marshal(map[string]interface{}{
				"id":           chunk.ID,
				"knowledge_id": knowledgeID,
				"content":      chunk.Content,
				"start":        chunk.Start,
				"end":          chunk.End,
				"collection":   collectionName,
			})
			if err != nil {
				return err
			}
			if err := txn.Set(chunkKey, chunkData); err != nil {
				return err
			}

			// Index chunk content for FTS.
			tokens := Tokenize(chunk.Content)
			for _, term := range tokens {
				ftsKey := []byte(fmt.Sprintf("%s%s:%s:%s:%s", ftsPrefix, term, coll.ID, knowledgeID, chunk.ID))
				if err := txn.Set(ftsKey, nil); err != nil {
					return err
				}
			}

			chunkIDs = append(chunkIDs, chunk.ID)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	record := &types.KnowledgeRecord{
		ID:         knowledgeID,
		Title:      title,
		Content:    content,
		SourceURL:  req.SourceURL,
		Author:     req.Author,
		Collection: collectionName,
		Metadata:   req.Metadata,
		Entities:   req.Entities,
		ChunkIDs:   chunkIDs,
		Version:    version,
		CreatedAt:  now,
		UpdatedAt:  now,
	}
	if existing != nil {
		record.CreatedAt = existing.CreatedAt
	}
	return record, nil
}

// GetKnowledge fetches a knowledge item by ID.
func (s *BadgerStore) GetKnowledge(collectionName, knowledgeID string) (*types.KnowledgeRecord, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	var record *types.KnowledgeRecord
	err = s.View(func(txn *badger.Txn) error {
		record, err = s.loadKnowledgeRecordTxn(coll.ID, knowledgeID)
		return err
	})
	return record, err
}

// UpdateKnowledge updates a knowledge item.
func (s *BadgerStore) UpdateKnowledge(collectionName, knowledgeID string, req types.KnowledgeUpdateRequest) (*types.KnowledgeRecord, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	existing, err := s.loadKnowledgeRecordTxn(coll.ID, knowledgeID)
	if err != nil {
		return nil, err
	}

	title := existing.Title
	if req.Title != nil {
		title = *req.Title
	}
	content := existing.Content
	if req.Content != nil {
		content = *req.Content
	}
	sourceURL := existing.SourceURL
	if req.SourceURL != nil {
		sourceURL = *req.SourceURL
	}
	author := existing.Author
	if req.Author != nil {
		author = *req.Author
	}
	metadata := existing.Metadata
	if req.Metadata != nil {
		metadata = cloneMap(existing.Metadata, req.Metadata)
	}

	chunkSize := 500
	if req.ChunkSize != nil {
		chunkSize = *req.ChunkSize
	}
	chunkOverlap := chunkSize / 4
	if req.ChunkOverlap != nil {
		chunkOverlap = *req.ChunkOverlap
	}

	chunks := ChunkBySize(content, chunkSize, chunkOverlap)

	s.mu.Lock()
	defer s.mu.Unlock()

	var chunkIDs []string
	now := time.Now()

	err = s.Update(func(txn *badger.Txn) error {
		// Delete old chunks.
		oldChunkPrefix := []byte(fmt.Sprintf("%s%s:%s:", knowChunkPrefix, coll.ID, knowledgeID))
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()
		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			if hasPrefix(key, oldChunkPrefix) {
				k := make([]byte, len(key))
				copy(k, key)
				if err := txn.Delete(k); err != nil {
					return err
				}
			}
		}

		// Save updated record.
		record := &types.KnowledgeRecord{
			ID:         knowledgeID,
			Title:      title,
			Content:    content,
			SourceURL:  sourceURL,
			Author:     author,
			Collection: collectionName,
			Metadata:   metadata,
			Entities:   existing.Entities,
			Version:    existing.Version + 1,
			CreatedAt:  existing.CreatedAt,
			UpdatedAt:  now,
		}

		recordData, err := marshal(record)
		if err != nil {
			return err
		}
		if err := txn.Set([]byte(fmt.Sprintf("%s%s:%s", knowPrefix, coll.ID, knowledgeID)), recordData); err != nil {
			return err
		}

		// Save new chunks.
		for _, chunk := range chunks {
			chunkKey := []byte(fmt.Sprintf("%s%s:%s:%s", knowChunkPrefix, coll.ID, knowledgeID, chunk.ID))
			chunkData, err := marshal(map[string]interface{}{
				"id":           chunk.ID,
				"knowledge_id": knowledgeID,
				"content":      chunk.Content,
				"start":        chunk.Start,
				"end":          chunk.End,
				"collection":   collectionName,
			})
			if err != nil {
				return err
			}
			if err := txn.Set(chunkKey, chunkData); err != nil {
				return err
			}

			// Index chunk for FTS.
			tokens := Tokenize(chunk.Content)
			for _, term := range tokens {
				ftsKey := []byte(fmt.Sprintf("%s%s:%s:%s:%s", ftsPrefix, term, coll.ID, knowledgeID, chunk.ID))
				if err := txn.Set(ftsKey, nil); err != nil {
					return err
				}
			}

			chunkIDs = append(chunkIDs, chunk.ID)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return &types.KnowledgeRecord{
		ID:         knowledgeID,
		Title:      title,
		Content:    content,
		SourceURL:  sourceURL,
		Author:     author,
		Collection: collectionName,
		Metadata:   metadata,
		Entities:   existing.Entities,
		ChunkIDs:   chunkIDs,
		Version:    existing.Version + 1,
		CreatedAt:  existing.CreatedAt,
		UpdatedAt:  now,
	}, nil
}

// DeleteKnowledge removes a knowledge item and its chunks/FTS entries.
func (s *BadgerStore) DeleteKnowledge(collectionName, knowledgeID string) error {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	return s.Update(func(txn *badger.Txn) error {
		// Delete knowledge record.
		if err := txn.Delete([]byte(fmt.Sprintf("%s%s:%s", knowPrefix, coll.ID, knowledgeID))); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Delete chunks and FTS entries.
		prefix := []byte(fmt.Sprintf("%s%s:%s:", knowChunkPrefix, coll.ID, knowledgeID))
		opts := badger.DefaultIteratorOptions
		opts.Prefix = prefix
		it := txn.NewIterator(opts)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			key := it.Item().Key()
			k := make([]byte, len(key))
			copy(k, key)
			if err := txn.Delete(k); err != nil {
				return err
			}
		}

		// Delete FTS entries for this knowledge.
		return deleteFTSKnowledge(txn, coll.ID, knowledgeID)
	})
}

// SearchKnowledge searches knowledge chunks and aggregates by knowledge item.
func (s *BadgerStore) SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	if topK <= 0 {
		topK = 10
	}

	// Search FTS for chunks matching query.
	tokens := Tokenize(query)
	if len(tokens) == 0 {
		return &types.KnowledgeSearchResponse{Query: query}, nil
	}

	// Collect matching chunk IDs with scores.
	type scoredChunk struct {
		knowledgeID string
		chunkID     string
		content     string
		score       float64
	}
	var matchedChunks []scoredChunk

	for _, term := range tokens {
		prefix := []byte(fmt.Sprintf("%s%s:%s:", ftsPrefix, term, coll.ID))
		err := s.View(func(txn *badger.Txn) error {
			opts := badger.DefaultIteratorOptions
			opts.Prefix = prefix
			it := txn.NewIterator(opts)
			defer it.Close()

			knowledgePrefix := len(fmt.Sprintf("%s%s:", ftsPrefix, term))
			for it.Rewind(); it.Valid(); it.Next() {
				key := it.Item().Key()
				// Key: fts:{term}:{collectionID}:{knowledgeID}:{chunkID}
				rest := string(key[knowledgePrefix:])
				parts := strings.SplitN(rest, ":", 2)
				if len(parts) != 2 {
					continue
				}
				matchedChunks = append(matchedChunks, scoredChunk{
					knowledgeID: parts[0],
					chunkID:     parts[1],
					score:       1.0,
				})
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if len(matchedChunks) == 0 {
		return &types.KnowledgeSearchResponse{Query: query}, nil
	}

	// Aggregate by knowledgeID.
	type knowledgeHit struct {
		knowledgeID string
		chunkIDs    []string
		snippets    []string
		score       float64
	}
	hitsMap := make(map[string]*knowledgeHit)
	for _, mc := range matchedChunks {
		h, ok := hitsMap[mc.knowledgeID]
		if !ok {
			h = &knowledgeHit{knowledgeID: mc.knowledgeID}
			hitsMap[mc.knowledgeID] = h
		}
		h.chunkIDs = append(h.chunkIDs, mc.chunkID)
		h.snippets = append(h.snippets, truncateString(mc.content, 200))
		h.score += mc.score
	}

	// Load knowledge records.
	var results []types.KnowledgeSearchHit
	for _, h := range hitsMap {
		record, err := s.loadKnowledgeRecordTxn(coll.ID, h.knowledgeID)
		if err != nil {
			continue
		}
		snippet := ""
		if len(h.snippets) > 0 {
			snippet = h.snippets[0]
		}
		results = append(results, types.KnowledgeSearchHit{
			KnowledgeID: h.knowledgeID,
			Title:       record.Title,
			SourceURL:   record.SourceURL,
			Author:      record.Author,
			Snippet:     snippet,
			Score:       h.score,
			ChunkIDs:    h.chunkIDs,
			Entities:    record.Entities,
			Metadata:    record.Metadata,
		})
	}

	// Sort by score descending.
	for i := 0; i < len(results)-1; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Score > results[i].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	if len(results) > topK {
		results = results[:topK]
	}

	return &types.KnowledgeSearchResponse{
		Query:   query,
		Results: results,
	}, nil
}

func (s *BadgerStore) loadKnowledgeRecordTxn(collectionID, knowledgeID string) (*types.KnowledgeRecord, error) {
	var record *types.KnowledgeRecord
	err := s.View(func(txn *badger.Txn) error {
		it, err := txn.Get([]byte(fmt.Sprintf("%s%s:%s", knowPrefix, collectionID, knowledgeID)))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return types.ErrNotFound
			}
			return err
		}

		err = it.Value(func(val []byte) error {
			record = &types.KnowledgeRecord{}
			return unmarshal(cloneBytes(val), record)
		})
		if err != nil {
			return err
		}

		// Load chunk IDs.
		chunkPrefix := fmt.Sprintf("%s%s:%s:", knowChunkPrefix, collectionID, knowledgeID)
		opts := badger.DefaultIteratorOptions
		opts.Prefix = []byte(chunkPrefix)
		chunkIt := txn.NewIterator(opts)
		defer chunkIt.Close()

		for chunkIt.Rewind(); chunkIt.Valid(); chunkIt.Next() {
			chunkKey := chunkIt.Item().Key()
			chunkID := string(chunkKey[len(chunkPrefix):])
			record.ChunkIDs = append(record.ChunkIDs, chunkID)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}
	return record, nil
}

func deleteFTSKnowledge(txn *badger.Txn, collectionID, knowledgeID string) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(ftsPrefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	pattern := ":" + collectionID + ":" + knowledgeID + ":"
	for it.Rewind(); it.Valid(); it.Next() {
		key := it.Item().Key()
		if idx := strings.Index(string(key), pattern); idx >= 0 && string(key[:idx]) >= ftsPrefix {
			k := make([]byte, len(key))
			copy(k, key)
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
	}
	return nil
}

func cloneMap(base map[string]string, overrides map[string]string) map[string]string {
	out := make(map[string]string)
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overrides {
		out[k] = v
	}
	return out
}

func truncateString(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxLen]) + "..."
}

func hasPrefix(b, prefix []byte) bool {
	return len(b) >= len(prefix) && string(b[:len(prefix)]) == string(prefix)
}
