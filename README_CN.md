# gracedb

Go 嵌入式 AI 记忆 + 知识图谱数据库

## CortexDB 功能覆盖

gracedb 以 CortexDB 为对标目标，经 42 个功能点逐项验证（加权计算），**当前覆盖约 92%**。

### 已完全覆盖（34+ 项，100%）

Embedder 接口、文本自动向量化、Quick 接口、向量 CRUD、HNSW/IVF/Flat/LSH 四种索引、全文检索（BM25 + 中文分词 + 同义词 + 模糊匹配）、RRF 混合融合、Metadata 过滤、知识存储、Agent Memory、属性图、GraphRAG 工具集（9 工具）、MCP 服务、备份/恢复、OpenTelemetry、语义路由、MemoryFlow 工作流、GraphFlow 工作流、标量/乘积量化、Reranker 可插拔、会话管理、文档管理、KnowledgeMemory 融合召回（Recall/Reflect/Consolidate + 图展开）、自动记忆提取（AutoRetain）、多索引组合、Ontology 高级 API（RDFS/SHACL 封装）、聚合查询 GROUP BY。

### gracedb 额外实现（CortexDB 未纳入）

RDF 三元组存储、SPARQL SELECT/ASK、N-Triples 导入导出、RDFS 推理、SHACL 验证、Hindsight 回忆策略。

### 部分实现（2 项）

| 功能 | 完成度 | 缺失部分 |
|------|--------|---------|
| LLM 驱动实体提取 | 30% | 仅规则版，需调用方自行实现 LLM Extractor |
| LLM Reflect | 60% | 规则版已有，需调用方注入 LLM Reflector |

---

## 文档

| 文档 | 内容 |
|------|------|
| [入门指南](docs/getting-started.md) | 安装、配置、快速开始、常见问题 |
| [API 参考](docs/api-reference.md) | 完整 API 接口、数据类型、存储键格式 |
| [架构概述](docs/architecture.md) | 分层架构、数据流、索引体系、扩展点 |
| [模块说明](docs/modules.md) | 各子包职责、关键类型、依赖关系 |
| [实施计划](IMPLEMENTATION_PLAN.md) | 分阶段开发路线图 |

## 特性

- **向量检索** — HNSW / IVF / Flat / LSH 多种索引，支持余弦相似度搜索
- **全文检索** — 中文分词 + 停用词过滤 + Levenshtein 模糊匹配 + RRF 混合融合
- **知识存储** — 自动分块 + FTS 索引 + 按文档聚合检索
- **Agent Memory** — scope/namespace/TTL 隔离，支持向量化 + FTS 双路检索
- **属性图** — 节点/边 CRUD，BFS/DFS/最短路径遍历
- **RDF/SPARQL** — N-Triples 导入导出，SPARQL SELECT/ASK，RDFS 推理，SHACL 验证
- **GraphRAG 工具集** — 9 个开箱即用工具，供 LLM 编排调用
- **MCP 服务** — Model Context Protocol 兼容，stdio 传输
- **备份/恢复** — Badger native 全量备份
- **OpenTelemetry** — 核心操作自动 span 和指标上报
- **KnowledgeMemory** — 融合记忆+知识召回，支持 Reflect/Consolidate
- **AutoRetain** — 对话中自动提取事实并存储

## 快速开始

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
    // 打开数据库
    db, err := gracedb.Open("/tmp/gracedb-data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 创建集合
    coll, _ := db.CreateCollection("my_docs")
    fmt.Println("created:", coll.Name)

    // 插入向量
    vec := []float32{0.1, 0.2, 0.3, 0.4, 0.5, 0.6, 0.7, 0.8}
    embID, _ := db.Upsert("my_docs", "doc-1", vec, "Hello world", nil, nil)
    fmt.Println("embedded:", embID)

    // 搜索
    results, _ := db.Search("my_docs", vec, gracedb.SearchOptions{
        TopK:            5,
        UseVectorSearch: true,
    })
    fmt.Println("found:", len(results), "results")

    // 备份
    db.Backup("/tmp/gracedb-backup.db")

    // Quick 接口
    q := db.Quick()
    id, _ := q.AddToCollection(ctx, "my_docs", vec, "Quick add")
    fmt.Println("quick add:", id)
}
```

## 配置

```go
db, _ := gracedb.Open("/tmp/data",
    gracedb.WithIndexType("hnsw"),                      // hnsw / ivf / flat / lsh
    gracedb.WithIndexTypes([]string{"hnsw", "lsh"}),    // 多索引混合搜索
    gracedb.WithSimilarity("cosine"),                   // cosine / euclidean
    gracedb.WithEmbedder(myEmbedder),                   // types.Embedder 接口实现
)
```

## 架构

```
┌─────────────────────────────────────────────┐
│                gracedb.DB                    │  ← 门面层
│  Quick / Toolbox / Backup / Trace / Ontology │
├─────────────────────────────────────────────┤
│              KnowledgeMemory                 │  ← Recall/Reflect/Consolidate
│  AutoRetain / GroupAggregate                 │
├─────────────────────────────────────────────┤
│              BadgerStore                     │  ← 持久化层 (含内存索引)
│  CRUD / Search / FTS / Index / Aggregation   │
├─────────────────────────────────────────────┤
│              GraphStore / RDF                │  ← 图引擎
│  Nodes/Edges/Traversal/SPARQL/RDFS/SHACL     │
├─────────────────────────────────────────────┤
│              Badger v4                       │  ← 存储引擎
│  LSM-tree / MVCC / ACID                      │
└─────────────────────────────────────────────┘
```

## 使用示例

### 知识管理

```go
record, _ := db.SaveKnowledge("docs", "wiki-1", "Go 语言",
    "Go 是一种静态类型、编译型编程语言...",
    gracedb.KnowledgeSaveRequest{ChunkSize: 500, ChunkOverlap: 50})

