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
- `control_plane.go`
  - 暴露控制面 bootstrap、scene/skill/runtime-config 接口和控制面 CORS 处理。
- `openapi.go`
  - 生成和暴露 OpenAPI 文档。
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
