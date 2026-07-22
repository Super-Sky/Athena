# Agent Trace Timeline / Agent 追踪时间线

## Background / 背景

Issue `Super-Sky/Athena#11` makes an Athena Agent Run debuggable as one ordered timeline for business applications and the Control Plane. A fund assistant can therefore show which model/tool/governance/data-provider path contributed to a decision without moving portfolio or trade truth into Athena.

Issue `Super-Sky/Athena#11` 将 Athena Agent Run 组织成业务应用和 Control Plane 都可读取的一条有序时间线。基金助手因而可以展示模型、工具、治理和数据 provider 调用如何参与决策，而无需把持仓或交易真相迁入 Athena。

## Contract / 契约

- `GET /api/agent/runs/:runID/timeline` is the app-facing read endpoint.
- `GET /api/control-plane/runtime/runs/:runID/timeline` is the authenticated admin read endpoint.
- Each item has stable `id`, `kind`, `timestamp`, optional `duration_ms`, `status`, `source`, `summary`, optional `step_id`, failure metadata, and safe detail.

两条端点都返回同一种投影。`kind` 包含 `loop_step`、`lifecycle`、`model_call`、`tool_call`、`governance`、`context`、`usage`、`delivery` 或通用 `trace`。详情只包含已持久化的 safe labels、redacted payload 和 metadata。

## Implementation / 实现

`internal/server/trace_timeline.go` calls the existing Agent Run trace read boundary, projects its records in memory, and stable-sorts them by source timestamp then ID. No new table, writer, or business audit store is introduced. Step duration is calculated only when both persisted start/end times are valid.

`internal/server/trace_timeline.go` 调用既有 Agent Run trace 读取边界，在内存中投影记录，并按源时间戳、再按 ID 稳定排序。不引入新表、writer 或业务审计库。只有持久化起止时间都有效时才计算 step duration。

The Control Plane exposes `运行观测` as a first-level navigation view. It combines a run browser, ordered timeline, safe step inspector, and summary counters for elapsed time, model calls, skill/tool calls, usage, and failures. The inspector separates allowlisted request, response, impact/state, performance, and run-manifest fields; absent persisted fields render as not recorded instead of being inferred.

Control Plane 将 `运行观测` 提升为一级导航页，组合 run 列表、有序时间线、安全步骤检查器，以及总耗时、模型调用、skill/tool、usage 和失败计数。检查器分开展示白名单请求、返回、影响/状态、性能与 run manifest；未持久化字段明确显示为未记录，不由前端推断。可见 detail 继续排除原始模型隐式推理、凭据和未脱敏业务载荷。

The operator shell groups navigation into runtime, build, governance, and development domains. The observability surface uses a stable three-pane debugger layout, a compact summary strip, explicit source/duration/trace identifiers, and an in-place refresh control. Lucide icons provide consistent visual recognition without changing route or API behavior.

运维壳层把导航收敛为运行、构建、治理和开发四组。运行观测采用稳定三栏调试器布局、紧凑摘要栏、明确的 source/duration/trace 标识和原地刷新入口。Lucide 图标只用于提升识别效率，不改变路由或 API 行为。

Model callback entries expose only persisted safe summaries: input/output counts, tool-call count, callback status, duration, token usage, and a redacted error summary when the callback fails. Raw prompts, model responses, credentials, provider headers, and hidden reasoning never enter the timeline contract.

模型回调条目只展示已持久化的安全摘要：输入/输出计数、tool call 数量、回调状态、耗时、Token 用量，以及失败时的脱敏错误摘要。原始 Prompt、模型返回、凭据、provider header 和隐式推理都不会进入 timeline 契约。

## Immutable Run Manifest / 不可变运行清单

At the Eino persistence node, Athena freezes `agent_run_manifest.v1` together with the newly created `TaskRun`. It records the executed model/provider, final assembled system-instruction digest, resolved skill and tool-schema digests, model/runtime governance references, context-asset references, evaluator references when used, system-truth references, and the runtime-contract revision. The manifest stores safe IDs, versions, sources, and SHA-256 values only; it never stores raw prompts, tool arguments/results, provider headers, context content, or policy bodies.

