package graph

import (
	"fmt"
	"sync"
	"time"

	"github.com/dgraph-io/badger/v4"
	"github.com/google/uuid"
)

const (
	nodePrefix       = "graph:node:"
	edgePrefix       = "graph:edge:"
	edgeFromPrefix   = "graph:edge:from:"
	edgeToPrefix     = "graph:edge:to:"
	edgeTypePrefix   = "graph:edge:type:"
	nodeTypePrefix   = "graph:node:type:"
)

// GraphStore is a Badger-backed property graph store.
type GraphStore struct {
	db *badger.DB
	mu sync.RWMutex
}

// NewGraphStore creates a new graph store.
func NewGraphStore(db *badger.DB) *GraphStore {
	return &GraphStore{db: db}
}

// UpsertNode inserts or updates a node.
func (g *GraphStore) UpsertNode(node *GraphNode) error {
	if node.ID == "" {
		node.ID = uuid.New().String()
	}
	if node.CreatedAt.IsZero() {
		node.CreatedAt = time.Now()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	return g.db.Update(func(txn *badger.Txn) error {
		data, err := node.Marshal()
		if err != nil {
			return err
		}
		key := []byte(nodePrefix + node.ID)
		if err := txn.Set(key, data); err != nil {
			return err
		}

		// Index by type.
		if node.Type != "" {
			typeKey := []byte(fmt.Sprintf("%s%s:%s", nodeTypePrefix, node.Type, node.ID))
			_ = txn.Set(typeKey, nil)
		}
		return nil
	})
}

// GetNode fetches a node by ID.
func (g *GraphStore) GetNode(id string) (*GraphNode, error) {
	var node *GraphNode
	err := g.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(nodePrefix + id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			node, err = UnmarshalNode(val)
			return err
		})
	})
	return node, err
}

// DeleteNode removes a node and its incident edges.
func (g *GraphStore) DeleteNode(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.db.Update(func(txn *badger.Txn) error {
		// Delete node.
		if err := txn.Delete([]byte(nodePrefix + id)); err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		// Collect edge IDs to delete.
		var edgeIDs []string

		// Delete outgoing edge indexes and collect edge IDs.
		fromPrefix := fmt.Sprintf("%s%s:", edgeFromPrefix, id)
		eIDs, err := scanIndexKeys(txn, fromPrefix)
		if err != nil {
			return err
		}
		edgeIDs = append(edgeIDs, eIDs...)
		if err := deleteEdgesByPrefix(txn, fromPrefix); err != nil {
			return err
		}

		// Delete incoming edge indexes and collect edge IDs.
		toPrefix := fmt.Sprintf("%s%s:", edgeToPrefix, id)
		eIDs, err = scanIndexKeys(txn, toPrefix)
		if err != nil {
			return err
		}
		edgeIDs = append(edgeIDs, eIDs...)
		if err := deleteEdgesByPrefix(txn, toPrefix); err != nil {
			return err
		}

		// Delete main edge entries.
		seen := make(map[string]bool)
		for _, eID := range edgeIDs {
			if seen[eID] {
				continue
			}
			seen[eID] = true
			if err := txn.Delete([]byte(edgePrefix + eID)); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

// UpsertEdge inserts or updates an edge.
func (g *GraphStore) UpsertEdge(edge *GraphEdge) error {
	if edge.ID == "" {
		edge.ID = uuid.New().String()
	}
	if edge.CreatedAt.IsZero() {
		edge.CreatedAt = time.Now()
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	return g.db.Update(func(txn *badger.Txn) error {
		data, err := edge.Marshal()
		if err != nil {
			return err
		}

		// Main edge entry.
		if err := txn.Set([]byte(edgePrefix+edge.ID), data); err != nil {
			return err
		}
		// From -> To index.
		fromKey := []byte(fmt.Sprintf("%s%s:%s:%s", edgeFromPrefix, edge.FromNodeID, edge.Type, edge.ID))
		if err := txn.Set(fromKey, []byte(edge.ID)); err != nil {
			return err
		}
		// To -> From index.
		toKey := []byte(fmt.Sprintf("%s%s:%s:%s", edgeToPrefix, edge.ToNodeID, edge.Type, edge.ID))
		if err := txn.Set(toKey, []byte(edge.ID)); err != nil {
			return err
		}
		// Type index.
		if edge.Type != "" {
			typeKey := []byte(fmt.Sprintf("%s%s:%s", edgeTypePrefix, edge.Type, edge.ID))
			_ = txn.Set(typeKey, []byte(edge.ID))
		}
		return nil
	})
}

// GetEdge fetches an edge by ID.
func (g *GraphStore) GetEdge(id string) (*GraphEdge, error) {
	var edge *GraphEdge
	err := g.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(edgePrefix + id))
		if err != nil {
			return err
		}
		return item.Value(func(val []byte) error {
			edge, err = UnmarshalEdge(val)
			return err
		})
	})
	return edge, err
}

// DeleteEdge removes an edge by ID.
func (g *GraphStore) DeleteEdge(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	return g.db.Update(func(txn *badger.Txn) error {
		edge, err := g.loadEdge(txn, id)
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return nil
			}
			return err
		}

		// Delete main entry and indexes.
		keys := [][]byte{
			[]byte(edgePrefix + id),
			[]byte(fmt.Sprintf("%s%s:%s:%s", edgeFromPrefix, edge.FromNodeID, edge.Type, id)),
			[]byte(fmt.Sprintf("%s%s:%s:%s", edgeToPrefix, edge.ToNodeID, edge.Type, id)),
		}
		if edge.Type != "" {
			keys = append(keys, []byte(fmt.Sprintf("%s%s:%s", edgeTypePrefix, edge.Type, id)))
		}
		for _, k := range keys {
			if err := txn.Delete(k); err != nil && err != badger.ErrKeyNotFound {
				return err
			}
		}
		return nil
	})
}

