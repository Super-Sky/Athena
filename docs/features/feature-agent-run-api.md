# Agent Run API Foundation

## 背景 / Background

Athena 需要为 `athena-fund-assistant` 等上层业务应用提供一个稳定、通用、目标驱动的运行入口。该入口不能把基金、股票、漫剧或其他业务对象写入 core，而是让业务应用通过上下文、资产、payload、工具声明和治理引用把业务语义传入 Athena。

Athena now exposes a stable app-facing Agent Run API for business applications. The API is intentionally generic: domain truth stays in the host application, while Athena owns runtime execution, trace readout, checkpoint metadata and transport contracts.

对应 issues / Canonical issues: `Super-Sky/Athena#7`, `Super-Sky/Athena#8`

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
- ordered `messages`
- assistant `tool_calls`
- correlated `tool_results`
- `trace_available`
- `resumed_from_run_id` for follow-up runs

## 当前实现 / Current Implementation

`internal/server/agent_runs.go` 是本功能的 HTTP transport 入口。它复用现有 app/runtime 主链：

- `POST /api/agent/runs` 解析 goal-first 请求，构造 `app.ChatRequest` 并通过 `OpenChatSession` 执行同步 MVP。
- OpenAI-compatible function declarations 和 `tool_choice` 会先经过结构校验，再转换为 provider-neutral `runtime.ToolDefinition` 与 canonical tool choice。
- Eino graph-native agent 使用调用方 schema 绑定模型工具，并通过正式模型选项执行 `none / auto / required / specific function` choice。
- 每次 prepared execution 持有一份 `ToolCallTranscript`。模型 post-handler 注册调用并补齐稳定 ID，tool middleware 与 ToolsNode post-handler 关联结果、错误和 timing。
- HTTP 响应按调用轮次返回 assistant/tool/final-assistant `messages`，同时保留顶层 `tool_calls` 和 `tool_results` 方便现有客户端读取。
- Runtime persistence 为每个调用写入包含 call ID、tool name、status、timing、argument keys/runes 和 result runes 的脱敏 trace，不保存原始参数与结果。
- Agent Run 当前是同步接口，因此返回完成后的 transcript；既有 `/api/chat/stream` 继续负责 `tool_calls` delta 合并与 tool lifecycle SSE，不在本切片新增第二套流协议。
- 省略 `task_type` 或传入 `agent_run` 时，内部 runtime task type 映射到已注册 `chat`，同时在 app context / input payload 中写入 `agent_run.v1` 契约。
- `GET /api/agent/runs/:runID` 和 `/trace` 通过 runtime persistence read boundary 读取 run、steps、events、trace、usage、projection 与 checkpoint safe readouts。
- `POST /api/agent/runs/:runID/resume` 先校验原 run 可读，再创建新的 follow-up run，并返回 `resumed_from_run_id`。
- `POST /api/agent/runs/:runID/cancel` 当前只暴露稳定路由。同步 MVP 不伪造异步取消，已终态 run 返回 `run_already_terminal`，非终态 run 返回 `sync_execution_not_cancellable`。

`internal/server/openapi.go` 已同步 typed OpenAPI schemas。server/runtime tests 覆盖 schema conversion、choice validation、stable ID、result correlation、tool errors、redacted trace 和 create/read/trace/resume/cancel 最小链路。

## 配置与依赖 / Configuration and Dependencies

- 本切片没有新增环境变量或数据库 migration。
- Stable synthesized IDs reuse the existing `github.com/google/uuid` dependency.
- OpenAI JSON Schema is converted through the existing Eino `jsonschema` and `ToolInfo` boundary.
- 真实执行仍要求调用方声明的 function name 已存在于 Athena tool registry，并要求已配置支持 tool calling 的模型。

## 当前状态 / Current Status

- issue `#7` 的 Agent Run API foundation 与 issue `#8` 的 tool-call contract 已实现并通过当前自动化、race、benchmark、OpenAPI 和本地 API smoke。
- issue `#8` 当前处于 `ready_for_delivery`，等待提交、push、stacked PR 与 issue 时间线同步。
- Remote app-owned tool registry/execution、HTTP callback、timeout/retry 和调用前治理属于 issue `#9`，未在本切片冒充完成。

## 边界 / Boundaries

Athena core 负责：

- 目标驱动 runtime 入口
- 通用 app/runtime 编排
- runtime persistence trace readout
- checkpoint safe metadata
- OpenAPI contract
- OpenAI-compatible transport conversion
- provider-neutral tool call transcript
- 已注册 Athena 工具的 graph-native execution 与 trace

Athena core 不负责：

- 基金、股票、漫剧等业务对象建模
- 业务账户授权同步
- 自动交易或资金操作
- 业务 evidence 的最终真相判断
- 业务远程 tool registry、HTTP callback、重试和治理执行

The `tools` field accepts OpenAI-compatible function declarations and string shorthand. Athena validates and converts them into a provider-neutral runtime contract, executes matching registered local tools, and returns an ordered call/result transcript. Remote business tool registration and execution remain issue `#9`.

## 骨架准入说明 / Backbone Rationale

- 该能力不能只放在基金或其他业务 adapter：稳定 call ID、tool-result correlation、provider-neutral transcript 和 trace redaction 是所有 tool-using agent 共用的 runtime 语义。
- The contract can serve fund research, content production, operations and other applications without adding domain objects to Athena.
- 新边界只有 transport conversion、canonical runtime contract、Eino adapter 和 safe persistence projector；OpenAI DTO 不进入 runtime core，Eino types 不进入 app-facing contract。
- 即使未来推翻基金助手场景，这套多轮 tool-call transcript、错误关联与 trace 边界仍然成立。

## 验证 / Verification

本功能的最小验证命令：

```bash
go test ./internal/runtime ./internal/app ./internal/server
go vet ./internal/runtime ./internal/app ./internal/server
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
- 当前只执行 Athena registry 中已有实现的工具；未注册业务工具会 fail closed，远程业务工具由 issue `#9` 接入。
- 同步 response 可返回 raw arguments/results；持久化 trace 始终使用脱敏摘要。
- Transcript 是单次 prepared execution 的有界内存对象，runner 完成后只复制到当前响应，不进入 session message history；超长 tool payload 的独立上限仍应在 remote execution issue 中补齐。
- The transcript is scoped to one prepared execution and is not retained in session history. Remote tools still need explicit payload-size limits.
- 没有配置 runtime persistence 时，read/trace/resume/cancel 无法提供 run 级读回能力。
- Agent Run API 进入 `internal/server/*` 公共契约区，后续改动必须继续保持通用性和场景隔离。

## Skill 结论 / Skill Decision

暂不新增独立 feature skill。当前 Agent Run API 与 tool-call contract 仍由同一公共运行入口维护；后续 remote registry、memory/context compression、trace 后台展示和业务应用接入会继续演进。维护入口保留在本 feature 文档、`docs/api.md`、`docs/architecture.md`、`docs/implementation.md` 与 server/runtime README。

No dedicated feature skill is created in this slice. The provider-neutral contract and its transport/runtime adapters are documented in the existing delivery, API, architecture, implementation, and module guides.
