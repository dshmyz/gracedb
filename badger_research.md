# Badger 数据库存储预研报告

## 1. 概述

Badger 是由 Dgraph Labs 开发的 Go 语言嵌入式 KV 存储库（`github.com/dgraph-io/badger`），当前稳定版本为 **v4**。它采用 LSM-tree（Log-Structured Merge Tree）架构，以低内存占用和高吞吐读写为特点，广泛应用于 IPFS（`go-ds-badger`）等对性能和资源敏感的场景。

**当前状态（2025-2026）**：Dgraph Labs 的战略重心已转向 Dgraph Cloud 和其图数据库产品，Badger 处于**维护模式**——接收 bug 修复和依赖更新，但新功能开发较少。如果新项目对活跃开发有高要求，**Pebble**（CockroachDB 出品）是更活跃的替代方案。但 Badger 的 Value Log 分离架构在大值场景下仍有独特优势。

---

## 2. 核心 API

### 2.1 打开/关闭数据库

```go
import "github.com/dgraph-io/badger/v4"

// 磁盘模式
opts := badger.DefaultOptions("/path/to/db")
db, err := badger.Open(opts)

// 纯内存模式（测试/临时数据）
opts := badger.DefaultOptions("").WithInMemory(true)
db, err := badger.Open(opts)

// 关闭
defer db.Close()
```

关键 Options：
- `WithValueLogFileSize(size)`：控制 Value Log 文件大小，默认 1GB
- `WithNumMemtables(n)`：内存中 LSM 表数量，默认 5
- `WithCompression(opt)`：启用 Snappy/ZSTD 压缩
- `WithEncryptionKey(key)`：启用 AES 加密
- `WithLoggingLevel(level)`：日志级别
- `WithInMemory(true)`：纯内存模式，不写磁盘

### 2.2 写入（Set）

```go
err := db.Update(func(txn *badger.Txn) error {
    // 简单写入
    return txn.Set([]byte("key"), []byte("value"))
})

// 带 TTL
entry := badger.NewEntry([]byte("key"), []byte("value")).WithTTL(24 * time.Hour)
err := db.Update(func(txn *badger.Txn) error {
    return txn.SetEntry(entry)
})
```

### 2.3 读取（Get）

```go
err := db.View(func(txn *badger.Txn) error {
    item, err := txn.Get([]byte("key"))
    if err != nil {
        return err
    }
    // 推荐：使用回调避免额外内存分配（zero-copy）
    return item.Value(func(val []byte) error {
        fmt.Printf("key = %s\n", val)
        return nil
    })
})
```

**重要**：`Item.Value()` 回调内的 `val` 只在回调期间有效，绝不能引用到回调外部。这是 Badger 的内存复用机制决定的。

### 2.4 删除（Delete）

```go
err := db.Update(func(txn *badger.Txn) error {
    return txn.Delete([]byte("key"))
})
```

删除只是逻辑标记，物理空间在 compaction 阶段回收。可调用 `db.PurgeOlderVersions()` 主动触发清理。

### 2.5 批量写入（WriteBatch）

```go
wb := db.NewWriteBatch()
defer wb.Cancel()

for i := 0; i < 10000; i++ {
    err := wb.Set([]byte(fmt.Sprintf("key%d", i)), []byte(fmt.Sprintf("val%d", i)))
    if err != nil {
        // handle error
    }
}

err := wb.Flush()  // 阻塞直到所有数据写入完成
```

WriteBatch 相比 Transaction 的优势：
- 更高的写入吞吐（跳过了事务的版本管理开销）
- 内部自动分片刷新
- 适合大数据量初始化导入

### 2.6 流式迭代（Iterator）与范围查询

```go
err := db.View(func(txn *badger.Txn) error {
    opt := badger.DefaultIteratorOptions
    opt.Prefix = []byte("user:")        // 前缀扫描
    opt.PrefetchValues = true            // 预取值（默认 true）
    opt.PrefetchSize = 10                // 预取数量
    opt.Reverse = false                  // 反向迭代

    it := txn.NewIterator(opt)
    defer it.Close()

    // 全量扫描
    for it.Rewind(); it.Valid(); it.Next() {
        item := it.Item()
        key := item.Key()
        item.Value(func(val []byte) error {
            fmt.Printf("%s -> %s\n", key, val)
            return nil
        })
    }

    // 范围扫描：从指定 key 开始
    for it.Seek([]byte("user:1000")); it.ValidForPrefix([]byte("user:")); it.Next() {
        // ...
    }

    return nil
})
```

