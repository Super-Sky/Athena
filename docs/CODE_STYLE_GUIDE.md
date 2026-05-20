# Athena 代码风格与实现约定

## 0. 最高标准

以下要求是本仓库当前实现与入库的最高标准，应优先于“只要能跑通就行”的思路：

- 双语注释优先：
  - 核心方法、核心类型、核心变量、核心常量、关键状态机分支，都应补英文在前、中文在后的双语注释。
  - 双语注释必须分两行书写，不接受单行混排。
  - 注释重点解释职责、边界、状态转换和设计原因，不解释显然语句。
- 测试先于入库：
  - 默认至少补齐受影响范围的单元测试。
  - 涉及协议、SSE、等待态、会话状态机、多步回流时，默认补系统测试或等价集成测试。
- 性能基线先于入库：
  - 关键链路默认补 benchmark 或复用已有 benchmark 重新验证。
  - 需要持续维护性能基线，避免每次提交都把主链变慢却无感知。
- 内存观察先于入库：
  - 涉及 queue、session、history、memory、stream buffer、缓存或长等待状态时，默认观察内存占用与对象滞留风险。
- 达标后再提交：
  - 代码、注释、测试、性能和必要的内存观察未达标前，不应进入正式提交或正式推送。

本规范目标是两件事同时成立：

- 保持当前最小 Agent 脚手架的结构清晰，不把业务逻辑重新堆回入口层。
- 在继续扩展模型、tool、session 和 SSE 能力时，维持可读、可测、可演进。

## 1. 适用范围

- 语言：Go
- 运行形态：HTTP + SSE Agent 服务
- 核心目录：
  - `main.go`
  - `internal/entry`
  - `internal/app`
  - `internal/runtime`
  - `internal/server`
  - `internal/model`
  - `internal/skills`
  - `internal/tools`
  - `internal/session`
  - `internal/memory`
  - `internal/policy`
  - `internal/observability`

## 2. 总体原则

- 入口薄：`main.go` 只保留启动流程，不承载业务逻辑。
- 入口分层：`main.go` 只保留启动流程，根依赖装配优先收口到 `internal/entry`。
- 用例编排：请求级路径选择、会话打开和补数回流统一放 `internal/app`。
- 运行骨架：`internal/runtime` 负责 `ExecutionSpec`、等待态和 Eino 执行封装。
- 接口清晰：HTTP 层只做请求解析、上下文注入、SSE 映射和错误返回。
- 兼容优先：修改 SSE 事件或 API 字段时，先评估兼容性，再改文档。
- 最小必要改动：不为“看起来更漂亮”做无关重构。
- 可观测优先：涉及 tool、stream、session、并发行为的改动，优先补日志和验证。

## 3. 分层职责

- `main.go`：读取配置并启动系统。
- `internal/entry`：根依赖装配与入口收口。
- `internal/config`：环境变量读取、默认值和启动校验。
- `internal/app`：请求级路径编排、会话打开、请求槽位与运行入口。
- `internal/runtime`：稳定执行骨架、`ExecutionSpec`、`TurnExecutor`、等待态与 loop 控制。
- `internal/server`：HTTP 路由、请求级上下文、SSE 事件映射、错误输出。
- `internal/model`：模型提供方接入与兼容差异收口。
- `internal/skills`：官方 skill 注册与适配入口。
- `internal/tools`：具体 tool 与 tool middleware。
- `internal/session`：会话存储接口与实现。
- `internal/memory`：上下文压缩、历史准备策略。
- `internal/policy` / `internal/customization`：能力边界、自定义输入、补数许可和等待超时边界。
- `internal/observability`：事件、trace、metrics、audit 与 debug artifacts 入口。

目录边界的“能做什么 / 不能做什么”正式约束，统一参考：

- `docs/v0.1.0/features/feature-目录边界.md`

## 4. Go 代码规范

- 强制 `gofmt`。
- 错误必须显式处理，不要静默忽略。
- 早返回优先，避免深层嵌套。
- 命名遵循 Go 习惯：`ID`、`URL`、`API`，不要混用 `Id/Url/Api`。
- 核心方法、核心类型、核心变量、核心常量默认补双语注释，格式统一为：
  - `// English sentence.`
  - `// 中文句子。`
