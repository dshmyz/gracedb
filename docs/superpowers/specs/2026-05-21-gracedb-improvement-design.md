---
name: gracedb 全链路完善设计
description: 按优先级四层（P0-P3）系统性完善 gracedb 嵌入式向量数据库的设计文档
type: project
---

# gracedb 全链路完善设计

> 2026-05-21
> 目标：按 "质量 → 运维 → 文档 → 功能" 四层优先级，系统性完善 gracedb 项目。

## 项目现状

- **代码规模**: 59 源文件 / ~10,352 行 Go / 12 包 / 19 测试文件
- **模块路径**: `github.com/dshmyz/gracedb` (已从 qdb 更名)
- **已实现**: 向量索引(HNSW/IVF/Flat/LSH)、量化、知识存储、Agent Memory、属性图、RDF/SPARQL/SHACL、GraphFlow、MemoryFlow、语义路由、MCP Server、RRF 混合检索、高级过滤

## 核心差距

| 等级 | 差距 | 影响 |
|------|------|------|
| P0 | gracedb 门面层 0 测试 (10 文件) | 质量无保障 |
| P0 | MCP/RDF 测试不足 | 协议兼容性和正确性无保证 |
| P0 | vectorSearch 仅走 flat scan，HNSW/IVF/LSH 未接入 | 性能严重劣化 |
| P0 | LoadIndex/SaveIndex 空实现 | 索引持久化缺失 |
| P1 | 无备份/恢复 API | 运维风险 |
| P1 | OpenTelemetry 导入但未使用 | 可观测性缺失 |
| P2 | CLAUDE.md 引用旧 pkg/qdb 结构 | 开发者误导 |
| P2 | 无 README / 无示例程序 | 新用户无法上手 |
| P2 | 无基准测试 | 性能无量化 |
| P3 | 多索引未集成 search pipeline | 混合检索不完整 |
| P3 | 无聚合查询 | 功能缺口 |
| P3 | 无地理空间搜索 | 功能缺口 |

---

## P0 — 质量根基

### 1. 门面层测试

#### `pkg/gracedb/db_test.go`
- `Open` 创建/打开/关闭生命周期
- Functional options 组合 (WithPath/WithEmbedder/WithIndexType)
- `Close` 后操作 → 返回错误
- `Vector()` / `Graph()` / `RDF()` 返回非 nil

#### `pkg/gracedb/vector_test.go`
- `Upsert` 单条 → 返回 embID → `GetEmbedding` 验证
- `UpsertBatch` 批量 → 返回 embID 列表 → 数量匹配
- `Search` 向量搜索 → 已知向量 → topK 正确
- `SearchFTS` / `SearchFTSWithContent`
- `DeleteEmbedding` / `DeleteByDocID` / `DeleteBatch`
- `RebuildIndex` → FTS 重建去重
- `EmbeddingCount` / `ListEmbeddingIDs`
- Metadata 过滤 / ACL 过滤 / 边界条件

#### `pkg/gracedb/text_test.go`
- `InsertText` → 有 Embedder → 自动向量化
- `InsertText` → 无 Embedder → 返回 `ErrEmbedderNotConfigured`
- `InsertTextBatch` → 含空文本 → 跳过空项
- `SearchText` → 有 Embedder → 向量化查询
- `SearchText` → 无 Embedder → FTS fallback

#### `pkg/gracedb/quick_test.go`
- `Quick().Add` / `Search` / `AddText` / `SearchText` / `SearchTextOnly`
- 集合为空时 Search 返回空切片
- 跨集合隔离验证

#### `pkg/gracedb/toolbox_test.go` (244 行，最重)
- 7 个工具 handler 全部测试
- `callSearchKnowledge` → 空 query → 错误 / 正常 query → 结果
- `callSaveKnowledge` → 完整保存流程
- `callSearchMemory` / `callSaveMemory`
- `callExpandGraph` → BFS 展开
- `callRecallKnowledgeMemory` → 知识+记忆融合
- `callBuildContext` → 截断逻辑验证
- `Definitions()` → 返回 7 个工具定义

### 2. MCP Server 测试

#### `pkg/mcp/server_test.go`
- `handleInitialize` → 返回协议版本 + serverInfo
- `handleToolsList` → 返回已注册工具列表
- `handleToolCall` → 成功 / 未知工具 / handler 错误
- `RunStdio` → 用 pipe 模拟 stdin/stdout roundtrip
- `FromToolbox` → 端到端完整流程

### 3. RDF Store 测试

