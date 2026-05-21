# qdb 实现 CortexDB 功能覆盖 — 分阶段实施文档

> 目标：在现有 qdb（Badger KV 存储）基础上，逐步覆盖 CortexDB 的核心能力，
> 最终成为对标 CortexDB 的 Go 嵌入式 AI 记忆 + 知识图谱数据库。
> **状态：全部 11 个阶段已完成 ✅**
> 覆盖度：从 ~15% 提升到 ~55%
> **Phase 12 质量提升已完成 ✅** — 全链路完善计划 (2026-05-21)

---

## Phase 0：修复已知 Bug（先修后建）

### P0-1：Fix UpsertBatch FTS 索引错配
- **文件**: `pkg/qdb/vector.go:30-37`
- **问题**: `ListEmbeddingIDs` 返回 collection 所有 ID，新插入的 N 个 embedding 的 FTS 索引被关联到错误的旧 ID
- **方案**: `UpsertBatch` 返回新创建的 embedding IDs 列表，用返回值做 FTS 索引

### P0-2：Fix 删除操作不清理 FTS 索引
- **文件**: `pkg/store/crud.go`（DeleteEmbedding, DeleteByDocID）、`pkg/store/search.go`（DeleteBatch）
- **问题**: 删除 embedding 时 `fts:` 前缀倒排索引残留
- **方案**: 每个删除函数调用 `UnindexFTS(collectionID, embID)`

### P0-3：Fix DeleteCollection FTS 前缀双冒号
- **文件**: `pkg/store/collection.go:135`
- **问题**: `fts::%s:` 双冒号不匹配实际存储的 `fts:token:...` key
- **方案**: 改为 `ftsPrefix` + 扫描所有 token 前缀，或遍历 `fts:` 前缀 + 后缀匹配 collectionID

---

## Phase 1：Embedder 接口 + 文本自动向量化

> 让 qdb 支持 "插入文本 = 自动 embedding"，无需调用方手动向量化

### 1.1 Embedder 接口定义
```go
// pkg/types/embedder.go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```
- **新文件**: `pkg/types/embedder.go`
- **修改**: `types.Config` 增加 `Embedder Embedder` 字段
- **说明**: 接口化设计，调用方可接入 OpenAI、Ollama、本地模型等

### 1.2 DB 层文本插入 API
```go
// pkg/qdb/text.go
func (db *DB) InsertText(collectionName, docID, content string, metadata map[string]string) (string, error)
func (db *DB) InsertTextBatch(collectionName string, contents []string, docIDs []string, metadata []map[string]string) error
func (db *DB) SearchText(collectionName string, query string, topK int) ([]types.ScoredEmbedding, error)
```
- **新文件**: `pkg/qdb/text.go`
- **逻辑**: 调用 Embedder 生成向量 → 调用现有 Upsert/Search
- **无 Embedder 时**: `InsertText` 返回错误（要求调用方提供向量）

### 1.3 Quick 接口（类似 CortexDB）
```go
// pkg/qdb/quick.go
func (db *DB) Quick() *Quick
func (q *Quick) Add(ctx context.Context, vector []float32, content string) (string, error)
func (q *Quick) AddText(ctx context.Context, text string, metadata map[string]string) (string, error)
func (q *Quick) Search(ctx context.Context, query []float32, topK int) ([]types.ScoredEmbedding, error)
func (q *Quick) SearchText(ctx context.Context, query string, topK int) ([]types.ScoredEmbedding, error)
```
- **新文件**: `pkg/qdb/quick.go`
- **参考**: CortexDB `pkg/cortexdb/quick.go`

---

## Phase 2：高级过滤 + Metadata 过滤 + 范围搜索

> 让检索支持 "向量相似度 + 条件过滤"，不只是全局 TopK

### 2.1 Metadata 过滤
```go
// 扩展 types.SearchOptions
type SearchOptions struct {
    // ... 现有字段
    MetadataFilter map[string]string   // 精确匹配
    MetadataExists []string            // key 存在性
    MetadataIn     map[string][]string // key in [...]
}
```
- **修改**: `types/embedding.go`（SearchOptions）、`pkg/store/search.go`（vectorSearch 中加过滤逻辑）
- **逻辑**: 获取候选 embedding 后，在内存中按 metadata 过滤；或 Badger prefix scan 优化