- 注释只解释边界条件、业务约束和设计原因，不解释显然代码。
- 新增导出类型或导出函数时，补充职责清晰的双语注释。
- 并发代码必须避免共享可变状态的隐式竞争。

## 5. 文件与目录规范

- 文件命名使用 `snake_case.go`。
- 与请求级编排无关的具体业务逻辑，不要塞进 `app.go`。
- `ExecutionSpec`、`TurnResult`、等待态等运行时对象统一收口在 `internal/runtime`。
- 模型供应商兼容差异只放模型层，不要散落到 server/tool/app。
- SSE 事件结构统一收口在 `internal/server`，不要多处各自拼装。
- demo tool 可以保留在 `internal/tools`，真实业务 tool 也应按能力继续拆分在该目录。

## 6. API 与 SSE 规范

- 健康检查只做活性检查，除非明确设计变更，不额外探测下游依赖。
- 主聊天入口保持单一：`/api/chat/stream`。
- 缺失信息、等待态和超时结果都通过统一 SSE 事件暴露，不单独发明旁路协议。
- 新增事件类型或修改事件字段时：
  - 先评估前端兼容影响。
  - 同步更新 `docs/api.md`。
  - 如涉及执行链变化，同步更新 `docs/architecture.md` 或 `docs/implementation.md`。
- 请求级字段优先保持稳定：
  - `request_id`
  - `session_id`
  - `type`

## 7. Tool 与模型扩展规范

- tool 描述必须明确“何时该用该 tool”。
- `Skill` 是高层能力包，`Tool` 是原子能力；不要反向让 tool 承担 skill 路由。
- 多个独立查询可并发时，优先保持模型提示词与执行策略一致。
- tool 生命周期事件应保持可观测，避免新增黑盒执行。
- 新增模型提供方时，优先保持统一返回 `ToolCallingChatModel`。
- 供应商特有兼容逻辑只能放 `internal/model`。

## 8. Session 与记忆规范

- session 存储层通过接口抽象，避免 HTTP 层直接依赖具体实现。
- 上下文压缩策略变更时，必须评估历史消息语义是否被破坏。
- 引入持久化 session 或长期记忆时，不要绕过现有 `Runtime` 主链。
- 外部补数回流优先通过 `app -> runtime` 重入，不要让 server 直接操纵会话状态机。

## 9. 测试与最小验证

- 变更 Go 代码后，至少执行受影响范围的编译或测试验证。
- 涉及核心路径的改动，不只要求“能编译”，还要有对应单测。
- 涉及协议、SSE、状态机、多轮回流时，默认补系统测试或等价集成测试。
- 涉及性能敏感链路时，默认补 benchmark 并记录当前基线。
- 涉及 queue、session、memory、缓存或长等待对象时，默认观察一次内存占用或对象滞留风险。
- 性能与内存的默认检查步骤统一参考：
  - `docs/v0.1.0/features/feature-可观测验证指南.md`
- 涉及 SSE 事件改动时，至少验证一次流式接口输出顺序和关键字段。
- 涉及 tool 并发、session 复用、context compression 时，优先补最小可重复验证步骤。
- 涉及 `information_request`、等待超时、降级兜底时，至少验证一条等待路径。
- 如果无法运行验证，必须在交付说明里明确写出原因。

## 10. AI 协作自检

- [ ] 没有把业务逻辑塞进 `main.go` 或 HTTP 层
- [ ] 没有把请求级路径分流、补数回流或等待控制塞进 server 层
- [ ] 没有把模型兼容逻辑散落到非模型层
- [ ] 没有把 `ExecutionSpec`、`WaitState` 等运行态对象散落到多个目录
- [ ] 没有无文档地修改 API 或 SSE 契约
- [ ] 没有破坏 request/session/tool 的可观测字段
- [ ] 核心代码已补齐必要的双语注释
- [ ] 已完成单测、系统测试、性能验证，并在需要时观察了内存行为
- [ ] 已执行最小验证或明确说明未验证原因
