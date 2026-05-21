# gracedb — Go 嵌入式 AI 记忆 + 知识图谱数据库

## 项目概述

gracedb 是一个基于 Badger KV 存储的 Go 嵌入式 AI 记忆与知识图谱数据库，提供向量检索、全文检索、知识管理、会话管理、属性图存储、RDF/SPARQL 和 MCP 服务等功能。目标成为轻量级、可嵌入的 AI 基础设施。

## 技术栈

| 类别 | 技术 |
|------|------|
| 语言 | Go 1.23 |
| 存储引擎 | Badger v4 (dgraph-io/badger/v4) |
| 向量索引 | HNSW, IVF, Flat, LSH (自实现) |
| 量化 | 标量量化、乘积量化 (自实现) |
| UUID | google/uuid |
| 遥测 | OpenTelemetry (otel) |
| 测试 | Go 标准库 testing |
| MCP | 自实现 Model Context Protocol 服务器 |
| NLP | gse (中文分词 + 停用词) |

## 目录结构

```
gracedb/
├── examples/
│   ├── main.go              # 示例程序，演示所有 API 用法
│   └── embedder_mock.go     # 固定维度的 mock embedder
├── go.mod / go.sum          # Go 模块声明与依赖锁定
├── README.md                # 项目主页
├── IMPLEMENTATION_PLAN.md   # 分阶段实施文档
├── docs/superpowers/specs/  # 设计文档
└── pkg/
    ├── gracedb/             # 主入口：DB facade、Quick API、工具箱、备份、遥测
    │   ├── db.go            # DB 结构体，Open/Close，功能选项
    │   ├── vector.go        # 向量 CRUD + 搜索 (OpenTelemetry 埋点)
    │   ├── text.go          # 文本插入/搜索 API（自动向量化）
    │   ├── quick.go         # Quick 快捷接口
    │   ├── collection.go    # 集合 CRUD
    │   ├── knowledge.go     # Knowledge API
    │   ├── memory.go        # Memory API
    │   ├── session.go       # Session API
    │   ├── doc.go           # Document API + Stats
    │   ├── toolbox.go       # GraphRAG 工具集定义 (7 工具)
    │   ├── testutil.go      # 测试工具 (mockEmbedder, testDB)
    │   ├── backup.go        # 备份/恢复 API
    │   └── trace.go         # OpenTelemetry span + metrics
    ├── store/               # Badger 持久化层
    │   ├── store.go         # BadgerStore 封装（含索引管理）
    │   ├── collection.go    # Collection CRUD
    │   ├── crud.go          # Embedding CRUD
    │   ├── search.go        # 向量检索 + FTS 混合 + LoadIndex/SaveIndex
    │   ├── embedding.go     # Store 接口定义
    │   ├── fts.go           # 全文检索倒排索引
    │   ├── fuzzy.go         # 模糊匹配 (Levenshtein)
    │   ├── stemmer.go       # 词干提取
    │   ├── thesaurus.go     # 同义词
    │   ├── similarity.go    # 余弦相似度
    │   ├── chunking.go      # 文本分块
    │   ├── session.go       # 会话/消息持久化
    │   ├── knowledge.go     # 知识项持久化
    │   ├── memory.go        # Memory 存储
    │   └── reranker.go      # BM25/Cosine 重排序 + RRF 融合
    ├── index/               # 向量索引
    │   ├── types.go         # Index 接口 + SearchResult
    │   ├── hnsw.go          # HNSW 图索引
    │   ├── ivf.go           # 倒排文件索引
    │   ├── flat.go          # 暴力搜索
    │   └── lsh.go           # 局部敏感哈希
    ├── quantization/        # 向量量化
    │   ├── scalar.go        # 标量量化
    │   └── product.go       # 乘积量化
    ├── types/               # 公共类型定义
    │   ├── embedding.go     # Embedding, SearchOptions, Config
    │   ├── knowledge.go     # KnowledgeRecord, Chunk
    │   ├── memory.go        # MemoryRecord, MemorySearchRequest
    │   ├── similarity.go    # 相似度函数
    │   ├── embedder.go      # Embedder 接口
    │   ├── reranker.go      # Reranker 接口
    │   └── errors.go        # 错误类型
    ├── graph/               # 属性图存储
    │   ├── store.go         # 图节点/边 CRUD
    │   ├── types.go         # 节点/边/图类型定义
    │   └── traversal.go     # BFS/DFS/最短路径
    ├── rdf/                 # RDF 三元组存储
    │   ├── store.go         # RDF triple CRUD
    │   ├── ntriples.go      # N-Triples 导入/导出
    │   ├── sparql.go        # SPARQL SELECT/ASK
    │   ├── rdfs.go          # RDFS 推理
    │   └── shacl.go         # SHACL 约束验证
    ├── graphflow/           # GraphRAG 工作流
    │   ├── pipeline.go      # 图增强检索流水线
    │   └── extract.go       # 实体提取
    ├── memoryflow/          # 记忆管理工作流
    │   ├── types.go         # Recall 类型
    │   ├── service.go       # MemoryFlow 服务
    │   ├── strategy.go      # 检索策略
    │   └── promotion.go     # 记忆晋升
    ├── hindsight/           # 记忆回溯策略
    │   └── strategy.go      # Hindsight recall 策略
    ├── semanticrouter/      # 语义路由
    │   ├── router.go        # 路由器
    │   ├── lexical.go       # 词法路由
    │   └── hybrid.go        # 混合路由
    └── mcp/                 # MCP 协议服务器
        └── server.go        # MCP JSON-RPC server
```