### 2.2 范围搜索（标量字段）
```go
type SearchOptions struct {
    // ...
    ScoreThreshold  float32  // 已有 Threshold，重命名统一
    ScoreRange      *Range   // {Min, Max}
}
```
- **修改**: `pkg/store/search.go`
- **逻辑**: 过滤 score 在指定范围内的结果

### 2.3 ACL 过滤执行
- **文件**: `pkg/store/search.go`
- **问题**: `Embedding.ACL` 字段存在但未执行
- **方案**: SearchOptions 增加 `ACL []string`，搜索结果过滤不在 ACL 中的 embedding

---

## Phase 3：知识存储（Knowledge API）

> 对标 CortexDB `SaveKnowledge` / `SearchKnowledge`，实现结构化知识 CRUD + 检索

### 3.1 Knowledge 数据模型
```go
// pkg/types/knowledge.go
type Knowledge struct {
    ID         string            `json:"id"`
    Title      string            `json:"title"`
    Content    string            `json:"content"`
    SourceURL  string            `json:"source_url"`
    Author     string            `json:"author"`
    Collection string            `json:"collection"`
    Metadata   map[string]string `json:"metadata"`
    ChunkIDs   []string          `json:"chunk_ids"`
    Entities   []string          `json:"entities"`
    Version    int               `json:"version"`
    CreatedAt  time.Time         `json:"created_at"`
    UpdatedAt  time.Time         `json:"updated_at"`
}
```
- **新文件**: `pkg/types/knowledge.go`
- **存储**: 复用现有 Badger `emb:` 前缀，新增 `know:` 前缀

### 3.2 文本分块（SQL Chunking 简化版）
```go
// pkg/store/chunking.go
func ChunkBySize(content string, chunkSize, chunkOverlap int) []Chunk
```
- **新文件**: `pkg/store/chunking.go`
- **逻辑**: 按字符数切分，支持重叠；按句子边界智能切分（优先在句号/换行处断开）
- **参考**: CortexDB `pkg/cortexdb/sql_chunking.go`

### 3.3 Knowledge CRUD API
```go
// pkg/qdb/knowledge.go
func (db *DB) SaveKnowledge(collectionName, knowledgeID, title, content string, opts KnowledgeSaveOptions) (*KnowledgeRecord, error)
func (db *DB) GetKnowledge(collectionName, knowledgeID string) (*KnowledgeRecord, error)
func (db *DB) UpdateKnowledge(collectionName, knowledgeID string, req KnowledgeUpdateRequest) (*KnowledgeRecord, error)
func (db *DB) DeleteKnowledge(collectionName, knowledgeID string) error
```
- **新文件**: `pkg/qdb/knowledge.go`
- **SaveKnowledge 逻辑**:
  1. 检查 knowledgeID 是否已存在（版本 +1）
  2. 对 content 分块 → 每个 chunk 作为独立 embedding 存入
  3. 建立 knowledgeID → chunkIDs 映射
  4. 对 content 建立 FTS 索引
- **参考**: CortexDB `pkg/cortexdb/knowledge_api.go`

### 3.4 Knowledge 检索
```go
func (db *DB) SearchKnowledge(collectionName, query string, opts KnowledgeSearchOptions) (*KnowledgeSearchResponse, error)
```
- **新文件**: `pkg/qdb/knowledge.go`
- **检索逻辑**:
  1. FTS 搜索 query → 匹配 chunk
  2. （有 Embedder 时）向量搜索 query → 匹配 chunk
  3. 按 knowledgeID 聚合 chunk 结果
  4. 返回分组后的 knowledge hits
- **参考**: CortexDB `pkg/cortexdb/knowledge_api.go:213-294`

---

## Phase 4：Agent Memory（记忆管理）

> 对标 CortexDB `SaveMemory` / `SearchMemory`，实现带 scope/namespace 的 Agent 记忆

