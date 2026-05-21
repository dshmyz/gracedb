# gracedb 架构概述

## 系统架构

gracedb 采用四层分层架构，每层职责清晰，依赖方向严格向下。

```
┌──────────────────────────────────────────────────────────┐
│                    门面层 (gracedb.DB)                     │
│  ┌─────────┐ ┌──────┐ ┌──────────┐ ┌────────┐ ┌───────┐ │
│  │Vector   │ │Text  │ │Knowledge │ │Memory  │ │Session│ │
│  └─────────┘ └──────┘ └──────────┘ └─────────┘ └───────┘ │
│  ┌─────────┐ ┌──────┐ ┌──────────┐ ┌────────┐ ┌───────┐ │
│  │Quick    │ │Toolbox│ │Backup   │ │Trace   │ │Aggregation│
│  └─────────┘ └──────┘ └──────────┘ └────────┘ └───────┘ │
├──────────────────────────────────────────────────────────┤
│              引擎层 (pkg/*)                                │
│  ┌─────────┐ ┌──────┐ ┌──────┐ ┌──────────┐ ┌─────────┐ │
│  │Graph    │ │RDF   │ │Index │ │MemoryFlow│ │GraphFlow│ │
│  └─────────┘ └──────┘ └──────┘ └──────────┘ └─────────┘ │
│  ┌─────────┐ ┌────────────┐ ┌──────────┐ ┌────────────┐ │
│  │Semantic  │ │Hindsight   │ │Quantization│ │MCP        │ │
│  │Router    │ │            │ │            │ │            │ │
│  └─────────┘ └────────────┘ └──────────┘ └────────────┘ │
├──────────────────────────────────────────────────────────┤
│              持久化层 (pkg/store)                          │
│  ┌─────────┐ ┌─────┐ ┌────────┐ ┌──────────┐ ┌────────┐ │
│  │CRUD     │ │Search│ │FTS    │ │Chunking  │ │Reranker│ │
│  └─────────┘ └─────┘ └────────┘ └──────────┘ └────────┘ │
├──────────────────────────────────────────────────────────┤
│              存储引擎 (Badger v4)                          │
│  LSM-tree / MVCC / ACID / Value Log GC                   │
└──────────────────────────────────────────────────────────┘
```

## 分层职责

### 门面层 (`pkg/gracedb/`)

唯一的对外接口。封装所有业务逻辑，调用方只需导入 `gracedb` 包。

- **Functional Options 模式**：`Open(path, opts...)` 通过 Option 函数灵活配置
- **遥测集成**：核心操作自动创建 OpenTelemetry span 和指标
- **Embedder 代理**：门面层持有 Embedder 引用，自动执行文本向量化
- **FTS 自动索引**：写入向量时，门面层自动建立 FTS 倒排索引

### 引擎层 (`pkg/*`)

独立子包，实现特定领域能力：

- **`index/`** — 向量索引（HNSW/IVF/Flat/LSH），纯 Go 实现
- **`graph/`** — 属性图存储，Badger-backed 节点/边管理
- **`rdf/`** — RDF 三元组存储，SPARQL 查询引擎
- **`quantization/`** — 向量量化（标量量化、乘积量化）
- **`mcp/`** — Model Context Protocol JSON-RPC 服务器
- **`memoryflow/`** — Agent 记忆工作流（转录 → 回忆 → 唤醒 → 日记 → 晋升）
- **`graphflow/`** — 语料到图工作流（提取 → 构建 → 分析 → 导出）
- **`semanticrouter/`** — 语义路由（向量相似度 + 词法匹配）
- **`hindsight/`** — 回忆策略插件

### 持久化层 (`pkg/store/`)

Badger 存储操作封装，包含：

- **CRUD** — 嵌入/集合/会话/文档/知识的增删改查
- **Search** — 向量相似度搜索（索引或全量扫描）+ RRF 混合融合
- **FTS** — 全文检索（gse 中文分词 + BM25 打分 + 同义词 + 模糊匹配 + 短语搜索）
- **Reranker** — BM25/Cosine 重排序 + RRF 多路融合
- **索引管理** — `LoadIndex`/`SaveIndex` 序列化内存索引快照

### 存储引擎 (Badger v4)

高性能嵌入式 LSM-tree KV 数据库，提供：

