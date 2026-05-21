package gracedb

import (
	"sync/atomic"

	"github.com/dshmyz/gracedb/pkg/types"
)

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

// AddMessage adds a message to a session. If auto-retain is enabled and a
// fact extractor is registered, extraction fires asynchronously.
func (db *DB) AddMessage(msg *types.Message) error {
	if err := db.store_.AddMessage(msg); err != nil {
		return err
	}

	globalAutoRetainMu.RLock()
	cfg := globalAutoRetainCfg
	enabled := cfg.Enabled
	globalAutoRetainMu.RUnlock()

	if !enabled || msg.SessionID == "" {
		return nil
	}

	// Check role filter.
	if len(cfg.RoleFilter) > 0 && !containsStr(cfg.RoleFilter, msg.Role) {
		return nil
	}

	// Increment session counter and fire if needed.
	counter := getOrCreateCounter(msg.SessionID)
	n := atomic.AddInt64(counter, 1)
	if int(n)%cfg.TriggerEvery == 0 {
		db.fireAutoRetain(msg.SessionID)
	}
	return nil
}

// GetSessionHistory returns messages for a session.
func (db *DB) GetSessionHistory(sessionID string, limit int) ([]*types.Message, error) {
	return db.store_.GetSessionHistory(sessionID, limit)
}

// DeleteMessage deletes a single message.
func (db *DB) DeleteMessage(sessionID, messageID string) error {
	return db.store_.DeleteMessage(sessionID, messageID)
}

// Helpers duplicated locally to avoid import cycles.
func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func firstNonEmptyStr(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func cloneAny(m map[string]any) map[string]any {
	if m == nil {
		return make(map[string]any)
	}
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = v
	}
	return out
}
