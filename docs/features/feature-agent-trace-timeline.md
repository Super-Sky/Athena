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

## Known Gap / 已知缺口

The timeline API already exposes safe `detail`, but immutable run-manifest revision references are not yet guaranteed as a stable top-level contract. The UI reads revision fields when present in run metadata and otherwise marks them as not recorded. Completing the manifest persistence and contract remains part of issue `#11`.

时间线 API 已暴露安全 `detail`，但不可变 run manifest 的版本引用还没有成为稳定顶层契约。界面会读取 run metadata 中已经存在的版本字段，缺失时显示“本次未记录”。manifest 持久化与契约补齐仍属于 issue `#11`。

## Verification / 验证

```bash
go test ./internal/server -run 'TestProjectAgentTraceTimeline|TestTimelineError|TestAgentTimelineRoutesAreRegistered' -count=1
cd web && npm ci && npm run build
```

The test covers timestamp ordering, loop/model/tool/governance/usage classification, duration, safe failure detail, and both route registrations. The UI build confirms the typed client and three-pane inspector compile. Browser smoke verifies the first-level navigation and confirms no horizontal page overflow at 1280px and 390px widths.

测试覆盖时间排序、loop/model/tool/governance/usage 分类、duration、安全失败详情和两条路由注册。UI build 确认 typed client 与可展开详情面板能够编译。

## Skill Decision / Skill 结论

No feature-specific skill is added. The stable maintenance entry is this document plus `internal/server/README.md`; existing `code-style`, `doc-index-sync`, `feature-doc-skill-sync`, and `repo-task-delivery` already cover the recurring checks.

当前不新增 feature 专属 skill。本文与 `internal/server/README.md` 是稳定维护入口；既有 `code-style`、`doc-index-sync`、`feature-doc-skill-sync` 和 `repo-task-delivery` 已覆盖重复检查。
