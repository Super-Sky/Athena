# workflow

## 模块职责

- `internal/workflow` 负责结构化工作流计划与步骤契约。

## 文件索引

- `generator.go`
  - 根据任务类型、scene 和上下文生成默认 `WorkflowPlan`。
- `plan.go`
  - 定义 `WorkflowPlan`、`WorkflowStep` 及其最小校验。
- `plan_test.go`
  - 验证工作流计划契约和默认生成器行为。
