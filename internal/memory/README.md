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

## 对外入口

- memory 相关类型与操作函数

## 关键依赖

- 被 `runtime/`、`app/` 间接消费

## 维护提示

- 变更 continuity 或 compaction 语义时，应同步检查这里。
