# API

## API 分层与 truth ownership

当前 API 文档描述的是已暴露 HTTP / SSE / control-plane / runtime 契约，不直接定义业务真相归属。

API 语义按以下边界理解：

- `Core`
  - runtime、session、task normalization、system-resources、control-plane、governance、model/provider 管理等平台基础接口。
- `Validation`
  - `/api/control-plane/runtime/...` 与 System Validation 相关接口，用于独立验收 core 能力闭环。
- `Enhancement`
  - skill package、scene、workflow、provider adapter、platform context 等让应用更快搭建的增强接口。
- `Application / Business Truth`
  - 业务对象、业务规则、业务状态和最终业务真相默认由宿主应用持有；Athena API 只消费上下文、引用、projection 或 runtime input，除非应用显式选择 Athena-managed enhancement 模式。

`Evidence`、`knowledge`、`skill` 等词在 API 文档中默认按 projection、candidate、enhancement 或 system truth asset 理解，不表示 Athena core 接管正式业务证据库、正式业务知识库或用户 skill 主库。

## Overview

当前脚手架暴露以下 HTTP 接口：

- `GET /swagger`
- `GET /swagger/openapi.json`
- `GET /healthz`
- `GET /api/control-plane/auth/status`
- `POST /api/control-plane/login`
- `POST /api/control-plane/logout`
- `GET /api/control-plane/bootstrap`
- `GET /api/control-plane/scenes`
- `PUT /api/control-plane/scenes/:id`
- `GET /api/control-plane/skills`
- `PUT /api/control-plane/skills/:name`
- `GET /api/control-plane/tools`
- `PUT /api/control-plane/tools/:name`
- `GET /api/control-plane/remote-tools`
- `PUT /api/control-plane/remote-tools/:name`
- `DELETE /api/control-plane/remote-tools/:name`
- `GET /api/control-plane/runtime-config`
- `PUT /api/control-plane/runtime-config`
- `POST /api/control-plane/runtime/validation-runs`
- `GET /api/control-plane/runtime/contracts/foundation`
- `PUT /api/control-plane/runtime/contracts/:contractID`
- `PUT /api/control-plane/runtime/task-types/:typeKey`
- `PUT /api/control-plane/runtime/hook-bindings/:bindingID`
- `GET /api/control-plane/runtime/runs`
- `GET /api/control-plane/runtime/runs/:runID`
- `GET /api/control-plane/runtime/runs/:runID/steps`
- `GET /api/control-plane/runtime/runs/:runID/lifecycle`
- `GET /api/control-plane/runtime/runs/:runID/traces`
- `GET /api/control-plane/runtime/runs/:runID/timeline`
- `GET /api/control-plane/runtime/runs/:runID/usage`
- `GET /api/control-plane/runtime/runs/:runID/projections`
- `GET /api/control-plane/runtime/runs/:runID/checkpoints`
- `GET /api/control-plane/governance`
- `PUT /api/control-plane/governance`
- `GET /api/control-plane/tool-governance/policy`
- `GET /api/control-plane/tool-governance/decisions`
- `POST /api/control-plane/tool-governance/evaluate`
- `GET /api/control-plane/validation-mcp/server`
- `GET /api/control-plane/validation-mcp/tools`
- `POST /api/control-plane/validation-mcp/invocations`
- `GET /api/control-plane/config-versions`
- `GET /api/control-plane/config-versions/:versionID`
- `POST /api/control-plane/config-versions/:versionID/rollback`
- `GET /api/system-resources`
- `POST /api/system-resources/sync`
- `POST /api/system-resources`
- `POST /api/system-resources/build-package`
- `GET /api/system-resources/:id`
- `DELETE /api/system-resources/:id`
- `GET /api/system-resources/:id/versions`
- `GET /api/system-resources/:id/versions/:versionID`
- `POST /api/system-resources/:id/versions/:versionID/rollback`
- `GET /api/system-resources/:id/audit`
- `PATCH /api/system-resources/:id/metadata`
- `GET /api/system-resources/:id/source`
- `PUT /api/system-resources/:id/source`
- `POST /api/system-resources/:id/parse`
- `POST /api/system-resources/:id/compile`
- `POST /api/system-resources/:id/activate`
- `GET /api/system-resources/:id/pipeline`
- `GET /api/system-resources/:id/parse-result`
- `GET /api/system-resources/:id/compile-result`
- `GET /api/system-resources/:id/debug-payload`
- `GET /api/system-resources/:id/download`
- `GET /api/system-resources/export`
- `POST /api/sessions`
- `GET /api/sessions`
- `GET /api/sessions/:id`
- `PATCH /api/sessions/:id`
- `POST /api/sessions/:id/archive`
- `POST /api/chat/stream`
- `GET /api/skills`
- `GET /api/skills/packages`
- `GET /api/skills/packages/:id`
- `GET /api/skills/packages/:id/revisions`
- `POST /api/skills/packages`
- `PUT /api/skills/packages/:id`
- `POST /api/skills/packages/:id/rollback`
- `PATCH /api/skills/packages/:id`
- `DELETE /api/skills/packages/:id`
- `GET /api/runtime/skills`
- `POST /api/chat/respond`
- `POST /api/agent/runs`
- `GET /api/agent/runs/:runID`
- `POST /api/agent/runs/:runID/resume`
- `POST /api/agent/runs/:runID/cancel`
- `GET /api/agent/runs/:runID/trace`
- `POST /api/runtime/respond`
- `POST /api/runtime/scenario/respond`
- `GET /api/models/providers`
- `POST /api/models/providers`
- `PUT /api/models/providers/:id`
- `PATCH /api/models/providers/:id`
- `DELETE /api/models/providers/:id`
- `POST /api/models/providers/:id/models`
- `PUT /api/models/providers/:id/models/:record_id`
- `PATCH /api/models/providers/:id/models/:record_id`
- `DELETE /api/models/providers/:id/models/:record_id`
- `POST /api/models/providers/:id/models/:record_id/test`

