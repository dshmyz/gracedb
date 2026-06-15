# gracedb

Go Embedded AI Memory + Knowledge Graph Database

## Overview

gracedb is a Go embedded AI memory and knowledge graph database built on Badger KV storage. It provides vector search, full-text search, knowledge management, session management, property graphs, RDF/SPARQL, and MCP services — all in a single, zero-external-dependency library.

## Maturity

gracedb is an early-stage embedded AI database. The core storage, vector search,
FTS, semantic memory, graph, RDF, and MCP surfaces are implemented and covered by
Go tests, but several advanced AI workflows are still simplified or
caller-extended. Treat the project as suitable for local applications,
experiments, and controlled internal use before hardening it for production.

### Stable Core

Embedder interface, text auto-vectorization, Quick API, vector CRUD,
HNSW/IVF/Flat/LSH indexes, full-text search, metadata filtering, semantic Agent
Memory, knowledge storage, session/document management, backup/restore, and
OpenTelemetry hooks.

### Experimental Or Simplified

| Area | Current state |
|------|---------------|
| LLM-driven entity extraction | Built-in extraction is heuristic; callers can inject LLM extractors. |
| LLM Reflect | Built-in reflection is rule-based; callers can inject LLM reflectors. |
| SPARQL | Supports a simplified SELECT/ASK subset. |
| GraphRAG workflows | Useful pipeline primitives, not a full managed RAG product. |
| Index recovery | Stored vectors are durable and searchable after reopen; production deployments should still add crash/recovery validation for their workload. |

## Documentation

| Document | Content |
|----------|---------|
| [Getting Started](docs/getting-started.md) | Installation, configuration, quick start, FAQ |
| [API Reference](docs/api-reference.md) | Complete API, data types, storage key formats |
| [Architecture](docs/architecture.md) | Layered architecture, data flow, index system, extension points |
| [Module Guide](docs/modules.md) | Package responsibilities, key types, dependency graph |
| [Implementation Plan](IMPLEMENTATION_PLAN.md) | Phased development roadmap |
| [中文版](README_CN.md) | 中文文档 |

## Features

- **Vector Search** — HNSW / IVF / Flat / LSH indexes with cosine similarity
- **Full-Text Search** — Chinese segmentation + stop words + Levenshtein fuzzy + RRF hybrid fusion
- **Knowledge Storage** — auto-chunking + FTS indexing + document-level aggregation
- **Agent Memory** — scope/namespace/TTL isolation with semantic + lexical hybrid retrieval
- **Property Graph** — node/edge CRUD, BFS/DFS/shortest path traversal
- **RDF/SPARQL** — N-Triples import/export, SPARQL SELECT/ASK, RDFS inference, SHACL validation
- **GraphRAG Toolbox** — 9 built-in tools for LLM orchestration
- **MCP Service** — Model Context Protocol compatible, stdio transport
- **Backup/Restore** — Badger native full backup
- **OpenTelemetry** — automatic spans and metrics on core operations
- **KnowledgeMemory** — fused memory + knowledge recall with reflection and consolidation
- **Auto-Retain** — automatic fact extraction during conversation

## Quick Start

```bash
go get github.com/dshmyz/gracedb
```

```go
package main

import (
    "fmt"
    "log"

    "github.com/dshmyz/gracedb/pkg/gracedb"
)

func main() {
    // Open database
    db, err := gracedb.Open("/tmp/gracedb-data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // Create collection
    coll, _ := db.CreateCollection("my_docs")
    fmt.Println("created:", coll.Name)

    // Insert vector
    vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
    embID, _ := db.Upsert("my_docs", "doc-1", vec, "Hello world", nil, nil)
    fmt.Println("embedded:", embID)

    // Search
    results, _ := db.Search("my_docs", vec, gracedb.SearchOptions{
        TopK:            5,
        UseVectorSearch: true,
    })
    fmt.Println("found:", len(results), "results")

    // Backup
    db.Backup("/tmp/gracedb-backup.db")

    // Quick API
    q := db.Quick()
    id, _ := q.AddToCollection(ctx, "my_docs", vec, "Quick add")
    fmt.Println("quick add:", id)
}
```

## Configuration

```go
db, _ := gracedb.Open("/tmp/data",
    gracedb.WithIndexType("hnsw"),         // hnsw / ivf / flat / lsh
    gracedb.WithIndexTypes([]string{"hnsw", "lsh"}), // multi-index hybrid
    gracedb.WithSimilarity("cosine"),       // cosine / euclidean
    gracedb.WithEmbedder(myEmbedder),       // types.Embedder interface
)
```

