# gracedb

Go 嵌入式 AI 记忆 + 知识图谱数据库

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
- **GraphRAG 工具集** — 7 个开箱即用工具，供 LLM 编排调用
- **MCP 服务** — Model Context Protocol 兼容，stdio 传输
- **备份/恢复** — Badger native 全量备份
- **OpenTelemetry** — 核心操作自动 span 和指标上报

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
    gracedb.WithIndexType("hnsw"),         // hnsw / ivf / flat / lsh
    gracedb.WithSimilarity("cosine"),       // cosine / euclidean
    gracedb.WithEmbedder(myEmbedder),       // types.Embedder 接口实现
)
```

## 架构

```
┌─────────────────────────────────────┐
│            gracedb.DB               │  ← 门面层
│  Quick / Toolbox / Backup / Trace   │
├─────────────────────────────────────┤
│         BadgerStore                 │  ← 持久化层 (含内存索引)
│  CRUD / Search / FTS / Index        │
├─────────────────────────────────────┤
│         GraphStore / RDF            │  ← 图引擎
│  Nodes/Edges/Traversal/SPARQL       │
├─────────────────────────────────────┤
│         Badger v4                   │  ← 存储引擎
│  LSM-tree / MVCC / ACID             │
└─────────────────────────────────────┘
```

## 示例

完整示例见 `examples/` 目录：

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
