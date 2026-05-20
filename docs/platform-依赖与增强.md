# 外部平台集成、增强层与适配

## 这份文档解决什么问题

这份文档专门回答三个问题：

- Athena 在对接外部平台时通常依赖什么
- Athena 的 enhancement layer 可以为应用增强什么
- Athena 作为通用 AI Agent Platform Runtime 能反过来提供什么

它不是完整 API 文档，完整字段仍以 `api.md` 为准。

## 总体边界

Athena 是运行时和 system truth 管理面，不是外部平台主源。

外部平台通常持有：

- 用户、工作区、集成实例等主数据
- 正式知识主源和长期资产
- 工作流正式执行状态机
- 自动化真正创建与执行
- 执行沙盒、调度和审计
- 业务对象、业务规则、业务状态和最终业务真相

Athena 持有：

- 任务归一化
- 场景匹配
- 过程流事件
- 结构化结果包
- 自动化草案
- 参考适配器上下文消费
- 运行时判断和执行治理意图
- repo-managed system truth 的运行时消费

## Enhancement layer 边界

Athena 可以通过 enhancement layer 支持应用更薄地接入，但这些能力不进入 core runtime 语义。

Enhancement layer 可以包含：

- 用户 skill 托管
- 应用知识库托管或引用
- 业务 workflow 和默认流程包
- 场景包与场景默认配置
- provider adapter，例如外部搜索、企业 API、MCP server、connector
- 应用 runtime 判断逻辑

这些增强支持两种 truth ownership：

- `Athena-managed`
  - 应用把用户 skill、应用知识或场景默认配置交给 Athena enhancement layer 管理。
- `App-managed`
  - 应用保留业务真相，Athena 只消费上下文、引用、projection 或 runtime input。

不论选择哪种模式，业务对象、业务规则、最终业务状态和正式业务证据语义默认仍属于应用层。

## Athena 当前对接外部平台时的常见依赖

### 1. 上游请求上下文

Athena 当前默认依赖平台在请求里注入：

- `workspace_id`
- `main_session_id`
- `user_id`
- `integration_instance_id`
- `task_type`
- `global_context`
- `app_context`

### 2. Platform context summary / detail

Athena 当前已经消费：

- `global_context.platform_context_catalog`
- `global_context.platform_context_access`
- `identity_summary`
- `memory_summary`
- `knowledge_summary`
- `skills_summary`
- `persona_summary`
- `persona_context`

平台需要为 Athena 提供：

- summary 数据
- 需要时可被 Athena 回调的 detail endpoint
- 可选的 session-scoped context access token

### 3. 正式执行与自动化落地

Athena 当前可以给出：

- `workflow_plan`
- `automation_plan_draft`
- `automation_create_payload`
- `ExecutionIntent`
- `ExecutionResult`

但真正负责落地的一侧仍然是平台：

- 创建正式自动化
- 执行 workflow
- 运行沙盒
- 回传执行结果

### 4. 正式知识主源

Athena 当前只承接知识执行面和候选，不持有平台的正式知识主库。

## Athena 当前已经给外部平台集成带来的增强

### 1. 用户可见步骤流

`/api/chat/stream` 现在已经支持：

- `progress_step`
- `done.detail.step_flow`

### 2. 统一任务归一化

Athena 当前已经承接并归一化：

- `chat`
- `inspection_task`
- `integration_event`
- `scheduled_job`
- `workflow_step_request`

### 3. Platform context 消费增强

Athena 当前不是只“接收 summary”，而是已经具备：

- 上下文类型选择
- 同轮 detail 预取
- `used_contexts`
- `context_usage`
- `context_details_requested`
- `context_details_loaded`

### 4. 自动化草案与创建 payload

Athena 当前已经能输出：

- `automation_plan_draft`
- `automation_create_payload`
- `user_visible_explanation`
- `interaction_mode`
- `interaction_progress`

### 5. Platform tool hints / descriptors

Athena 当前已经为后续平台工具契约预留结构化接点：

- `platform_tool_hints`
- `platform_tool_descriptors`

### 6. 独立 control-plane 与控制台

Athena 当前已经自带：

- control-plane API
- `web/` React 控制台
- Swagger 可视化和 API debug

## 当前对接时需要平台特别配合的点

### 必选

- 给 Athena 稳定传 `workspace_id`、`main_session_id`
- 明确 `task_type`
- 对接 `/api/chat/stream` 或 `/api/chat/respond`
- 接住结构化结果里的 `automation_create_payload`、`workflow_plan`、`content_cards`

### 强烈建议

- 注入 `platform_context_catalog` 和各类 summary
- 在需要 detail 时提供只读 detail endpoint
- 提供 `platform_context_access.token`

### 可选但收益明显

- 让前端直接消费 `progress_step`
- 把 `done.detail.step_flow` 用作折叠和摘要来源
- 在平台上消费 `platform_tool_hints / platform_tool_descriptors`

## 当前还没有完全闭环的部分

- 平台执行沙盒与 Athena 执行治理 contract 还没有端到端闭环
- 自动化正式创建仍由平台执行，Athena 只给 payload
- 工作流正式执行态仍在平台，不在 Athena
- 知识正式主源和知识版本管理仍在平台

## 推荐阅读顺序

1. `当前能力总览.md`
2. `api.md`
3. `features/feature-platform上下文协同.md`
4. `features/feature-自动化计划与主会话协作.md`
5. `features/feature-用户可见步骤流.md`
