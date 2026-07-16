# v2.1.0 Master Plan

## Issue

- canonical issue: `Super-Sky/Athena#1`
- title: `v2.1.0 RuntimeContract foundation 收口与 Batch 2 计划冻结`
- current state: `ready_for_delivery`
- branch: `codex/v2.1-runtime-contract-batch2-issue-1`

## Scope Guard

v2.1.0 只接受能强化通用 agent runtime foundation，或能证明 foundation 真实可用的工作。任何业务真相、业务 evidence 或应用专属判断逻辑都不能借本期进入 core。

- `Core`: RuntimeContract, registered task type, hook binding, System Truth lifecycle, semantic projection boundary, checkpoint-backed waiting read surface.
- `Validation`: System Validation readout, deterministic validation run, Validation MCP, sandbox boundary, repeatable API and DOM smoke checks.
- `Enhancement`: scenario packages, app-specific skills, app knowledge, provider adapters.
- `Application / Business Truth`: business evidence, formal business object state, and app-owned rules.

Supplement、Compaction、Capability Studio、external MCP registry、cleanup jobs 和业务 evidence ownership 继续保持 deferred，除非后续 issue 明确拉入。Issue `#10` 已明确拉入 scoped memory / context ownership API；它只保存应用摘要，业务 evidence ownership 仍保持 deferred。

## Batch 2 Checklist

- [x] 恢复与 `Super-Sky/Athena#1` 绑定的 v2.1.0 master plan 和可勾选 checklist。
- [x] 增加可重复运行的 RuntimeContract foundation API smoke，覆盖 read/write/readout 相关 surface。
- [x] 为 System Validation 页面增加稳定 DOM anchors，支持严格浏览器自动化。
- [x] 在脚本索引和 feature 文档中记录可重复 smoke 路径。
- [x] 对运行中的 Web 控制台执行严格 Codex in-app Browser DOM 验收。
- [x] 产品化 checkpoint-backed waiting run readout，不暴露 Eino private checkpoint payload。
- [x] 为 `inspection_task`、`integration_event`、`scheduled_job`、`workflow_step_request` 定义 registered task type validator contract。
- [x] 深化 System Truth `source -> draft -> compile -> active -> rollback` write/edit path 规划。
- [x] 收口 semantic projection boundary，确保 projection candidate 不升级为业务 `EvidenceRecord`。
- [x] 将剩余 direct respond rich delivery 兼容拼装从 transport 收敛到 app/runtime graph node 或 Batch 2 read model。

## Application Runtime MVP Checklist

- canonical issue: `Super-Sky/Athena#8`
- branch: `codex/openai-tool-calls-issue-8`
- base branch: `codex/app-agent-run-api-issue-7`
- current state: `ready_for_delivery`

- [x] 定义 provider-neutral tool definition、tool choice、tool call、tool result 与 transcript contract。
- [x] 将 OpenAI-compatible `tools` / `tool_choice` 校验并转换为 canonical runtime contract。
- [x] 从 Eino graph-native loop 采集稳定 `tool_call_id`、arguments、result、status 与 timing。
- [x] 在 Agent Run response 返回 OpenAI-compatible assistant `tool_calls`。
- [x] 在 runtime trace 中保存可关联、经过安全处理的 tool call / result timeline。
- [x] 补齐声明转换、ID 关联、非法参数与工具执行错误测试。
- [x] 同步 API、架构、实现与 feature 文档，并完成交付门禁。

## Remote Business Tool Checklist

- canonical issue: `Super-Sky/Athena#9`
- branch: `codex/remote-tool-registry-issue-9`
- base branch: `codex/openai-tool-calls-issue-8`
- current state: `ready_for_delivery`