#### `pkg/rdf/rdf_test.go` (补充)
- SPARQL 查询解析正确性
- SHACL 约束验证
- N-Triples 序列化/反序列化
- RDFS 推理

### 4. 测试工具

#### `pkg/gracedb/testutil.go`
```go
// mockEmbedder: 返回固定维度向量，无外部 API 调用
func newMockEmbedder(dim int) *mockEmbedder

// testDB(t): 返回临时目录的 DB，t.Cleanup 自动关闭
func testDB(t *testing.T, opts ...Option) *DB

// seedCollection(t, db): 插入已知测试数据
func seedCollection(t *testing.T, db *DB, name string, count int)
```

### 5. HNSW/IVF/LSH 索引集成

#### 问题
`pkg/store/search.go` 的 `vectorSearch` 直接调用 `ReadVectors()` 全量暴力扫描，完全绕过了 `pkg/index/` 下已实现的 HNSW/IVF/LSH 索引。

#### 改动文件
- `pkg/store/store.go` — BadgerStore 增加索引字段
- `pkg/store/search.go` — vectorSearch 走索引路径
- `pkg/index/types.go` — 补充 Index 接口
- 各索引实现 (`hnsw.go`, `ivf.go`, `flat.go`, `lsh.go`) — 实现接口

#### 设计
```go
// pkg/index/types.go — 补充 Index 接口
type Index interface {
    AddVector(id string, vec []float32)
    RemoveVector(id string)
    Search(query []float32, topK int) []SearchResult
    Marshal() ([]byte, error)
    Unmarshal(data []byte) error
    Size() int
    Dimension() int
}

// pkg/store/store.go — BadgerStore 扩展
type BadgerStore struct {
    // ... 现有字段
    indexes map[string]index.Index // collectionID → 索引实例
    idxType string                  // "hnsw" / "ivf" / "flat" / "lsh"
    idxCfg    index.Config          // 索引构造参数
}

// Upsert 时同步索引
func (s *BadgerStore) Upsert(...) (string, error) {
    // ... 现有存储逻辑
    embID, vector, collectionID
    if idx, ok := s.indexes[collectionID]; ok {
        idx.AddVector(embID, vector)
    }
}

// vectorSearch 走索引 → 无索引时 fallback flat scan
func (s *BadgerStore) vectorSearch(collectionID string, query []float32, opts types.SearchOptions) ([]types.ScoredEmbedding, error) {
    if idx := s.indexes[collectionID]; idx != nil {
        results := idx.Search(query, opts.TopK)
        // 转 ScoredEmbedding，走 metadata/ACL 过滤 (复用现有逻辑)
        return results, nil
    }
    // Fallback: 现有 flat scan 逻辑
}

// LoadIndex → 从 Badger 读向量 → 构建索引 → 尝试加载快照
func (s *BadgerStore) LoadIndex(collectionName string) error {
    coll := GetCollection(collectionName)
    idx := index.New(s.idxType, coll.VectorDimension, s.idxCfg)
    // 尝试加载快照
    if data, err := loadIndexSnapshot(coll.ID); err == nil {
        idx.Unmarshal(data)
    } else {
        // 从零构建: 读全部向量 → AddVector
        for id, vec := range s.ReadVectors(coll.ID) {
            idx.AddVector(id, vec)
        }
    }
    s.indexes[coll.ID] = idx
}

// SaveIndex → 序列化索引快照到 Badger
func (s *BadgerStore) SaveIndex(collectionName string) error {
    coll := GetCollection(collectionName)
    idx := s.indexes[coll.ID]
    data, err := idx.Marshal()
    // 写入 Badger key: "idx:<collID>:snapshot"
}
```

---

## P1 — 运维能力

### 6. 备份/恢复

#### `pkg/gracedb/backup.go`

```go
// Backup creates a full database backup to the given path.
func (db *DB) Backup(path string) error

// Restore restores the database from a backup at the given path.
func (db *DB) Restore(path string) error
```

**实现要点**:
- 使用 Badger native `Backup()` / `Load()` API
- 备份前 flush memtable: `db.store_.DB().Sync()`
- 恢复流程: 关闭当前 DB → 从备份文件 Load → 重建索引
- 恢复后需调用 `RebuildIndex` (索引不持久化到备份中)

### 7. OpenTelemetry 接入

#### `pkg/gracedb/trace.go`

**约定**: 使用 `otel.Tracer("gracedb")` 创建 span。未配置 exporter 时 noop tracer 自动短路，零性能损耗。