聊天接口当前同时暴露：

- `POST /api/chat/stream`
- `POST /api/chat/respond`
- `POST /api/runtime/respond` 是通用 direct respond adapter：它复用 app/runtime 主路径产出一次直接响应，不定义新的场景专属 core API。
- `POST /api/runtime/scenario/respond` 是 legacy scenario judgment 兼容入口：它继续承接 `RuntimeScenarioRequest` / `RuntimeScenarioResponse` 形态和 evidence supplement 流程，不作为新的 core direct respond contract。

Agent Run API 当前暴露：

- `POST /api/agent/runs`
  - 面向业务应用创建一次目标驱动 run，输入以 `goal`、`success_criteria`、`constraints`、`budget`、`context_assets`、`tools`、`memory_scope` 和 `governance_refs` 为核心。
  - Creates one app-facing goal-driven run. The current MVP executes synchronously through the existing app/runtime path and returns `run_id`, request status, stop reason, output, trace summary and checkpoint readouts when runtime persistence is configured.
  - `tools` 接受 OpenAI-compatible function tool 或字符串简写，并转换为 provider-neutral runtime declarations；当前只允许调用 Athena 已注册的工具。内置 `calculator`、`current_time`、`json_schema_validate` 可直接启用；业务远程工具注册与执行属于 issue `#9`。
  - `tool_choice` 支持 `none`、`auto`、`required` 和指定 function object。省略时，无工具默认为 `none`，有工具默认为 `auto`。
  - The response exposes ordered `messages`, assistant `tool_calls`, correlated `tool_results`, stable call IDs and final `output`. Top-level call/result arrays are compatibility projections of the canonical transcript.
- `GET /api/agent/runs/:runID`
  - 读取单个 run 的 app-facing 状态摘要，底层复用 persisted `TaskRun` 与 trace summary。
  - Reads the app-facing run status summary from persisted runtime records.
- `POST /api/agent/runs/:runID/resume`
  - 基于原 run 发起一次补数续跑；当前实现会先确认原 run 可从 runtime persistence 读回，再创建新的 runtime run，并在响应中写入 `resumed_from_run_id`。
  - Resumes by first validating the original run readout, then starting a follow-up run with supplement / resume token metadata instead of mutating the original synchronous run in place.