- ACID 事务
- MVCC 并发
- Value Log 自动 GC
- 在线备份/恢复
- 前缀扫描（支撑所有二级索引）

## 数据流

### 向量插入流程

```
db.Upsert(collection, docID, vector, content, metadata, acl)
    │
    ├──► store.Upsert(...)              # 写入 Badger
    │       ├── emb:<cid>:<eid>         # 元数据
    │       ├── emb:vec:<cid>:<eid>     # 向量
    │       └── emb:cont:<cid>:<eid>    # 内容
    │
    └──► store.IndexFTS(cid, eid, content)  # FTS 索引
            └── fts:<token>:<cid>:<eid> = TF count
```

### 检索流程

```
db.Search(collection, query, opts)
    │
    ├──► store.Search(...)
    │       │
    │       ├── 向量搜索（如果 UseVectorSearch）
    │       │   ├── 有内存索引 → idx.Search(query, topK)
    │       │   └── 无索引 → 全量扫描 + CosineSimilarity
    │       │
    │       └── FTS 搜索（如果 UseTextSearch）
    │           ├── TokenizeForQuery → 分词 + 同义词 + 模糊
    │           └── BM25 打分 + 排序
    │
    └── RRF Fusion（双路搜索时）
        └── score = 1/(k + rank_vector) + 1/(k + rank_fts)
```

### 知识存储流程

```
db.SaveKnowledge(collection, id, title, content, opts)
    │
    ├──► ChunkBySize(content, chunkSize, chunkOverlap)
    │       └── 按字符数切分，优先在句子边界断开
    │
    ├──► 每个 chunk → store.Upsert (作为独立 embedding)
    │
    └──► 建立 knowledgeID → chunkIDs 映射
         └── fts 索引（对 content 全文）
```

### 记忆管理流程

```
db.SaveMemory(req)
    │
    ├──► resolveMemoryBucket(scope, userID, sessionID, namespace)
    │       ├── global   → mem:bucket:global:{namespace}:{id}
    │       ├── user     → mem:bucket:user:{userID}:{namespace}:{id}
    │       └── session  → mem:bucket:session:{sessionID}:{namespace}:{id}
    │
    └──► store.SaveMemory(...)
```

## 向量索引体系

### 四种索引类型对比

| 索引 | 算法 | 适用场景 | 内存占用 | 精度 |
|------|------|----------|----------|------|
| **HNSW** | 分层导航小世界图 | 大规模 (>10K 向量) | 中等 | 近似 (可配置 efSearch) |
| **IVF** | 倒排文件索引 | 中等规模 | 低 | 近似 (依赖分区数) |
| **Flat** | 暴力全量扫描 | 小规模 (<1K 向量) | 低 | 精确 |
| **LSH** | 局部敏感哈希 | 超大规模 | 最低 | 近似 |

### 索引生命周期

1. **创建**：`newIndex()` 根据 `Config.IndexType` 实例化
2. **写入**：`Upsert` 同步更新内存索引（`idx.Insert(vector, id)`）
3. **搜索**：`vectorSearch` 优先使用内存索引，回退到 Badger 全量扫描
4. **持久化**：`SaveIndex` 序列化索引快照到 Badger `idx:snapshot:<cid>`
5. **恢复**：`LoadIndex` 优先加载快照，失败则从向量重建
6. **删除**：`DeleteEmbedding` 调用 `idx.RemoveVector(id)`

### 量化集成

`pkg/quantization/` 提供两种量化方式：

- **标量量化**：将 float32 向量压缩为低位整数，减少存储
- **乘积量化 (PQ)**：将高维向量划分为子空间，每个子空间独立量化

量化是可选的优化层，不影响搜索精度接口。

## FTS 全文检索系统

### 分词管线

```
content → gse 分词 → 停用词过滤 → 词干提取 → 同义词扩展 → 写入 Badger
```

- **gse 分词**：支持中英文混合分词，延迟初始化（sync.Once）
- **停用词**：内置 50+ 中英文停用词
- **词干提取**：英文词干还原（porter stemmer 简化版）
- **同义词**：内置同义词表，索引时展开所有同义词变体

### 查询增强

- **短语搜索**：`"exact phrase"` 双引号包裹，要求所有 token 出现在同一文档
- **前缀搜索**：`term*` 星号后缀，前缀匹配
- **模糊搜索**：`FTSSearchOptions.FuzzyMaxDist` 控制 Levenshtein 编辑距离
- **同义词扩展**：查询时自动展开同义词

