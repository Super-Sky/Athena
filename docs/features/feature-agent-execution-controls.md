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

## Goal And Budget Controls / 目标与预算控制

Agent Run accepts `success_criteria` plus the following optional hard limits under `budget`: `max_duration_ms`, `deadline_at`, `max_model_calls`, `max_tool_calls`, and `max_tokens`. Limits are validated before execution, and unknown budget fields are rejected. Duration covers runtime preparation and model/tool execution; when duration and absolute deadline are both present, the earlier deadline wins.

Agent Run 通过 `success_criteria` 接收成功条件，并在 `budget` 下支持 `max_duration_ms`、`deadline_at`、`max_model_calls`、`max_tool_calls` 与 `max_tokens` 五个可选硬限制。所有限制均在执行前校验，未知 budget 字段会被拒绝；duration 覆盖 runtime 准备与模型/工具执行，若 duration 与绝对 deadline 同时存在，则以更早者为准。

The Eino graph counts model calls before provider invocation and reserves each tool-call batch before ToolsNode execution. Token usage is enforced after each model response because provider usage is only authoritative at that point; when `max_tokens` is configured, missing provider usage fails closed as an unrecoverable provider-contract error. Exceeding a reported call/token limit produces `budget_exhausted`; context deadline produces `deadline_exceeded`. The graph's default 64-step ceiling is a fail-safe, not goal-completion semantics, and explicit call budgets derive a tighter ceiling.

Eino graph 会在 provider 调用前计入模型调用，并在 ToolsNode 执行前整体预留一批工具调用。Token usage 只能在每次模型返回权威 usage 后执行，因此按单次模型调用粒度停止；配置 `max_tokens` 后若 provider 缺失 usage，会以不可恢复的 provider 契约错误 fail closed。已报告调用/token 超限映射为 `budget_exhausted`，context deadline 映射为 `deadline_exceeded`。默认 64 step 仅是防失控护栏，不代表目标完成；显式调用预算会导出更紧的上限。

Success criteria are currently injected into the frozen execution guidance with `success_evaluation_mode=prompt_guard_only`. This prevents a fixed-round loop from being treated as completion, but it is not an independent evaluator. A later slice must add evaluator/critic evidence before Athena can claim verified goal satisfaction.

当前成功条件会写入冻结的 execution guidance，并明确标记 `success_evaluation_mode=prompt_guard_only`。它避免把固定轮数误当成完成，但不等同于独立 evaluator；Athena 只有在后续切片补齐 evaluator/critic 证据后，才能声明目标已经经过验证地满足。

## Boundaries / 边界

- This slice does not use fixed-round termination as completion semantics.
- This slice does not claim independently verified success-criteria evaluation.
- Redis queue, idempotency locks, retry/backoff, asynchronous cancellation, and checkpoint dispatch remain later #22 slices.
- No application or fund business state enters Athena core.

- 本切片不把固定轮数停止作为完成语义。
- 本切片尚不声明成功条件已经过独立 evaluator 验证。
- Redis queue、幂等锁、retry/backoff、异步取消和 checkpoint dispatch 留在 #22 后续切片。
- 不向 Athena core 引入应用或基金业务状态。

## Verification / 验证

`execution_control_test.go` covers strict budget parsing, deadline precedence, task contract extraction, model/tool/token enforcement, fail-closed token accounting, and checkpoint-resumed counters through the real Eino Runner. `execution_stop_reason_test.go` covers every public reason and rejects unknown explicit values. Eino graph tests verify persisted terminal lifecycle records use the same normalized reason. Agent Run tests verify request validation and authoritative persisted reasons.

`execution_control_test.go` 覆盖预算解析、deadline 优先级、task 契约提取及模型/工具/token 执行边界。`execution_stop_reason_test.go` 覆盖全部公开原因并拒绝未知显式值。Eino graph 测试验证持久化终态 lifecycle 使用同一规范原因；Agent Run 测试验证请求校验和权威持久化原因。

## Skill Decision / Skill 结论

No feature-specific skill is added. The runtime code, this feature document, and existing `eino-agent`, `code-style`, and `repo-task-delivery` skills provide the stable maintenance path.

当前不新增 feature 专属 skill。runtime 代码、本文以及既有 `eino-agent`、`code-style`、`repo-task-delivery` 已构成稳定维护入口。
