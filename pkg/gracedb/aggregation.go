package gracedb

import "github.com/dshmyz/gracedb/pkg/store"

// Aggregate performs an aggregation over embedding metadata.
func (db *DB) Aggregate(collectionName string, metadataKey string, aggType store.AggregationType) (*store.AggregationResult, error) {
	return db.store_.Aggregate(collectionName, metadataKey, aggType)
}
