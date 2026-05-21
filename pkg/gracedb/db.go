package gracedb

import (
	"github.com/dshmyz/gracedb/pkg/graph"
	"github.com/dshmyz/gracedb/pkg/mcp"
	"github.com/dshmyz/gracedb/pkg/rdf"
	"github.com/dshmyz/gracedb/pkg/store"
	"github.com/dshmyz/gracedb/pkg/types"
)

// DB is the main database facade.
type DB struct {
	store_   *store.BadgerStore
	embedder types.Embedder
	graph_   *graph.GraphStore
	rdf_     *rdf.Store
}

// Option is a functional option for configuring the database.
type Option func(*types.Config)

// WithPath sets the database storage path.
func WithPath(path string) Option {
	return func(c *types.Config) {
		c.Path = path
	}
}

// WithIndexType sets the vector index type ("hnsw" or "flat").
func WithIndexType(indexType string) Option {
	return func(c *types.Config) {
		c.IndexType = indexType
	}
}

// WithSimilarity sets the similarity function ("cosine", "euclidean").
func WithSimilarity(fn string) Option {
	return func(c *types.Config) {
		c.SimilarityFn = fn
	}
}

// WithEmbedder sets the embedder for text-to-vector operations.
func WithEmbedder(e types.Embedder) Option {
	return func(c *types.Config) {
		c.Embedder = e
	}
}

// WithIndexTypes sets multiple vector index types for hybrid search.
// Use instead of WithIndexType when you want to combine indexes (e.g., hnsw + lsh).
func WithIndexTypes(idxTypes []string) Option {
	return func(c *types.Config) {
		c.IndexTypes = idxTypes
	}
}

// Open opens or creates a gracedb database.
func Open(path string, opts ...Option) (*DB, error) {
	cfg := types.DefaultConfig()
	cfg.Path = path
	for _, o := range opts {
		o(cfg)
	}

	s, err := store.New(cfg)
	if err != nil {
		return nil, err
	}

	db := &DB{
		store_:   s,
		embedder: cfg.Embedder,
		graph_:   graph.NewGraphStore(s.DB()),
		rdf_:     rdf.NewStore(s.DB()),
	}

	return db, nil
}

// Vector returns the underlying BadgerStore.
func (db *DB) Vector() *store.BadgerStore {
	return db.store_
}

// Graph returns the property graph store.
func (db *DB) Graph() *graph.GraphStore {
	return db.graph_
}

// RDF returns the RDF triple store.
func (db *DB) RDF() *rdf.Store {
	return db.rdf_
}

// HasEmbedder reports whether an embedder is configured.
func (db *DB) HasEmbedder() bool {
	return db.embedder != nil
}

// NewMCPServer creates an MCP server exposing all gracedb tools.
func (db *DB) NewMCPServer(name, version string) *mcp.Server {
	toolbox := db.GraphRAGTools()
	defs := toolbox.Definitions()
	return mcp.FromToolbox(name, version, defs, toolbox.Call)
}

// Close closes the database.
func (db *DB) Close() error {
	return db.store_.Close()
}
