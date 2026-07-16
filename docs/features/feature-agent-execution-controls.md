# Agent Execution Controls / Agent 执行控制

## Background / 背景

Issue `Super-Sky/Athena#22` turns Agent Run termination into a stable runtime contract. Business applications must not infer completion from internal lifecycle strings or a fixed loop count.

Issue `Super-Sky/Athena#22` 将 Agent Run 终止语义收敛为稳定 runtime 契约。业务应用不能依赖内部 lifecycle 字符串或固定 loop 轮数推断完成状态。

## Stop Reason Contract / 停止原因契约

`ExecutionStopReason` contains exactly:

- `success`
- `budget_exhausted`
- `deadline_exceeded`
- `awaiting_input`
- `awaiting_external_data`
- `governance_denied`
- `cancelled`
- `unrecoverable_error`

`internal/runtime/execution_stop_reason.go` normalizes request status, terminal status, cancellation, deadline errors, and explicit governed outcomes. Unknown internal error text is never promoted into the public reason.

`internal/runtime/execution_stop_reason.go` 统一规范 request status、terminal status、取消、deadline 错误和显式治理结果。未知内部错误文本不会直接成为公开停止原因。

The terminal projector writes the normalized reason to both run and step lifecycle records. Agent Run responses treat the run-level `run_terminal_observed` event as the authoritative terminal fact, so synthetic baseline events and step-only outcomes cannot override the API result.

terminal projector 会把规范化原因同时写入 run 与 step lifecycle。Agent Run response 只把 run 级 `run_terminal_observed` 视为权威终态，因此基础合成事件和 step 局部结果不能覆盖 API 结果。

## Boundaries / 边界

- This slice does not implement fixed-round termination.
- This slice does not claim that request budgets are enforced yet.
- Redis queue, idempotency locks, retry/backoff, asynchronous cancellation, and checkpoint dispatch remain later #22 slices.
- No application or fund business state enters Athena core.

- 本切片不实现固定轮数终止。
- 本切片尚不声明请求预算已经执行。
- Redis queue、幂等锁、retry/backoff、异步取消和 checkpoint dispatch 留在 #22 后续切片。
- 不向 Athena core 引入应用或基金业务状态。

## Verification / 验证

`execution_stop_reason_test.go` covers every public reason and rejects unknown explicit values. Eino graph tests verify persisted terminal lifecycle records use the same normalized reason. Agent Run tests verify the HTTP response consumes that persisted reason.

`execution_stop_reason_test.go` 覆盖全部公开原因并拒绝未知显式值。Eino graph 测试验证持久化终态 lifecycle 使用同一规范原因；Agent Run 测试验证 HTTP response 消费该持久化原因。

## Skill Decision / Skill 结论

No feature-specific skill is added. The runtime code, this feature document, and existing `eino-agent`, `code-style`, and `repo-task-delivery` skills provide the stable maintenance path.

当前不新增 feature 专属 skill。runtime 代码、本文以及既有 `eino-agent`、`code-style`、`repo-task-delivery` 已构成稳定维护入口。
