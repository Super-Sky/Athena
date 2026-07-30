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

The Control Plane console fetches the protected endpoint and renders expandable rows. The visible detail intentionally excludes raw model prompts, raw tool arguments/results, credentials, and business-domain payloads.

Control Plane 控制台读取受保护端点并渲染可展开行。可见 detail 有意排除原始模型 prompt、原始工具参数/结果、凭据和业务领域载荷。

## Verification / 验证

```bash
go test ./internal/server -run 'TestProjectAgentTraceTimeline|TestTimelineError|TestAgentTimelineRoutesAreRegistered' -count=1
cd web && npm ci && npm run build
```

The test covers timestamp ordering, loop/model/tool/governance/usage classification, duration, safe failure detail, and both route registrations. The UI build confirms the typed client and expandable detail panel compile.

测试覆盖时间排序、loop/model/tool/governance/usage 分类、duration、安全失败详情和两条路由注册。UI build 确认 typed client 与可展开详情面板能够编译。

## Skill Decision / Skill 结论

No feature-specific skill is added. The stable maintenance entry is this document plus `internal/server/README.md`; existing `code-style`, `doc-index-sync`, `feature-doc-skill-sync`, and `repo-task-delivery` already cover the recurring checks.

当前不新增 feature 专属 skill。本文与 `internal/server/README.md` 是稳定维护入口；既有 `code-style`、`doc-index-sync`、`feature-doc-skill-sync` 和 `repo-task-delivery` 已覆盖重复检查。