- `POST /api/agent/runs/:runID/cancel`
  - 当前同步 MVP 不伪造异步取消；已终态 run 返回 `409` 和 `run_already_terminal`，非终态 run 返回 `409` 和 `sync_execution_not_cancellable`。
  - The route is stable, but asynchronous cancellation is not implemented in this slice.
- `GET /api/agent/runs/:runID/trace`
  - 返回该 run 的 `RuntimeRun`、`RuntimeStep`、`RuntimeLifecycleEvent`、`RuntimeTrace`、`Usage`、`ProjectionCandidate`、checkpoint safe readouts 和聚合 summary。
  - Returns the full safe trace timeline assembled from runtime persistence.
- `GET /api/agent/runs/:runID/timeline`
  - 将现有的 step、lifecycle、trace、usage 和 projection records 依时间投影为一条业务应用可直接展示的列表；每条 entry 包含 timestamp、duration、status、source、error 和安全 detail。
  - Projects existing persisted records into one ordered, app-readable timeline. It does not create a duplicate trace store and never returns raw prompts, tool arguments, tool results, or business payloads.

Agent Run API 边界：

- Athena core 不接管业务对象、业务证据或业务状态；业务仓仍通过 `context_assets`、`global_context`、`app_context` 和 `input_payload` 注入应用语义。
- The API contract is generic. Fund, stock, drama, or other domain objects must stay in the business application layer.
- 工具参数和结果可在同步响应中返回；持久化 trace 只保存 call ID、tool name、状态、时序、参数键和长度等安全摘要，不保存原始参数或结果。
- Raw tool arguments/results are available to the synchronous caller, while persisted traces use a redacted correlation timeline.
- 省略或传入 `task_type=agent_run` 时，当前内部 runtime task type 映射到已注册 `chat`，并把 `agent_run.v1` 契约写入 app context / input payload；未来注册式 task type 可以显式传入其他 `task_type`。
- If runtime persistence is not configured, read/trace endpoints return `503`; create responses may still complete but cannot expose a persisted trace.

Control Plane additionally exposes `GET /api/control-plane/runtime/runs/:runID/timeline` behind its existing authentication boundary. The Admin UI renders this same projection as expandable safe-detail rows for model, tool, governance, context, loop, usage, and delivery investigation.

## 应用拥有的 Memory / Context API

Athena 提供四条通用 API，供业务应用保存安全摘要、按归属读取摘要，并组装为可注入的上下文资产：

- `POST /api/memory/write` 写入 `app_id + owner_id + scope` 三元组下的版本化摘要；必填字段为 `kind` 与 `summary`，可附带应用定义的 `schema_version` 和字符串元数据。
- `POST /api/memory/query` 只能按完整的 `app_id + owner_id + scope` 查询。响应返回安全的 `memory_query` trace，不会跨 owner 或 scope 合并结果。
- `POST /api/context-assets/resolve` 将查询结果转成只读 `memory_view` 资产，`source_kind=app_memory`，可直接作为 `context_assets` 注入。
- `POST /api/context-assets/assemble` 将应用记忆摘要与调用方传入的通用 assets 组装成 `UsageTrace`、effective views 和 `external_context_compression.v1` 统计形状。

The contract deliberately accepts summaries rather than domain records. `app_id`, `owner_id`, and `scope` are mandatory on every read and write; the response trace records only the operation and record IDs. Athena core does not create fund, portfolio, trade, or other business tables. The MVP store is process-local and is therefore suitable for local/demo wiring; an application must use its own durable business store until a configured persistence adapter is introduced.

Validation MCP 当前暴露：

- `GET /api/control-plane/validation-mcp/server` 返回内置 `athena-validation-mcp` server 描述、轻量 transport 和已摄取 tool schemas。
- `GET /api/control-plane/validation-mcp/tools` 返回当前阶段两个确定性 tool schemas：`security_context_echo` 与 `risk_signal_lookup`。
- `POST /api/control-plane/validation-mcp/invocations` 会先走 tool governance，再执行确定性 validation tool，并返回 request、governance decision、result summary、safe trace 和 redacted payload。
- 这些接口属于 Validation layer，不声明标准 MCP transport，不接外部 MCP registry，也不把 validation result 定义成业务 evidence truth。

