# internal

## 模块职责

- `internal/` 承载 Athena 的核心运行时代码。
- 它负责请求编排、运行时执行、能力治理、传输协议、会话存储和观测等内部实现。

## 不负责什么

- 不负责版本化需求文档与实施计划
- 不负责仓库治理规则正文
- 不负责产品层页面与长期资产主库

## 子目录索引

- `app/`
  - 用例编排层，负责请求进入 Athena 后的服务级 orchestration。
- `config/`
  - 配置加载、解析和配置结构定义。
- `customization/`
  - 请求级 customization 数据结构和解析入口。
- `controlplane/`
  - 独立控制面的文件化 override、有效配置视图和读写管理逻辑。
- `entry/`
  - 进程启动装配、命令入口和依赖创建。
- `memory/`
  - 基于内存的上下文/记忆相关实现。
- `model/`
  - 模型供应商、模型治理和密钥存储相关能力。
- `observability/`
  - trace、metrics、audit 等可观测能力入口。
- `inspection/`
  - 体检进度事件与结构化报告契约。
- `alerts/`
  - 结构化告警契约。
- `automation/`
  - 自动化任务与候选基础契约。
- `policy/`
  - 能力治理和策略判断的基础抽象。
- `knowledge/`
  - 知识检索命中、候选更新与运行时计数契约。
- `postgresutil/`
  - PostgreSQL 相关的通用重试和底层辅助能力。
- `runtime/`
  - 核心执行骨架、ExecutionSpec、等待态与 loop 控制。
- `sandbox/`
  - Core sandbox boundary 契约，当前提供最小 `external_sandbox_ref` validation result。
- `runtime/scene/`
  - 场景命中、场景切换建议与场景运行时契约。
- `runtimeassets/`
  - 运行时任务资产和 skill 元数据 registry。
- `workflow/`
  - 结构化工作流计划与步骤契约。
- `server/`
  - HTTP、SSE、OpenAPI 和 transport 映射层。
- `session/`
  - session store 及 waiting/resume 持久化能力。
- `skills/`
  - skill registry、package、loader 与治理链。
- `tools/`
  - tool registry 和 tool middleware。
- `validationmcp/`
  - Validation layer 的轻量 `athena-validation-mcp` 适配器，用于验证 tool schema、governance decision、安全调用结果和 redacted trace。

## 文件索引

- 当前以子目录为主，顶层不直接承载业务源码文件。
- 后续若 `internal/` 顶层新增源码文件，应在这里补充全文件索引，而不是只依赖目录名。

## 对外入口

- `entry/`
  - 进程级入口装配
- `server/`
  - transport 对外暴露入口
- `app/`
  - 服务编排主入口
- `runtime/`
  - 执行骨架主入口

## 关键依赖

- 被 `main.go` 通过 `internal/entry` 间接装配和消费。
- 大部分目录会跨层依赖 `runtime`、`session`、`model`、`skills` 和 `tools`。

## 常见修改路径

- 新接口或协议：优先看 `server/` + `app/` + `runtime/`
- 新运行时能力：优先看 `runtime/` + `app/`
- 新 sandbox boundary：优先看 `sandbox/` + `runtime/` + `app/`
- 新 skill / tool 治理：优先看 `skills/` + `tools/`
- 新 validation MCP 验证能力：优先看 `validationmcp/` + `app/` + `server/`
- 新静态任务资产：优先看 `runtimeassets/`

## 维护提示

- 新增或拆分子目录时，先更新这里的子目录索引。
- 若某个子目录开始承担稳定职责，应补自己的 `README.md`，而不是只在这里保留一句话描述。