- [x] 定义 remote tool registration、execution request/response 与 normalized error contract。
- [x] 提供线程安全 dynamic tool catalog，让 runtime resolver 与 Eino executor 读取同一快照。
- [x] 提供 control-plane authenticated registration/list/delete API，并在重启后恢复注册。
- [x] 对 endpoint 执行 origin allowlist、timeout、response-size、retry 与 idempotency 校验。
- [x] 在任何远程 HTTP 调用前执行 tool governance，并落实 deny/redaction/sandbox decision。
- [x] 将远程执行关联到 canonical tool call/result trace 与 generic usage。
- [x] 用本地 fake business service 验证注册、执行、超时、重试、错误和治理拒绝。
- [x] 同步 OpenAPI、API/架构/实现/能力总览、feature 文档与真实场景测试。

## Deterministic Built-in Tool Checklist

- canonical issue: `Super-Sky/Athena#14`
- branch: `codex/builtin-tools-issue-14`
- base branch: `codex/remote-tool-registry-issue-9`
- current state: `ready_for_delivery`

- [x] 注册 calculator、current_time 和 JSON Schema subset validator 到 live catalog。
- [x] 将三条内置工具标记为 `builtin_deterministic`、无副作用且无需确认。
- [x] 为表达式边界、时区、schema mismatch、畸形 schema 和 runtime declaration admission 补齐测试。
- [x] 全仓测试、内置工具 race、benchmark 与 Agent Run integration 测试通过；app package race 仍暴露既有 `scene.SetSourcesRoot` 测试初始化竞争，单独跟踪。
- [ ] 补齐受限 HTTP/search/file Enhancement 方案，不在 Core 工具切片中实现。

## External Memory and Context Asset Checklist

- canonical issue: `Super-Sky/Athena#10`
- branch: `codex/external-memory-issue-10`
- base branch: `codex/builtin-tools-issue-14`
- current state: `ready_for_delivery`

- [x] 提供带 `app_id + owner_id + scope` ownership 三元组的应用摘要 write/query API。
- [x] 将作用域摘要解析为只读 `app_memory` / `memory_view` assets，并返回操作 trace。
- [x] 组装 effective views 与 `external_context_compression.v1` summary 形状，不引入业务领域表。
- [x] 覆盖 HTTP ownership isolation、resolve/assemble 和无效 ownership 测试。
- [ ] 为生产部署增加可配置的 durable persistence adapter；当前线程安全 in-process store 仅用于 local/demo MVP。

## Agent Trace Timeline Checklist

- canonical issue: `Super-Sky/Athena#11`
- branch: `codex/agent-trace-timeline-issue-11`
- base branch: `codex/external-memory-issue-10`
- current state: `implementing`

- [x] 复用 persisted step、lifecycle、trace、usage 与 projection records，按时间投影统一 timeline。
- [x] 每条 timeline entry 返回 timestamp、duration、status、source、error metadata 与安全 detail，不复制 raw payload。
- [x] 提供 app-facing 与 Control Plane read API，并让后台展示可展开 step/detail 列表。
- [x] 覆盖时间顺序、model/tool/governance 分类、失败 detail、路由与 OpenAPI 测试。
- [x] 将运行观测提升为一级导航，提供 run 列表、统一时间线、请求/返回/影响/性能安全检查器和移动端紧凑导航。
- [x] 将 model/provider、最终 assembled prompt、skill、tool schema、policy、context、实际使用的 evaluator 与 runtime contract revision 收敛为 `agent_run_manifest.v1` 不可变顶层契约；旧 run 返回 `legacy_unavailable`，不按当前配置反推。
- [x] issue #26 为全部 app-facing Agent Run 路由增加独立 app token、精确 `(workspace_id, app_instance_id)` scope、权威 create/resume ownership 与同形跨租户 `404`；双入口 DTO 同步递归脱敏。

## Goal-driven Execution Controls Checklist

- canonical issue: `Super-Sky/Athena#22`
- branch: `codex/agent-run-contract-issue-22`
- base branch: `codex/agent-run-read-auth-issue-26`
- current state: `implementing`