Runtime validation trigger 当前会把 Phase 1-5 串成一条 deterministic validation flow：

- `POST /api/control-plane/runtime/validation-runs`
  - 先通过 Eino Graph foundation 写入 runtime run / step / lifecycle / trace / usage / projection。
  - 再调用 Validation MCP `risk_signal_lookup`，通过 tool governance 生成 decision，并把 MCP trace / usage / projection 写入 runtime persistence。
  - 最后生成 `external_sandbox_ref` structured result，写入 sandbox lifecycle event、trace、generic usage 和 projection candidate。
  - 响应包含 `validation_mcp`、`sandbox`、`sandbox_trace`、`sandbox_usage` 和 `sandbox_projection`，用于 System Validation 页面验收。

## Remote business tool registry

`/api/control-plane/remote-tools` 允许独立业务服务把 HTTP tool 实现注册进 Athena live catalog。注册内容不保存 credentials；`endpoint` 必须命中 `REMOTE_TOOL_ALLOWED_ORIGINS` 的 exact origin，schema 根节点必须为 object，`timeout_ms` 范围为 `1..30000`，`retry_max_attempts` 范围为 `0..3`。带副作用且非幂等的工具禁止重试。

Callback 使用 `remote_tool_execution.v1`。请求包含 `request_id`、`tool_call_id`、`registration_id`、`app_id`、`tool_name`、JSON object `arguments`、`attempt` 和安全 metadata。响应必须回传相同 ID，并返回 `status=ok` + `content`，或标准化 `error.code/message/retryable`。

Athena 禁止 callback redirect，并在任何网络请求前执行 tool governance。Raw arguments/results 只在当前执行链内流转；持久化 trace 与 generic metric 只记录 origin、attempt、duration、decision ID、status 和 normalized error code 等安全元数据。

## V1 协议补充

V1 当前统一任务输入与结果模型。`task_type` 是通用 runtime task type 字符串；以下值继续兼容已有调用，但只代表 legacy-compatible / future registered semantics，不是 Athena core-native 类型枚举：

- `chat`
- `inspection_task`
- `integration_event`
- `scheduled_job`
- `workflow_step_request`

Phase 0 core normalization 只校验通用边界，不再把 `inspection_task`、`scheduled_job`、`workflow_step_request` 等场景字段作为 core 必填规则。宿主应用或后续注册式 task type validator 可以继续在边界外施加业务字段要求。

当前关键 SSE 事件：

- `progress_step`
- `workflow_plan`
- `inspection_progress`
- `card_created`
- `right_panel_view`

当前关键结果字段：

- `next_questions`
- `content_cards`
- `knowledge_candidates`
- `main_answer`
- `structured_result`
- `result_summary`
- `right_panel_view`
- `score_delta`
- `delivery_profile`
- `execution_intent`
- `execution_result`

当前代码已开放的统一请求字段包括：

- `task_type`
- `task_subtype`
- `scene`
- `main_session_id`
- `workspace_id`
- `app_instance_id`
- `app_session_id`
- `integration_instance_id`
- `workflow_run_id`
- `step_id`
- `trigger_type`
- `automation_task_id`
- `user_language`
- `desired_output_mode`
- `global_context`
- `app_context`
- `input_payload`

## 主体上下文约定

- `session_id` 只承担会话连续性，不承担用户层归属。
- 宿主系统持有跨 session 的用户层主数据。
- Athena 只按需消费：
  - `user_id`
  - 平台注入的必要用户上下文
- 默认 `chat` 不再因为缺少 `user_id` 一律进入 waiting。
- 只有明确依赖主体归属的能力或任务，才会把 `user_id / 用户上下文` 作为硬依赖。

## context assets 注入约定

请求侧可注入：

- `global_context.context_assets`
- `global_context.context_assets_resolved`

请求级 customization 可临时覆盖：

- `context_asset_overrides`
- `disabled_asset_types`
- `asset_priority_overrides`

