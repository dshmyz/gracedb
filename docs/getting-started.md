# gracedb 入门指南

## 环境要求

- Go 1.23 或更高版本
- 无其他外部依赖（Badger、gse 等均通过 go.mod 管理）

## 安装

```bash
go get github.com/dshmyz/gracedb
```

## 快速开始

### 1. 打开数据库

```go
package main

import (
    "fmt"
    "log"

    "github.com/dshmyz/gracedb/pkg/gracedb"
)

func main() {
    // 磁盘模式
    db, err := gracedb.Open("/tmp/gracedb-data")
    if err != nil {
        log.Fatal(err)
    }
    defer db.Close()

    // 内存模式（path 为空）
    memDB, _ := gracedb.Open("")
    defer memDB.Close()
}
```

### 2. 配置向量模型（可选）

如果不配置 Embedder，仍然可以使用向量 API（手动传入向量）和 FTS 全文检索。

```go
// 实现 types.Embedder 接口
type MyEmbedder struct{}

func (e *MyEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
    // 调用你的向量模型，返回固定维度的 float32 切片
    return []float32{0.1, 0.2, 0.3, 0.4, 0.5}, nil
}

func (e *MyEmbedder) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
    // 批量向量化
}

func (e *MyEmbedder) Dimension() int {
    return 5 // 返回向量维度
}

// 使用
db, _ := gracedb.Open("/tmp/data",
    gracedb.WithEmbedder(&MyEmbedder{}),
    gracedb.WithIndexType("hnsw"),    // hnsw / ivf / flat / lsh
    gracedb.WithSimilarity("cosine"), // cosine / euclidean
)
```

### 3. 基本操作

```go
// 创建集合
coll, _ := db.CreateCollection("documents")
fmt.Println("集合:", coll.Name)

// 插入向量
embID, _ := db.Upsert("documents", "doc-1",
    []float32{0.1, 0.2, 0.3, 0.4, 0.5},  // 向量
    "Hello world",                        // 内容（自动建立 FTS 索引）
    map[string]string{"source": "test"},  // 元数据
    nil,                                  // ACL
)

// 搜索
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

### 4. 文本操作（需 Embedder）

```go
// 插入文本（自动向量化）
id, _ := db.InsertText("documents", "text-1", "这是一段中文文本", nil)

// 文本检索（有 Embedder 时向量化搜索，无 Embedder 时 FTS）
results, _ := db.SearchText("documents", "中文", 10)
```

### 5. Quick 接口

Quick 提供简化的 API，无需手动管理 ID：

```go
q := db.Quick()

// 添加（自动生成 UUID）
id, _ := q.Add(ctx, vector, "content")
id, _ = q.AddText(ctx, "some text", nil)

// 搜索
results, _ := q.Search(ctx, queryVector, 10)
results, _ = q.SearchText(ctx, "search query", 10)

// 纯文本搜索（不需要 Embedder）
results, _ = q.SearchTextOnly(ctx, "keyword", 10)
```

### 6. 知识管理

```go
// 保存知识（自动分块）
record, _ := db.SaveKnowledge("documents", "wiki-1",
    "Go 语言",
    "Go 是一种静态类型、编译型编程语言，由 Google 开发...",
    gracedb.KnowledgeSaveRequest{
        ChunkSize:    500,  // 每块字符数
        ChunkOverlap: 50,   // 重叠字符数
    },
)

// 搜索知识
resp, _ := db.SearchKnowledge("documents", "编程语言", 5)
for _, hit := range resp.Results {
    fmt.Printf("%s: %s\n", hit.Title, hit.Snippet)
}
```

### 7. 记忆管理

```go
// 保存记忆
record, _ := db.SaveMemory(types.MemorySaveRequest{
    MemoryID:   "mem-1",
    Content:    "用户喜欢 Go 语言",
    Scope:      "user",        // global / user / session
    UserID:     "user-123",
    Namespace:  "preferences", // 可选的命名空间隔离
    TTLSeconds: 3600,          // 可选 TTL
})