- [x] 在 foundation 中幂等注册默认 `chat` task type 与 active validator contract，修复 PostgreSQL strict resolution 下默认 Agent Run 返回 `unsupported_task_type`。
- [x] 用受认证 app identity 发起省略 `task_type` 的真实 PostgreSQL Agent Run，并验证 run ownership、immutable manifest 与授权 timeline。
- [ ] 定义并落实 success criteria evaluation、budget、deadline 与完整 stop-reason taxonomy。
- [ ] 接入 Redis-backed enqueue、idempotency lock、retry/backoff、cancel 与 checkpoint/resume contract。
- [ ] 补齐异步 HTTP/SSE、OpenAPI、Docker env、Redis 故障降级和系统回归测试。

## Acceptance Gates

- 在启用 runtime persistence 的真实后端上通过 API smoke：
  - `/api/control-plane/runtime/contracts/foundation`
  - `/api/control-plane/runtime/validation-runs`
  - runtime run detail, steps, lifecycle, traces, usage, projections, checkpoint safe metadata
- Web build 通过。
- runtime read/write 与 validation 的聚焦 Go 测试通过。
- 关闭 Browser checklist 项前，System Validation tab 的 DOM smoke 必须通过。
- 任何 commit 或 push 前必须执行 `repo-task-delivery`。

## Current Verification Commands

```bash
go test ./internal/controlplane ./internal/server ./internal/app ./internal/runtime
go test ./internal/runtime ./internal/app ./internal/server
cd web && npm run build
PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build
python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py
python3 scripts/check_no_absolute_paths.py
python3 scripts/control_plane_runtime_foundation_smoke.py --base-url http://127.0.0.1:8090
python3 scripts/control_plane_runtime_foundation_smoke.py --base-url http://127.0.0.1:8090 --web-url http://127.0.0.1:5173
```

API smoke 需要一个已启用 runtime persistence 的运行中后端。`--web-url` 路径还需要 Python Playwright 与运行中的 Web 控制台。

## Current Evidence