**覆盖操作**:
- Upsert / UpsertBatch → span: `gracedb.Upsert` (属性: collection)
- Search → span: `gracedb.Search` (属性: topK, useVector, useFTS)
- Delete 系列 → span: `gracedb.Delete` (属性: collection, id)
- 指标: `gracedb.search.duration` (histogram), `gracedb.upsert.count` (counter)

---

## P2 — 文档与体验

### 8. 重写文档

#### `CLAUDE.md`
- 修正: `pkg/qdb` → `pkg/gracedb`
- 模块名: `github.com/dshmyz/gracedb`
- 新增包: `graphflow`, `memoryflow`, `hindsight`, `semanticrouter`, `rdf`, `quantization`
- 更新目录结构

#### `IMPLEMENTATION_PLAN.md`
- 更新覆盖度真实评估
- 标记新增包状态 (hindsight, graphflow 等)
- 记录 P0-P3 补全计划为下一阶段

#### `README.md` (新建)
- 项目简介 + 特性列表
- 快速开始示例代码
- 架构概览
- 链接到 examples/

### 9. 示例程序

#### `examples/main.go`
演示完整工作流:
1. Open → 创建 DB
2. CreateCollection → 创建集合
3. Upsert → 插入向量
4. InsertText → 插入文本
5. Search / SearchText → 检索
6. SaveKnowledge → 知识存储
7. SaveMemory → Agent 记忆
8. 属性图操作
9. Quick 接口
10. Close

#### `examples/embedder_mock.go`
- 固定维度的 mock embedder，无需外部 API

### 10. 基准测试

#### `pkg/store/bench_test.go` (扩展)
- `BenchmarkUpsert100` / `BenchmarkUpsert1000` / `BenchmarkUpsert10000`
- `BenchmarkSearchFlat` / `BenchmarkSearchHNSW` / `BenchmarkSearchIVF`
- `BenchmarkFTS`

#### `pkg/gracedb/bench_test.go` (新建)
- 门面层端到端基准
- `BenchmarkToolboxCall`
- `BenchmarkQuickSearch`

---

## P3 — 功能完善

### 11. 多索引集成 Search Pipeline

**状态**: `pkg/index/multi.go` 存在但未接入 `BadgerStore.Search`。

**设计**: 在 `SearchOptions` 中增加 `UseMultiIndex bool`，当开启时并行查询多个索引 → 结果合并 → RRF 融合 (复用现有 `rrfFusion`)。

### 12. 聚合查询

#### `pkg/store/aggregation.go` (新建)

```go
type AggregationType string // count, sum, avg, min, max

func (s *BadgerStore) Aggregate(collectionID, metadataKey string, aggType AggregationType) (*types.AggregationResult, error)
```

**实现**: Badger prefix scan → 读取 metadata → 提取字段值 → 计算聚合。可复用 `vectorSearch` 的过滤逻辑先筛后算。

### 13. 地理空间搜索 (可选)

#### `pkg/store/geosearch.go` (新建)

**设计**: SearchOptions 增加 `GeoRadius *GeoQuery`。metadata 中 lat/lon 字段 → GeoHash 索引 → 查询时前缀匹配 → 精确距离过滤。

---

## 实施顺序

| 步骤 | 层级 | 内容 | 状态 |
|------|------|------|------|
| 1 | P0 | 测试工具 + db_test + vector_test | ✅ 完成 |
| 2 | P0 | text_test + quick_test + toolbox_test | ✅ 完成 |
| 3 | P0 | mcp/server_test + rdf 测试补充 | ✅ 完成 |
| 4 | P0 | Index 接口补充 + 各索引实现 | ✅ 完成 |
| 5 | P0 | vectorSearch 集成索引 + LoadIndex/SaveIndex | ✅ 完成 |
| 6 | P1 | 备份/恢复 API | ✅ 完成 |
| 7 | P1 | OpenTelemetry 接入 | ✅ 完成 |
| 8 | P2 | 重写文档 + README + examples | ✅ 完成 |
| 9 | P2 | 基准测试 | 待做 |
| 10 | P3 | 聚合查询 + 多索引 + 地理空间(可选) | 待做 |

## 验收标准

- [ ] 所有新增测试通过 (`go test ./...` 绿色)
- [ ] gracedb 门面层测试覆盖率 > 70%
- [ ] vectorSearch 走索引路径，性能优于 flat scan 3x+ (1000+ 向量)
- [ ] `go build ./...` 零警告
- [ ] 示例程序可运行 (`go run examples/main.go`)
- [ ] README 包含完整快速开始
- [ ] 基准测试可运行并输出结果