Athena 在 Eino persistence node 创建 `TaskRun` 时同步冻结 `agent_run_manifest.v1`。清单记录实际执行的 model/provider、最终 system instruction 摘要、已解析 skill 与 tool schema 摘要、模型和 runtime 治理引用、context asset 引用、实际使用的 evaluator 引用、system truth 引用和 runtime contract 版本。清单只保存安全 ID、版本、来源与 SHA-256，不保存 Prompt 原文、tool 参数/结果、provider header、context 内容或策略正文。

The executor refreshes tool refs from the same dynamic catalog snapshot and canonical schemas used to bind the model, so a registry hot update cannot leave resolver-time schemas in the persisted manifest. Model digests include the actual endpoint identity and resolved parameters without exposing them. Context refs merge resolved and still-requested assets by asset ID, while completeness checks mark expected but unavailable tool/context/governance/contract revisions as `partial`.

执行器会使用绑定模型时同一份动态 catalog 快照与 canonical schema 刷新 tool refs，避免 registry 热更新后仍保存 resolver 阶段旧 schema。Model digest 会纳入实际 endpoint identity 与 resolved parameters，但不展示原值。Context refs 按 asset ID 合并已解析和仍待解析资产；预期存在但缺失的 tool/context/governance/contract 版本会把清单标记为 `partial`。

The timeline response exposes the manifest once at top level with `manifest_status`. Historical runs remain readable with `legacy_unavailable`; the server never reconstructs historical revisions from current configuration. The UI consumes only this typed top-level contract and no longer guesses revision keys from run metadata.

timeline 响应只在顶层返回一次清单，并同时返回 `manifest_status`。历史 run 保持可读并标记为 `legacy_unavailable`；服务端不会用当前配置反推历史版本。后台只消费这份 typed 顶层契约，不再从 run metadata 猜测版本字段。

## Known Gap / 已知缺口

The app-facing Agent Run read endpoints still need an authenticated app identity and `(run_id, workspace_id, app_instance_id)` authorization boundary. Until that gate lands, production exposure of app-facing trace endpoints must be disabled or protected by an upstream gateway. Control Plane authentication remains required for the admin endpoint.

面向应用的 Agent Run 读取端点仍需补齐 app identity，以及 `(run_id, workspace_id, app_instance_id)` 授权边界。在该门禁落地前，生产环境必须关闭 app-facing trace 端点或由上游网关保护；后台端点继续强制 Control Plane 登录认证。

## Verification / 验证

```bash
go test ./internal/runtime ./internal/server -count=1
cd web && npm ci && npm run build
```

Tests cover deterministic/redaction-safe manifest hashing, Eino persistence capture, legacy compatibility, timestamp ordering, classification, callback status/duration, safe model summaries, safe failure detail, and route registration. The UI build confirms the typed client and three-pane inspector compile. Browser smoke with a seven-step trace fixture verifies the grouped navigation, request/response/impact/performance inspector, and no horizontal page overflow at 1440px and 390px widths.

The 100-tool manifest benchmark on the local Intel development machine measured 177-229 us/op, about 27.1 KB/op, and 41 allocations/op across three isolated runs. Manifest construction is one write-time operation and performs no per-reference database reads.

本地 Intel 开发机上的 100-tool manifest benchmark 三轮隔离结果为 177-229 us/op、约 27.1 KB/op、41 allocs/op。清单只在写入时构建一次，不会对每条引用执行数据库读取。

测试覆盖时间排序、loop/model/tool/governance/usage 分类、duration、安全失败详情和两条路由注册。UI build 确认 typed client 与三栏详情面板能够编译；包含七步 trace fixture 的 Browser smoke 已在 1440px 与 390px 验证分组导航、请求/返回/影响/性能详情和无横向页面溢出。

## Skill Decision / Skill 结论

No feature-specific skill is added. The stable maintenance entry is this document plus `internal/server/README.md`; existing `code-style`, `doc-index-sync`, `feature-doc-skill-sync`, and `repo-task-delivery` already cover the recurring checks.

当前不新增 feature 专属 skill。本文与 `internal/server/README.md` 是稳定维护入口；既有 `code-style`、`doc-index-sync`、`feature-doc-skill-sync` 和 `repo-task-delivery` 已覆盖重复检查。
