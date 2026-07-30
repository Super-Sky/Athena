# memory

## 模块职责

- `internal/memory` 承载内存态上下文/记忆相关实现。

## 不负责什么

- 不负责长期知识主库

## 子目录索引

- 当前无子目录

## 文件索引

- `memory.go`
  - 实现内存态记忆与上下文相关能力。
- `memory_test.go`
  - 验证内存态记忆行为。
- `external.go`
  - 实现应用拥有的 `app_id + owner_id + scope` 作用域摘要 store 与安全操作 trace；默认实现仅用于进程内 local/demo MVP。
- `external_test.go`
  - 验证外部摘要的 owner/scope 隔离。

## 对外入口

- memory 相关类型与操作函数，包括 `ExternalStore`

## 关键依赖

- 被 `runtime/`、`app/` 间接消费

## 维护提示

- 变更 continuity 或 compaction 语义时，应同步检查这里。
- 不要把基金、持仓、交易或其他业务对象加入此模块；它只保存应用提供的通用摘要。
