package gracedb

import (
	"github.com/dshmyz/gracedb/pkg/store"
	"github.com/dshmyz/gracedb/pkg/types"
)

// SearchGeo performs geospatial filtering on embeddings with lat/lon metadata.
func (db *DB) SearchGeo(collectionName string, query store.GeoQuery, opts types.SearchOptions) ([]types.ScoredEmbedding, error) {
	return db.store_.SearchGeo(collectionName, query, opts)
}
