package store

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/dgraph-io/badger/v4"
	"github.com/dshmyz/gracedb/pkg/types"
)

// AggregationType specifies the type of aggregation to perform.
type AggregationType string

const (
	AggCount AggregationType = "count"
	AggSum   AggregationType = "sum"
	AggAvg   AggregationType = "avg"
	AggMin   AggregationType = "min"
	AggMax   AggregationType = "max"
)

// AggregationResult holds the result of an aggregation query.
type AggregationResult struct {
	Type       AggregationType
	MetadataKey string
	Count      int
	Sum        float64
	Avg        float64
	Min        float64
	Max        float64
}

// Aggregate performs an aggregation over embedding metadata in a collection.
// For count, metadataKey can be empty.
func (s *BadgerStore) Aggregate(collectionName string, metadataKey string, aggType AggregationType) (*AggregationResult, error) {
	coll, err := s.GetCollection(collectionName)
	if err != nil {
		return nil, err
	}

	result := &AggregationResult{
		Type:        aggType,
		MetadataKey: metadataKey,
		Min:         math.MaxFloat64,
		Max:         -math.MaxFloat64,
	}

	ids, err := s.ListEmbeddingIDs(coll.ID)
	if err != nil {
		return nil, err
	}

	err = s.View(func(txn *badger.Txn) error {
		for _, embID := range ids {
			item, err := txn.Get([]byte(fmt.Sprintf("%s%s:%s", embPrefix, coll.ID, embID)))
			if err != nil {
				continue
			}
			var emb types.Embedding
			if err := item.Value(func(val []byte) error {
				return unmarshal(cloneBytes(val), &emb)
			}); err != nil {
				continue
			}

			if aggType == AggCount {
				result.Count++
				continue
			}

			if emb.Metadata == nil {
				continue
			}

			valStr, ok := emb.Metadata[metadataKey]
			if !ok {
				continue
			}

			val, err := strconv.ParseFloat(strings.TrimSpace(valStr), 64)
			if err != nil {
				continue
			}

			result.Count++
			result.Sum += val
			if val < result.Min {
				result.Min = val
			}
			if val > result.Max {
				result.Max = val
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	if result.Count > 0 {
		result.Avg = result.Sum / float64(result.Count)
	} else {
		result.Min = 0
		result.Max = 0
	}

	return result, nil
}