// GetNeighbors returns neighbors of a node.
func (g *GraphStore) GetNeighbors(nodeID string, opts NeighborOptions) ([]*GraphNode, []*GraphEdge, error) {
	if opts.Direction == "" {
		opts.Direction = "out"
	}

	var edgeIDs []string
	err := g.db.View(func(txn *badger.Txn) error {
		if opts.Direction == "out" || opts.Direction == "both" {
			prefix := fmt.Sprintf("%s%s:", edgeFromPrefix, nodeID)
			eIDs, err := g.scanEdgePrefix(txn, prefix, opts)
			if err != nil {
				return err
			}
			edgeIDs = append(edgeIDs, eIDs...)
		}
		if opts.Direction == "in" || opts.Direction == "both" {
			prefix := fmt.Sprintf("%s%s:", edgeToPrefix, nodeID)
			eIDs, err := g.scanEdgePrefix(txn, prefix, opts)
			if err != nil {
				return err
			}
			edgeIDs = append(edgeIDs, eIDs...)
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	if opts.Limit > 0 && len(edgeIDs) > opts.Limit {
		edgeIDs = edgeIDs[:opts.Limit]
	}

	var nodes []*GraphNode
	var edges []*GraphEdge
	seen := make(map[string]*GraphNode)

	for _, eID := range edgeIDs {
		edge, err := g.GetEdge(eID)
		if err != nil {
			continue
		}
		edges = append(edges, edge)

		// Determine which endpoint is the neighbor.
		neighborID := edge.ToNodeID
		if opts.Direction == "in" {
			neighborID = edge.FromNodeID
		}

		if _, ok := seen[neighborID]; !ok {
			node, err := g.GetNode(neighborID)
			if err != nil {
				continue
			}
			// Filter by node type.
			if len(opts.NodeTypes) > 0 && !containsStr(opts.NodeTypes, node.Type) {
				continue
			}
			seen[neighborID] = node
			nodes = append(nodes, node)
		}
	}

	return nodes, edges, nil
}

func (g *GraphStore) scanEdgePrefix(txn *badger.Txn, prefix string, opts NeighborOptions) ([]string, error) {
	opts2 := badger.DefaultIteratorOptions
	opts2.Prefix = []byte(prefix)
	it := txn.NewIterator(opts2)
	defer it.Close()

	var edgeIDs []string
	for it.Rewind(); it.Valid(); it.Next() {
		item := it.Item()
		var eID string
		item.Value(func(val []byte) error {
			eID = string(val)
			return nil
		})

		// Load edge to filter by type.
		if len(opts.EdgeTypes) > 0 {
			edge, err := g.loadEdge(txn, eID)
			if err != nil {
				continue
			}
			if !containsStr(opts.EdgeTypes, edge.Type) {
				continue
			}
		}
		edgeIDs = append(edgeIDs, eID)
	}
	return edgeIDs, nil
}

func (g *GraphStore) loadEdge(txn *badger.Txn, id string) (*GraphEdge, error) {
	item, err := txn.Get([]byte(edgePrefix + id))
	if err != nil {
		return nil, err
	}
	var edge *GraphEdge
	err = item.Value(func(val []byte) error {
		edge, err = UnmarshalEdge(val)
		return err
	})
	return edge, err
}

func scanIndexKeys(txn *badger.Txn, prefix string) ([]string, error) {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(prefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	var ids []string
	for it.Rewind(); it.Valid(); it.Next() {
		var val string
		it.Item().Value(func(v []byte) error {
			val = string(v)
			return nil
		})
		if val != "" {
			ids = append(ids, val)
		}
	}
	return ids, nil
}

func deleteEdgesByPrefix(txn *badger.Txn, prefix string) error {
	opts := badger.DefaultIteratorOptions
	opts.Prefix = []byte(prefix)
	it := txn.NewIterator(opts)
	defer it.Close()

	for it.Rewind(); it.Valid(); it.Next() {
		key := make([]byte, len(it.Item().Key()))
		copy(key, it.Item().Key())
		if err := txn.Delete(key); err != nil {
			return err
		}
	}
	return nil
}

func containsStr(slice []string, s string) bool {
	for _, x := range slice {
		if x == s {
			return true
		}
	}
	return false
}
