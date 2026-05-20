# postgresutil

## 模块职责

- `internal/postgresutil` 提供 PostgreSQL 相关的底层通用辅助能力。

## 不负责什么

- 不负责 session/model 的完整业务存储逻辑

## 子目录索引

- 当前无子目录

## 文件索引

- `retry.go`
  - 提供 PostgreSQL 相关重试辅助逻辑。

## 对外入口

- retry helper

## 关键依赖

- 被 `session/`、`model/` 间接消费

## 维护提示

- 数据库重试语义调整时，优先检查这里的通用边界是否还成立。