**范围查询能力总结**：
- `Prefix`：前缀匹配扫描
- `Seek(key)`：从指定 key 开始迭代（利用 LSM 树的有序性，跳跃到目标位置）
- `Reverse`：反向迭代
- `PrefetchValues`：关闭后可只遍历 key，适合 key 列表场景
- Badger 的迭代器必须在 `db.View` 事务内使用，且必须 `defer it.Close()` 否则会导致 LSM 表无法释放

---

## 3. LSM-tree 架构与性能特点

### 3.1 架构设计

Badger 的 LSM-tree 实现有以下核心组件：

```
┌─────────────────────────────────────────────────┐
│                   MemTable                       │  ← 内存中的跳表
├─────────────────────────────────────────────────┤
│                Level 0 (SST)                     │  ← 内存刷盘后的文件
├─────────────────────────────────────────────────┤
│              Level 1 ~ Level N (SST)             │  ← 合并后的有序文件
├─────────────────────────────────────────────────┤
│                  Value Log                       │  ← 大值存储区
└─────────────────────────────────────────────────┘
```

**关键设计决策 — Value Log 分离**：

传统 LSM-tree 将所有 KV 数据存放在 SST 文件中。Badger 将**大值（大于阈值，默认 1MB）**存储到独立的 Value Log 文件中，SST 只存 key + value 指针。这种设计带来：

- **写入放大降低**：compaction 不需要搬运大值
- **读取效率**：小值直接在 SST 中返回，大值通过指针跳转
- **空间回收**：Value Log 有独立的 GC 机制（`db.RunValueLogGC()`）

### 3.2 性能特点

| 场景 | 性能表现 | 原因 |
|------|----------|------|
| 顺序写入 | 极高 | LSM-tree 顺序写 MemTable + Value Log |
| 点查 | 高 | 先查 MemTable，再二分查找各级 SST |
| 范围扫描 | 极高 | SST 文件内 key 有序 |
| 随机写入 | 高 | 同顺序写入，LSM 无随机写 |
| 大值存储 | 优秀 | Value Log 分离避免写入放大 |
| 删除 | 中等 | 需要写入 tombstone + compaction 回收 |

**与 Pebble 对比**（2024-2025 社区共识）：
- **Pebble** 写吞吐通常更优，因为 compaction 策略更激进
- **Badger** 在大值场景下表现更好（Value Log 分离）
- **Pebble** 活跃度更高，**Badger** API 更成熟

---

## 4. 事务模型与并发控制

### 4.1 事务模型

Badger 使用 **MVCC（Multi-Version Concurrency Control）+ 乐观并发控制（Optimistic Concurrency Control, OCC）**：

```go
// 只读事务
err := db.View(func(txn *badger.Txn) error {
    // ... 读操作
    return nil
})

// 读写事务
err := db.Update(func(txn *badger.Txn) error {
    // ... 读写操作
    return nil
})
```

### 4.2 并发控制机制

- **读事务**：多个读事务可以**完全并发**执行，互不阻塞。每个读事务看到事务开始时的一致性快照。
- **写事务**：使用乐观锁。多个写事务可以并发执行，但在提交时检测冲突。如果两个事务修改了同一个 key，后提交的会失败，返回 `ErrConflict`。
- **冲突解决**：失败的事务需要由调用方重试。

```go
for {
    err := db.Update(func(txn *badger.Txn) error {
        item, err := txn.Get([]byte("counter"))
        if err != nil {
            return err
        }
        var val int
        item.Value(func(v []byte) error {
            val, _ = strconv.Atoi(string(v))
            return nil
        })
        val++
        return txn.Set([]byte("counter"), []byte(strconv.Itoa(val)))
    })
    if err == nil {
        break  // 成功
    }
    if err == badger.ErrConflict {
        continue  // 冲突，重试
    }
    return err  // 其他错误
}
```

