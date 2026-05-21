# gracedb Getting Started

## Requirements

- Go 1.23 or later
- No external dependencies (Badger, gse, etc. are all managed via go.mod)

## Installation

```bash
go get github.com/dshmyz/gracedb
```

## Quick Start

### 1. Open the Database

```go
package main

import (
    "fmt"
    "log"

    "github.com/dshmyz/gracedb/pkg/gracedb"
)

func main() {
    // Disk mode
    db, err := gracedb.Open("/tmp/gracedb-data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // In-memory mode (empty path)
    memDB, _ := gracedb.Open("")
    defer memDB.Close()
}
```

### 2. Configure an Embedder (Optional)

Without an Embedder, you can still use the vector API (pass vectors manually) and FTS full-text search.

```go
// Implement the types.Embedder interface
type MyEmbedder struct{}

func (e *MyEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    // Call your vector model, return a fixed-dimension float32 slice
    return []float32{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func (e *MyEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    // Batch embedding
}

func (e *MyEmbedder) Dimension() int {
    return 5 // Return vector dimension
}

// Usage
db, _ := gracedb.Open("/tmp/data",
    gracedb.WithEmbedder(&MyEmbedder{}),
    gracedb.WithIndexType("hnsw"),    // hnsw / ivf / flat / lsh
    gracedb.WithSimilarity("cosine"), // cosine / euclidean
)
```

### 3. Basic Operations

```go
// Create collection
coll, _ := db.CreateCollection("documents")
fmt.Println("collection:", coll.Name)

// Insert vector
embID, _ := db.Upsert("documents", "doc-1",
    []float32{0.1, 0.2, 0.3, 0.4, 0.5},  // vector
    "Hello world",                        // content (auto FTS indexed)
    map[string]string{"source": "test"},  // metadata
    nil,                                  // ACL
)

// Search
results, _ := db.Search("documents",
    []float32{0.1, 0.2, 0.3, 0.4, 0.5},
    gracedb.SearchOptions{
        TopK:            10,
        UseVectorSearch: true,
    },
)
for _, r := range results {
    fmt.Printf("ID: %s, Score: %.4f, Content: %s\n", r.ID, r.Score, r.Content)
}
```

### 4. Text Operations (Requires Embedder)

```go
// Insert text (auto-vectorized)
id, _ := db.InsertText("documents", "text-1", "This is some text", nil)

// Text search (vectorized with Embedder, FTS fallback without)
results, _ := db.SearchText("documents", "text", 10)
```

### 5. Quick API

Quick provides simplified APIs without manual ID management:

```go
q := db.Quick()

// Add (auto-generates UUID)
id, _ := q.Add(ctx, vector, "content")
id, _ = q.AddText(ctx, "some text", nil)

// Search
results, _ := q.Search(ctx, queryVector, 10)
results, _ = q.SearchText(ctx, "search query", 10)

// Pure text search (no Embedder needed)
results, _ = q.SearchTextOnly(ctx, "keyword", 10)
```

### 6. Knowledge Management

```go
// Save knowledge (auto-chunked)
record, _ := db.SaveKnowledge("documents", "wiki-1",
    "Go Language",
    "Go is a statically typed, compiled programming language developed by Google...",
    gracedb.KnowledgeSaveRequest{
        ChunkSize:    500,  // characters per chunk
        ChunkOverlap: 50,   // overlap characters
    },
)

// Search knowledge
resp, _ := db.SearchKnowledge("documents", "programming language", 5)
for _, hit := range resp.Results {
    fmt.Printf("%s: %s\n", hit.Title, hit.Snippet)
}
```

### 7. Agent Memory

```go
// Save memory
record, _ := db.SaveMemory(types.MemorySaveRequest{
    MemoryID:   "mem-1",
    Content:    "User prefers Go over Python",
    Scope:      "user",         // global / user / session
    UserID:     "user-123",
    Namespace:  "preferences",  // optional namespace isolation
    TTLSeconds: 3600,           // optional TTL
})

// Search memory
resp, _ := db.SearchMemory(types.MemorySearchRequest{
    Query:   "prefers",
    UserID:  "user-123",
    Scope:   "user",
    TopK:    5,
})
```

