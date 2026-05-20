# observability

## 模块职责

- `internal/observability` 提供 trace、metrics、audit 的统一入口。

## 不负责什么

- 不负责具体业务判断

## 子目录索引

- 当前无子目录

## 文件索引

- `observability.go`
  - 定义可观测能力的统一装配和使用入口。
- `observability_test.go`
  - 验证可观测能力装配行为。

## 对外入口

- observability 装配入口

## 关键依赖

- 被 `entry/`、`app/`、`runtime/` 间接消费

## 维护提示

- 新 trace/audit 点接入时，优先检查这里是否已有统一入口。
