# store

## 模块职责

- `internal/runtime/store` 负责统一的 runtime store 边界，承载事件、计数和可重建索引的最小契约。

## 文件索引

- `store.go`
  - 定义 runtime store 统一接口以及事件、计数、索引结构。
- `store_test.go`
  - 验证 runtime store 基础结构最小行为。
