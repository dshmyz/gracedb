package gracedb

import "github.com/dshmyz/gracedb/pkg/types"

// CreateDocument creates a new document.
func (db *DB) CreateDocument(doc *types.Document) error {
	return db.store_.CreateDocument(doc)
}

// GetDocument retrieves a document by ID.
func (db *DB) GetDocument(id string) (*types.Document, error) {
	return db.store_.GetDocument(id)
}

// ListDocuments returns all documents.
func (db *DB) ListDocuments() ([]*types.Document, error) {
	return db.store_.ListDocuments()
}

// DeleteDocument deletes a document by ID.
func (db *DB) DeleteDocument(id string) error {
	return db.store_.DeleteDocument(id)
}

// Stats returns database statistics.
func (db *DB) Stats() (types.StoreStats, error) {
	return db.store_.Stats()
}