// 搜索记忆
resp, _ := db.SearchMemory(types.MemorySearchRequest{
    Query:   "喜欢",
    UserID:  "user-123",
    Scope:   "user",
    TopK:    5,
})
```

### 8. 属性图

```go
g := db.Graph()

// 创建节点
g.UpsertNode(&graph.GraphNode{
    ID:   "person-1",
    Type: "person",
    Properties: map[string]string{
        "name": "张三",
    },
})

g.UpsertNode(&graph.GraphNode{
    ID:   "lang-1",
    Type: "language",
    Properties: map[string]string{
        "name": "Go",
    },
})

// 创建边
g.UpsertEdge(&graph.GraphEdge{
    FromNodeID: "person-1",
    ToNodeID:   "lang-1",
    Type:       "likes",
    Weight:     1.0,
})

// 查询邻居
nodes, edges, _ := g.GetNeighbors("person-1", graph.NeighborOptions{
    Direction: "out",  // out / in / both
    Limit:     10,
})

// 图遍历
result, _ := g.BFS("person-1", graph.NeighborOptions{MaxDepth: 2})
```

### 9. RDF/SPARQL

```go
rdf := db.RDF()

// 插入三元组
rdf.UpsertTriple(&rdf.Triple{
    Subject:   rdf.NewIRI("http://example.org/person/1"),
    Predicate: rdf.NewIRI("http://example.org/likes"),
    Object:    rdf.NewIRI("http://example.org/lang/go"),
})

// SPARQL 查询
results, _ := rdf.SPARQLSelect(`
    SELECT ?s ?p ?o
    WHERE { ?s ?p ?o . }
`)

// ASK 查询
exists, _ := rdf.SPARQLAsk(`
    ASK WHERE { <http://example.org/person/1> ?p ?o . }
`)
```

### 10. 备份与恢复

```go
// 备份
err := db.Backup("/tmp/gracedb-backup.db")

// 恢复（需要先打开一个空 DB）
db2, _ := gracedb.Open("/tmp/gracedb-restored")
err = db2.Restore("/tmp/gracedb-backup.db")
```

### 11. MCP 服务

```go
server := db.NewMCPServer("gracedb", "1.0.0")
err := server.RunStdio(context.Background())
```

运行后，可通过 stdio 与 MCP 客户端通信（如 Claude Desktop、Cursor 等）。

## 索引管理

向量索引默认在内存中维护。对于持久化场景：

```go
// 加载索引（从快照或重建）
db.LoadIndex("documents")

// 保存索引快照
db.SaveIndex("documents")

// 重建索引（清理所有 FTS 条目后重建）
db.RebuildIndex("documents")
```

**重要**：应用启动后需调用 `LoadIndex` 加载索引，否则搜索会回退到全量扫描（慢但正确）。

## 完整示例

运行内置示例程序：

```bash
go run examples/main.go
```

示例演示：集合 CRUD、向量插入/搜索、文本操作、会话管理、知识管理、记忆管理、属性图、备份恢复等全部 API。

## 常见问题

### Q: 向量维度不匹配怎么办？

gracedb 不强制验证向量维度。确保所有插入同一集合的向量维度一致。Embedder 的 `Dimension()` 方法仅供参考。

### Q: FTS 搜索和向量搜索的区别？

- **FTS**：基于分词匹配，不需要 Embedder，适合关键词搜索
- **向量搜索**：基于语义相似度，需要 Embedder，适合语义理解

两者可以同时使用，结果通过 RRF 融合。

### Q: 内存模式 vs 磁盘模式？

- `gracedb.Open("")` — 内存模式，数据不持久化，适合测试
- `gracedb.Open("/path/to/data")` — 磁盘模式，数据持久化到 Badger

### Q: 如何提升搜索性能？

1. 使用 HNSW 索引（默认），并调用 `LoadIndex` 加载
2. 对大集合使用 `SaveIndex` 持久化索引快照
3. 使用 Metadata 过滤减少候选集
4. 调整 HNSW 参数：`EfSearch` 越高精度越高但越慢

### Q: FTS 支持中文吗？

支持。使用 gse 分词器，内置中文分词和停用词表。分词器延迟初始化，首次使用时自动加载词典。