## 关键命令

```bash
# 构建
go build ./...

# 运行测试
go test ./...

# 运行示例
go run examples/main.go

# 运行单包测试
go test ./pkg/store/

# 详细输出
go test -v ./pkg/index/

# 基准测试
go test -bench=. ./pkg/store/
go test -bench=. ./pkg/gracedb/

# 格式化
gofmt -w .

# 检查依赖
go mod tidy
```

## 架构设计

- **Functional Options 模式**：`Open(path, opts...)` 通过 `Option` 函数配置 DB
- **分层架构**：`gracedb`（门面层）→ `store`（持久化层）→ Badger（存储引擎）
- **Embedder 接口化**：`types.Embedder` 接口支持 OpenAI、Ollama 等后端
- **向量索引**：HNSW/IVF/Flat/LSH，通过 `Config.IndexType` 配置，Upsert 同步更新内存索引
- **索引持久化**：`LoadIndex`/`SaveIndex` 序列化索引快照到 Badger
- **备份/恢复**：基于 Badger native backup API
- **OpenTelemetry**：核心操作自动 span 创建和指标上报，未配置 exporter 时零损耗

## 编码规范

- Go 标准格式化（gofmt）
- 包级导出的类型/函数使用注释文档
- 接口定义在 `pkg/types/`，实现在对应子包
- 测试文件与源码同目录（`*_test.go`），使用标准 `testing` 包
- 错误处理：定义包级 sentinel error（`pkg/types/errors.go`）
- 配置通过 functional option 传递，不使用全局状态
- 所有测试使用标准库 testing，不依赖第三方断言库

## 注意事项

- Badger 需要调用 `Close()` 确保数据刷盘
- FTS 索引 key 格式为 `fts:<token>:<collectionID>:<embeddingID>`
- `UpsertBatch` 返回新创建的 embedding ID 列表
- Embedder 为 nil 时 `InsertText` 返回 `ErrEmbedderNotConfigured`
- `SearchText` 无 Embedder 时自动 fallback 到 FTS
- 门面层 API 接受 collection name（非 UUID），内部自动转换为 collection ID
- 所有索引为纯 Go 实现，无 C 依赖

## 提交规范

所有 git commit 必须使用**中英文双语 commit message**。

格式：`<type>: <英文摘要> / <中文摘要>`

示例：
- `feat: add KnowledgeMemory recall / 添加 KnowledgeMemory 召回功能`
- `docs: update API reference / 更新 API 参考文档`
- `fix: resolve FTS index key mismatch / 修复 FTS 索引键不匹配`
- `refactor: simplify store initialization / 简化 store 初始化`
- `test: add benchmark for group aggregation / 添加分组聚合基准测试`

类型（type）使用 conventional commits 规范：`feat`, `fix`, `docs`, `test`, `refactor`, `chore`, `perf`。
