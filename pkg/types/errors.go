package types

import "errors"

var (
	ErrNotFound              = errors.New("gracedb: not found")
	ErrCollectionExists      = errors.New("gracedb: collection already exists")
	ErrDimensionMismatch     = errors.New("gracedb: vector dimension mismatch")
	ErrInvalidVector         = errors.New("gracedb: invalid vector: empty or nil")
	ErrEmbedderNotConfigured = errors.New("gracedb: embedder not configured")
	ErrEmptyText             = errors.New("gracedb: empty text")
	ErrClosed                = errors.New("gracedb: database is closed")
)
