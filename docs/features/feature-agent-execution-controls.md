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

## Durable Async Agent Run / 持久异步 Agent Run

When `ASYNC_JOBS_ENABLED=true`, `POST /api/agent/runs` can enter the durable path by sending `Prefer: respond-async` and an `Idempotency-Key`. The API returns `202 Accepted`, a stable app-facing `run_id` equal to the async job ID, a `Location` header, and an `events_url`.

当 `ASYNC_JOBS_ENABLED=true` 时，`POST /api/agent/runs` 可通过 `Prefer: respond-async` 与 `Idempotency-Key` 进入持久异步路径。API 返回 `202 Accepted`、稳定的应用侧 `run_id`（等于 async job ID）、`Location` header 和 `events_url`。

PostgreSQL is authoritative for job state, encrypted request/result payloads, idempotency, attempts, leases, outbox records and lifecycle events. Redis Streams only carries `outbox_id / job_id / run_id` references. Raw prompts, tool arguments, tool results and business objects are not written to Redis.

PostgreSQL 是 job 状态、加密请求/结果、幂等、尝试次数、租约、outbox 和生命周期事件的权威来源。Redis Streams 只承载 `outbox_id / job_id / run_id` 引用，不写入 raw prompt、工具参数、工具结果或业务对象。

The worker claims a Redis delivery, wins a PostgreSQL job lease, renews both PostgreSQL and Redis pending idle state while the handler runs, and ACKs Redis only after PostgreSQL reaches a stable terminal, waiting, or retry state. Lease conflicts are left pending for reclaim; already-terminal duplicates are ACKed. If a cancel request is recovered after a worker crash, `AcquireLease` finalizes the job as `cancelled` in PostgreSQL and returns `ErrAlreadyTerminal` so the reclaimed Redis delivery can be ACKed.

worker 会领取 Redis delivery、赢得 PostgreSQL job lease，并在 handler 执行期间同时续租 PostgreSQL 与 Redis pending idle 状态；只有 PostgreSQL 到达稳定终态、等待态或重试态后才 ACK Redis。lease 冲突会保留为 pending 等待回收；已终态重复消息会被 ACK。若 worker 崩溃后恢复到已请求取消的 job，`AcquireLease` 会在 PostgreSQL 中落 `cancelled` 终态并返回 `ErrAlreadyTerminal`，使重领的 Redis delivery 可被 ACK。

`GET /api/agent/runs/:runID/events` streams the PostgreSQL event cursor as resumable SSE. Clients may resume with `Last-Event-ID` or `?after=<cursor>`. Event names include `created`, `queued`, `running`, `waiting`, `resumed`, `cancel_requested`, `cancelled`, `completed`, `failed`, and `retry_scheduled`.

`GET /api/agent/runs/:runID/events` 会把 PostgreSQL event cursor 作为可续传 SSE 输出。客户端可通过 `Last-Event-ID` 或 `?after=<cursor>` 恢复读取。事件名包括 `created`、`queued`、`running`、`waiting`、`resumed`、`cancel_requested`、`cancelled`、`completed`、`failed` 和 `retry_scheduled`。

The `worker` process command starts the durable outbox dispatcher and Redis worker. Docker Compose now includes a Redis service and an `athena-worker` service that shares the same image/config as the API service.

`worker` 进程命令会启动持久 outbox dispatcher 与 Redis worker。Docker Compose 现在包含 Redis 服务和 `athena-worker` 服务，worker 与 API 服务共用同一镜像和配置。

## Boundaries / 边界

- This slice does not use fixed-round termination as completion semantics.
- This slice does not claim independently verified success-criteria evaluation.
- Redis-backed enqueue, idempotency, retry/backoff, asynchronous cancellation, resumable event streaming, and Docker worker wiring are implemented in this slice.
- Checkpoint execution is reused through the existing runtime path; full graph-native checkpoint resurrection remains a later runtime slice.
- Independent evaluator/critic success verification remains a later #22 slice.
- No application or fund business state enters Athena core.

- 本切片不把固定轮数停止作为完成语义。
- 本切片尚不声明成功条件已经过独立 evaluator 验证。
- Redis-backed enqueue、幂等、retry/backoff、异步取消、可续传事件流和 Docker worker 接线已在本切片实现。
- checkpoint 执行复用现有 runtime 路径；完整 graph-native checkpoint resurrection 仍留给后续 runtime 切片。
- 独立 evaluator/critic 成功验证仍留在 #22 后续切片。
- 不向 Athena core 引入应用或基金业务状态。

## Verification / 验证

`execution_control_test.go` covers strict budget parsing, deadline precedence, task contract extraction, model/tool/token enforcement, fail-closed token accounting, and checkpoint-resumed counters through the real Eino Runner. `execution_stop_reason_test.go` covers every public reason and rejects unknown explicit values. Eino graph tests verify persisted terminal lifecycle records use the same normalized reason. Agent Run tests verify request validation and authoritative persisted reasons. `internal/asyncjob` tests cover encrypted payloads, PostgreSQL idempotency/outbox/lease/retry/wait/resume/cancel/event cursor behavior, Redis reference-only delivery, pending reclaim/renewal, worker ACK behavior, and the HTTP dispatcher/worker/read path.

`execution_control_test.go` 覆盖预算解析、deadline 优先级、task 契约提取及模型/工具/token 执行边界。`execution_stop_reason_test.go` 覆盖全部公开原因并拒绝未知显式值。Eino graph 测试验证持久化终态 lifecycle 使用同一规范原因；Agent Run 测试验证请求校验和权威持久化原因。`internal/asyncjob` 测试覆盖加密 payload、PostgreSQL 幂等/outbox/lease/retry/wait/resume/cancel/event cursor、Redis 只投递引用、pending 回收/续租、worker ACK 行为，以及 HTTP dispatcher/worker/read 链路。

Current verification commands:

- `env -u APP_ENV go test ./...`
- `env -u APP_ENV go test ./internal/asyncjob -count=1`
- `ATHENA_ASYNC_JOB_PG_TEST_DSN='...' env -u APP_ENV go test ./internal/asyncjob -run 'Test(PostgresStoreJobOutboxLeaseRetryAndCancel|DispatcherAndWorkerCompleteDurableJob|PostgresStoreWaitingResumeAndFailureEvents|PostgresStoreAcquireLeaseFinalizesRecoveredCancel)' -count=1`
- `ATHENA_ASYNC_JOB_REDIS_TEST_URL=redis://127.0.0.1:56379/0 env -u APP_ENV go test ./internal/asyncjob -run 'TestRedisDelivery(ReclaimsAbandonedPendingMessage|RenewKeepsPendingMessageWithCurrentWorker)' -count=1`
- `ATHENA_ASYNC_JOB_PG_TEST_DSN='...' env -u APP_ENV go test ./internal/server -run TestAsyncAgentRunHTTPDispatcherWorkerAndRead -count=1`
- `docker compose -f deploy/docker-compose.cloud.yml --env-file deploy/athena.env.example config --no-interpolate`

## Skill Decision / Skill 结论

No feature-specific skill is added. The runtime code, this feature document, and existing `eino-agent`, `code-style`, and `repo-task-delivery` skills provide the stable maintenance path.

当前不新增 feature 专属 skill。runtime 代码、本文以及既有 `eino-agent`、`code-style`、`repo-task-delivery` 已构成稳定维护入口。