### 8. Property Graph

```go
g := db.Graph()

// Create nodes
g.UpsertNode(&graph.GraphNode{
    ID:   "person-1",
    Type: "person",
    Properties: map[string]string{
        "name": "Alice",
    },
})

g.UpsertNode(&graph.GraphNode{
    ID:   "lang-1",
    Type: "language",
    Properties: map[string]string{
        "name": "Go",
    },
})

// Create edge
g.UpsertEdge(&graph.GraphEdge{
    FromNodeID: "person-1",
    ToNodeID:   "lang-1",
    Type:       "likes",
    Weight:     1.0,
})

// Query neighbors
nodes, edges, _ := g.GetNeighbors("person-1", graph.NeighborOptions{
    Direction: "out",  // out / in / both
    Limit:     10,
})

// Graph traversal
result, _ := g.BFS("person-1", graph.NeighborOptions{MaxDepth: 2})
```

### 9. RDF/SPARQL

```go
rdf := db.RDF()

// Insert triple
rdf.UpsertTriple(&rdf.Triple{
    Subject:   rdf.NewIRI("http://example.org/person/1"),
    Predicate: rdf.NewIRI("http://example.org/likes"),
    Object:    rdf.NewIRI("http://example.org/lang/go"),
})

// SPARQL query
results, _ := rdf.SPARQLSelect(`
    SELECT ?s ?p ?o
    WHERE { ?s ?p ?o . }
`)

// ASK query
exists, _ := rdf.SPARQLAsk(`
    ASK WHERE { <http://example.org/person/1> ?p ?o . }
`)
```

### 10. Backup & Restore

```go
// Backup
err := db.Backup("/tmp/gracedb-backup.db")

// Restore (need to open an empty DB first)
db2, _ := gracedb.Open("/tmp/gracedb-restored")
err = db2.Restore("/tmp/gracedb-backup.db")
```

### 11. MCP Server

```go
server := db.NewMCPServer("gracedb", "1.0.0")
err := server.RunStdio(context.Background())
```

After running, communicate with MCP clients (e.g., Claude Desktop, Cursor) via stdio.

## Index Management

Vector indexes are maintained in memory by default. For persistent scenarios:

```go
// Load index (from snapshot or rebuild)
db.LoadIndex("documents")

// Save index snapshot
db.SaveIndex("documents")

// Rebuild index (clears all FTS entries and rebuilds)
db.RebuildIndex("documents")
```

**Important**: Call `LoadIndex` after startup, otherwise search falls back to full scan (correct but slow).

## Full Example

Run the built-in example program:

```bash
go run examples/main.go
```

Demonstrates: collection CRUD, vector insert/search, text operations, session management, knowledge management, memory management, property graph, backup/restore, and more.

## FAQ

### Q: What if vector dimensions mismatch?

gracedb does not enforce vector dimension validation. Ensure all vectors inserted into the same collection have consistent dimensions. The Embedder's `Dimension()` method is for reference only.

### Q: Difference between FTS and vector search?

- **FTS**: Token-based matching, no Embedder needed, good for keyword search
- **Vector search**: Semantic similarity based, requires Embedder, good for semantic understanding

Both can be used simultaneously, with results fused via RRF.

### Q: In-memory vs disk mode?

- `gracedb.Open("")` — In-memory mode, data not persisted, good for testing
- `gracedb.Open("/path/to/data")` — Disk mode, data persisted to Badger

### Q: How to improve search performance?

1. Use HNSW index (default) and call `LoadIndex` to load it
2. Use `SaveIndex` to persist index snapshots for large collections
3. Use Metadata filters to reduce candidate set
4. Tune HNSW parameters: higher `EfSearch` = higher accuracy but slower

### Q: Does FTS support Chinese?

Yes. Uses the gse segmenter with built-in Chinese segmentation and stop words. The segmenter is lazily initialized on first use.