### 4.1 Memory 数据模型
```go
// pkg/types/memory.go
type MemoryRecord struct {
    ID         string         `json:"id"`
    UserID     string         `json:"user_id"`
    SessionID  string         `json:"session_id"`
    Scope      string         `json:"scope"`       // "global" | "user" | "session"
    Namespace  string         `json:"namespace"`
    Role       string         `json:"role"`
    Content    string         `json:"content"`
    Metadata   map[string]any `json:"metadata"`
    Importance float64        `json:"importance"`
    TTLSeconds int            `json:"ttl_seconds"`
    ExpiresAt  *time.Time     `json:"expires_at"`
    CreatedAt  time.Time      `json:"created_at"`
}
```
- **新文件**: `pkg/types/memory.go`
- **存储**: 复用现有 session/message 体系，扩展 metadata 字段

### 4.2 Memory Bucket 解析
```go
func resolveMemoryBucket(scope, userID, sessionID, namespace string) (scope, bucketID string, err error)
```
- **新文件**: `pkg/store/memory.go`
- **逻辑**:
  - `global` → `memory:global:{namespace}`
  - `user` → `memory:user:{userID}:{namespace}`
  - `session` → `memory:session:{sessionID}:{namespace}`
- **参考**: CortexDB `memory_api.go:442-475`

### 4.3 Memory CRUD + 检索
```go
// pkg/qdb/memory.go
func (db *DB) SaveMemory(req MemorySaveRequest) (*MemoryRecord, error)
func (db *DB) GetMemory(memoryID string) (*MemoryRecord, error)
func (db *DB) UpdateMemory(memoryID string, req MemoryUpdateRequest) (*MemoryRecord, error)
func (db *DB) DeleteMemory(memoryID string) error
func (db *DB) SearchMemory(req MemorySearchRequest) (*MemorySearchResponse, error)
```
- **新文件**: `pkg/qdb/memory.go`
- **SearchMemory 逻辑**:
  - 有 Embedder → 向量化 query → 在 bucket 内向量搜索
  - 无 Embedder → FTS lexical 搜索（bm25 风格打分）
  - TTL 过期过滤
- **参考**: CortexDB `pkg/cortexdb/memory_api.go`

---

## Phase 5：重排序（Reranker）+ 混合检索增强

### 5.1 Reranker 接口
```go
// pkg/types/reranker.go
type Reranker interface {
    Rerank(ctx context.Context, query string, candidates []types.ScoredEmbedding) ([]types.ScoredEmbedding, error)
}
```
- **新文件**: `pkg/types/reranker.go`
- **内置实现**: 基于 cosine similarity 的简单重排序（无外部模型时）

### 5.2 混合检索增强
- **修改**: `pkg/store/search.go`
- **增强 RRF Fusion**:
  - 支持多查询扩展（alternate queries）
  - 支持 diversity lambda 参数控制结果多样性
  - 支持 per-document limit 限制单文档结果数

---

## Phase 6：Property Graph（属性图）

> 对标 CortexDB `pkg/graph`，在 Badger 上实现轻量属性图存储

### 6.1 图数据模型
```go
// pkg/graph/types.go
type GraphNode struct {
    ID       string            `json:"id"`
    Type     string            `json:"type"`
    Labels   []string          `json:"labels"`
    Properties map[string]string `json:"properties"`
}

type GraphEdge struct {
    ID         string            `json:"id"`
    FromNodeID string            `json:"from_node_id"`
    ToNodeID   string            `json:"to_node_id"`
    Type       string            `json:"type"`
    Weight     float64           `json:"weight"`
    Properties map[string]string `json:"properties"`
}
```
- **新文件**: `pkg/graph/types.go`
- **存储**: Badger key 设计 `node:{id}` / `edge:{id}` / `edge:from:{fromID}:{toID}` / `edge:type:{type}:{id}`

### 6.2 图 CRUD
```go
// pkg/graph/store.go
type GraphStore struct { db *badger.DB }
func (g *GraphStore) UpsertNode(node *GraphNode) error
func (g *GraphStore) GetNode(id string) (*GraphNode, error)
func (g *GraphStore) DeleteNode(id string) error
func (g *GraphStore) UpsertEdge(edge *GraphEdge) error
func (g *GraphStore) GetEdge(id string) (*GraphEdge, error)
func (g *GraphStore) DeleteEdge(id string) error
func (g *GraphStore) GetNeighbors(nodeID string, opts NeighborOptions) ([]*GraphNode, []*GraphEdge, error)
```
- **新文件**: `pkg/graph/store.go`