当前 system truth 的正式资产类型是：

- `persona`
- `agent_profile`
- `policy_rule`
- `user_profile`
- `memory_view`
- `scene`
- `workflow`
- `contract`
- `skill`

当前返回痕迹包括：

- `used_context_assets`
- `resident_assets`
- `on_demand_assets`
- `suppressed_assets`
- `asset_conflicts_resolved`
- `requested_asset_details`
- `loaded_asset_details`
- `candidate_asset_targets`
- `candidate_asset_diffs`
- `candidate_asset_updates`
- `asset_usage_trace`

当前 runtime 统一生效视图：

- `effective_persona`
- `effective_agent_profile`
- `effective_user_profile`
- `effective_memory_view`
- `effective_scene`
- `effective_workflow`
- `effective_policy_rules`
- `effective_contracts`
- `effective_skills`

## 交互模式路由约定

platform 当前推荐通过：

- `input_payload.interaction_context.entry_mode`
- `input_payload.interaction_context.user_selected_mode`

`entry_mode` 当前推荐枚举：

- `default_chat`
- `automation_create`
- `automation_confirm`
- `result_explanation`

`user_selected_mode` 当前推荐枚举：

- `chat`
- `automation_draft`

## persona context 注入约定

- platform 用户 persona 推荐注入：
  - `global_context.persona_context`
- 推荐字段：
  - `id`
  - `name`
  - `description`
  - `style_rules`
  - `example_dialog`
- 当前边界：
  - `persona_id` 主要用于日志和追踪
  - `description / style_rules / example_dialog` 用于 Athena 理解表达风格
  - persona 只影响表达方式，不影响事实判断、风险标准和证据门槛

## platform context 注入约定

platform 当前可注入：

- `global_context.platform_context_catalog`
- `global_context.identity_summary`
- `global_context.memory_summary`
- `global_context.knowledge_summary`
- `global_context.skills_summary`
- `global_context.persona_summary`
- `global_context.persona_context`
- `global_context.platform_context_access`

Athena 当前会优先消费 summary，再决定是否返回：

- `used_contexts`
- `context_usage`
- `context_details_requested`

`context_details_loaded` 表示同轮已经成功预取并参与推理的 detail 类型。

当 summary 不足时，Athena 当前会优先在同轮尝试预取 platform detail：

- 优先使用 `platform_context_catalog` 中声明的 `detail_url / detail_endpoint / detail_api / detail_path`
- 若 catalog 未显式给出，则回退 `PLATFORM_CONTEXT_BASE_URL + /api/v1/platform-context/{type}`
- 若 `global_context.platform_context_access.token` 存在，则优先使用该 token 作为 detail 请求凭证
- 只有在没有 context access token 时，才回退固定 auth token 或入站 `Authorization` 转发
- 若仍无法预取，才通过：
  - `context_details_requested`
  - `platform_tool_hints`
  发出 detail 请求信号

## 控制面与 Web 控制台约定

- `GET /api/control-plane/bootstrap`
  - 返回控制台启动载荷：
    - `scenes`
    - `skills`
    - `tools`
    - `governance`
    - `runtime`
    - `config_versions`
    - `swagger_spec_url`
- `GET/PUT /api/control-plane/scenes/:id`
  - 管理 scene override
- `GET/PUT /api/control-plane/skills/:name`
  - 管理 skill override
- `GET/PUT /api/control-plane/tools/:name`
  - 管理 tool registry 的白名单元数据 override
- `GET/PUT/DELETE /api/control-plane/remote-tools/:name`
  - 管理 app-owned HTTP tool 注册；接口复用 Control Plane auth，成功后立即更新 live catalog
- `GET/PUT /api/control-plane/runtime-config`
  - 兼容保留的 runtime tuning 入口
- `POST /api/control-plane/runtime/validation-runs`
  - 通过 Eino runtime graph foundation 触发一次内部验证写入，返回本次生成的 `TaskRun`、`TaskStep`、lifecycle events、安全 `RuntimeTrace`、generic `Usage` 和 minimal `ProjectionCandidate`
  - 当前 trigger 会显式绑定默认 `runtime_contract_id`，因此新的 validation run 会额外写入 contract-aware `runtime_hook_binding` traces 与 `runtime_hook` usage