## Architecture

```
┌─────────────────────────────────────────────┐
│                gracedb.DB                    │  ← Facade
│  Quick / Toolbox / Backup / Trace / Ontology │
├─────────────────────────────────────────────┤
│              KnowledgeMemory                 │  ← Recall/Reflect/Consolidate
│  AutoRetain / GroupAggregate                 │
├─────────────────────────────────────────────┤
│         BadgerStore                          │  ← Persistence (with in-memory index)
│  CRUD / Search / FTS / Index / Aggregation   │
├─────────────────────────────────────────────┤
│         GraphStore / RDF                     │  ← Graph engine
│  Nodes/Edges/Traversal/SPARQL/RDFS/SHACL     │
├─────────────────────────────────────────────┤
│         Badger v4                            │  ← Storage engine
│  LSM-tree / MVCC / ACID                      │
└─────────────────────────────────────────────┘
```

## Usage Examples

### Text Auto-Vectorization

```go
db, _ := gracedb.Open("/tmp/data", gracedb.WithEmbedder(myEmbedder))

// Insert text (auto-vectorized)
id, _ := db.InsertText("docs", "text-1", "This is Chinese text", nil)

// Text search (vectorized or FTS fallback)
results, _ := db.SearchText("docs", "Chinese", 10)
```

### Knowledge Management

```go
// Save knowledge (auto-chunked)
record, _ := db.SaveKnowledge("docs", "wiki-1", "Go Language",
    "Go is a statically typed, compiled language...",
    types.KnowledgeSaveRequest{ChunkSize: 500, ChunkOverlap: 50})

// Search knowledge
resp, _ := db.SearchKnowledge("docs", "programming language", 5)
```

### Agent Memory

```go
// Save memory with scope/namespace
db.SaveMemory(types.MemorySaveRequest{
    MemoryID:  "mem-1",
    Content:   "User prefers Go over Python",
    Scope:     "user",
    UserID:    "user-123",
    Namespace: "preferences",
    TTLSeconds: 3600,
})

// Search memory
resp, _ := db.SearchMemory(types.MemorySearchRequest{
    Query:  "prefers",
    UserID: "user-123",
    Scope:  "user",
    TopK:   5,
})
```

### Property Graph

```go
g := db.Graph()

g.UpsertNode(&graph.GraphNode{
    ID: "person-1", Type: "person",
    Properties: map[string]string{"name": "Alice"},
})
g.UpsertNode(&graph.GraphNode{
    ID: "lang-1", Type: "language",
    Properties: map[string]string{"name": "Go"},
})
g.UpsertEdge(&graph.GraphEdge{
    FromNodeID: "person-1", ToNodeID: "lang-1",
    Type: "likes", Weight: 1.0,
})

// Query neighbors
nodes, edges, _ := g.GetNeighbors("person-1", graph.NeighborOptions{
    Direction: "out", Limit: 10,
})

// Traversal
result, _ := g.BFS("person-1", graph.NeighborOptions{MaxDepth: 2})
```

### RDF/SPARQL