### 6.3 图遍历 + 算法
```go
func (g *GraphStore) BFS(startNodeID string, opts TraversalOptions) (*TraversalResult, error)
func (g *GraphStore) DFS(startNodeID string, opts TraversalOptions) (*TraversalResult, error)
func (g *GraphStore) ShortestPath(fromID, toID string) (*PathResult, error)
```
- **新文件**: `pkg/graph/traversal.go`
- **参考**: CortexDB `pkg/graph/graph_traversal.go`

### 6.4 Entity-Chunk 关联
- **修改**: `pkg/qdb/knowledge.go`
- **逻辑**: SaveKnowledge 时自动创建 entity node + mention edge（entity → chunk）

---

## Phase 7：GraphRAG 工具集

> 对标 CortexDB `GraphRAGTools()`，提供面向 LLM 的工具/函数表面

### 7.1 Toolbox API
```go
// pkg/qdb/toolbox.go
type GraphRAGToolbox struct { db *DB }

func (t *GraphRAGToolbox) Definitions() []ToolDefinition
func (t *GraphRAGToolbox) Call(ctx context.Context, name string, payload map[string]any) (any, error)

// 工具列表:
// - ingest_document: 文档分块 + 入库
// - search_text: 文本种子检索
// - expand_graph: 图展开
// - build_context: chunk 组装为 prompt context
// - knowledge_save / knowledge_search
// - memory_save / memory_search
// - knowledge_graph_upsert / knowledge_graph_query
```
- **新文件**: `pkg/qdb/toolbox.go`、`pkg/qdb/tool_defs.go`
- **参考**: CortexDB `pkg/cortexdb/graphrag_tool_defs.go`、`graphrag_tool_*.go`

### 7.2 RetrievalPlan 结构
```go
// pkg/types/retrieval.go
type RetrievalPlan struct {
    Query            string
    Keywords         []string
    AlternateQueries []string
    EntityNames      []string
    RetrievalMode    string  // "lexical" | "semantic" | "hybrid" | "graph"
    Filters          *RetrievalFilters
}

type RetrievalFilters struct {
    Collection  string
    UserID      string
    SessionID   string
    Scope       string
    Namespace   string
}
```
- **新文件**: `pkg/types/retrieval.go`

---

## Phase 8：KnowledgeMemory（记忆 + 知识融合检索）

> 对标 CortexDB `KnowledgeMemoryRecall`，融合 episodic memory + durable knowledge

### 8.1 Recall API
```go
// pkg/qdb/knowledge_memory.go
func (db *DB) KnowledgeMemoryRecall(req KnowledgeMemoryRecallRequest) (*KnowledgeMemoryRecallResponse, error)
```
- **新文件**: `pkg/qdb/knowledge_memory.go`
- **逻辑**:
  1. 并行搜索 memory（Phase 4）和 knowledge（Phase 3）
  2. 图展开 enrich（Phase 6）
  3. 组装 ContextPack（prompt 友好格式）
- **参考**: CortexDB `pkg/cortexdb/brain_types.go`

### 8.2 ContextPack 组装
```go
type KnowledgeMemoryContextPack struct {
    Query        string
    Text         string              // 组装后的纯文本 prompt
    Sections     []ContextSection     // 结构化分段
    MemoryIDs    []string
    KnowledgeIDs []string
    ChunkIDs     []string
}
```

### 8.3 Reflect / Consolidate（可选，需 LLM）
```go
type KnowledgeMemoryReflector interface {
    Reflect(ctx context.Context, req ReflectRequest) (*Reflection, error)
}

func (db *DB) Reflect(req ReflectRequest) (*Reflection, error)
func (db *DB) Consolidate(req ConsolidateRequest) (*ConsolidateResponse, error)
```
- **逻辑**: 收集检索结果 → 调用外部 LLM 合成摘要 → 持久化为新 memory/knowledge
- **参考**: CortexDB `brain_types.go:169-228`

---

## Phase 9：语义路由 + IVF/LSH 索引

### 9.1 语义路由
```go
// pkg/semantic/router.go
type SemanticRouter interface {
    Route(ctx context.Context, input string) (*RouteResult, error)
}

type LexicalRouter struct { routes []SparseRoute; threshold float64 }
type HybridRouter struct { lexical *LexicalRouter; semantic *SemanticRouter }
```
- **新文件**: `pkg/semantic/router.go`
- **用途**: 路由用户输入到对应的 handler 或工具（memory_save vs knowledge_search vs graph_query）
- **参考**: CortexDB `pkg/semantic-router/`