- `GET /api/control-plane/runtime/contracts/foundation`
  - 返回 RuntimeContract、TaskTypeRegistry、HookBinding、active System Truth pointer、System Truth source / draft / compile 摘要和 store capability surface
  - foundation records 会在服务启动和 `POST /api/system-resources/sync` 后按 active truth 自动补齐
  - bootstrap 默认注册 `chat` 及其 validator contract，保证 `/api/chat/*` 与 `/api/agent/runs` 在 PostgreSQL runtime store 下开箱可执行
- `PUT /api/control-plane/runtime/contracts/:contractID`
  - 按稳定 `contractID` 创建或更新一条 `RuntimeContract`
  - payload 会经过 runtime contract 安全校验（status 枚举、credential-like plaintext 拦截）
- `PUT /api/control-plane/runtime/task-types/:typeKey`
  - 按稳定 `typeKey` 创建或更新一条 `TaskTypeRegistration`
  - 若已有记录会保留 `created_at` 与既有 `id`
- `PUT /api/control-plane/runtime/hook-bindings/:bindingID`
  - 按稳定 `bindingID` 创建或更新一条 `HookBinding`
  - `binding_ref` 必须命中 internal allowlist（例如 `runtime_contract_guard`、`system_truth_guard`、`projection_boundary_guard`）
- `GET /api/control-plane/runtime/system-truth/lifecycle`
  - 读取 System Truth source、draft、compile result 和 active pointer history，支持 `asset_id`、`source_id`、`draft_id`、`status`、`limit`
- `POST /api/control-plane/runtime/system-truth/sources`
  - 追加一条 `SystemTruthSource`；若未传 `content_hash`，后端按 `content` 生成 `sha256:` 摘要
- `POST /api/control-plane/runtime/system-truth/drafts`
  - 基于 source 追加一条 `SystemTruthDraft`；`asset_id` 省略时继承 source，且显式传入时必须与 source 一致
- `POST /api/control-plane/runtime/system-truth/drafts/:draftID/compile`
  - 基于 draft 追加一条 `SystemTruthCompileResult`；成功 compile 未传 `compiled_payload` 时默认使用 draft content
- `POST /api/control-plane/runtime/system-truth/compile-results/:compileID/activate`
  - 激活成功 compile result，追加一条 audited active pointer；失败 compile 不能 activate
- `POST /api/control-plane/runtime/system-truth/active-versions/:activeID/rollback`
  - 回滚到历史 active version；实现方式是追加新的 active pointer，并记录 `rollback_from_id`，不改写历史 source / draft / compile
- `GET /api/control-plane/runtime/runs`
  - 读取 Phase 1 持久化的 `TaskRun` 列表，支持 `workspace_id`、`status`、`limit`
- `GET /api/control-plane/runtime/runs/:runID`
  - 读取单个持久化 `TaskRun`；缺失时返回 `404`
- `GET /api/control-plane/runtime/runs/:runID/steps`
  - 读取该 run 下的 `TaskStep` timeline
- `GET /api/control-plane/runtime/runs/:runID/lifecycle`
  - 读取该 run 下的 run / step lifecycle events
- `GET /api/control-plane/runtime/runs/:runID/traces`
  - 读取该 run 下的安全 `RuntimeTrace` 摘要，支持 `step_id`、`limit`
- `GET /api/control-plane/runtime/runs/:runID/usage`
  - 读取该 run 下的 generic `Usage` 记录，支持 `step_id`、`limit`
- `GET /api/control-plane/runtime/runs/:runID/projections`
  - 读取该 run 下的 `ProjectionCandidate` 记录，支持 `step_id`、`limit`。
  - 当前读模型会额外暴露 `schema_version`、`semantic_payload`、`artifact_refs`、`ui_hints` 和 `materialization_target`。
  - projection 写入前要求 `schema_version` 使用 `runtime_projection.*`，`materialization_target.core_materialization_scope` 保持 `projection_candidate_only`；candidate/read model 不得声明为业务 `EvidenceRecord`、business evidence、business truth 或 formal business object。
