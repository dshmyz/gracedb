package gracedb

import "github.com/dshmyz/gracedb/pkg/store"

// Aggregate performs an aggregation over embedding metadata.
func (db *DB) Aggregate(collectionName string, metadataKey string, aggType store.AggregationType) (*store.AggregationResult, error) {
	return db.store_.Aggregate(collectionName, metadataKey, aggType)
}

// GroupAggregate performs aggregation grouped by a metadata key (GROUP BY).
func (db *DB) GroupAggregate(collectionName string, groupKey string, valueKey string, aggType store.AggregationType) (map[string]*store.AggregationResult, error) {
	return db.store_.GroupAggregate(collectionName, groupKey, valueKey, aggType)
}
