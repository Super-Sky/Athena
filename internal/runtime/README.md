# runtime

## 模块职责

- `internal/runtime` 是 Athena 的核心执行骨架，负责 ExecutionSpec、loop 控制、等待态和执行引擎编排。

## 不负责什么

- 不负责 HTTP 协议接入
- 不直接承载上层产品页面语义

## 子目录索引

- `task/`
  - internal task 模型与 task normalization 相关代码。
- `persona/`
  - 用户级 persona context 解析与表达风格 guidance 代码。
- `intent/`
  - 通用交互意图、route 和 clarification 解析代码。
- `scene/`
  - 场景命中、场景切换建议和相关契约。
- `store/`
  - 统一 runtime store 边界与可重建运行时状态契约。

## 文件索引

- `eino.go`
  - 提供基于 Eino 的默认执行器适配，并把模型、工具和 instruction 装配到 graph-native agent。
- `eino_checkpoint.go`
  - 定义 Athena `WaitState` / resume token 到 Eino checkpoint ID 的确定性映射，并提供 runtime-private Eino checkpoint byte store 边界与内存实现。
- `eino_callback_recorder.go`
  - 采集 graph-native model/tool callback 的安全摘要，不保存 raw prompt、raw tool arguments 或 raw tool result。
- `eino_callback_projector.go`
  - 将已采集 callback 摘要写入 `RuntimeTrace` 和 generic `Usage`。
- `eino_graph.go`
  - 提供 Eino Graph runtime foundation，把当前单轮执行器包装为 graph node 边界，并把 graph 输出投影到 runtime persistence。
- `eino_graph_test.go`
  - 验证 graph node 边界、Eino callback 采集、persistence projection、terminal projector、checkpoint ref 和 graph-wrapped turn executor。
- `eino_native_agent.go`
  - 实现默认 graph-native ChatModel / ToolsNode ReAct loop，并通过 Eino local state 保留模型与工具往返历史；checkpoint store 可用时会启用 Eino checkpoint / interrupt / resume。
- `eino_native_agent_test.go`
  - 验证 graph-native model-only、tool-loop、ReAct state 传递、工具中断和 checkpoint resume 行为。
- `base_capabilities.go`
  - 定义交付物写入、只读资源读取、结构化解析和 runtime 状态查询 contract。
- `base_capabilities_test.go`
  - 验证路径安全、UTF-8 文本写入、结构化解析和 runtime 状态查询行为。
- `execution.go`
  - 定义执行治理 contract、风险分类和显式执行意图解析逻辑。
- `execution_test.go`
  - 验证执行治理 contract 和显式执行场景的最小行为。
- `interfaces.go`
  - 定义 runtime 关键抽象接口。
- `persistence.go`
  - 定义 TaskRun、TaskStep、RuntimeTrace、Usage、LifecycleEvent、ProjectionCandidate 和 RuntimeContract 的核心持久化契约。
- `postgres_persistence.go`
  - 实现 PostgreSQL 版 runtime persistence store、RuntimeContract store、checkpoint store 与 migration model。
- `persistence_writer.go`
  - 提供内部 deterministic writer，用于写入最小 runtime persistence record set。
- `terminal_projector.go`
  - 将 runner 最终输出、失败终态、graph callback 摘要、model/tool callback 摘要和 generic usage 安全投影到 Phase 1 runtime persistence objects。
- `results.go`
  - 定义完整结果包、摘要、卡片、右栏和成长信号等通用结果结构。
- `service.go`
  - 实现 runtime 主链和默认编排逻辑。
- `service_test.go`
  - 验证 runtime 主链行为。
- `types.go`
  - 定义 ExecutionSpec、Action、ProcessingSpec 等核心类型。

## 对外入口

- `NewService`
- `NewEinoGraphTurnExecutor`
- runtime interfaces
- runtime types

## 关键依赖

- 被 `app/` 直接消费
- 依赖 `policy/`、`session/`、`skills/`、`tools/`

## 维护提示

- `service.go` 体量较大，新增能力时优先考虑拆分而不是继续堆叠。
- Eino Graph 是当前默认执行承载面；剩余手写编排逻辑应逐步收敛为 graph node 或明确标记为兼容边界。
- 类型和接口变更通常会联动 `app/`、`server/` 和测试。
