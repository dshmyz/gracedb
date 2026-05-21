# gracedb 模块说明

各子包的职责、关键类型和使用场景。

## pkg/gracedb — 门面层

主入口包。`DB` 结构体聚合所有子系统能力，是绝大多数调用方的唯一交互点。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `db.go` | `DB` 结构、`Open/Close`、配置选项 |
| `vector.go` | 向量 CRUD + 搜索（含 OpenTelemetry 埋点） |
| `text.go` | 文本自动向量化 |
| `quick.go` | Quick 快捷接口 |
| `collection.go` | 集合 CRUD |
| `knowledge.go` | 知识管理 API |
| `memory.go` | 记忆管理 API |
| `session.go` | 会话/消息 API |
| `doc.go` | 文档 API + Stats |
| `toolbox.go` | GraphRAG 工具集（7 工具） |
| `backup.go` | 备份/恢复 |
| `trace.go` | OpenTelemetry span + metrics |
| `testutil.go` | 测试工具（mockEmbedder, testDB） |
| `aggregation.go` | 聚合查询 |
| `geosearch.go` | 地理搜索 |

**设计模式**：

- **Functional Options**：`Open(path, opts...)` 替代大 Config 结构
- **Facade**：隐藏 store/index 细节，对外暴露统一接口
- **代理模式**：门面层持有 Embedder，自动执行文本→向量转换

---

## pkg/store — 持久化层

Badger KV 封装层，包含所有 CRUD 操作、搜索逻辑和索引管理。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `store.go` | BadgerStore 初始化、事务封装、View/Update 包装 |
| `crud.go` | 嵌入 CRUD、文档 CRUD |
| `collection.go` | 集合 CRUD |
| `search.go` | 向量搜索（索引/全量扫描）、RRF 融合、索引持久化 |
| `fts.go` | 全文检索（分词、BM25、同义词、模糊匹配） |
| `reranker.go` | BM25/Cosine 重排序实现 |
| `session.go` | 会话/消息持久化 |
| `knowledge.go` | 知识项持久化 |
| `memory.go` | 记忆存储（bucket 解析、TTL 过滤） |
| `chunking.go` | 文本分块（按字符数 + 句子边界） |
| `similarity.go` | 余弦相似度、欧氏距离 |
| `fuzzy.go` | Levenshtein 模糊匹配 |
| `stemmer.go` | Porter 词干提取 |
| `thesaurus.go` | 同义词表 |
| `aggregation.go` | 聚合查询（count/avg/sum/min/max） |
| `geosearch.go` | 地理坐标搜索 |
| `embedding.go` | Store 接口定义 |

**注意**：`BadgerStore` 是线程安全的（通过 `sync.RWMutex`），但向量索引（HNSW 等）本身无锁，需要外部同步。

---

## pkg/index — 向量索引

四种向量索引的纯 Go 实现。

### HNSW (`hnsw.go`)

分层导航小世界图索引。

- **参数**：`M`（最大邻居数）、`EfConstruction`（构建时候选数）、`EfSearch`（搜索时候选数）
- **序列化**：gob 编码，支持 `Marshal/Unmarshal`
- **线程安全**：无内部锁，单线程构建

### IVF (`ivf.go`)

倒排文件索引。

- **参数**：分区数、每分区样本数
- **适合**：中等规模数据集
- **原理**：向量空间分区，分区内暴力搜索

### Flat (`flat.go`)

暴力搜索。

- **无参数**：每次搜索计算所有距离
- **适合**：小规模数据集或精确搜索场景

### LSH (`lsh.go`)

局部敏感哈希。

- **参数**：哈希表数、哈希位数
- **适合**：超大规模数据集，内存受限场景

### 多索引 (`multi.go`)

组合多个索引类型，支持跨索引搜索。

**公共接口** (`types.go`)：

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

## pkg/quantization — 向量量化

### 标量量化 (`scalar.go`)

将 float32 向量压缩为低位整数表示，减少存储空间。

### 乘积量化 (`product.go`)

将高维向量划分为子空间，每个子空间独立量化并通过码本编码。

量化是可选优化层，不影响搜索接口。量化后的向量可以与原始向量混合使用（需注意精度损失）。

---

## pkg/graph — 属性图存储

基于 Badger 的轻量级属性图引擎。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `types.go` | 节点、边、路径、遍历结果类型定义 |
| `store.go` | CRUD 操作、索引管理、邻居查询 |
| `traversal.go` | BFS/DFS/最短路径算法 |

**键设计**：

每个边操作维护 4 个 Badger 键：
1. 主键：`graph:edge:<id>` → JSON
2. 出边索引：`graph:edge:from:<fromID>:<type>:<edgeID>`
3. 入边索引：`graph:edge:to:<toID>:<type>:<edgeID>`
4. 类型索引：`graph:edge:type:<type>:<edgeID>`

节点同样按类型索引：`graph:node:type:<type>:<id>`

---

## pkg/rdf — RDF/SPARQL

RDF 三元组存储，支持简化 SPARQL 查询。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `store.go` | 三元组 CRUD、模式查询、SPARQL SELECT/ASK |
| `sparql.go` | 简化 SPARQL 解析器（词法分析 + 三元模式解析） |
| `ntriples.go` | N-Triples 格式导入/导出 |
| `rdfs.go` | RDFS 推理规则（subClassOf、subPropertyOf、domain、range） |
| `shacl.go` | SHACL 约束验证（minCount、maxCount、datatype、nodeKind） |

