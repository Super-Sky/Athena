# task

## 模块职责

- `internal/runtime/task` 承载 internal task 模型和 task normalization 相关能力。

## 不负责什么

- 不负责完整 runtime 主链

## 子目录索引

- 当前无子目录

## 文件索引

- `task.go`
  - 定义 internal task 结构和相关处理逻辑。
- `task_test.go`
  - 验证 task normalization 和 task 结构行为。

## 对外入口

- internal task types
- normalization helpers

## 关键依赖

- 被 `app/task.go` 和 `runtime/` 消费

## 维护提示

- 新任务类型扩展时，优先先看这里是否需要调整 task 结构。