resp, _ := db.SearchKnowledge("docs", "编程语言", 5)
```

### Agent Memory

```go
db.SaveMemory(types.MemorySaveRequest{
    MemoryID:  "mem-1",
    Content:   "用户喜欢 Go 语言",
    Scope:     "user",
    UserID:    "user-123",
    Namespace: "preferences",
    TTLSeconds: 3600,
})
```

### KnowledgeMemory 召回/反思/固化

```go
km := db.KnowledgeMemory(nil) // nil = 使用规则版反射器

// 召回：融合记忆 + 知识 + 图展开
resp, _ := km.Recall(ctx, knowledge.KnowledgeMemoryRecallRequest{
    Query:         "用户有什么偏好？",
    TopKMemories:  5,
    TopKKnowledge: 4,
    MaxHops:       2,
})

// 反思：合成结构化摘要
reflection, _ := km.Reflect(ctx, knowledge.KnowledgeMemoryReflectRequest{
    Query: "用户偏好",
})

// 固化：存储摘要 + 可选晋升为知识
consolidated, _ := km.Consolidate(ctx, knowledge.KnowledgeMemoryConsolidateRequest{
    Reflect: knowledge.KnowledgeMemoryReflectRequest{Query: "用户偏好"},
    PromoteToKnowledge: true,
})
```

### 属性图

```go
g := db.Graph()
g.UpsertNode(&graph.GraphNode{ID: "person-1", Type: "person"})
g.UpsertNode(&graph.GraphNode{ID: "lang-1", Type: "language"})
g.UpsertEdge(&graph.GraphEdge{
    FromNodeID: "person-1", ToNodeID: "lang-1",
    Type: "likes", Weight: 1.0,
})

nodes, edges, _ := g.GetNeighbors("person-1", graph.NeighborOptions{Direction: "out"})
```

### RDF/SPARQL

```go
rdf := db.RDF()
rdf.UpsertTriple(&rdf.Triple{
    Subject:   rdf.NewIRI("http://example.org/person/1"),
    Predicate: rdf.NewIRI("http://example.org/likes"),
    Object:    rdf.NewIRI("http://example.org/lang/go"),
})

results, _ := rdf.SPARQLSelect(`SELECT ?s ?p ?o WHERE { ?s ?p ?o . }`)
```

### Ontology 管理

```go
o := db.Ontology()
o.DefineClass("http://example.org/Person", "")
o.DefineClass("http://example.org/Developer", "http://example.org/Person")
o.DefineProperty("http://example.org/knows",
    "http://example.org/Person", "http://example.org/Person")

o.AddFact("http://example.org/person/alice", "http://example.org/knows", "Bob")

count, _ := o.Infer()    // RDFS 推理
report, _ := o.Validate() // SHACL 验证
```

### 自动记忆提取（AutoRetain）

```go
db.SetFactExtractor(func(ctx context.Context, msgs []*types.Message) ([]gracedb.ExtractedFact, error) {
    // 从对话中提取事实（可接入 LLM）
    return []gracedb.ExtractedFact{
        {ID: "fact-1", Content: "用户喜欢 Go", Type: "preference"},
    }, nil
})

db.SetAutoRetain(gracedb.AutoRetainConfig{
    Enabled:      true,
    WindowSize:   6,
    TriggerEvery: 2, // 每 2 条消息触发
})

db.AddMessage(&types.Message{SessionID: "sess-1", Role: "user", Content: "我喜欢 Go"})
db.AddMessage(&types.Message{SessionID: "sess-1", Role: "assistant", Content: "Go 很棒！"})
// → AutoRetain 触发，自动提取并存储为记忆
```

### 聚合查询

```go
// 简单聚合
result, _ := db.Aggregate("docs", "score", store.AggAvg)
fmt.Printf("平均分: %.2f\n", result.Avg)

// 分组聚合 (GROUP BY)
groups, _ := db.GroupAggregate("docs", "category", "price", store.AggAvg)
for category, r := range groups {
    fmt.Printf("%s: 平均=%.2f, 数量=%d\n", category, r.Avg, r.Count)
}
```

### MCP 服务

```go
server := db.NewMCPServer("gracedb", "1.0.0")
server.RunStdio(context.Background())
```

### 备份与恢复

```go
db.Backup("/tmp/backup.db")

db2, _ := gracedb.Open("/tmp/restored")
db2.Restore("/tmp/backup.db")
```

## 索引管理

```go
db.LoadIndex("docs")   // 启动后加载索引
db.SaveIndex("docs")   // 持久化索引快照
db.RebuildIndex("docs") // 重建 FTS 索引
```

**注意**：应用启动后需调用 `LoadIndex`，否则搜索会回退到全量扫描（正确但慢）。

## 示例

```bash
go run examples/main.go
```

## 测试

```bash
go test ./...              # 全部测试
go test -v ./pkg/index/    # 详细输出
go test -bench=. ./pkg/store/  # 基准测试
```

## 许可证

MIT
