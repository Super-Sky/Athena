# validationmcp

## 模块职责

- `internal/validationmcp` 承载 Athena Validation layer 的轻量 `athena-validation-mcp` 适配器。
- 它负责确定性 tool schema listing、安全 tool invocation、whitelist-safe trace 和 credential-like payload redaction。

## 不负责什么

- 不实现标准 MCP transport。
- 不接外部 MCP registry。
- 不承载 core runtime truth 或业务 evidence truth。
- 不执行 external sandbox。

## 子目录索引

- 当前没有子目录。

## 文件索引

- `types.go`
  - 定义 validation MCP server、tool schema、invocation request、invocation result 和 trace 的 JSON contract。
- `server.go`
  - 实现内置 `athena-validation-mcp` server、两个确定性工具和 schema 列表 / 调用入口。
- `redaction.go`
  - 提供 credential-like key 的递归脱敏与通用 map clone helper。
- `server_test.go`
  - 验证 schema listing、deterministic invocation 和 credential redaction。

## 对外入口

- `NewServer`
- `Server.Info`
- `Server.ListTools`
- `Server.Invoke`
- `RedactSensitiveMap`

## 关键依赖

- 被 `app/` 消费，用于在 control-plane validation path 中先走 tool governance，再执行确定性 validation MCP tool。
- 被 `server/` 通过 control-plane handlers 暴露给后台管理页面。

## 维护提示

- 新增工具时必须同时补 schema、deterministic handler、redaction 回归和 System Validation 展示路径。
- 该目录只保存 validation MCP 的安全验证能力；真实业务工具应继续留在应用层或 `internal/tools` 等既有 tool 边界内。
