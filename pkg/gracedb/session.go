package gracedb

import "github.com/dshmyz/gracedb/pkg/types"

// CreateSession creates a new session.
func (db *DB) CreateSession(name string) (*types.Session, error) {
	return db.store_.CreateSession(name)
}

// GetSession retrieves a session by ID.
func (db *DB) GetSession(id string) (*types.Session, error) {
	return db.store_.GetSession(id)
}

// ListSessions returns all sessions.
func (db *DB) ListSessions() ([]*types.Session, error) {
	return db.store_.ListSessions()
}

// DeleteSession deletes a session and all its messages.
func (db *DB) DeleteSession(id string) error {
	return db.store_.DeleteSession(id)
}

// AddMessage adds a message to a session.
func (db *DB) AddMessage(msg *types.Message) error {
	return db.store_.AddMessage(msg)
}

// GetSessionHistory returns messages for a session.
func (db *DB) GetSessionHistory(sessionID string, limit int) ([]*types.Message, error) {
	return db.store_.GetSessionHistory(sessionID, limit)
}

// DeleteMessage deletes a single message.
func (db *DB) DeleteMessage(sessionID, messageID string) error {
	return db.store_.DeleteMessage(sessionID, messageID)
}
