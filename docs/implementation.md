# 当前实现现状与扩展边界

Athena 当前已经不再以“安全产品专用后端”定义自己，而是以“通用 AI Agent Platform Runtime”组织实现。安全仍然是当前最成熟的参考场景包，属于 enhancement layer；外部平台接入是可选适配层，而不是产品本体定义。

## 当前已具备

- HTTP + SSE 主入口
- `/api/chat/stream` 的用户可见 `progress_step` 过程流事件
- `/api/chat/stream` 的 `done.detail.step_flow` 终态步骤流摘要
- `entry / server / app / runtime / session / observability` 基础骨架
- waiting / supplement / resume 主链
- 五类任务统一请求契约的第一阶段接线
- `internal/runtime/scene`、`internal/workflow`、`internal/inspection`、`internal/alerts`、`internal/automation`、`internal/knowledge` 的最小结构骨架
- `internal/controlplane` 的 scene / skill / tool / governance override 与配置版本快照管理骨架
- `internal/runtime/store` 的统一 runtime store 边界骨架
- `internal/runtime/execution` 的执行治理 contract 与风险分类骨架
- `internal/runtime` 的基础能力 contract 与显式接线：
  - `artifact_write`
  - `read_only_resource_read`
  - `structured_data_parse`
  - `local_data_transform`
  - `fact_quality_gate`
  - `query_runtime_state`
- `internal/model` 的模型参数策略中心：
  - `ModelPolicyContext`
  - `ResolvedModelParameters`
  - 参数模板
  - 单点 resolver
  - `internal/model/parameters/` 独立目录
- 完整结果输出所需的 `CompleteResult / ResultSummary / ContentCard / RightPanelView / ScoreDelta` 基础类型
- Eino Graph runtime foundation：
  - 默认 chat/direct respond runtime 主链通过 `runtime.NewEinoGraphTurnExecutor` 包装现有 Eino ADK turn executor
  - 默认 turn agent 内部已使用 graph-native ChatModel / ToolsNode loop，并通过 Eino local state 保存 ReAct 消息历史
  - graph 节点覆盖 context、capability、governance、turn execution、schema validation、projection candidate 和 persistence projection
  - graph callbacks / outputs 可投影到 Phase 1 `TaskRun / TaskStep / RuntimeTrace / Usage / LifecycleEvent / ProjectionCandidate`
  - runtime-private Eino checkpoint store 已接入 graph-native loop；Postgres store 通过 `runtime_graph_checkpoints` 表保存 opaque checkpoint payload 与 safe metadata
- runtime core persistence foundation：
  - Postgres migration/store 已覆盖 `TaskRun`、`TaskStep`、`RuntimeTrace`、generic `Usage`、`TaskRunLifecycleEvent` 和 minimal projection candidate
  - `PersistenceWriter` 提供事务化 deterministic minimal record set 写入
- 严格场景命中、场景切换建议和 `guide questions` 的第一阶段运行时接线
- inspection / alert / automation / knowledge candidates / score delta 在 structured respond 路径上的最小结果壳接线
- 自动化计划草案与用户可读计划说明已开始接入 structured respond 路径
- 自动化交互模式识别与 route 结果已开始接入 structured respond 路径
- `intent_resolution` 已开始接入 structured respond 路径
- `automation_create_payload` 已开始接入自动化草案路径
- `platform_tool_hints / platform_tool_descriptors` 已开始接入适配器结果增强路径
- 一套外部平台上下文 summary / detail 样本已开始接入运行时与结果增强路径
- `knowledge_retrieval` 已开始接入知识候选路径
- 独立控制面接口已开始接入：
  - `/api/control-plane/bootstrap`
  - `/api/control-plane/scenes`
  - `/api/control-plane/skills`
  - `/api/control-plane/tools`
  - `/api/control-plane/runtime-config`
  - `/api/control-plane/runtime/validation-runs`
  - `/api/control-plane/runtime/contracts/foundation`
  - `/api/control-plane/runtime/contracts/:contractID`
  - `/api/control-plane/runtime/task-types/:typeKey`
  - `/api/control-plane/runtime/hook-bindings/:bindingID`
  - `/api/control-plane/runtime/runs`
  - `/api/control-plane/runtime/runs/:runID`
  - `/api/control-plane/runtime/runs/:runID/steps`
  - `/api/control-plane/runtime/runs/:runID/lifecycle`
  - `/api/control-plane/runtime/runs/:runID/traces`
  - `/api/control-plane/runtime/runs/:runID/usage`
  - `/api/control-plane/runtime/runs/:runID/projections`
  - `/api/control-plane/governance`
  - `/api/control-plane/tool-governance/policy`
  - `/api/control-plane/tool-governance/decisions`
  - `/api/control-plane/tool-governance/evaluate`
  - `/api/control-plane/validation-mcp/server`
  - `/api/control-plane/validation-mcp/tools`
  - `/api/control-plane/validation-mcp/invocations`
  - `/api/control-plane/config-versions`
