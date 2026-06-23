# gracedb API Reference

Complete API reference covering all public interfaces, types, and configuration options.

## Table of Contents

- [Core Entry](#core-entry)
- [Collection Management](#collection-management)
- [Vector Operations](#vector-operations)
- [Text Operations](#text-operations)
- [Search](#search)
- [Knowledge Management](#knowledge-management)
- [Memory Management](#memory-management)
- [Session Management](#session-management)
- [Document Management](#document-management)
- [Property Graph](#property-graph)
- [RDF/SPARQL](#rdfsparql)
- [GraphRAG Toolbox](#graphrag-toolbox)
- [MCP Server](#mcp-server)
- [Backup & Restore](#backup--restore)
- [Aggregation](#aggregation)
- [Geo Search](#geo-search)
- [Index Management](#index-management)
- [Quick API](#quick-api)
- [MemoryFlow Workflow](#memoryflow-workflow)
- [KnowledgeMemory (Recall/Reflect/Consolidate)](#knowledgememory-recallreflectconsolidate)
- [Auto-Retain](#auto-retain)
- [Ontology API](#ontology-api)
- [GraphFlow Workflow](#graphflow-workflow)
- [Semantic Router](#semantic-router)
- [Hindsight Strategy](#hindsight-strategy)
- [Configuration Options](#configuration-options)
- [Data Types](#data-types)
- [Error Types](#error-types)

---

## Core Entry

### `gracedb.Open(path string, opts ...Option) (*DB, error)`

Opens or creates a gracedb database instance. Empty `path` enables in-memory mode.

```go
db, err := gracedb.Open("/tmp/data",
    gracedb.WithEmbedder(myEmbedder),
    gracedb.WithIndexType("hnsw"),
    gracedb.WithIndexTypes([]string{"hnsw", "lsh"}), // multi-index hybrid
    gracedb.WithSimilarity("cosine"),
)
```

### `(*DB).Close() error`

Closes the database, ensuring data is flushed to disk.

### `(*DB).Vector() *store.BadgerStore`

Returns the underlying BadgerStore instance.

### `(*DB).Graph() *graph.GraphStore`

Returns the property graph store.

### `(*DB).RDF() *rdf.Store`

Returns the RDF triple store.

### `(*DB).HasEmbedder() bool`

Reports whether an embedder is configured.

### `(*DB).Quick() *Quick`

Returns the Quick helper for simplified operations.

### `(*DB).GraphRAGTools() *GraphRAGToolbox`

Returns the GraphRAG toolbox for LLM orchestration (9 tools).

### `(*DB).NewMCPServer(name, version string) *mcp.Server`

Creates an MCP server exposing all gracedb tools.

### `(*DB).KnowledgeMemory(reflector KnowledgeMemoryReflector) *knowledge.KnowledgeMemory`

Returns the KnowledgeMemory facade for fused recall, reflection, and consolidation.

### `(*DB).Ontology() *Ontology`

Returns the ontology manager for RDF/RDFS/SHACL operations.

---

## Collection Management

### `(*DB).CreateCollection(name string) (*types.Collection, error)`

Creates a new collection.

### `(*DB).GetCollection(name string) (*types.Collection, error)`

Gets a collection by name.

### `(*DB).ListCollections() ([]*types.Collection, error)`

Lists all collections.

### `(*DB).DeleteCollection(name string) error`

Deletes a collection and all its data.

---

## Vector Operations

### `(*DB).Upsert(collectionName, docID string, vector []float32, content string, metadata map[string]string, acl []string) (string, error)`

Inserts or updates a single vector. Returns embedding ID. Auto-indexes content for FTS.

### `(*DB).UpsertBatch(collectionName string, vectors [][]float32, contents []string, docIDs []string, metadata []map[string]string) error`

Batch insert or update vectors.

### `(*DB).GetEmbedding(collectionName, embID string, includeVector bool) (*types.Embedding, error)`

Gets an embedding by ID.

### `(*DB).DeleteEmbedding(collectionName, embID string) error`

Deletes a single embedding and its FTS index.

### `(*DB).DeleteByDocID(collectionName, docID string) error`

Deletes all embeddings for a document.

### `(*DB).DeleteEmbeddingBatch(collectionName string, ids []string) error`

Batch delete embeddings.

### `(*DB).EmbeddingCount(collectionName string) (int, error)`

Returns the number of embeddings in a collection.

### `(*DB).ListEmbeddingIDs(collectionName string) ([]string, error)`

Lists all embedding IDs for a collection.

---

## Text Operations

### `(*DB).InsertText(collectionName, docID, content string, metadata map[string]string) (string, error)`

Inserts text with automatic embedding.

### `(*DB).InsertTextBatch(collectionName string, contents []string, docIDs []string, metadata []map[string]string) ([]string, error)`

Batch insert texts with automatic embedding.

### `(*DB).SearchText(collectionName string, query string, topK int) ([]types.ScoredEmbedding, error)`

Text search. Vectorized with Embedder, FTS fallback without.

---

## Search

### `(*DB).Search(collectionName string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error)`

Hybrid search supporting vector similarity, full-text search, or RRF-fused combination.

`SearchOptions` fields:

| Field | Type | Description |
|-------|------|-------------|
| `TopK` | `int` | Number of results to return |
| `Threshold` | `float32` | Minimum similarity threshold |
| `Filter` | `map[string]string` | Reserved filter field |
| `Collection` | `string` | FTS search target collection name |
| `UseVectorSearch` | `bool` | Enable vector search |
| `UseTextSearch` | `bool` | Enable full-text search |
| `MetadataFilter` | `map[string]string` | Exact metadata key=value match |
| `ACL` | `[]string` | Access control list filter |
| `MetadataExists` | `[]string` | Require metadata keys to exist |
| `DocID` | `string` | Filter by document ID |
| `QueryText` | `string` | Original text query for reranker |
| `Reranker` | `types.Reranker` | Secondary re-ranking after retrieval |

---

## Knowledge Management

### `(*DB).SaveKnowledge(collectionName, knowledgeID, title, content string, opts types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error)`

Saves or replaces a knowledge item. Auto-chunks and indexes for FTS.

### `(*DB).GetKnowledge(collectionName, knowledgeID string) (*types.KnowledgeRecord, error)`

Gets a knowledge item.

### `(*DB).UpdateKnowledge(collectionName, knowledgeID string, req types.KnowledgeUpdateRequest) (*types.KnowledgeRecord, error)`

Updates a knowledge item.

### `(*DB).DeleteKnowledge(collectionName, knowledgeID string) error`

Deletes a knowledge item and its chunks.

### `(*DB).SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error)`

Searches knowledge items, aggregating chunk results by document.

---

## Memory Management

Memory management provides Agent memory with scope/namespace/TTL isolation. When
`WithEmbedder` is configured, `SaveMemory` automatically stores a semantic
vector, and `SearchMemory` combines semantic vector retrieval with lexical
inverted-index retrieval. Without an embedder, memory search still works in
lexical mode.

Scope resolution:

| Scope | Required fields | Bucket |
|-------|-----------------|--------|
| `global` | none | `memory:global:<namespace>` |
| `user` | `UserID` | `memory:user:<userID>:<namespace>` |
| `session` | `SessionID` | `memory:session:<sessionID>:<namespace>` |

Empty `Namespace` resolves to `default`. Search is limited to the resolved bucket
and does not cross user/session/namespace boundaries.

### `(*DB).SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error)`

Stores memory with scope/namespace bucketing, importance, and TTL. With an
embedder, `Embed(content)` is persisted under `mem:vec:<bucketID>:<memoryID>`.
The lexical index is stored under `mem:fts:<term>:<bucketID>:<memoryID>`.

### `(*DB).GetMemory(memoryID string) (*types.MemoryRecord, error)`

Gets a memory record.

### `(*DB).UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error)`

Updates a memory record. Content changes rebuild the semantic vector and lexical
index and remove old index entries.

### `(*DB).DeleteMemory(memoryID string) error`

Deletes a memory record and its metadata, content, semantic vector, lexical
index, and memoryID bucket index.

### `(*DB).SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error)`

Searches memories in the resolved bucket. Default score fusion:

```text
final = semantic*0.60 + lexical*0.25 + importance*0.10 + recency*0.05
```

Each `MemorySearchHit` includes `Score`/`FinalScore` and explanation fields:
`SemanticScore`, `LexicalScore`, `ImportanceScore`, and `RecencyScore`. Expired
memories are filtered before results are returned.

Optional `MemorySearchRequest` ranking weights:

| Field | Default | Description |
|-------|---------|-------------|
| `SemanticWeight` | `0.60` | Semantic vector score weight |
| `LexicalWeight` | `0.25` | Lexical retrieval score weight |
| `ImportanceWeight` | `0.10` | Memory importance weight |
| `RecencyWeight` | `0.05` | Updated-at freshness weight |
| `RecencyHalfLife` | `7 days` | Recency score half-life; shorter values favor newer memories more strongly |

When all four weights are `0`, gracedb uses defaults. If any weight is set, the
caller-provided weight combination is used as-is. `RecencyHalfLife=0` uses the
default 7-day half-life.

---

## Session Management

### `(*DB).CreateSession(name string) (*types.Session, error)`

Creates a session.

### `(*DB).GetSession(id string) (*types.Session, error)`

Gets a session.

### `(*DB).ListSessions() ([]*types.Session, error)`

Lists all sessions.

### `(*DB).DeleteSession(id string) error`

Deletes a session and all its messages.

### `(*DB).AddMessage(msg *types.Message) error`

Adds a message to a session. Triggers AutoRetain if configured.

### `(*DB).GetSessionHistory(sessionID string, limit int) ([]*types.Message, error)`

Gets session message history.

### `(*DB).DeleteMessage(sessionID, messageID string) error`

Deletes a single message.

---

## Document Management

### `(*DB).CreateDocument(doc *types.Document) error`

Creates a document.

### `(*DB).GetDocument(id string) (*types.Document, error)`

Gets a document.

### `(*DB).ListDocuments() ([]*types.Document, error)`

Lists all documents.

### `(*DB).DeleteDocument(id string) error`

Deletes a document.

### `(*DB).Stats() (types.StoreStats, error)`

Returns database statistics.

---

## Property Graph

### `(*graph.GraphStore).UpsertNode(node *graph.GraphNode) error`

Inserts or updates a node.

### `(*graph.GraphStore).GetNode(id string) (*graph.GraphNode, error)`

Gets a node.

### `(*graph.GraphStore).DeleteNode(id string) error`

Deletes a node and its incident edges.

### `(*graph.GraphStore).UpsertEdge(edge *graph.GraphEdge) error`

Inserts or updates an edge. Auto-maintains from/to/type indexes.

### `(*graph.GraphStore).GetEdge(id string) (*graph.GraphEdge, error)`

Gets an edge.

### `(*graph.GraphStore).DeleteEdge(id string) error`

Deletes an edge and its indexes.

### `(*graph.GraphStore).GetNeighbors(nodeID string, opts graph.NeighborOptions) ([]*graph.GraphNode, []*graph.GraphEdge, error)`

Gets node neighbors. Supports direction, edge type, node type filters, and limit.

### `(*graph.GraphStore).BFS(startNodeID string, opts graph.NeighborOptions) (*graph.TraversalResult, error)`

Breadth-first traversal.

### `(*graph.GraphStore).DFS(startNodeID string, opts graph.NeighborOptions) (*graph.TraversalResult, error)`

Depth-first traversal.

### `(*graph.GraphStore).ShortestPath(fromID, toID string) (*graph.PathResult, error)`

Shortest path (BFS-based, unweighted).

---

## RDF/SPARQL

### `(*rdf.Store).UpsertTriple(t *rdf.Triple) error`

Inserts or replaces an RDF triple.

### `(*rdf.Store).DeleteTriple(id string) error`

Deletes a triple.

### `(*rdf.Store).Query(pattern rdf.TriplePattern) ([]rdf.Triple, error)`

Pattern match query. Nil fields are wildcards.

### `(*rdf.Store).SPARQLSelect(query string) ([]map[string]rdf.Term, error)`

Simplified SPARQL SELECT.

### `(*rdf.Store).SPARQLAsk(query string) (bool, error)`

Simplified SPARQL ASK.

### `(*rdf.Store).RegisterNamespace(prefix, uri string)`

Registers a prefix mapping.

### `(*rdf.Store).ExpandPrefix(prefixed string) string`

Expands prefix to full IRI.

### `(*rdf.Store).Count() (int, error)`

Returns total triple count.

### `(*rdf.Store).ImportNTriples(data string) (int, error)`

Imports N-Triples format data.

### `(*rdf.Store).ExportNTriples() (string, error)`

Exports as N-Triples format.

### `(*rdf.Store).RDFSInfer() (int, error)`

RDFS inference, materializes new triples.

### `(*rdf.Store).SHACLValidate() (*rdf.ValidationResult, error)`

SHACL constraint validation.

---

## GraphRAG Toolbox

9 tools available via `db.GraphRAGTools()`.

| Tool | Description | Required Params |
|------|-------------|-----------------|
| `search_knowledge` | Search knowledge items | `query` |
| `save_knowledge` | Save knowledge item | `knowledge_id`, `content` |
| `search_memory` | Search memories | `query` |
| `save_memory` | Save memory | `memory_id`, `content`, `scope` |
| `expand_graph` | Expand graph neighborhood | `node_ids`, `max_depth` |
| `recall_knowledge_memory` | Fused memory + knowledge recall | `query` |
| `reflect` | Synthesize structured reflection | `query` |
| `consolidate` | Reflect, store summary, promote to knowledge | `query` |
| `build_context` | Assemble prompt context pack | `query` |

---

## KnowledgeMemory (Recall/Reflect/Consolidate)

### `(*knowledge.KnowledgeMemory).Recall(ctx, req) (*KnowledgeMemoryRecallResponse, error)`

Fused retrieval: memory + knowledge + graph expansion (if `MaxHops > 0`).

### `(*knowledge.KnowledgeMemory).Reflect(ctx, req) (*KnowledgeMemoryReflection, error)`

Synthesizes a structured reflection (Summary, Themes, Entities, Facts) from retrieved context. Uses rule-based reflector if none provided.

### `(*knowledge.KnowledgeMemory).Consolidate(ctx, req) (*KnowledgeMemoryConsolidateResponse, error)`

Reflects over context, stores summary as memory, optionally promotes to durable knowledge.

### `(*knowledge.KnowledgeMemory).Remember(ctx, req) (*types.MemoryRecord, error)`

Stores one episodic memory item.

---

## Auto-Retain

### `(*DB).SetFactExtractor(fn FactExtractor)`

Registers a fact extractor for auto-retention during conversations.

### `(*DB).SetAutoRetain(cfg AutoRetainConfig)`

Enables automatic fact extraction on `AddMessage`.

```go
db.SetFactExtractor(func(ctx context.Context, msgs []*types.Message) ([]ExtractedFact, error) {
    // Extract facts (can use LLM here)
    return []ExtractedFact{
        {ID: "fact-1", Content: "User likes Go", Type: "preference"},
    }, nil
})

db.SetAutoRetain(AutoRetainConfig{
    Enabled:      true,
    WindowSize:   6,
    TriggerEvery: 2,
})
```

---

## Ontology API

### `(*Ontology).DefineClass(classIRI, parentIRI string) error`

Defines an RDFS class with optional parent (subClassOf).

### `(*Ontology).DefineProperty(propIRI, domainIRI, rangeIRI string) error`

Defines a property with optional domain and range.

### `(*Ontology).DefineSubProperty(childIRI, parentIRI string) error`

Defines a subPropertyOf relationship.

### `(*Ontology).AddType(resourceIRI, classIRI string) error`

Asserts that a resource is an instance of a class.

### `(*Ontology).AddFact(resourceIRI, predicateIRI, value string) error`

Adds a simple triple with literal value.

### `(*Ontology).AddTypedFact(resourceIRI, predicateIRI, value, datatype string) error`

Adds a triple with typed literal value.

### `(*Ontology).AddRelation(subjectIRI, predicateIRI, objectIRI string) error`

Adds an object property (resource → resource).

### `(*Ontology).Infer() (int, error)`

Runs RDFS inference, materializes new triples. Returns count.

### `(*Ontology).ClearInferred() error`

Removes all previously inferred triples.

### `(*Ontology).DefineShape(shapeIRI, targetClassIRI string, constraints []SHACLConstraint) error`

Defines a SHACL node shape with property constraints.

### `(*Ontology).Validate() (*rdf.ValidationReport, error)`

Runs SHACL validation against all defined shapes.

### `(*Ontology).Query(subject, predicate, object string) ([]rdf.Triple, error)`

Pattern match query.

### `(*Ontology).SPARQLSelect(query string) ([]map[string]rdf.Term, error)`

SPARQL SELECT query.

### `(*Ontology).SPARQLAsk(query string) (bool, error)`

SPARQL ASK query.

---

## MCP Server

### MCP Methods

| Method | Description |
|--------|-------------|
| `initialize` | Initialize connection |
| `tools/list` | List available tools |
| `tools/call` | Call a tool |

---

## Backup & Restore

### `(*DB).Backup(path string) error`

Creates a full backup. Flushes to disk first. Uses Badger native backup API.

### `(*DB).Restore(backupPath string) error`

Restores from backup. Rebuilds all collection vector indexes after restore.

---

## Aggregation

### `(*DB).Aggregate(collectionName string, metadataKey string, aggType store.AggregationType) (*store.AggregationResult, error)`

Aggregates over embedding metadata (count/sum/avg/min/max).

### `(*DB).GroupAggregate(collectionName string, groupKey string, valueKey string, aggType store.AggregationType) (map[string]*store.AggregationResult, error)`

GROUP BY aggregation. Groups by `groupKey`, aggregates `valueKey`.

---

## Geo Search

### `(*DB).SearchGeo(collectionName string, query store.GeoQuery, opts types.SearchOptions) ([]types.ScoredEmbedding, error)`

Geospatial filtering on embeddings with lat/lon metadata using Haversine distance.

---

## Index Management

### `(*DB).LoadIndex(collectionName string) error`

Loads in-memory vector index for a collection. Prioritizes Badger snapshot, falls back to rebuild.

### `(*DB).SaveIndex(collectionName string) error`

Persists in-memory index snapshot to Badger.

### `(*DB).RebuildIndex(collectionName string) error`

Rebuilds FTS index. Clears old entries then rebuilds.

---

## Quick API

### `(*Quick).Add(ctx, vector, content) (string, error)`

Adds vector with auto-generated UUID.

### `(*Quick).AddToCollection(ctx, collection, vector, content) (string, error)`

Adds vector to specific collection.

### `(*Quick).Search(ctx, query, topK) ([]types.ScoredEmbedding, error)`

Vector search.

### `(*Quick).AddText(ctx, text, metadata) (string, error)`

Adds text (auto-vectorized).

### `(*Quick).SearchText(ctx, query, topK) ([]types.ScoredEmbedding, error)`

Text search.

### `(*Quick).SearchTextOnly(ctx, query, topK) ([]types.ScoredEmbedding, error)`

Pure FTS search (no Embedder needed).

---

## MemoryFlow Workflow

### `(*memoryflow.Service).IngestTranscript(ctx, req) (*IngestTranscriptResponse, error)`

Stores transcript as episodic memory, grouping user/assistant pairs.

### `(*memoryflow.Service).Recall(ctx, req) (*RecallResponse, error)`

Fused memory + knowledge recall with strategy plugin support.

### `(*memoryflow.Service).WakeUpLayers(ctx, req) (*WakeUpLayersResponse, error)`

Assembles multi-tier wake-up context (L0 Identity → L1 Recent Memories → L2 Knowledge → L3 Full Context).

### `(*memoryflow.Service).CloseSession(ctx, req) (*CloseSessionResponse, error)`

Closes session, stores transcript, optionally promotes knowledge.

### `(*memoryflow.Service).AddDiaryEntry(ctx, req) (*types.MemoryRecord, error)`

Adds diary entry.

### `(*memoryflow.Service).ListDiaryEntries(ctx, req) ([]types.MemoryRecord, error)`

Lists diary entries.

---

## GraphFlow Workflow

### `graphflow.Build(ctx, store, results, opts) error`

Writes extraction results to graph store. Supports deduplication and minimum weight filtering.

### `graphflow.Analyze(ctx, results, req) (*GraphReport, error)`

Computes graph statistics (nodes, edges, degree distribution, density, isolated nodes, type counts).

### `graphflow.RenderReport(report, format string) string`

Renders analysis report as text/markdown/HTML.

---

## Semantic Router

### `(*semanticrouter.Router).Add(route *Route) error`

Adds a route. Caches utterance vectors automatically.

### `(*semanticrouter.Router).Route(ctx, text) (*RouteResult, error)`

Performs semantic routing, returns best matching route with confidence score.

---

## Hindsight Strategy

### `(*hindsight.Strategy).Recall(ctx, req, next) (*memoryflow.RecallResponse, error)`

Enriches recall request with bank ID, entity names, and keyword cues, then delegates to next handler.

---

## Configuration Options

### `gracedb.WithPath(path string) Option`

Sets storage path. Empty string enables in-memory mode.

### `gracedb.WithIndexType(indexType string) Option`

Sets vector index type: `hnsw` (default), `ivf`, `flat`, `lsh`.

### `gracedb.WithIndexTypes(types []string) Option`

Sets multiple index types for hybrid search (e.g., `["hnsw", "lsh"]`). Creates `MultiIndex` internally.

### `gracedb.WithSimilarity(fn string) Option`

Sets similarity function: `cosine` (default), `euclidean`.

### `gracedb.WithEmbedder(e types.Embedder) Option`

Sets the embedder for text-to-vector operations.

---

## Data Types

### `types.Embedding`

| Field | Type | Description |
|-------|------|-------------|
| `ID` | `string` | Unique identifier |
| `CollectionID` | `string` | Collection UUID |
| `Collection` | `string` | Collection name |
| `Vector` | `[]float32` | Vector data |
| `Content` | `string` | Raw text |
| `DocID` | `string` | Document ID |
| `Metadata` | `map[string]string` | Metadata |
| `ACL` | `[]string` | Access control list |
| `CreatedAt` | `time.Time` | Creation time |

### `types.ScoredEmbedding`

Embedding + `Score float32`.

### `types.MemoryRecord`

Memory record with scope/namespace/TTL support, semantic vector, importance, and
updated_at.

### `types.MemorySearchHit`

Memory search result with `Score`/`FinalScore` and ranking explanation fields:
`SemanticScore`, `LexicalScore`, `ImportanceScore`, and `RecencyScore`.

### `types.KnowledgeRecord`

Knowledge record with versioning and auto-chunking.

### `types.Collection`

Collection (namespace) info.

### `types.Session` / `types.Message`

Session and message models.

### `types.Document`

Generic document model.

### `graph.GraphNode` / `graph.GraphEdge`

Property graph node and edge.

### `rdf.Term` / `rdf.Triple`

RDF term and triple.

---

## Error Types

| Error | Description |
|-------|-------------|
| `types.ErrNotFound` | Record not found |
| `types.ErrCollectionExists` | Collection already exists |
| `types.ErrDimensionMismatch` | Vector dimension mismatch |
| `types.ErrInvalidVector` | Invalid vector (empty or nil) |
| `types.ErrEmbedderNotConfigured` | Embedder not configured |
| `types.ErrEmptyText` | Empty text |

---

## Storage Key Format

| Key Pattern | Purpose |
|-------------|---------|
| `coll:<name>` | Collection metadata |
| `emb:<cid>:<eid>` | Embedding metadata |
| `emb:vec:<cid>:<eid>` | Vector data |
| `emb:cont:<cid>:<eid>` | Content text |
| `fts:<token>:<cid>:<eid>` | FTS inverted index (value = TF count) |
| `graph:node:<id>` | Graph node |
| `graph:edge:<id>` | Graph edge |
| `graph:edge:from:<fromID>:<type>:<edgeID>` | Outgoing edge index |
| `graph:edge:to:<toID>:<type>:<edgeID>` | Incoming edge index |
| `rdf:t:<id>` | RDF triple |
| `rdf:s:<subject>:<id>` | Subject index |
| `rdf:p:<predicate>:<id>` | Predicate index |
| `rdf:o:<object>:<id>` | Object index |
| `idx:snapshot:<cid>` | Vector index snapshot |
| `sess:<id>` | Session |
| `msg:<sessionID>:<msgID>` | Message |
| `doc:<id>` | Document |
| `know:<collection>:<id>` | Knowledge item |
| `mem:<bucketID>:<id>` | Memory metadata |
| `mem:content:<bucketID>:<id>` | Memory content |
| `mem:vec:<bucketID>:<id>` | Memory semantic vector |
| `mem:fts:<token>:<bucketID>:<id>` | Memory lexical inverted index |
| `mem:idx:<id>` | Memory ID to bucketID index |
| `geo:<collectionID>:<docID>` | Geographic coordinate |