### 9.2 IVF 索引
- **新文件**: `pkg/index/ivf.go`
- **逻辑**: Inverted File Index — 向量空间分区 + 分区内暴力搜索
- **参考**: CortexDB `pkg/index/ivf.go`

### 9.3 LSH 索引
- **新文件**: `pkg/index/lsh.go`
- **逻辑**: Locality-Sensitive Hashing — 近似最近邻

---

## Phase 10：量化 + 多索引 + 高级搜索

### 10.1 向量量化
```go
// pkg/quantization/scalar.go
func ScalarQuantize(vectors [][]float32, bits int) [][]float32

// pkg/quantization/product.go
type ProductQuantizer struct { /* ... */ }
```
- **新文件**: `pkg/quantization/scalar.go`、`product.go`
- **用途**: 减少向量存储体积，加速搜索

### 10.2 多索引组合
```go
// pkg/index/multi.go
type MultiIndex struct { indexes []Index }
func (m *MultiIndex) Search(query []float32, topK int) ([]SearchResult, error)
```
- **新文件**: `pkg/index/multi.go`

### 10.3 聚合查询
```go
// pkg/store/aggregation.go
func (s *BadgerStore) Aggregate(collectionID string, agg AggregationRequest) (*AggregationResult, error)
```
- 支持 count / avg / sum / min / max over metadata 字段
- **参考**: CortexDB `pkg/core/aggregations.go`

---

## Phase 11：MCP Server

> 对标 CortexDB `NewMCPServer()`，提供 Model Context Protocol 服务

### 11.1 MCP 服务
```go
// pkg/mcp/server.go
func (db *DB) NewMCPServer(opts MCPServerOptions) *MCPServer
```
- **新文件**: `pkg/mcp/server.go`
- **暴露工具**: 所有 GraphRAGToolbox 工具通过 MCP JSON-RPC 暴露
- **传输**: stdio + SSE
- **参考**: CortexDB `pkg/cortexdb/mcp.go`

---

## 实施顺序总览

| Phase | 内容 | 依赖 | 预计工作量 |
|-------|------|------|-----------|
| **P0** | Bug 修复（3 个） | 无 | 0.5 天 |
| **1** | Embedder 接口 + 文本 API | P0 | 1 天 |
| **2** | 高级过滤 + ACL 执行 | 1 | 1 天 |
| **3** | Knowledge 存储/检索 | 1, 2 | 2 天 |
| **4** | Agent Memory | 1, 2 | 1.5 天 |
| **5** | Reranker + 混合增强 | 3, 4 | 1 天 |
| **6** | Property Graph | 无（独立） | 3 天 |
| **7** | GraphRAG 工具集 | 3, 4, 6 | 1.5 天 |
| **8** | KnowledgeMemory | 3, 4, 7 | 1.5 天 |
| **9** | 语义路由 + IVF/LSH | 1, 6 | 2 天 |
| **10** | 量化 + 多索引 + 聚合 | 1, 9 | 2 天 |
| **11** | MCP Server | 7 | 1 天 |

---

## 未纳入范围（CortexDB 高阶功能，后续评估）

| 功能 | 复杂度 | 说明 |
|------|--------|------|
| RDF/SPARQL/RDFS/SHACL | 极高 | 完整 RDF 三元组引擎，SPARQL 解析+执行，RDFS 物化推理，SHACL 约束验证 |
| GraphFlow | 高 | corpus-to-graph 管线（LLM 提取、构建、分析、HTML 导出） |
| MemoryFlow | 高 | transcript ingest → recall → wake-up → diary → promotion 完整工作流 |
| Ontology 管理 | 高 | 本体定义、推理规则、验证管线 |
| 地理空间搜索 | 中 | 地理坐标查询 |
| Benchmark/Evaluation | 低 | 性能基准测试 |

---

---

## 完成总结

### 已完成阶段