- `web/` React 控制台已开始接入：
  - scene / skill / tool / model / governance 编辑
  - 配置版本查看与回滚
  - 基于 OpenAPI 的通用接口调试页
  - Swagger 嵌入展示
  - Phase 1 runtime persistence read API 调试
  - control-plane 最小登录与锁定状态
  - active truth dir 的 `system-resources` 管理、source 保存后自动 `parse / compile / activate`、debug payload 生成
  - `System Resources` 页面支持显式“遍历 sources 并编译”与“构建 compiled 包”
  - system resource 版本快照、版本详情、审计轨迹与回滚
  - parse / compile 结果下载与 truth dir 快照导出
  - `Models` 页面提供 Provider Readiness，展示 default model 缺失并支持一键设为 default
  - `System Validation` 页面支持 Tool Governance Validation，能触发 validation tool request 并展示 persisted decision log
  - `System Validation` 页面支持 Validation MCP，能展示 `athena-validation-mcp` server、tool schemas、governance decision、result summary 和 redacted trace
  - `System Validation` 页面支持 MCP / Sandbox Validation，能展示 deterministic validation run 的 `external_sandbox_ref` mode、execution ref、structured result、audit summary、sandbox trace 和 projection
  - `System Validation` 页面支持 contract foundation readout，能展示 RuntimeContract、TaskTypeRegistry、HookBinding、active System Truth pointer 和 foundation capability surface
  - `System Validation` 页面支持 foundation JSON 编辑与保存，可直接调用 runtime contract/task type/hook binding 控制面写接口并回读验证
  - runtime foundation snapshot 会在服务启动和 `SyncSystemResources` 后自动同步；新的 runtime validation run 会带出 `runtime_hook_binding` traces 与 `runtime_hook` usage
  - `Release Readiness` 页面使用 bootstrap、system resources、provider/model 和 OpenAPI 数据汇总 v2.0.0 成品门禁，并把 gate 标记为 ready / warning / blocked；页面可直接触发 runtime validation 并展示 run / step / MCP / sandbox 结果

## 当前 system truth 模型

当前仓库已经完成单一真相目录切换：

- 唯一人工维护主源：`config/system/truth/sources/`
- 当前已经真实落地并被编译消费的 baseline 结构：
  - `sources/core/SOUL.md`
  - `sources/core/AGENTS.md`
  - `sources/core/policy_rule/*.md`
  - `sources/core/tool_governance_policy/*.md`
  - `sources/core/user_profile/*.md`
  - `sources/core/memory_view/*.md`
  - `sources/scenes/<scene_id>/SCENE.md`
  - `sources/scenes/<scene_id>/workflow.yaml`
  - `sources/scenes/<scene_id>/contract/*.yaml`
  - `sources/scenes/<scene_id>/policy_rule/*.md`
  - `sources/scenes/<scene_id>/skills/<skill_id>/SKILL.md`
- 当前正式 source 模型收敛为 `sources/core/` + `sources/scenes/<scene_id>/`。
- legacy 顶层资产目录（如 `sources/rule_spec/`、`sources/skill_bundle/`、`sources/skill_summary/`、`sources/user_profile/`、`sources/memory_view/`）不作为 sync 消费入口。

当前 control-plane sync 会把上述主源编译成 `output/system-state/` 下每个 `<asset-id>/` 的 `meta / parse_result / compile_result / pipeline / versions` active state 文件集合；`output/system-assets/` 只用于额外导出的 compiled assets 发布产物，且会被构建命令删除后重建。

当前已经稳定存在的 scene 集合：

- `default`
- `application_dialogue`
- `security_review`
- `inspection`
- `alerts`
- `workflow`
- `knowledge`

## 当前 runtime 上下文装配

`internal/contextassets` 当前统一装配：

- `global_context.context_assets`
- `global_context.context_assets_resolved`
- request-scoped:
  - `context_asset_overrides`
  - `disabled_asset_types`
  - `asset_priority_overrides`

runtime 当前输出统一生效视图：

