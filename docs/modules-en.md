# gracedb Module Guide

Responsibilities, key types, and usage scenarios for each sub-package.

## pkg/gracedb — Facade Layer

Main entry point. The `DB` struct aggregates all subsystem capabilities. Callers only need to import the `gracedb` package.

| File | Responsibility |
|------|---------------|
| `db.go` | `DB` struct, `Open/Close`, configuration options, `WithIndexTypes` |
| `vector.go` | Vector CRUD + search (with OpenTelemetry instrumentation) |
| `text.go` | Text auto-vectorization |
| `quick.go` | Quick simplified API |
| `collection.go` | Collection CRUD |
| `knowledge.go` | Knowledge management API |
| `memory.go` | Memory management API |
| `session.go` | Session/message API with AutoRetain integration |
| `doc.go` | Document API + Stats |
| `toolbox.go` | GraphRAG toolbox (9 tools) |
| `backup.go` | Backup/restore |
| `trace.go` | OpenTelemetry spans + metrics |
| `testutil.go` | Test utilities (mockEmbedder, testDB) |
| `aggregation.go` | Aggregation + GroupAggregate |
| `geosearch.go` | Geospatial search |
| `knowledge_memory.go` | KnowledgeMemory facade, AutoRetain, Ontology adapter |
| `ontology.go` | Ontology API (RDFS/SHACL wrapper) |

**Design Patterns**:

- **Functional Options**: `Open(path, opts...)` instead of large Config struct
- **Facade**: Hides store/index details, exposes unified interface
- **Proxy**: Facade holds Embedder, auto-executes text→vector conversion

---

## pkg/knowledge — KnowledgeMemory

Fused recall, reflection, and consolidation of memory and knowledge.

| File | Responsibility |
|------|---------------|
| `types.go` | Request/response types, ContextPack, DBInterface, Graph types |
| `knowledge_memory.go` | KnowledgeMemory: Recall/Reflect/Consolidate, graph expansion |

**Workflow**:

```
Recall → fused memory + knowledge + graph expansion (BFS)
  │
  ├──► Reflect → structured summary (LLM or rule-based)
  │       ├── Summary, Themes, Entities, Facts
  │       └── Source provenance tracking
  │
  └──► Consolidate → store summary + optionally promote to knowledge
```

---

## pkg/store — Persistence Layer

Badger KV wrapper with all CRUD, search, and index management.

| File | Responsibility |
|------|---------------|
| `store.go` | BadgerStore initialization, transaction wrapping |
| `crud.go` | Embedding CRUD, document CRUD |
| `collection.go` | Collection CRUD |
| `search.go` | Vector search, RRF fusion, index persistence, reranker support, multi-index |
| `fts.go` | Full-text search (tokenization, BM25, synonyms, fuzzy matching) |
| `reranker.go` | BM25/Cosine re-ranking |
| `session.go` | Session/message persistence |
| `knowledge.go` | Knowledge persistence |
| `memory.go` | Memory persistence (bucket resolution, TTL filtering) |
| `chunking.go` | Text chunking (by character count + sentence boundary) |
| `similarity.go` | Cosine similarity, Euclidean distance |
| `fuzzy.go` | Levenshtein fuzzy matching |
| `stemmer.go` | Porter stemmer |
| `thesaurus.go` | Synonym table |
| `aggregation.go` | Aggregation + GroupAggregate (GROUP BY) |
| `geosearch.go` | Geospatial coordinate search |

---

## pkg/index — Vector Indexes

Pure Go implementations of four vector index types.

### HNSW (`hnsw.go`)

Hierarchical Navigable Small World graph index.

### IVF (`ivf.go`)

Inverted File Index — vector space partitioning with in-partition brute-force.

### Flat (`flat.go`)

Brute-force search.

### LSH (`lsh.go`)

Locality-Sensitive Hashing.

### Multi-Index (`multi.go`)

Combines multiple index types with weighted RRF fusion.

**Common Interface** (`types.go`):

```go
type Index interface {
    Insert(vector []float32, id string)
    InsertBatch(vectors [][]float32, ids []string)
    RemoveVector(id string)
    Search(query []float32, topK int) ([]SearchResult, error)
    Len() int
    Marshal() ([]byte, error)
    Unmarshal(data []byte) error
}
```

---

## pkg/quantization — Vector Quantization

### Scalar Quantization (`scalar.go`)

Compresses float32 vectors to low-bit integer representation.

### Product Quantization (`product.go`)

Partitions high-dimensional vectors into subspaces, each independently quantized via codebook.

