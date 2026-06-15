# gracedb API 参考

完整 API 参考，覆盖所有公开接口、类型和配置选项。

## 目录

- [核心入口](#核心入口)
- [集合管理](#集合管理)
- [向量操作](#向量操作)
- [文本操作](#文本操作)
- [检索](#检索)
- [知识管理](#知识管理)
- [记忆管理](#记忆管理)
- [会话管理](#会话管理)
- [文档管理](#文档管理)
- [属性图](#属性图)
- [RDF/SPARQL](#rdfsparql)
- [GraphRAG 工具箱](#graphrag-工具箱)
- [MCP 服务](#mcp-服务)
- [备份与恢复](#备份与恢复)
- [聚合查询](#聚合查询)
- [地理搜索](#地理搜索)
- [索引管理](#索引管理)
- [Quick 接口](#quick-接口)
- [MemoryFlow 工作流](#memoryflow-工作流)
- [GraphFlow 工作流](#graphflow-工作流)
- [语义路由](#语义路由)
- [Hindsight 策略](#hindsight-策略)
- [配置选项](#配置选项)
- [数据类型](#数据类型)
- [错误类型](#错误类型)

---

## 核心入口

### `gracedb.Open(path string, opts ...Option) (*DB, error)`

打开或创建 gracedb 数据库实例。`path` 为空时使用内存模式。

```go
db, err := gracedb.Open("/tmp/data",
    gracedb.WithEmbedder(myEmbedder),
    gracedb.WithIndexType("hnsw"),
    gracedb.WithSimilarity("cosine"),
)
```

### `(*DB).Close() error`

关闭数据库，确保数据刷盘。

### `(*DB).Vector() *store.BadgerStore`

返回底层 BadgerStore 实例。

### `(*DB).Graph() *graph.GraphStore`

返回属性图存储实例。

### `(*DB).RDF() *rdf.Store`

返回 RDF 三元组存储实例。

### `(*DB).HasEmbedder() bool`

检查是否配置了 Embedder。

### `(*DB).Quick() *Quick`

返回 Quick 快捷操作接口。

### `(*DB).GraphRAGTools() *GraphRAGToolbox`

返回 GraphRAG 工具箱，供 LLM 编排调用。

### `(*DB).NewMCPServer(name, version string) *mcp.Server`

创建 MCP 服务器，暴露所有 GraphRAG 工具。

---

## 集合管理

集合是向量和嵌入的逻辑命名空间，类似于数据库中的表。

### `(*DB).CreateCollection(name string) (*types.Collection, error)`

创建新集合。

### `(*DB).GetCollection(name string) (*types.Collection, error)`

按名称获取集合。

### `(*DB).ListCollections() ([]*types.Collection, error)`

列出所有集合。

### `(*DB).DeleteCollection(name string) error`

删除集合及其所有数据（向量、FTS 索引等）。

---

## 向量操作

### `(*DB).Upsert(collectionName, docID string, vector []float32, content string, metadata map[string]string, acl []string) (string, error)`

插入或更新单个向量。返回 embedding ID。自动建立 FTS 索引（content 非空时）。

### `(*DB).UpsertBatch(collectionName string, vectors [][]float32, contents []string, docIDs []string, metadata []map[string]string) error`

批量插入或更新向量。

### `(*DB).GetEmbedding(collectionName, embID string, includeVector bool) (*types.Embedding, error)`

按 ID 获取嵌入。`includeVector` 控制是否返回向量数据。

### `(*DB).DeleteEmbedding(collectionName, embID string) error`

删除单个嵌入及其 FTS 索引。

### `(*DB).DeleteByDocID(collectionName, docID string) error`

删除指定文档的所有嵌入。

### `(*DB).DeleteEmbeddingBatch(collectionName string, ids []string) error`

批量删除嵌入。

### `(*DB).EmbeddingCount(collectionName string) (int, error)`

返回集合中的嵌入数量。

### `(*DB).ListEmbeddingIDs(collectionName string) ([]string, error)`

列出集合中所有嵌入 ID。

---

## 文本操作

文本操作需要配置 `Embedder`，自动将文本转换为向量。

### `(*DB).InsertText(collectionName, docID, content string, metadata map[string]string) (string, error)`

插入文本，自动调用 Embedder 生成向量并存储。

### `(*DB).InsertTextBatch(collectionName string, contents []string, docIDs []string, metadata []map[string]string) ([]string, error)`

批量插入文本。过滤空内容后调用 EmbedBatch。

### `(*DB).SearchText(collectionName string, query string, topK int) ([]types.ScoredEmbedding, error)`

文本检索。有 Embedder 时向量化后搜索；无 Embedder 时自动回退到 FTS。

---

## 检索

### `(*DB).Search(collectionName string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error)`

混合检索，支持向量相似度搜索、全文检索或两者融合的 RRF 混合搜索。

`SearchOptions` 字段：

| 字段 | 类型 | 说明 |
|------|------|------|
| `TopK` | `int` | 返回结果数量 |
| `Threshold` | `float32` | 最低相似度阈值 |
| `Filter` | `map[string]string` | 预留过滤字段 |
| `Collection` | `string` | FTS 搜索目标集合名 |
| `UseVectorSearch` | `bool` | 启用向量搜索 |
| `UseTextSearch` | `bool` | 启用全文搜索 |
| `Metadata` | `[]string` | 预留元数据字段 |
| `MetadataFilter` | `map[string]string` | 精确匹配元数据键值 |
| `ACL` | `[]string` | 访问控制列表过滤 |
| `MetadataExists` | `[]string` | 要求指定元数据键存在 |
| `DocID` | `string` | 按文档 ID 过滤 |

### `(*DB).SearchFTS(collectionName string, query string) ([]string, error)`

纯全文检索，返回 embedding ID 列表。

### `(*DB).SearchFTSWithContent(collectionName string, query string, topK int) ([]types.ScoredEmbedding, error)`

全文检索，返回带内容的 ScoredEmbedding。

---

## 知识管理

知识存储提供结构化的文档管理，支持自动分块和 FTS 索引。

### `(*DB).SaveKnowledge(collectionName, knowledgeID, title, content string, opts types.KnowledgeSaveRequest) (*types.KnowledgeRecord, error)`

保存或替换知识项。自动分块并建立 FTS 索引。

### `(*DB).GetKnowledge(collectionName, knowledgeID string) (*types.KnowledgeRecord, error)`

获取知识项。

### `(*DB).UpdateKnowledge(collectionName, knowledgeID string, req types.KnowledgeUpdateRequest) (*types.KnowledgeRecord, error)`

更新知识项字段。

### `(*DB).DeleteKnowledge(collectionName, knowledgeID string) error`

删除知识项及其所有分块。

### `(*DB).SearchKnowledge(collectionName, query string, topK int) (*types.KnowledgeSearchResponse, error)`

搜索知识项，按文档聚合分块结果。

---

## 记忆管理

记忆管理提供 scope/namespace/TTL 隔离的 Agent 记忆。配置 `WithEmbedder` 后，
`SaveMemory` 会自动写入语义向量，`SearchMemory` 会执行语义向量检索 + 词法
倒排索引检索，并按 hybrid score 返回结果；未配置 Embedder 时仍可使用词法检索。

Scope 解析规则：

| Scope | 必填字段 | Bucket |
|-------|----------|--------|
| `global` | 无 | `memory:global:<namespace>` |
| `user` | `UserID` | `memory:user:<userID>:<namespace>` |
| `session` | `SessionID` | `memory:session:<sessionID>:<namespace>` |

`Namespace` 为空时使用 `default`。搜索只在解析后的 bucket 内执行，不会跨 user/session/namespace 串结果。

### `(*DB).SaveMemory(req types.MemorySaveRequest) (*types.MemoryRecord, error)`

存储记忆，支持 scope（global/user/session）、namespace、importance 和 TTL。
有 Embedder 时会调用 `Embed(content)` 并持久化到 `mem:vec:<bucketID>:<memoryID>`，
同时写入 `mem:fts:<term>:<bucketID>:<memoryID>` 词法索引。

### `(*DB).GetMemory(memoryID string) (*types.MemoryRecord, error)`

获取记忆。

### `(*DB).UpdateMemory(req types.MemoryUpdateRequest) (*types.MemoryRecord, error)`

更新记忆。内容变更时会重建语义向量和 `mem:fts` 词法索引，并删除旧索引项。

### `(*DB).DeleteMemory(memoryID string) error`

删除记忆，同时删除元数据、内容、语义向量、词法索引和 memoryID bucket 索引。

### `(*DB).SearchMemory(req types.MemorySearchRequest) (*types.MemorySearchResponse, error)`

搜索记忆。默认融合：

```text
final = semantic*0.60 + lexical*0.25 + importance*0.10 + recency*0.05
```

返回的每条 `MemorySearchHit` 包含 `Score`/`FinalScore`，以及
`SemanticScore`、`LexicalScore`、`ImportanceScore`、`RecencyScore` 解释字段。
过期记忆会在返回前过滤。

`MemorySearchRequest` 可选权重字段：

| 字段 | 默认值 | 说明 |
|------|--------|------|
| `SemanticWeight` | `0.60` | 语义向量分权重 |
| `LexicalWeight` | `0.25` | 词法检索分权重 |
| `ImportanceWeight` | `0.10` | 记忆重要性权重 |
| `RecencyWeight` | `0.05` | 更新时间新鲜度权重 |

四个权重全为 `0` 时使用默认值；只要设置任意权重，就完全使用调用方提供的组合。

---

## 会话管理

### `(*DB).CreateSession(name string) (*types.Session, error)`

创建会话。

### `(*DB).GetSession(id string) (*types.Session, error)`

获取会话。

### `(*DB).ListSessions() ([]*types.Session, error)`

列出所有会话。

### `(*DB).DeleteSession(id string) error`

删除会话及其所有消息。

### `(*DB).AddMessage(msg *types.Message) error`

添加消息到会话。

### `(*DB).GetSessionHistory(sessionID string, limit int) ([]*types.Message, error)`

获取会话历史消息。

### `(*DB).DeleteMessage(sessionID, messageID string) error`

删除单条消息。

---

## 文档管理

### `(*DB).CreateDocument(doc *types.Document) error`

创建文档。

### `(*DB).GetDocument(id string) (*types.Document, error)`

获取文档。

### `(*DB).ListDocuments() ([]*types.Document, error)`

列出所有文档。

### `(*DB).DeleteDocument(id string) error`

删除文档。

### `(*DB).Stats() (types.StoreStats, error)`

返回数据库统计（集合数、嵌入数、会话数等）。

---

## 属性图

属性图存储通过 `db.Graph()` 访问。

### `(*graph.GraphStore).UpsertNode(node *graph.GraphNode) error`

插入或更新节点。ID 为空时自动生成。

### `(*graph.GraphStore).GetNode(id string) (*graph.GraphNode, error)`

获取节点。

### `(*graph.GraphStore).DeleteNode(id string) error`

删除节点及其关联边。

### `(*graph.GraphStore).UpsertEdge(edge *graph.GraphEdge) error`

插入或更新边。自动建立 from/to/type 索引。

### `(*graph.GraphStore).GetEdge(id string) (*graph.GraphEdge, error)`

获取边。

### `(*graph.GraphStore).DeleteEdge(id string) error`

删除边及其索引。

### `(*graph.GraphStore).GetNeighbors(nodeID string, opts graph.NeighborOptions) ([]*graph.GraphNode, []*graph.GraphEdge, error)`

获取节点邻居。支持方向（in/out/both）、边类型过滤、节点类型过滤和数量限制。

### `(*graph.GraphStore).BFS(startNodeID string, opts graph.NeighborOptions) (*graph.TraversalResult, error)`

广度优先遍历。

### `(*graph.GraphStore).DFS(startNodeID string, opts graph.NeighborOptions) (*graph.TraversalResult, error)`

深度优先遍历。

### `(*graph.GraphStore).ShortestPath(fromID, toID string) (*graph.PathResult, error)`

最短路径（BFS 实现）。

---

## RDF/SPARQL

RDF 存储通过 `db.RDF()` 访问。

### `(*rdf.Store).UpsertTriple(t *rdf.Triple) error`

插入或替换 RDF 三元组。ID 为空时自动根据三元组内容生成哈希。

### `(*rdf.Store).DeleteTriple(id string) error`

删除三元组。

### `(*rdf.Store).Query(pattern rdf.TriplePattern) ([]rdf.Triple, error)`

模式匹配查询。Nil 字段为通配符。

### `(*rdf.Store).SPARQLSelect(query string) ([]map[string]rdf.Term, error)`

简化 SPARQL SELECT 查询。

```go
results, _ := store.SPARQLSelect(`SELECT ?s ?p ?o WHERE { ?s ?p ?o . }`)
```

### `(*rdf.Store).SPARQLAsk(query string) (bool, error)`

简化 SPARQL ASK 查询。

### `(*rdf.Store).RegisterNamespace(prefix, uri string)`

注册前缀映射。

### `(*rdf.Store).ExpandPrefix(prefixed string) string`

展开前缀为完整 IRI。

### `(*rdf.Store).Count() (int, error)`

返回三元组总数。

### `(*rdf.Store).ImportNTriples(data string) (int, error)`

导入 N-Triples 格式数据。

### `(*rdf.Store).ExportNTriples() (string, error)`

导出为 N-Triples 格式。

### `(*rdf.Store).RDFSInfer() (int, error)`

RDFS 推理，生成派生三元组。

### `(*rdf.Store).SHACLValidate() (*rdf.ValidationResult, error)`

SHACL 约束验证。

---

## GraphRAG 工具箱

通过 `db.GraphRAGTools()` 访问，提供 7 个供 LLM 编排调用的工具。

### 工具列表

| 工具名 | 说明 | 必需参数 |
|--------|------|----------|
| `search_knowledge` | 搜索知识项 | `query` |
| `save_knowledge` | 保存知识项 | `knowledge_id`, `content` |
| `search_memory` | 搜索记忆 | `query` |
| `save_memory` | 保存记忆 | `memory_id`, `content`, `scope` |
| `expand_graph` | 图邻域展开 | `node_ids`, `max_depth` |
| `recall_knowledge_memory` | 融合知识+记忆检索 | `query` |
| `build_context` | 组装 prompt context | `query` |

### `(*GraphRAGToolbox).Definitions() []mcp.ToolDef`

返回所有工具定义，用于 MCP 注册。

### `(*GraphRAGToolbox).Call(ctx context.Context, name string, payload map[string]any) (any, error)`

调用指定工具。

---

## MCP 服务

MCP 服务通过标准输入/输出提供 JSON-RPC 接口。

### MCP 方法

| 方法 | 说明 |
|------|------|
| `initialize` | 初始化连接 |
| `tools/list` | 列出可用工具 |
| `tools/call` | 调用工具 |

### 使用示例

```go
server := db.NewMCPServer("gracedb", "1.0.0")
server.RunStdio(context.Background())
```

---

## 备份与恢复

### `(*DB).Backup(path string) error`

创建全量备份。备份前自动刷盘。使用 Badger native backup API。

### `(*DB).Restore(backupPath string) error`

从备份恢复。恢复后自动重建所有集合的向量索引。

---

## 聚合查询

### `(*DB).Aggregate(collectionName string, req types.AggregationRequest) (*types.AggregationResult, error)`

对集合中的元数据字段执行聚合操作（count/avg/sum/min/max）。

---

## 地理搜索

### `(*DB).UpsertGeoPoint(collectionName, docID string, lat, lon float64, metadata map[string]string) error`

存储地理坐标点。

### `(*DB).SearchGeoRadius(collectionName string, lat, lon, radiusKm float64, topK int) ([]types.GeoResult, error)`

按半径搜索附近点。

---

## 索引管理

### `(*DB).LoadIndex(collectionName string) error`

为集合加载内存向量索引。优先从 Badger 快照恢复，失败则从向量重建。

### `(*DB).SaveIndex(collectionName string) error`

将内存索引快照持久化到 Badger。

### `(*DB).RebuildIndex(collectionName string) error`

重建集合的 FTS 索引。先清理旧条目再重建。

---

## Quick 接口

Quick 提供简化的 API，适合快速原型开发。

### `(*Quick).Add(ctx context.Context, vector []float32, content string) (string, error)`

添加向量，自动生成 UUID。

### `(*Quick).AddToCollection(ctx context.Context, collection string, vector []float32, content string) (string, error)`

添加到指定集合。

### `(*Quick).Search(ctx context.Context, query []float32, topK int) ([]types.ScoredEmbedding, error)`

向量搜索。

### `(*Quick).SearchInCollection(ctx context.Context, collection string, query []float32, topK int) ([]types.ScoredEmbedding, error)`

在指定集合中搜索。

### `(*Quick).AddText(ctx context.Context, text string, metadata map[string]string) (string, error)`

添加文本（自动向量化）。

### `(*Quick).AddTextToCollection(ctx context.Context, collection string, text string, metadata map[string]string) (string, error)`

在指定集合中添加文本。

### `(*Quick).SearchText(ctx context.Context, query string, topK int) ([]types.ScoredEmbedding, error)`

文本搜索。

### `(*Quick).SearchTextInCollection(ctx context.Context, collection string, query string, topK int) ([]types.ScoredEmbedding, error)`

在指定集合中文本搜索。

### `(*Quick).SearchTextOnly(ctx context.Context, query string, topK int) ([]types.ScoredEmbedding, error)`

纯 FTS 搜索（无需 Embedder）。

### `(*Quick).SearchTextOnlyInCollection(ctx context.Context, collection string, query string, topK int) ([]types.ScoredEmbedding, error)`

在指定集合中纯 FTS 搜索。

---

## MemoryFlow 工作流

MemoryFlow 提供完整的 Agent 记忆工作流：对话转录、回忆、唤醒层、日记和知识晋升。

```go
import "github.com/dshmyz/gracedb/pkg/memoryflow"

svc := memoryflow.New(db,
    &memoryflow.DefaultQueryPlanner{},
    &memoryflow.DefaultSessionExtractor{},
)
```

### `(*memoryflow.Service).IngestTranscript(ctx context.Context, req IngestTranscriptRequest) (*IngestTranscriptResponse, error)`

存储对话转 Episodic Memory，自动将 user/assistant 配对分组。

### `(*memoryflow.Service).Recall(ctx context.Context, req RecallRequest) (*RecallResponse, error)`

融合记忆和知识检索，支持策略插件。

### `(*memoryflow.Service).WakeUpLayers(ctx context.Context, req WakeUpLayersRequest) (*WakeUpLayersResponse, error)`

组装多级唤醒上下文（L0 身份 → L1 近期记忆 → L2 知识 → L3 全文）。

### `(*memoryflow.Service).CloseSession(ctx context.Context, req CloseSessionRequest) (*CloseSessionResponse, error)`

关闭会话，存储对话并可选地晋升知识。

### `(*memoryflow.Service).AddDiaryEntry(ctx context.Context, req DiaryEntryRequest) (*types.MemoryRecord, error)`

添加日记条目。

### `(*memoryflow.Service).ListDiaryEntries(ctx context.Context, req DiaryListRequest) ([]types.MemoryRecord, error)`

列出日记条目。

---

## GraphFlow 工作流

GraphFlow 提供 corpus-to-graph 工作流：实体提取、图构建、分析和导出。

### `graphflow.Build(ctx context.Context, store GraphStore, results []ExtractionResult, opts BuildOptions) error`

将提取结果添加到图中，支持去重和最小权重过滤。

### `graphflow.Analyze(ctx context.Context, results []ExtractionResult, req AnalyzeRequest) (*GraphReport, error)`

计算图统计（节点数、边数、度分布、密度、孤立节点等）。

### `graphflow.RenderReport(report *GraphReport, format string) string`

将分析报告渲染为文本/markdown/HTML。

### `(*graphflow.HeuristicExtractor).Extract(text, docID string) ExtractionResult`

基于启发式规则的实体关系提取（将大写单词视为实体，相邻实体之间创建边）。

---

## 语义路由

语义路由基于向量相似度将用户查询分类到预定义路由。

```go
router := semanticrouter.NewRouter(embedder, semanticrouter.DefaultConfig())
router.Add(&semanticrouter.Route{
    Name:       "save_memory",
    Utterances: []string{"记住这个", "保存这条信息", "添加到记忆"},
    Handler:    myHandler,
})
result, _ := router.Route(ctx, "记住这个用户喜欢咖啡")
```

### `(*semanticrouter.Router).Add(route *Route) error`

添加路由。自动缓存 utterance 的向量。

### `(*semanticrouter.Router).Route(ctx context.Context, text string) (*RouteResult, error)`

执行语义路由，返回最佳匹配路由和置信度分数。

### 词法路由

`LexicalRouter` 基于关键词匹配（无需 Embedder），适合无向量模型的轻量场景。

### 混合路由

`HybridRouter` 组合词法路由和语义路由，优先尝试词法匹配，回退到语义匹配。

---

## Hindsight 策略

Hindsight 是 memoryflow 回忆策略插件，通过 bank ID、实体名称和关键词线索丰富回忆请求。

```go
strategy := hindsight.NewStrategy(hindsight.StrategyOptions{
    BankID:      "my-bank",
    EntityNames: []string{"user-preferences"},
    Keywords:    []string{"important"},
    RetrievalMode: "hybrid",
})

svc := memoryflow.New(db, planner, extractor,
    memoryflow.WithRecallStrategy(strategy),
)
```

### `(*hindsight.Strategy).Recall(ctx context.Context, req memoryflow.RecallRequest, next memoryflow.RecallFunc) (*memoryflow.RecallResponse, error)`

丰富回忆请求后委托给下一个处理器。自动合并关键词、实体名称，选择检索模式。

---

## 配置选项

### `gracedb.WithPath(path string) Option`

设置存储路径。空字符串启用内存模式。

### `gracedb.WithIndexType(indexType string) Option`

设置向量索引类型：`hnsw`（默认）、`ivf`、`flat`、`lsh`。

### `gracedb.WithSimilarity(fn string) Option`

设置相似度函数：`cosine`（默认）、`euclidean`。

### `gracedb.WithEmbedder(e types.Embedder) Option`

设置嵌入模型。实现 `types.Embedder` 接口即可接入任意向量模型。

### `types.DefaultConfig() *types.Config`

返回默认配置，含以下默认值：

| 配置项 | 默认值 |
|--------|--------|
| `SimilarityFn` | `cosine` |
| `IndexType` | `hnsw` |
| `HNSWConfig.M` | 16 |
| `HNSWConfig.EfConstruction` | 64 |
| `HNSWConfig.EfSearch` | 50 |
| `AutoSave` | `true` |

---

## 数据类型

### `types.Embedding`

向量嵌入数据模型。

| 字段 | 类型 | 说明 |
|------|------|------|
| `ID` | `string` | 唯一标识 |
| `CollectionID` | `string` | 集合 UUID |
| `Collection` | `string` | 集合名称 |
| `Vector` | `[]float32` | 向量数据 |
| `Content` | `string` | 原始文本 |
| `DocID` | `string` | 文档 ID |
| `Metadata` | `map[string]string` | 元数据 |
| `ACL` | `[]string` | 访问控制列表 |
| `CreatedAt` | `time.Time` | 创建时间 |

### `types.ScoredEmbedding`

带分数的嵌入结果，嵌入结构 + `Score float32`。

### `types.Collection`

集合（命名空间）信息。

### `types.MemoryRecord`

记忆记录，支持 scope/namespace/TTL、语义向量、importance 和 updated_at。

### `types.MemorySearchHit`

记忆检索结果，包含 `Score`/`FinalScore` 以及 `SemanticScore`、
`LexicalScore`、`ImportanceScore`、`RecencyScore` 排序解释字段。

### `types.KnowledgeRecord`

知识记录，支持版本管理和自动分块。

### `types.Session` / `types.Message`

会话和消息模型。

### `types.Document`

通用文档模型。

### `graph.GraphNode` / `graph.GraphEdge`

属性图节点和边。

### `rdf.Term` / `rdf.Triple`

RDF 术语和三元组。

---

## 错误类型

| 错误 | 说明 |
|------|------|
| `types.ErrNotFound` | 记录不存在 |
| `types.ErrCollectionExists` | 集合已存在 |
| `types.ErrDimensionMismatch` | 向量维度不匹配 |
| `types.ErrInvalidVector` | 无效向量（空或 nil） |
| `types.ErrEmbedderNotConfigured` | 未配置 Embedder |
| `types.ErrEmptyText` | 空文本 |

---

## 存储键格式

了解底层 Badger 键格式对调试和高级查询有帮助：

| 键格式 | 说明 |
|--------|------|
| `coll:<name>` | 集合元数据 |
| `emb:<collectionID>:<embID>` | 嵌入元数据 |
| `emb:vec:<collectionID>:<embID>` | 向量数据 |
| `emb:cont:<collectionID>:<embID>` | 内容文本 |
| `fts:<token>:<collectionID>:<embID>` | FTS 倒排索引（值为 TF 计数） |
| `graph:node:<id>` | 图节点 |
| `graph:edge:<id>` | 图边 |
| `graph:edge:from:<fromID>:<type>:<edgeID>` | 出边索引 |
| `graph:edge:to:<toID>:<type>:<edgeID>` | 入边索引 |
| `rdf:t:<id>` | RDF 三元组 |
| `rdf:s:<subject>:<id>` | 主题索引 |
| `rdf:p:<predicate>:<id>` | 谓词索引 |
| `rdf:o:<object>:<id>` | 客体索引 |
| `idx:snapshot:<collectionID>` | 向量索引快照 |
| `sess:<id>` | 会话 |
| `msg:<sessionID>:<msgID>` | 消息 |
| `doc:<id>` | 文档 |
| `know:<collection>:<id>` | 知识项 |
| `mem:<bucketID>:<id>` | 记忆元数据 |
| `mem:content:<bucketID>:<id>` | 记忆内容 |
| `mem:vec:<bucketID>:<id>` | 记忆语义向量 |
| `mem:fts:<token>:<bucketID>:<id>` | 记忆词法倒排索引 |
| `mem:idx:<id>` | 记忆 ID 到 bucketID 的索引 |
| `geo:<collectionID>:<docID>` | 地理坐标 |
