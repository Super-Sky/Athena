# Athena 在整体产品架构中的定位

## 定位

Athena 是：

- 通用 AI Agent Platform Runtime
- 可嵌入上层应用的 agent 平台 runtime
- 结构化结果与过程流生产者
- context asset 消费与运行时治理层
- workflow / skill / rule / system truth 的可治理执行面

Athena 不是：

- 某个单一业务产品的全部后端
- 外部平台主数据真源
- 工作流正式执行基础设施
- 宿主应用的最终 UI 产品

## 分层边界

Athena 在整体产品架构中按四层定位：

- `Core`
  - agent runtime、task / step、tool execution、governance、trace、usage、sandbox boundary、system truth、projection 基础能力。
- `Validation`
  - System Validation、验证型 MCP server、deterministic validation flow，用于独立证明 core 能力闭环。
- `Enhancement`
  - 用户 skill、应用知识库、业务 workflow、场景包、provider adapter、应用 runtime 判断逻辑。
- `Application / Business Truth`
  - 业务对象、业务规则、业务状态和最终业务真相，默认由宿主应用持有；也可以通过 enhancement 选择 Athena-managed。


## 与上层应用的关系

Athena 应当位于上层应用 / 平台与模型、工具、运行时治理之间。

更准确地说，上层应用通过集成适配层把自己的上下文、任务和动作需求挂到 Athena 上；Athena 负责运行时推理、治理、状态推进和结果交付。

## 与宿主系统的边界

宿主系统负责：

- 上游应用和集成实例主数据
- 长期业务资产主库
- 正式工作流执行状态机
- 执行沙盒、调度和审计
- 宿主应用的最终用户产品界面
- 业务对象、业务规则、业务状态和最终业务真相

Athena 负责：

- 任务归一化
- scene / intent / workflow 运行时选择
- waiting / resume / deferred queue
- context asset 消费与默认 system bindings
- execution governance
- 结构化结果、过程流和 candidate update
- system truth、control-plane、版本快照、回滚与调试面
- 通过 enhancement 支持用户 skill、应用知识库或业务 workflow 的 Athena-managed 托管模式

Athena 不应因为某个应用选择托管更多资产，就把这些应用资产提升为 core runtime 语义。

## 对未来其他平台的约束

后续新的平台接入也应遵守同一边界：

- 平台保留自己的主数据与正式执行系统
- Athena 提供通用 agent 运行时
- 平台差异通过 adapter 表达，不进入 Athena 核心 contract

## 部署形态

当前推荐部署形态是：

- Athena API
  - 作为上层应用的内部运行时服务
- Athena Web
  - 作为内部控制面和调试面

对最终终端用户是否暴露 Athena Web，不是 Athena 的产品边界要求，而由宿主系统决定。