- `effective_persona`
- `effective_agent_profile`
- `effective_user_profile`
- `effective_memory_view`
- `effective_scene`
- `effective_workflow`
- `effective_policy_rules`
- `effective_contracts`
- `effective_skills`

默认装配模型：

- resident:
  - `persona.default`
  - `agent_profile.default`
  - `user_profile.default`
  - `memory_view.default`
  - `policy_rule.core.*`
- scene-selected:
  - `scene.<scene_id>`
  - `workflow.<scene_id>.main`
  - `policy_rule.<scene_id>.*`
  - `contract.<scene_id>.*`
  - `skill.<scene_id>.*`
- fallback:
  - 未命中明确 scene 时，统一加载 `scene.default`
  - 同时加载 `workflow.default.main`

## 当前 workflow / scene / skill 主链

- scene 真相来自 compiled `scene.*` 资产
- workflow 真相来自 `sources/scenes/*/workflow.yaml` 的编译结果
- skill / contract 当前能力类型已支持，但 baseline 内容仍主要来自 control-plane 与后续补充资产
- workflow engine 以 `workflow.<scene_id>.main` 为正式输入
- rule checkpoint 统一分布在：
  - `pre_inference`
  - `pre_tool_call`
  - `pre_finalize`
  - `pre_candidate_update`
- contract 当前负责：
  - 输出结构约束
  - evidence 完整性约束
  - completion 判定

当前实现边界：

- LLM 负责阶段内理解、分析、表达与结构化生成
- workflow engine 负责阶段流转、waiting / resume / failure
- policy_rule 负责门禁和检查
- contract 负责结构和完成判定

## 当前参考适配器边界

当前已经接入一套外部平台上下文样本：

- `platform_context_catalog`
- `platform_context_access`
- `identity_summary`
- `memory_summary`
- `knowledge_summary`
- `skills_summary`
- `persona_summary`
- `persona_context`

detail 读取当前会在进入 runtime 前尝试同轮预取，并把结果回填到 `global_context`。

这些能力当前保留为参考适配器兼容面，而不是 Athena 内核定义。

## 当前 RuntimeTask 边界

当前 app 层会把 chat/direct respond 等入口归一化为通用 `RuntimeTask` 后再进入 runtime。`inspection_task`、`integration_event`、`scheduled_job`、`workflow_step_request` 保留为 legacy-compatible / future registered semantics；Phase 0 不把这些场景字段作为 core 必填规则，也不实现完整注册式 task type validator。

`/api/runtime/respond` 是 generic direct respond adapter，复用同一 app/runtime 主路径，不代表独立的场景专属 runtime。旧的 `RuntimeScenarioRequest` / `RuntimeScenarioResponse` judgment 流程保留在 `/api/runtime/scenario/respond` 兼容入口，用于承接既有 mosi/OpenClaw 类场景包。

## V1 仍需继续深化的部分

- 注册式任务类型的完整 runtime 行为：
  - `inspection_task`
  - `integration_event`
  - `scheduled_job`
  - `workflow_step_request`
- App 场景上下文装配
- Inspection / Alert / Automation 的真实分析与输出主链
- 知识索引构建与刷新
- 知识候选产出与回收
- `next_questions` 的真实生成策略
- `main_answer / structured_result / result_summary / content_cards / right_panel_view / score_delta` 的更细粒度产出逻辑
- `ExecutionIntent / ExecutionResult` 在显式执行治理场景下的结构化接线
- 共享根目录下的交付物写入能力：
  - 相对路径安全校验
  - owner 分层
  - UTF-8 文本写入
  - 自动版本化
- Athena 执行治理 contract 与外部执行沙盒 contract 的正式分层
- 外部执行宿主侧沙盒模块与结果回传闭环
- 模型参数策略中心的更广接线
- 将剩余 direct respond rich delivery 兼容拼装继续从 transport 层收敛到 app/runtime graph node 或 Batch 2 read model
- Batch 2 read API / UI 中对 checkpoint-backed waiting run 的产品化展示和恢复入口
- 控制面当前只开放白名单内配置项；更细粒度的权限分层和审批流仍可继续深化
- system object 的 Git baseline 回收仍是显式 workflow，不提供 control-plane 一键写 Git

## 工程约束

- 继续采用 `config/config.yml + config/config.<APP_ENV>.yml + 环境变量覆盖`
- 当前配置风格保持与现有仓内实现和外部宿主部署习惯兼容
- 提供 `version` 命令
- 通过 `mkh_utils` 输出版本信息