- 2026-05-26 本机临时 PostgreSQL `55432` 环境通过 `go run . migrate`。
- 2026-05-26 API smoke 通过，foundation 返回 `contracts=1`、`task_types=1`、`hook_bindings=3`、`active_system_truths=54`。
- 2026-05-26 Web DOM smoke 通过，System Validation 页面新增锚点均可定位。
- 2026-05-26 Codex in-app Browser 切到 System Validation tab 后确认新增锚点 `missing=[]`。
- 2026-05-26 self-review 后收紧 DOM smoke：所有新增 `data-testid` 必须唯一且可见，并确保浏览器失败时也会关闭。
- 2026-05-26 checkpoint-backed waiting readout 已接入 `GET /api/control-plane/runtime/runs/:runID/checkpoints`、OpenAPI、System Validation 与 smoke 脚本；API 只暴露 `payload_size` / `payload_sha256` 等安全摘要，不返回 raw payload 或 resume token。
- 2026-05-26 聚焦验证通过：`go test ./internal/runtime ./internal/app ./internal/server`、`cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`。
- 2026-05-26 真实 API + DOM smoke 通过，返回 `run_id=163b71fa-8c18-4a03-bdb8-6de12f168e0e`、`records.steps=1`、`records.lifecycle=9`、`records.traces=5`、`records.usage=5`、`records.projections=3`、`records.checkpoints=0`、`web.dom=ok`。
- 2026-05-29 registered task type validator contract 已接入四个 legacy task type，active task type 写入校验要求 `default_contract_id`、`input_schema` 和 `validator_refs.validators`；draft 仍可暂存。
- 2026-05-29 聚焦验证通过：`go test ./internal/runtime ./internal/app ./internal/server`、`cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`、`python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py`。
- 2026-05-29 真实 API + DOM smoke 通过，返回 `run_id=f09ad491-9df4-47c4-b92c-97adf7098c24`、`contracts=5`、`task_types=5`、`hook_bindings=3`、`active_system_truths=54`、`web.dom=ok`。
- 2026-06-10 System Truth lifecycle write/edit path 已接入 Control Plane API、OpenAPI、Postgres read/list、app 编排、System Validation readout 与 smoke 断言；rollback 通过追加 active pointer 并记录 `rollback_from_id` 实现，不改写历史。
- 2026-06-10 聚焦验证通过：`go test ./internal/runtime ./internal/app ./internal/server`、`cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`、`python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py`。
- 2026-06-10 semantic projection boundary 已收口：`ProjectionCandidate` 写入前强制 `runtime_projection.*` schema、默认 `projection_candidate_only` materialization target，并拒绝 EvidenceRecord-like candidate/schema/target/typed semantic payload；System Validation 与 smoke 已新增 boundary readout / 断言。
- 2026-06-10 聚焦验证通过：`go test ./internal/runtime ./internal/app ./internal/server`、`cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`、`python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py`。
- 2026-06-10 direct respond rich delivery 已收口到 app 层 Batch 2 read model：transport 保留解析、schema 修复、HTTP/SSE 映射，`internal/app/direct_respond_rich_delivery.go` 负责 result summary、cards、right panel、workflow、automation、context assets 和 base capability 兼容结果拼装。
- 2026-06-10 聚焦验证通过：`go test ./internal/app ./internal/server`。
- 2026-07-09 issue #8 已完成 OpenAI-compatible tools/tool_choice transport conversion、provider-neutral runtime transcript、Eino adapter、ordered messages、call/result projection 与 redacted trace。
- 2026-07-09 验证通过：`env -u APP_ENV go test ./...`、`go test -race ./internal/runtime ./internal/server`、`go vet ./internal/app ./internal/server`、Web Node 24 build、绝对路径检查、health/OpenAPI/invalid-schema API smoke。
- 2026-07-09 transcript benchmark 三次结果为 `7699-11073 ns/op`、`784 B/op`、`9 allocs/op`；单次 execution transcript 不进入 session history。
- `go vet ./internal/runtime` 的 `EinoGraphFoundation contains sync.Mutex` 值传递告警在 issue #8 基线分支同样存在，不属于本次改动。
- Codex in-app Browser 已确认 `http://127.0.0.1:8091/swagger` 加载为 `Athena Swagger UI`；Swagger 大 DOM 读取超时，typed schema 由真实 `/swagger/openapi.json` 响应补充验证。
- 2026-07-09 issue #9 已完成 remote registry、dynamic catalog、governance-before-network、HTTP callback、网络预算、safe trace/usage 与重启恢复。
- 2026-07-09 全仓验证通过：`env -u APP_ENV go test ./...`；fake business service 测试覆盖注册、执行、redaction、deny、timeout、retry、correlation、response budget、redirect、restore 与 delete。
- 2026-07-10 issue #14 开始增加确定性 Core 工具包：calculator、IANA timezone current_time 与 fail-closed JSON Schema subset validator；不引入网络、文件、凭据或业务对象。
- 2026-07-10 issue #14 验证通过：`go test ./...`、`go test -race ./internal/tools`、calculator benchmark（约 `1893 ns/op`、`0 B/op`、`0 allocs/op`）及 Agent Run tool call ID/governance integration tests。`go test -race ./internal/tools ./internal/app ./internal/runtime` 仍报告 `scene.SetSourcesRoot` 的既有 app 测试初始化竞争，未由本工具切片引入。

## Next Iteration Plan

Batch 2 与 issues #8/#9 checklist 已完成。下一轮进入 `athena-fund-assistant` 首批真实数据 provider 与 business tool callback 集成。

执行清单：

1. 提交、推送 issue `#9` 分支并创建 stacked PR，回 issue 时间线同步交付证据。
2. 由 `athena-fund-assistant` 先验证免费 A 股基金/美股数据源结构，再编码首批 snapshot tools。
3. 使用本地 Docker 网络注册 fund snapshot / portfolio / journal 业务工具，并执行双服务 E2E。
4. 若目标分支最终准备合入 `master`，先执行 `master-merge-gate` 并取得人工确认。