```go
rdf := db.RDF()

// Insert triple
rdf.UpsertTriple(&rdf.Triple{
    Subject:   rdf.NewIRI("http://example.org/person/1"),
    Predicate: rdf.NewIRI("http://example.org/likes"),
    Object:    rdf.NewIRI("http://example.org/lang/go"),
})

// SPARQL query
results, _ := rdf.SPARQLSelect(`SELECT ?s ?p ?o WHERE { ?s ?p ?o . }`)

// ASK query
exists, _ := rdf.SPARQLAsk(`ASK WHERE { <http://example.org/person/1> ?p ?o . }`)
```

### Ontology Management

```go
o := db.Ontology()

// Define ontology
o.DefineClass("http://example.org/Person", "")
o.DefineClass("http://example.org/Developer", "http://example.org/Person")
o.DefineProperty("http://example.org/knows", "http://example.org/Person", "http://example.org/Person")

// Add facts
o.AddFact("http://example.org/person/alice", "http://example.org/knows", "Bob")

// RDFS inference (materialize new triples)
count, _ := o.Infer()

// SHACL validation
report, _ := o.Validate()
```

### KnowledgeMemory (Recall/Reflect/Consolidate)

```go
km := db.KnowledgeMemory(nil) // nil = use rule-based reflector

// Recall: fused memory + knowledge + graph expansion
resp, _ := km.Recall(ctx, knowledge.KnowledgeMemoryRecallRequest{
    Query:         "what does the user like?",
    TopKMemories:  5,
    TopKKnowledge: 4,
    MaxHops:       2, // graph expansion depth
})

// Reflect: synthesize structured summary
reflection, _ := km.Reflect(ctx, knowledge.KnowledgeMemoryReflectRequest{
    Query: "user preferences",
})
fmt.Println("Summary:", reflection.Summary)
fmt.Println("Themes:", reflection.Themes)

// Consolidate: store summary + optionally promote to knowledge
consolidated, _ := km.Consolidate(ctx, knowledge.KnowledgeMemoryConsolidateRequest{
    Reflect: knowledge.KnowledgeMemoryReflectRequest{
        Query: "user preferences",
    },
    PromoteToKnowledge: true,
})
```

### Auto-Retain (Automatic Fact Extraction)

```go
db.SetFactExtractor(func(ctx context.Context, msgs []*types.Message) ([]gracedb.ExtractedFact, error) {
    // Extract facts from conversation (can use LLM here)
    return []gracedb.ExtractedFact{
        {ID: "fact-1", Content: "User likes Go", Type: "preference"},
    }, nil
})

db.SetAutoRetain(gracedb.AutoRetainConfig{
    Enabled:      true,
    WindowSize:   6,
    TriggerEvery: 2, // extract every 2 messages
})

// AddMessage now triggers extraction automatically
db.AddMessage(&types.Message{SessionID: "sess-1", Role: "user", Content: "I like Go"})
db.AddMessage(&types.Message{SessionID: "sess-1", Role: "assistant", Content: "Go is great!"})
// → AutoRetain fires, extracts facts, stores as memory
```

### Aggregation

```go
// Simple aggregation
result, _ := db.Aggregate("docs", "score", store.AggAvg)
fmt.Printf("Average score: %.2f\n", result.Avg)

// GROUP BY aggregation
groups, _ := db.GroupAggregate("docs", "category", "price", store.AggAvg)
for category, r := range groups {
    fmt.Printf("%s: avg=%.2f, count=%d\n", category, r.Avg, r.Count)
}
```

### MCP Server

```go
server := db.NewMCPServer("gracedb", "1.0.0")
server.RunStdio(context.Background())
```

### Backup & Restore

```go
db.Backup("/tmp/backup.db")

db2, _ := gracedb.Open("/tmp/restored")
db2.Restore("/tmp/backup.db")
```

## Index Management

```go
// Load index (from snapshot or rebuild) — call on startup
db.LoadIndex("docs")

// Save index snapshot
db.SaveIndex("docs")

// Rebuild FTS index
db.RebuildIndex("docs")
```

**Important**: Call `LoadIndex` after startup, otherwise search falls back to full scan (correct but slow).

## Storage Key Format

| Key Pattern | Purpose |
|-------------|---------|
| `coll:<name>` | Collection metadata |
| `emb:<cid>:<eid>` | Embedding metadata |
| `emb:vec:<cid>:<eid>` | Vector data |
| `fts:<token>:<cid>:<eid>` | FTS inverted index (value = TF count) |
| `mem:<bucket>:<id>` | Memory metadata |
| `mem:content:<bucket>:<id>` | Memory content |
| `mem:vec:<bucket>:<id>` | Memory semantic vector |
| `mem:fts:<token>:<bucket>:<id>` | Memory lexical inverted index |
| `mem:idx:<id>` | Memory ID to bucket index |
| `graph:node:<id>` | Graph node |
| `graph:edge:<id>` | Graph edge |
| `rdf:t:<id>` | RDF triple |
| `idx:snapshot:<cid>` | Vector index snapshot |
| `sess:<id>` | Session |

## Examples

```bash
go run examples/main.go
```

## Testing

```bash
go test ./...              # All tests
go test -v ./pkg/index/    # Verbose output
go test -bench=. ./pkg/store/  # Benchmarks
```

## Acknowledgments

gracedb is inspired by and built upon the work of these excellent projects:

- **[CortexDB](https://github.com/liliang-cn/cortexdb)** — gracedb targets CortexDB feature parity as its design reference. The KnowledgeMemory, AutoRetain, GraphRAG toolbox, and MemoryFlow workflows are all modeled after CortexDB's architecture and APIs.
- **[Badger](https://github.com/dgraph-io/badger)** — gracedb uses Badger as its underlying storage engine. Badger's high-performance LSM-tree, MVCC, and ACID transaction support make gracedb's embedded design possible.

## License

MIT