**支持的 SPARQL 特性**：
- SELECT 变量投影
- ASK 存在性检查
- WHERE 中的三元模式（变量/IRI/字面量混合）
- 变量绑定传播
- LIMIT 限制

**不支持的特性**：
- OPTIONAL / FILTER / UNION
- 聚合函数（COUNT、GROUP BY）
- 属性路径
- 完整 SPARQL 1.1 语法

---

## pkg/mcp — MCP 协议服务器

简化版 Model Context Protocol JSON-RPC 服务器。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `server.go` | JSON-RPC 路由、stdio 传输、工具注册 |

**协议方法**：
- `initialize` — 返回服务器信息和协议版本
- `tools/list` — 返回所有注册工具
- `tools/call` — 调用指定工具，返回文本内容

**传输**：每行一个 JSON 请求/响应，通过 stdin/stdout 通信。

---

## pkg/memoryflow — 记忆工作流

完整的 Agent 记忆工作流管线。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `types.go` | 回忆请求/响应、转录、晋升、唤醒层类型 |
| `service.go` | 核心服务（转录、回忆、唤醒、日记、会话关闭） |
| `strategy.go` | 回忆策略接口 + Passthrough 默认实现 |
| `promotion.go` | 知识晋升逻辑（分类、评分） |

**工作流**：

```
IngestTranscript → episodic memory
       │
       ├── Recall → fused memory + knowledge
       │      │
       │      └── 可挂载策略插件（如 Hindsight）
       │
       ├── WakeUpLayers → L0-L3 context tiers
       │
       └── CloseSession → ingest + promote knowledge
              │
              ├── SessionExtractor → 提取候选知识
              └── PromotionPolicy → 过滤晋升
```

**扩展点**：
- `QueryPlanner` — 自定义检索计划
- `SessionExtractor` — 自定义对话知识提取
- `PromotionPolicy` — 自定义晋升规则
- `RecallStrategy` — 自定义回忆策略

---

## pkg/graphflow — 语料到图工作流

从文本到属性图的完整管线。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `pipeline.go` | 类型定义、Build、Analyze、RenderReport |
| `extract.go` | HeuristicExtractor（基于规则的实体提取） |

**功能**：
- **Build** — 将提取结果写入图存储，支持去重和最小权重过滤
- **Analyze** — 图统计（节点/边数、度分布、密度、孤立节点、类型计数）
- **RenderReport** — 输出为 text / markdown / HTML

**HeuristicExtractor**：
简单规则提取器：将大写单词视为实体，相邻实体之间创建共现边。适合英文文本演示，生产环境应替换为 LLM-based 提取器。

---

## pkg/semanticrouter — 语义路由

将用户查询路由到预定义的处理路径。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `router.go` | 语义路由（向量相似度匹配） |
| `lexical.go` | 词法路由（关键词匹配） |
| `hybrid.go` | 混合路由（词法 + 语义） |

**使用场景**：
- 意图分类：将用户输入路由到 save_memory / knowledge_search / graph_query 等 handler
- 在 LLM 调用前做预分类，减少不必要的 API 调用

**语义路由流程**：
1. 查询向量通过 Embedder 生成
2. 与每个 Route 的 Utterances 向量计算余弦相似度
3. 返回最佳匹配（分数 ≥ Threshold 才视为匹配）
4. Utterances 向量自动缓存，避免重复计算

**词法路由**：
无需 Embedder，基于关键词重叠度打分。适合无向量模型的轻量场景。

---

## pkg/hindsight — 回忆策略插件

MemoryFlow 的可插拔回忆策略，通过 bank ID、实体名称和关键词线索丰富回忆请求。

**关键文件**：

| 文件 | 职责 |
|------|------|
| `strategy.go` | Hindsight 策略实现 |

**工作原理**：
1. 接收 RecallRequest
2. 注入 namespace（从 BankID 派生）、关键词、实体名称
3. 选择检索模式（graph / lexical / hybrid / semantic）
4. 委托给下一个 RecallFunc 执行

**BankID 处理**：
自动清洗并规范化 BankID 为合法的 namespace 字符串（小写、限制长度、超长时截断+哈希）。

---

## pkg/types — 公共类型

所有子包共享的类型和接口定义。

**关键文件**：

| 文件 | 内容 |
|------|------|
| `embedding.go` | Embedding、ScoredEmbedding、SearchOptions、Collection、Session、Message、Document、Config、StoreStats |
| `embedder.go` | Embedder 接口 |
| `reranker.go` | Reranker 接口 |
| `knowledge.go` | KnowledgeRecord、KnowledgeSaveRequest、KnowledgeSearchRequest/Response/Hit、Chunk |
| `memory.go` | MemoryRecord、MemorySave/Update/ Search 请求与响应 |
| `similarity.go` | 相似度计算函数 |
| `errors.go` | Sentinel 错误定义 |

---

## 模块依赖关系

```
gracedb (门面)
├── store (持久化)
│   ├── index (向量索引)
│   ├── types (公共类型)
│   └── quantization (量化)
├── graph (属性图)
├── rdf (RDF/SPARQL)
├── mcp (MCP 服务)
│   └── types
├── memoryflow (记忆工作流)
│   ├── types
│   └── hindsight (策略插件)
├── graphflow (语料到图)
│   └── graph
├── semanticrouter (语义路由)
│   └── types (Embedder 接口)
└── types (公共类型)
```

各引擎子包（graph、rdf、memoryflow 等）互相独立，只依赖 `pkg/types` 定义接口和数据模型。门面层负责编排和调用。
