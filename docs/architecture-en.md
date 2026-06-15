# gracedb Architecture Overview

## System Architecture

gracedb uses a four-layer architecture with clear responsibilities and strict downward dependencies.

```
┌─────────────────────────────────────────────┐
│                gracedb.DB                    │  ← Facade
│  Quick / Toolbox / Backup / Trace / Ontology │
├─────────────────────────────────────────────┤
│              KnowledgeMemory                 │  ← Recall/Reflect/Consolidate
│  AutoRetain / GroupAggregate                 │
├─────────────────────────────────────────────┤
│              Engine Layer (pkg/*)            │
│  Graph / RDF / Index / MemoryFlow / GraphFlow│
│  SemanticRouter / Hindsight / Quantization    │
├─────────────────────────────────────────────┤
│         BadgerStore                          │  ← Persistence (with in-memory index)
│  CRUD / Search / FTS / Index / Aggregation   │
├─────────────────────────────────────────────┤
│         Badger v4                            │  ← Storage engine
│  LSM-tree / MVCC / ACID                      │
└─────────────────────────────────────────────┘
```

## Layer Responsibilities

### Facade Layer (`pkg/gracedb/`)

The sole public interface. Encapsulates all business logic.

- **Functional Options**: `Open(path, opts...)` via Option functions
- **Telemetry**: Core operations auto-create OpenTelemetry spans and metrics
- **Embedder Proxy**: Holds Embedder reference, auto-executes text→vector conversion
- **FTS Auto-Indexing**: Automatically builds FTS inverted index when writing vectors

### Engine Layer (`pkg/*`)

Independent sub-packages implementing domain capabilities:

- **`index/`** — Vector indexes (HNSW/IVF/Flat/LSH), pure Go
- **`graph/`** — Property graph store, Badger-backed node/edge management
- **`rdf/`** — RDF triple store, SPARQL query engine
- **`quantization/`** — Vector quantization (scalar, product)
- **`mcp/`** — Model Context Protocol JSON-RPC server
- **`memoryflow/`** — Agent memory workflow (transcript → recall → wake-up → diary → promotion)
- **`knowledge/`** — KnowledgeMemory (Recall/Reflect/Consolidate)
- **`graphflow/`** — Corpus-to-graph workflow (extract → build → analyze → export)
- **`semanticrouter/`** — Semantic routing (vector similarity + lexical matching)
- **`hindsight/`** — Recall strategy plugin

### Persistence Layer (`pkg/store/`)

Badger storage wrapper:

- **CRUD** — Embedding/collection/session/document/knowledge CRUD
- **Search** — Vector similarity search (index or full scan) + RRF hybrid fusion
- **FTS** — Full-text search (gse Chinese segmentation + BM25 scoring + synonyms + fuzzy + phrase)
- **Reranker** — BM25/Cosine re-ranking + RRF multi-way fusion
- **Index Management** — `LoadIndex`/`SaveIndex` serializes in-memory index snapshots

### Storage Engine (Badger v4)

High-performance embedded LSM-tree KV database:

- ACID transactions
- MVCC concurrency
- Value Log automatic GC
- Online backup/restore
- Prefix scanning (powers all secondary indexes)

## Data Flow

### Vector Insertion

```
db.Upsert(collection, docID, vector, content, metadata, acl)
    │
    ├──► store.Upsert(...)              # Write to Badger
    │       ├── emb:<cid>:<eid>         # Metadata
    │       ├── emb:vec:<cid>:<eid>     # Vector
    │       └── emb:cont:<cid>:<eid>    # Content
    │
    └──► store.IndexFTS(cid, eid, content)  # FTS index
            └── fts:<token>:<cid>:<eid> = TF count
```

### Search Flow

```
db.Search(collection, query, opts)
    │
    ├──► store.Search(...)
    │       │
    │       ├── Vector search (if UseVectorSearch)
    │       │   ├── In-memory index → idx.Search(query, topK)
    │       │   └── No index → Full scan + CosineSimilarity
    │       │
    │       └── FTS search (if UseTextSearch)
    │           ├── TokenizeForQuery → tokenize + synonyms + fuzzy
    │           └── BM25 scoring + sort
    │
    ├──► RRF Fusion (both enabled)
    │       └── score = 1/(k + rank_vector) + 1/(k + rank_fts)
    │
    └──► Reranker (if configured)
            └── Reranker.Rerank(queryText, results)
```