---

## pkg/graph — Property Graph Store

Lightweight property graph engine on Badger.

| File | Responsibility |
|------|---------------|
| `types.go` | Node, edge, path, traversal result types |
| `store.go` | CRUD operations, index management, neighbor queries |
| `traversal.go` | BFS/DFS/shortest path algorithms |

**Key Design**: Each edge operation maintains 4 Badger keys (main + 2 direction indexes + type index).

---

## pkg/rdf — RDF/SPARQL

RDF triple store with simplified SPARQL query engine.

| File | Responsibility |
|------|---------------|
| `store.go` | Triple CRUD, pattern queries, SPARQL SELECT/ASK |
| `sparql.go` | Simplified SPARQL parser (lexer + triple pattern parsing) |
| `ntriples.go` | N-Triples format import/export |
| `rdfs.go` | RDFS inference (6 rules: subClassOf transitivity, type propagation, subPropertyOf, domain, range) |
| `shacl.go` | SHACL constraint validation (minCount, maxCount, datatype, pattern) |

---

## pkg/mcp — MCP Protocol Server

Simplified Model Context Protocol JSON-RPC server.

**Protocol Methods**:
- `initialize` — Returns server info and protocol version
- `tools/list` — Returns all registered tools
- `tools/call` — Calls a tool, returns text content

**Transport**: One JSON request/response per line via stdin/stdout.

---

## pkg/memoryflow — Memory Workflow

Complete Agent memory workflow pipeline.

| File | Responsibility |
|------|---------------|
| `types.go` | Recall request/response, transcript, promotion, wake-up layer types |
| `service.go` | Core service (transcript, recall, wake-up, diary, session close) |
| `strategy.go` | Recall strategy interface + Passthrough default |
| `promotion.go` | Knowledge promotion logic (classification, scoring) |

**Extension Points**:
- `QueryPlanner` — Custom retrieval plan
- `SessionExtractor` — Custom dialogue knowledge extraction
- `PromotionPolicy` — Custom promotion rules
- `RecallStrategy` — Custom recall strategy

---

## pkg/graphflow — Corpus-to-Graph Workflow

Complete pipeline from text to property graph.

| File | Responsibility |
|------|---------------|
| `pipeline.go` | Types, Build, Analyze, RenderReport |
| `extract.go` | HeuristicExtractor (rule-based entity extraction) |

**Functions**:
- **Build** — Writes extraction results to graph store
- **Analyze** — Graph statistics (nodes, edges, degree distribution, density)
- **RenderReport** — Output as text / markdown / HTML

---

## pkg/semanticrouter — Semantic Router

Routes user queries to predefined processing paths.

| File | Responsibility |
|------|---------------|
| `router.go` | Semantic routing (vector similarity matching) |
| `lexical.go` | Lexical routing (keyword matching) |
| `hybrid.go` | Hybrid routing (lexical + semantic) |

**Use Case**: Intent classification before LLM calls, reducing unnecessary API invocations.

---

## pkg/hindsight — Recall Strategy Plugin

Pluggable recall strategy for MemoryFlow, enriching recall requests with bank ID, entity names, and keyword cues.

---

## pkg/types — Common Types

Shared types and interfaces across all sub-packages.

| File | Content |
|------|---------|
| `embedding.go` | Embedding, ScoredEmbedding, SearchOptions, Collection, Config, StoreStats |
| `embedder.go` | Embedder interface |
| `reranker.go` | Reranker interface |
| `knowledge.go` | KnowledgeRecord, KnowledgeSaveRequest/Response/Hit, Chunk |
| `memory.go` | MemoryRecord, MemorySave/Update/Search requests and responses |
| `similarity.go` | Similarity computation functions |
| `errors.go` | Sentinel error definitions |

---

## Module Dependencies

```
gracedb (facade)
├── store (persistence)
│   ├── index (vector indexes)
│   ├── types (common types)
│   └── quantization (quantization)
├── knowledge (KnowledgeMemory)
│   └── types
├── graph (property graph)
├── rdf (RDF/SPARQL)
├── mcp (MCP service)
│   └── types
├── memoryflow (memory workflow)
│   ├── types
│   └── hindsight (strategy plugin)
├── graphflow (corpus-to-graph)
│   └── graph
├── semanticrouter (semantic routing)
│   └── types (Embedder interface)
└── types (common types)
```

Each engine sub-package (graph, rdf, memoryflow, etc.) is independent, only depending on `pkg/types`. The facade layer orchestrates and calls.
