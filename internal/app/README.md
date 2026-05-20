# app

## 模块职责

- `internal/app` 是 Athena 的服务级编排层，负责把 transport 请求接入 session、runtime、skills、models 和治理能力。

## 不负责什么

- 不负责 HTTP 协议映射
- 不负责底层 runtime loop 实现

## 子目录索引

- 当前无子目录

## 文件索引

- `app.go`
  - 定义 `Service` 总编排器和服务装配入口，并把 runtime persistence store 注入默认 Eino Graph 执行面。
- `app_test.go`
  - 验证 `Service` 主编排路径的基础行为。
- `control_plane.go`
  - 封装控制面 scene、skill、tool、governance、system resources、版本快照和认证相关用例。
- `errors.go`
  - 定义 app 层对外暴露的稳定错误类型。
- `fast_path.go`
  - 定义 fast path 扩展点和短路结果结构。
- `model_parameters.go`
  - 负责把请求级模型选择、provider/model override 和参数策略解析为 runtime 可消费的模型参数。
- `model_parameters_test.go`
  - 验证 app 层模型参数解析和 provider/model 选择行为。
- `models.go`
  - 封装模型治理用例，如 provider/model 列表与测试。
- `orchestration_test.go`
  - 验证编排状态和主链 orchestration 行为。
- `runtime_read.go`
  - 提供 Control Plane 读取 Phase 1 runtime persistence objects 的 app-layer read boundary。
- `runtime_scenarios.go`
  - 实现 runtime judgment 路径和首批场景化 runtime 响应编排。
- `runtime_scenarios_test.go`
  - 验证 runtime judgment 路径及其 waiting/decision 行为。
- `runtime_validation.go`
  - 提供 Control Plane runtime validation trigger，通过 Eino Graph foundation 写入一组安全 runtime persistence records。
- `sessions.go`
  - 实现 session 资源列表、详情和治理用例。
- `skills.go`
  - 实现可见 skill、skill package 和治理链用例。
- `skills_test.go`
  - 验证 skill loader 和可见 skill 用例行为。
- `task.go`
  - 实现 chat 请求到 internal task 的归一化入口。
- `task_test.go`
  - 验证 task normalization 行为。

## 对外入口

- `Service`
- `NewService`
- `NewServiceWithRuntimeStore`
- `AnalyzeRuntimeScenario`
- `CreateRuntimeValidationRun`
- `ListRuntimeRuns`
- `OpenChatSession`

## 关键依赖

- 依赖 `runtime/`、`session/`、`skills/`、`model/`
- 被 `server/` 作为应用层入口消费

## 维护提示

- 新接口通常先落 `server/`，但核心业务编排应收口在这里。
- 当前 chat/direct respond 主链通过 `runtime.NewEinoGraphTurnExecutor` 进入 Eino Graph foundation；server 只保留协议映射和兼容响应拼装。
- `runtime_scenarios.go` 体量较大，后续应优先继续拆分。