- `GET /api/control-plane/runtime/runs/:runID/checkpoints`
  - 读取该 run 推导出的 checkpoint-backed waiting readout 安全摘要
  - 只返回 `checkpoint_id`、`run_id`、`stage`、`resume_token_present`、`payload_size`、`payload_sha256`、`created_at`、`updated_at`、`snapshot_available` 和 `source`
  - 不返回 Eino private checkpoint payload，也不返回 resume token 原文；没有 checkpoint metadata 时返回空 `items`
- `GET/PUT /api/control-plane/governance`
  - 推荐使用的治理策略控制面入口
- `GET /api/control-plane/tool-governance/policy`
  - 读取由 `tool_governance_policy` system truth 编译出的 effective policy
- `GET /api/control-plane/tool-governance/decisions`
  - 读取最近 persisted tool governance decision log
- `POST /api/control-plane/tool-governance/evaluate`
  - 对一条 validation tool request 产生 allow / deny / allow_with_redaction / require_sandbox_ref 决策并写入 log
- `GET /api/control-plane/config-versions`
  - 返回配置版本快照列表
- `GET /api/control-plane/config-versions/:versionID`
  - 返回单个版本快照及完整配置文档
- `POST /api/control-plane/config-versions/:versionID/rollback`
  - 基于某个历史版本生成一次新的回滚后配置

控制面当前只开放白名单内元数据与治理项：

- scenes
- skills
- tools
- governance
- tool governance policy / decision log
- runtime read API
- config version rollback

控制面当前不开放：

- Runtime read API 当前只读 persisted core runtime objects；不会暴露 Eino checkpoint opaque payload，也不会把 business EvidenceRecord 当作 core truth 返回；projection candidate 只代表 runtime candidate/read model 边界
- Runtime contract foundation read API 只暴露 Athena-owned contract / registry / hook / system truth lifecycle 摘要，不暴露 Eino private callback payload 或任意可执行用户代码
- 原始模型参数
- execution governance 底线
- fact quality / evidence gate 底线
- `structured_result` 主 schema
- platform 主数据

## system-resources 约定

`system-resources` 只认新的单一真相结构：

- `sources/core/SOUL.md`
- `sources/core/AGENTS.md`
- `sources/core/policy_rule/*.md`
- `sources/core/user_profile/*.md`
- `sources/core/memory_view/*.md`
- `sources/scenes/*/SCENE.md`
- `sources/scenes/*/workflow.yaml`
- `sources/scenes/*/contract/*.yaml`
- `sources/scenes/*/policy_rule/*.md`
- `sources/scenes/*/skills/<skill_id>/SKILL.md`
- `sources/scenes/*/skills/<skill_id>/scripts/**`
- `sources/scenes/*/skills/<skill_id>/references/**`
- `sources/scenes/*/skills/<skill_id>/assets/**`

系统资源唯一正式资产 ID 规则：

- `sources/core/SOUL.md` -> `persona.default`
- `sources/core/AGENTS.md` -> `agent_profile.default`
- `sources/core/policy_rule/<id>.md` -> `policy_rule.core.<id>`
- `sources/core/user_profile/<id>.md` -> `user_profile.<id>`
- `sources/core/memory_view/<id>.md` -> `memory_view.<id>`
- `sources/scenes/<scene_id>/SCENE.md` -> `scene.<scene_id>`
- `sources/scenes/<scene_id>/workflow.yaml` -> `workflow.<scene_id>.main`
- `sources/scenes/<scene_id>/contract/<id>.yaml` -> `contract.<scene_id>.<id>`
- `sources/scenes/<scene_id>/policy_rule/<id>.md` -> `policy_rule.<scene_id>.<id>`
- `sources/scenes/<scene_id>/skills/<skill_id>/` -> `skill.<scene_id>.<skill_id>`

## 模型参数策略边界

- 保持 `internal/model` 作为参数策略中心
- 场景、workflow、contract、policy_rule、skill 决定运行时行为边界
- 参数 resolver 不承载第二套业务真相
