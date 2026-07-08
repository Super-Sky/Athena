# Agent Run API Foundation

## 背景 / Background

Athena 需要为 `athena-fund-assistant` 等上层业务应用提供一个稳定、通用、目标驱动的运行入口。该入口不能把基金、股票、漫剧或其他业务对象写入 core，而是让业务应用通过上下文、资产、payload、工具声明和治理引用把业务语义传入 Athena。

Athena now exposes a stable app-facing Agent Run API for business applications. The API is intentionally generic: domain truth stays in the host application, while Athena owns runtime execution, trace readout, checkpoint metadata and transport contracts.

对应 issue / Canonical issue: `Super-Sky/Athena#7`

## 契约 / Contract

当前 API surface:

- `POST /api/agent/runs`
- `GET /api/agent/runs/:runID`
- `POST /api/agent/runs/:runID/resume`
- `POST /api/agent/runs/:runID/cancel`
- `GET /api/agent/runs/:runID/trace`

核心输入 / Core inputs:

- `goal` / `query`
- `success_criteria`
- `constraints`
- `budget`
- `context_assets`
- `tools`
- `tool_choice`
- `memory_scope`
- `governance_refs`
- app/session/workspace/model/context fields already used by Athena transport

核心输出 / Core outputs:

- `run_id`
- `status`
- `stop_reason`
- `output`
- `action` / `wait_state`
- `run`
- `trace_summary`
- `checkpoints`
- `trace_available`
- `resumed_from_run_id` for follow-up runs

## 当前实现 / Current Implementation

`internal/server/agent_runs.go` 是本功能的 HTTP transport 入口。它复用现有 app/runtime 主链：

- `POST /api/agent/runs` 解析 goal-first 请求，构造 `app.ChatRequest` 并通过 `OpenChatSession` 执行同步 MVP。
- 省略 `task_type` 或传入 `agent_run` 时，内部 runtime task type 映射到已注册 `chat`，同时在 app context / input payload 中写入 `agent_run.v1` 契约。
- `GET /api/agent/runs/:runID` 和 `/trace` 通过 runtime persistence read boundary 读取 run、steps、events、trace、usage、projection 与 checkpoint safe readouts。
- `POST /api/agent/runs/:runID/resume` 先校验原 run 可读，再创建新的 follow-up run，并返回 `resumed_from_run_id`。
- `POST /api/agent/runs/:runID/cancel` 当前只暴露稳定路由。同步 MVP 不伪造异步取消，已终态 run 返回 `run_already_terminal`，非终态 run 返回 `sync_execution_not_cancellable`。

`internal/server/openapi.go` 已同步 OpenAPI paths 和 schemas。`internal/server/agent_runs_test.go` 覆盖 OpenAI-compatible tool declaration 解析、create/read/trace/resume/cancel 最小链路。

## 边界 / Boundaries

Athena core 负责：

- 目标驱动 runtime 入口
- 通用 app/runtime 编排
- runtime persistence trace readout
- checkpoint safe metadata
- OpenAPI contract
- declarative tool input capture

Athena core 不负责：

- 基金、股票、漫剧等业务对象建模
- 业务账户授权同步
- 自动交易或资金操作
- 业务 evidence 的最终真相判断
- 真实 tool-call execution loop 或远程 tool registry 的完整实现

The current `tools` field accepts OpenAI-compatible function tool declarations and string shorthand, then maps tool names into customization enabled tools. Actual `tool_calls`, tool execution, remote tool registry and governance decisions remain future tool-contract work.

## 验证 / Verification

本功能的最小验证命令：

```bash
go test ./internal/server -run 'TestOpenAPISpecEndpoint|Test(ParseAgentRunStartRequestAcceptsOpenAITools|AgentRunEndpointsCreateReadTraceAndCancel)$' -count=1
go vet ./internal/server
python3 scripts/check_no_absolute_paths.py
```

本地服务 smoke:

```bash
HTTP_PORT=8090 SESSION_STORE_DRIVER=memory MODEL_STORE_DRIVER=memory go run . api-server
curl -sS http://127.0.0.1:8090/healthz
curl -sS http://127.0.0.1:8090/swagger/openapi.json
```

浏览器验证应优先使用 Codex in-app Browser 插件。若插件标签页控制不可用，应记录插件阻塞，并保留 curl / Go test 证据作为后备验证。

Use the Codex in-app Browser plugin first for local browser checks. If the browser backend is reachable but tab control times out, record that as an environment validation limitation instead of claiming browser smoke passed.

## 风险 / Risks

- 当前运行模式是同步 MVP，`cancel` 不是异步取消。
- `resume` 创建 follow-up run，不会原地改写原 run。
- `tools` 仍是声明态输入，不代表真实 tool call 已完成。
- 没有配置 runtime persistence 时，read/trace/resume/cancel 无法提供 run 级读回能力。
- Agent Run API 进入 `internal/server/*` 公共契约区，后续改动必须继续保持通用性和场景隔离。

## Skill 结论 / Skill Decision

暂不新增独立 feature skill。当前 Agent Run API 仍是基础契约切片，后续真实 tool-call loop、memory/context compression、trace 后台展示和业务应用接入会继续演进。维护入口暂时保留在本 feature 文档、`docs/api.md`、`docs/architecture.md`、`docs/implementation.md` 和 `internal/server/README.md`。

No dedicated feature skill is created in this slice. The maintenance surface is still evolving, and the existing repository delivery and server/API docs are the correct entrypoints for now.