### KnowledgeMemory Flow

```
km.Recall(query)
    │
    ├──► SearchMemory + SearchKnowledge (parallel)
    ├──► Entity extraction from results
    ├──► Graph expansion (BFS from entity nodes, if MaxHops > 0)
    └──► ContextPack assembly ([MEMORIES]/[KNOWLEDGE]/[GRAPH] sections)

km.Reflect(query)
    │
    ├──► Recall
    ├──► Reflector (LLM or rule-based)
    │       ├── Summary (concatenated top facts)
    │       ├── Themes (from entities + query keywords)
    │       ├── Entities
    │       └── Facts
    └──► KnowledgeMemoryReflection

km.Consolidate(query)
    │
    ├──► Reflect
    ├──► Store summary as memory (role="summary")
    └──► Optionally promote to durable knowledge
```

### Memory Management Flow

```
db.SaveMemory(req)
    │
    ├──► db.embedder.Embed(content)      # when an Embedder is configured
    │
    ├──► resolveMemoryBucket(scope, userID, sessionID, namespace)
    │       ├── global   → memory:global:{namespace}
    │       ├── user     → memory:user:{userID}:{namespace}
    │       └── session  → memory:session:{sessionID}:{namespace}
    │
    └──► store.SaveMemory(...)
            ├── mem:<bucketID>:<id>             # metadata
            ├── mem:content:<bucketID>:<id>     # content
            ├── mem:vec:<bucketID>:<id>         # semantic vector
            ├── mem:fts:<term>:<bucketID>:<id>  # lexical inverted index
            ├── mem:idx:<id>                    # id → bucketID
            └── memoryIndexes[bucketID]         # in-memory vector index
```

### Memory Search Flow

```
db.SearchMemory(req)
    │
    ├──► db.embedder.Embed(query)        # when an Embedder is configured
    │
    └──► store.SearchMemory(...)
            ├── vector search inside the resolved bucket
            ├── mem:fts lexical search inside the resolved bucket
            ├── semantic/lexical/importance/recency score fusion
            │   └── final = semantic*0.60 + lexical*0.25 + importance*0.10 + recency*0.05
            └── expired-memory filtering before TopK
```

## Vector Index System

### Four Index Types

| Index | Algorithm | Use Case | Memory | Accuracy |
|-------|-----------|----------|--------|----------|
| **HNSW** | Hierarchical Navigable Small World | Large scale (>10K vectors) | Medium | Approximate (configurable efSearch) |
| **IVF** | Inverted File Index | Medium scale | Low | Approximate (partition-dependent) |
| **Flat** | Brute-force | Small scale (<1K vectors) | Low | Exact |
| **LSH** | Locality-Sensitive Hashing | Very large scale | Lowest | Approximate |

### Multi-Index Hybrid

Configure multiple index types via `WithIndexTypes([]string{"hnsw", "lsh"})`. Results are fused using weighted RRF across all indexes.

### Index Lifecycle

1. **Create**: `newIndex()` based on `Config.IndexType` / `Config.IndexTypes`
2. **Write**: `Upsert` synchronously updates in-memory index
3. **Search**: `vectorSearch` prioritizes in-memory index, falls back to Badger full scan
4. **Persist**: `SaveIndex` serializes index snapshot to Badger `idx:snapshot:<cid>`
5. **Restore**: `LoadIndex` loads snapshot first, rebuilds from vectors if unavailable
6. **Delete**: `DeleteEmbedding` calls `idx.RemoveVector(id)`

## FTS Full-Text Search System

### Tokenization Pipeline

```
content → gse segmentation → stop word filter → stemmer → synonym expansion → write to Badger
```

### Query Enhancements

- **Phrase search**: `"exact phrase"` double-quoted, requires all tokens in same document
- **Prefix search**: `term*` asterisk suffix, prefix matching
- **Fuzzy search**: `FTSSearchOptions.FuzzyMaxDist` controls Levenshtein edit distance
- **Synonym expansion**: Query terms automatically expanded to synonyms

### BM25 Scoring

Standard BM25 formula:

```
score = IDF × (TF × (k1 + 1)) / (TF + k1 × (1 - b + b × docLen / avgDocLen))
```

