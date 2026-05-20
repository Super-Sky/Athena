# 项目定位与范围

## 1. Athena 是什么

Athena 是一个通用 AI Agent Platform Runtime，用于为上层应用提供：

- 统一任务归一化
- 上下文资产装配
- 执行治理
- session / waiting / resume
- 结构化结果与过程流交付
- 可治理的 system truth 与 control-plane
- 独立验证能力

Athena 的目标不是绑定某个单一业务，而是成为可嵌入上层应用的通用 agent 平台 runtime。

安全是当前第一套明确验证过的参考场景包，不再是产品本体定义。外部平台接入是可选适配层，不再是 Athena 的设计中心。

## 1.1 能力分层

Athena 后续规划和功能细化默认按四层判断：

- `Core`
  - 不做就不是通用 AI agent 平台的跨应用基础能力。
  - 例如 runtime、task / step、tool execution、governance、trace、usage、sandbox boundary、system truth、projection 基础能力。
- `Validation`
  - 不做就无法独立验证 core 是否真实工作。
  - 例如 System Validation、验证型 MCP server、deterministic validation flow。
- `Enhancement`
  - 不做不影响平台成立，但能让应用更快搭建或更完整。
  - 例如用户 skill 接管、应用知识库、业务 workflow、场景包、provider adapter、应用 runtime 判断逻辑。
- `Application / Business Truth`
  - 业务对象、业务规则、业务状态、最终业务真相。
  - 默认属于应用层；Athena 可通过 enhancement 支持 Athena-managed 或 app-managed 两种模式。

核心原则是：Athena core 只承载跨应用必需的平台能力，增强层可以做厚，但不能反向污染 core runtime。

## 2. Athena 不是什么

Athena 不是：

- 产品前后端
- 页面模板系统
- 外部平台主数据真源
- 长期知识主库
- 工作流正式执行主控
- 共享工程规格主仓

Athena 可以消费这些外部系统提供的输入，但不默认拥有它们。

## 3. 当前输入范围

Athena 当前至少承接：

- `chat`
- `inspection_task`
- `integration_event`
- `scheduled_job`
- `workflow_step_request`

这些输入都应被归一化到通用 `RuntimeTask` 运行时壳中，而不是让上游平台契约直接渗透到核心。除 `chat` 外的场景化 `task_type` 当前仅作为兼容旧调用和未来注册式语义的提示，不代表 core-native 枚举。

## 4. 核心原则

- 核心内核保持场景无关、业务无关
- 场景能力优先通过 system truth / workflow / skill / rule 表达
- 平台差异优先通过 adapter 层承接
- 输出优先结构化、可审计、可恢复
- 知识、记忆、画像等长期输入优先抽成 context assets，而不是散落字段
- 工作流、规则、技能优先做成可治理资产，而不是写死在主链逻辑里

## 5. 当前的参考场景与参考适配器

### 5.1 安全场景

安全当前是 Athena 的第一套参考场景包，用于验证：

- inspection / alerts / workflow
- 风险判断与解释
- 更严格的治理和审计要求

它证明 Athena 可以承接复杂领域，但不再定义 Athena 的全部边界。

### 5.2 外部平台适配样本

Athena 当前已经验证过一套外部平台适配样本，用于验证：

- 上游上下文注入
- detail 回读
- action payload 回写
- 外部平台与 Athena 之间的集成边界

Athena 可以继续兼容已有接入，但后续不再为某个平台单独定义产品真相。

## 6. System truth 与业务真相

Athena 管理最终 system content 和平台级 system truth，包括 system prompt、system policy、tool governance、runtime rules、system skills、system knowledge、validation contracts 等。

用户层和业务层 truth 支持两种 ownership 模式：

- `Athena-managed`
  - 应用把用户 skill、业务知识、业务配置或场景默认逻辑交给 Athena enhancement layer 托管。
- `App-managed`
  - 应用保留业务数据真相，Athena 只消费上下文、引用、projection 或 runtime input。

这两种模式都不能改变 core 边界：业务对象、业务状态、最终业务规则和正式业务 evidence 语义默认仍属于应用层。

## 7. 当前知识与上下文策略

Athena 当前持有的是运行时上下文与执行面，不是外部正式主库。

当前长期保留在 Athena 的是：

- system truth
- context assets
- 知识执行面、引用面与运行时缓存
- 与 session 绑定的上下文连续性

当前不默认持有的仍然是：

- 外部平台长期主数据
- 正式知识主库
- 宿主应用用户可见的业务真源
- 用户层 / 业务层正式 skill 主库，除非应用显式选择 Athena-managed enhancement 模式

## 8. 核心范围控制

如果应用需要 Athena 接管用户 skill、业务知识库、业务 workflow、行业规则或应用特定判断逻辑，应优先放在 enhancement layer / extension package 中，而不是进入 core runtime。

判断标准：

- 去掉具体应用后仍然必须存在的能力，才可能进入 `Core`。
- 用于证明 core 真实工作的能力，进入 `Validation`。
- 能提升应用搭建效率但不决定平台是否成立的能力，进入 `Enhancement`。
- 业务真相、业务对象和最终业务状态，留在 `Application / Business Truth`。
