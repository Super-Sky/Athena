# session

## 模块职责

- `internal/session` 提供 session store、waiting/resume 状态保存和相关持久化能力。

## 不负责什么

- 不负责 HTTP 接口层

## 子目录索引

- 当前无子目录

## 文件索引

- `postgres.go`
  - PostgreSQL session store 实现。
- `postgres_integration_test.go`
  - PostgreSQL session store 集成测试。
- `postgres_test.go`
  - PostgreSQL session store 单元测试。
- `session.go`
  - session 相关核心结构和抽象接口。
- `session_test.go`
  - session 基础行为测试。

## 对外入口

- session store interfaces
- postgres store implementation

## 关键依赖

- 被 `app/`、`runtime/`、`entry/` 消费

## 维护提示

- waiting / resume 语义变化时，要同步检查这里和 `runtime/` 的 preserved state 逻辑。