Default parameters: k1=1.2, b=0.75. TF count stored in Badger values.

### RRF Hybrid Fusion

Reciprocal Rank Fusion combines vector and FTS results:

```
RRF(d) = Σ 1 / (k + rank_i(d))     # k=60 (standard)
```

## Property Graph Design

### Badger Key Design

```
graph:node:<id>                    → JSON serialized node
graph:edge:<id>                    → JSON serialized edge
graph:edge:from:<fromID>:<type>:<edgeID>  → Outgoing edge index
graph:edge:to:<toID>:<type>:<edgeID>      → Incoming edge index
graph:edge:type:<type>:<edgeID>           → Type index
graph:node:type:<type>:<id>               → Node type index
```

## MCP Protocol Integration

### JSON-RPC 2.0 Flow

```
Client                     Server
  │                          │
  │── initialize ───────────►│
  │◄── serverInfo ──────────│
  │                          │
  │── tools/list ──────────►│
  │◄── tool definitions ────│
  │                          │
  │── tools/call ──────────►│
  │   {name, arguments}     │
  │◄── content (text) ──────│
```

## OpenTelemetry Integration

Core operations automatically instrumented:

- **Upsert / UpsertBatch**: span + counter
- **Search**: span + duration metric
- **Delete**: span

Zero performance overhead when no exporter is configured (noop provider).

## Concurrency Safety

| Component | Lock Strategy |
|-----------|--------------|
| `BadgerStore` | `sync.RWMutex` — Upsert/Delete write lock, Search read lock |
| `GraphStore` | `sync.RWMutex` — Node/edge write lock |
| `MCP Server` | `sync.RWMutex` — Tool registration write lock, call read lock |
| `SemanticRouter` | `sync.RWMutex` — Route registration write lock, Route read lock |
| `HNSW` | No lock — Single-threaded index build (requires external sync) |

## Extension Points

### Embedder Interface

Implement `types.Embedder` to connect any vector model:

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```

### Reranker Interface

Implement `types.Reranker` for custom re-ranking:

```go
type Reranker interface {
    Rerank(query string, candidates []ScoredEmbedding) ([]ScoredEmbedding, error)
}
```

### KnowledgeMemory Reflector

Implement `knowledge.KnowledgeMemoryReflector` for LLM-powered reflection:

```go
type KnowledgeMemoryReflector interface {
    Reflect(ctx context.Context, req KnowledgeMemoryReflectRequest, input KnowledgeMemoryReflectInput) (*KnowledgeMemoryReflection, error)
}
```

### MemoryFlow Strategies

- `QueryPlanner` — Custom retrieval plan generation
- `SessionExtractor` — Custom knowledge extraction
- `PromotionPolicy` — Custom promotion filtering
- `RecallStrategy` — Custom recall strategy (e.g., Hindsight plugin)

### Auto-Retain FactExtractor

Implement `gracedb.FactExtractor` for automatic fact extraction during conversations.

## CortexDB Feature Coverage

After step-by-step validation of 42 feature points (weighted calculation), **current coverage is ~92%**.

### Remaining Gaps

| Feature | Status | Missing | Weight |
|---------|--------|---------|--------|
| LLM-driven entity extraction | ⚠️ 30% | Heuristic-only, no LLM | 4 |
| LLM Reflect | ⚠️ 60% | Rule-based available, needs LLM reflector injection | 4 |

Both gaps are by design — gracedb provides the interfaces (`GraphStore`, `KnowledgeMemoryReflector`), callers implement LLM logic in their own projects.

## Performance Characteristics

### Index Selection Guide

| Data Volume | Recommended Index | Reason |
|-------------|-------------------|--------|
| < 1,000 | Flat | Exact, minimal overhead |
| 1K - 100K | HNSW (efSearch=50) | Accuracy/speed balance |
| 100K - 1M | IVF (nlist=√N) | Memory-efficient |
| > 1M | LSH | Lowest memory, fastest |

### In-Memory Index vs Full Scan

- **In-memory index**: Search O(log N) (HNSW), but requires resident memory
- **Full scan**: Search O(N), reads vectors from Badger each time, suitable for cold data or small datasets

### FTS Performance

- Query O(prefix scan matching document count)
- BM25 scoring executed in memory
- For large document sets, limit TopK or use Metadata filters to reduce candidates
