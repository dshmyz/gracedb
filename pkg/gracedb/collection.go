package gracedb

import "github.com/dshmyz/gracedb/pkg/types"

// CreateCollection creates a new vector collection.
func (db *DB) CreateCollection(name string) (*types.Collection, error) {
	return db.store_.CreateCollection(name)
}

// GetCollection retrieves a collection by name.
func (db *DB) GetCollection(name string) (*types.Collection, error) {
	return db.store_.GetCollection(name)
}

// ListCollections returns all collections.
func (db *DB) ListCollections() ([]*types.Collection, error) {
	return db.store_.ListCollections()
}

// DeleteCollection deletes a collection and all its data.
func (db *DB) DeleteCollection(name string) error {
	return db.store_.DeleteCollection(name)
}