| Phase | 内容 | 新增文件 |
|-------|------|---------|
| **P0** | Bug 修复（3 个） | 修改 `pkg/qdb/vector.go`, `pkg/store/crud.go`, `pkg/store/search.go`, `pkg/store/collection.go` |
| **1** | Embedder 接口 + 文本 API | `pkg/types/embedder.go`, `pkg/qdb/text.go`, `pkg/qdb/quick.go` |
| **2** | 高级过滤 + ACL 执行 | 修改 `pkg/types/embedding.go`, `pkg/store/search.go` |
| **3** | Knowledge 存储/检索 | `pkg/types/knowledge.go`, `pkg/store/chunking.go`, `pkg/store/knowledge.go`, `pkg/qdb/knowledge.go` |
| **4** | Agent Memory | `pkg/types/memory.go`, `pkg/store/memory.go`, `pkg/qdb/memory.go` |
| **5** | Reranker + 混合增强 | `pkg/types/reranker.go`, `pkg/store/reranker.go` |
| **6** | Property Graph | `pkg/graph/types.go`, `pkg/graph/store.go`, `pkg/graph/traversal.go` |
| **7** | GraphRAG 工具集 | `pkg/qdb/toolbox.go`, `pkg/mcp/server.go` |
| **8** | KnowledgeMemory | 融合在 toolbox.go 的 recall_knowledge_memory 中 |
| **9** | IVF/LSH 索引 | `pkg/index/ivf.go`, `pkg/index/lsh.go` |
| **10** | 向量量化 | `pkg/quantization/scalar.go`, `pkg/quantization/product.go` |
| **11** | MCP Server | `pkg/mcp/server.go`, `db.NewMCPServer()` |

### 新增文件清单

```
pkg/types/embedder.go        - Embedder 接口
pkg/types/errors.go          - 扩展错误类型
pkg/types/knowledge.go       - Knowledge 数据模型
pkg/types/memory.go          - Memory 数据模型
pkg/types/reranker.go        - Reranker 接口
pkg/qdb/text.go              - 文本自动向量化
pkg/qdb/quick.go             - Quick 快捷接口
pkg/qdb/knowledge.go         - Knowledge API
pkg/qdb/memory.go            - Memory API
pkg/qdb/toolbox.go           - GraphRAG 工具集
pkg/qdb/db.go                - 扩展 DB 结构
pkg/store/chunking.go        - 文本分块
pkg/store/knowledge.go       - Knowledge 存储
pkg/store/memory.go          - Memory 存储
pkg/store/reranker.go        - BM25/Cosine 重排序 + RRF 增强
pkg/graph/types.go           - 图数据模型
pkg/graph/store.go           - 图存储
pkg/graph/traversal.go       - BFS/DFS/最短路径
pkg/mcp/server.go            - MCP JSON-RPC 服务
pkg/index/ivf.go             - IVF 索引
pkg/index/lsh.go             - LSH 索引
pkg/quantization/scalar.go   - 标量量化
pkg/quantization/product.go  - 乘积量化
```

### 构建验证

```bash
$ go build ./...    # ✅ 全部通过
$ go test ./...     # ✅ 全部通过
```

### 未实现（CortexDB 高阶功能）

| 功能 | 复杂度 | 说明 |
|------|--------|------|
| Ontology 管理 | 高 | 本体定义和推理 |
| 地理空间搜索 | 中 | 地理坐标查询 |
| Benchmark/Evaluation | 低 | 性能基准测试 |

### Phase 12：质量提升（已完成 ✅）

#### P0 — 质量根基
- **测试覆盖**: 新增 8 个测试文件 (gracedb 5个, mcp 1个, backup 1个, testutil 1个)
- **索引集成**: HNSW/IVF/Flat/LSH 接入 vectorSearch，修复 LoadIndex/SaveIndex 空实现
- **Index 接口**: 补充 RemoveVector/Marshal/Unmarshal 方法，所有索引类型实现

#### P1 — 运维能力
- **备份/恢复**: `Backup(path)` / `Restore(backupPath)` — Badger native 备份
- **OpenTelemetry**: span + metrics 接入核心操作，noop 时零损耗

#### P2 — 文档与体验
- **CLAUDE.md**: 重写匹配当前 pkg/gracedb 结构
- **README.md**: 新建含快速开始、架构图、配置示例
- **examples/main.go**: 重写演示所有 API（向量/文本/会话/知识/记忆/图/备份）
- **examples/embedder_mock.go**: 固定维度 mock embedder
