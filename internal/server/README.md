# server

## 模块职责

- `internal/server` 负责 HTTP、SSE、OpenAPI 和 transport 层请求映射。

## 不负责什么

- 不负责核心业务编排
- 不负责 runtime 内部算法

## 子目录索引

- `swaggerui/`
  - 本地嵌入的 Swagger UI 静态资源。

## 文件索引

- `http.go`
  - 注册 HTTP 路由、请求结构和大部分 transport handler。
- `http_test.go`
  - 验证 HTTP 路由和 transport 行为。
- `agent_runs.go`
  - 暴露面向业务应用的 Agent Run API，转换 OpenAI-compatible tool contracts，并把 goal-first 请求映射到通用 app/runtime 主链和 runtime persistence trace readout。
- `agent_runs_test.go`
  - 验证 Agent Run 请求解析、budget 校验、tool schema / choice 错误、call/result message 映射以及 create/read/trace/resume/cancel 路由。
- `app_auth.go`
  - 使用独立 app token 和精确 workspace/app-instance scope 保护全部 app-facing Agent Run 路由，并统一资源越权响应。
- `app_auth_test.go`
  - 验证身份缺失、scope 注入/冲突、跨 workspace/app-instance 隔离、同形 404、灰度开关和 trace redaction。
- `external_memory.go`
  - 暴露应用拥有 memory write/query 与 context asset resolve/assemble HTTP 合约。
- `external_memory_test.go`
  - 验证 ownership isolation、摘要资产解析和压缩响应形状。
- `trace_timeline.go`
  - 将既有 runtime step、lifecycle、trace、usage 和 projection 安全投影为 app-facing / Control Plane 共用的统一时间线。
- `trace_timeline_test.go`
  - 验证时间顺序、model/tool/governance 分类、失败信息和路由注册，不创建重复 trace store。
- `control_plane.go`
  - 暴露控制面 bootstrap、scene/skill/runtime-config、remote tool registry 接口和控制面 CORS 处理。
- `remote_tools_test.go`
  - 验证 remote tool 注册、HTTP 执行、治理/trace、重启恢复、删除和 origin 拒绝闭环。
- `openapi.go`
  - 生成和暴露 OpenAPI 文档。
- `runtime_read.go`
  - 将 runtime persistence records 转换为 Control Plane 和 Agent Run 共用 DTO，并递归脱敏凭据型 metadata/payload 键。
- `request.go`
  - 定义和处理 transport 层请求结构辅助逻辑。
- `respond.go`
  - 处理 chat/respond 相关 transport 逻辑。
- `respond_test.go`
  - 验证 respond 路径行为。
- `runtime_scenarios.go`
  - 处理 runtime judgment 路径的 HTTP 接入。
- `swagger_assets.go`
  - 装配 Swagger 相关静态资源暴露能力。

## 对外入口

- `NewHTTPServer`

## 关键依赖

- 依赖 `app/`
- 被 `entry/` 装配为对外服务入口

## 维护提示

- `http.go` 当前体量很大，新增接口时要考虑是否值得继续拆分。
- 协议字段变化时，需同步检查 OpenAPI、测试和 docs/api.md。