### BM25 打分

标准 BM25 公式：

```
score = IDF × (TF × (k1 + 1)) / (TF + k1 × (1 - b + b × docLen / avgDocLen))
```

默认参数：k1=1.2, b=0.75。TF 计数从分词时统计并存储在 Badger 值中。

### RRF 混合融合

Reciprocal Rank Fusion 结合向量搜索和 FTS 结果：

```
RRF(d) = Σ 1 / (k + rank_i(d))     # k=60（标准值）
```

结果按 RRF 分数降序排列，自动去重。

## 属性图设计

### Badger 键设计

```
graph:node:<id>                    → JSON 序列化节点
graph:edge:<id>                    → JSON 序列化边
graph:edge:from:<fromID>:<type>:<edgeID>  → 出边索引
graph:edge:to:<toID>:<type>:<edgeID>      → 入边索引
graph:edge:type:<type>:<edgeID>           → 类型索引
graph:node:type:<type>:<id>               → 节点类型索引
```

每个边操作维护 4 个索引键（主键 + 2 个方向 + 类型），确保双向遍历和类型过滤效率。

### 遍历算法

- **BFS**：标准队列实现，支持 MaxDepth 限制
- **DFS**：递归实现，支持 MaxDepth 限制
- **ShortestPath**：基于 BFS 的最短路径（无权图）

## MCP 协议集成

### JSON-RPC 2.0 流程

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

### 工具注册

`FromToolbox` 函数将 GraphRAGToolbox 的工具定义自动转换为 MCP 工具：

```go
server := mcp.FromToolbox(name, version, toolbox.Definitions(), toolbox.Call)
```

传输层使用 stdio，每行一个 JSON-RPC 请求。

## OpenTelemetry 集成

核心操作自动埋点：

- **Upsert / UpsertBatch**：span + 计数器
- **Search**：span + 耗时指标
- **Delete**：span

未配置 exporter 时使用 noop provider，零性能损耗。

## 并发安全

| 组件 | 锁策略 |
|------|--------|
| `BadgerStore` | `sync.RWMutex` — Upsert/Delete 写锁，Search 读锁 |
| `GraphStore` | `sync.RWMutex` — 节点/边操作写锁 |
| `MCP Server` | `sync.RWMutex` — 工具注册写锁，调用读锁 |
| `SemanticRouter` | `sync.RWMutex` — 路由注册写锁，Route 读锁 |
| `HNSW` | 无锁 — 单线程索引构建（需外部同步） |

## 扩展点

### Embedder 接口

实现 `types.Embedder` 即可接入任意向量模型：

```go
type Embedder interface {
    Embed(ctx context.Context, text string) ([]float32, error)
    EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
    Dimension() int
}
```

### Reranker 接口

实现 `types.Reranker` 可自定义重排序逻辑：

```go
type Reranker interface {
    Rerank(ctx context.Context, query string, candidates []ScoredEmbedding) ([]ScoredEmbedding, error)
}
```

### MemoryFlow 策略

- `QueryPlanner` — 自定义检索计划生成
- `SessionExtractor` — 自定义知识提取
- `PromotionPolicy` — 自定义晋升过滤
- `RecallStrategy` — 自定义回忆策略（如 Hindsight 插件）

### 向量索引

所有索引实现 `index.Index` 接口，可自定义新索引类型：

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

## 性能特征

### 索引选择指南

| 数据量 | 推荐索引 | 理由 |
|--------|----------|------|
| < 1,000 | Flat | 精确，开销最小 |
| 1K - 100K | HNSW (efSearch=50) | 精度/速度平衡 |
| 100K - 1M | IVF (nlist=√N) | 内存可控 |
| > 1M | LSH | 内存最小，速度最快 |

### 内存索引 vs 全量扫描

- **内存索引**：搜索 O(log N)（HNSW），但需常驻内存
- **全量扫描**：搜索 O(N)，每次从 Badger 读取向量，适合冷数据或小数据集

### FTS 性能

- 查询 O(前缀扫描匹配文档数)
- BM25 打分在内存中执行
- 大量文档时建议限制 TopK 或使用 Metadata 过滤减少候选集