### 4.3 事务约束

- 事务应**短小精悍**：事务持有 LSM 表引用，长时间运行的事务会阻塞 compaction 和内存回收
- `Item` 不能在事务外使用：Badger 复用内存 buffer，事务结束后 buffer 无效
- 不支持跨数据库事务：事务仅在当前 DB 实例内有效

---

## 5. Go 项目集成与最佳实践

### 5.1 依赖引入

```bash
go get github.com/dgraph-io/badger/v4
```

### 5.2 封装建议

建议在 cortexdb 中定义统一的 KV 接口层，Badger 作为后端实现之一：

```go
// 抽象接口（不与具体实现耦合）
type KVStore interface {
    Get(key []byte) ([]byte, error)
    Set(key, value []byte) error
    Delete(key []byte) error
    Iterate(prefix []byte, fn func(key, value []byte) error) error
    Batch() BatchWriter
    Close() error
}

// Badger 实现
type BadgerStore struct {
    db *badger.DB
}

func (s *BadgerStore) Get(key []byte) ([]byte, error) {
    var val []byte
    err := s.db.View(func(txn *badger.Txn) error {
        item, err := txn.Get(key)
        if err != nil {
            return err
        }
        return item.Value(func(v []byte) error {
            val = append([]byte(nil), v...)  // 拷贝到事务外安全使用
            return nil
        })
    })
    return val, err
}

func (s *BadgerStore) Set(key, value []byte) error {
    return s.db.Update(func(txn *badger.Txn) error {
        return txn.Set(key, value)
    })
}
```

### 5.3 最佳实践清单

| 类别 | 建议 |
|------|------|
| **事务粒度** | `View`/`Update` 回调内只做必要操作，避免长事务 |
| **内存管理** | 使用 `item.Value(callback)` 而非 `item.ValueCopy()`，零拷贝更高效 |
| **迭代器** | 必须 `defer it.Close()`，否则 LSM 表无法回收 |
| **批量导入** | 使用 `NewWriteBatch()` 而非 `Update` 循环，吞吐高一个数量级 |
| **大值场景** | 利用 Value Log 分离，调整 `WithValueLogFileSize` |
| **压缩** | 启用 `WithCompression(options.ZSTD)` 节省磁盘 |
| **Value Log GC** | 定期调用 `db.RunValueLogGC(0.5)` 回收废弃的 Value Log |
| **磁盘满处理** | 监控磁盘使用率，compaction 需要额外空间 |
| **备份** | 使用 `db.Backup(writer, sinceTs)` 做在线热备份 |
| **错误处理** | 写事务要处理 `badger.ErrConflict` 并重试 |

### 5.4 常见陷阱

1. **在事务外持有 `Item` 引用** → 导致数据损坏或 panic
2. **迭代器忘记 Close** → 内存泄漏，LSM 表无法释放
3. **长事务** → 阻塞 compaction，写入性能急剧下降
4. **大事务批量写入** → 用 `WriteBatch` 替代
5. **不监控 Value Log 膨胀** → 定期运行 GC，否则磁盘使用持续增长

---

## 6. 总结与建议

### Badger 作为 cortexdb 底层存储的适用性

**优势**：
- 纯 Go 嵌入式库，无外部进程依赖，部署简单
- LSM-tree 架构保证写入性能，适合写多读少的场景
- Value Log 分离对大值场景友好
- 支持 ACID 事务，MVCC 提供一致读
- 支持 TTL、压缩、加密等企业级功能
- 流式迭代 + 前缀扫描满足范围查询需求

**需关注**：
- Dgraph Labs 维护节奏放缓，社区活跃度有限
- 乐观并发控制在高冲突场景下需要重试逻辑
- Value Log 需要主动 GC，否则磁盘使用膨胀
- 与 Pebble 相比，在纯小值高吞吐场景下写性能略逊

**建议**：如果 cortexdb 的需求包含大值存储、嵌入式部署、事务一致性，Badger 是一个成熟的选择。建议先做抽象的 KV 接口层，这样后续如果替换为 Pebble 或其他实现，上层业务代码不受影响。
