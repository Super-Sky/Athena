# context

## 模块职责

- `internal/extensions/platform/context` 负责解析 platform 注入的 catalog 与 summary，在 summary 不足时同轮预取 detail，并输出 Athena 可消费的上下文使用决策。

## 不负责什么

- 不持有 platform 主数据
- 不替代 runtime 主线的 scene / intent / policy 逻辑

## 文件索引

- `context.go`
  - 解析 `platform_context_catalog`、`platform_context_access`、各类 summary / detail，并生成 `used_contexts / context_usage / context_details_requested / context_details_loaded` 和 guidance。
- `client.go`
  - 根据 catalog 或配置的 base URL 同轮读取 platform detail，并把 detail 回填到 `global_context`；读取时优先透传 context access token。
- `client_test.go`
  - 验证 platform detail 同轮预取与 header 透传。
- `context_test.go`
  - 验证上下文选择、detail 请求和领域提示的确定性。
