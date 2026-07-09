# Remote Business Tools

## 背景 / Background

Athena 需要让 `athena-fund-assistant` 等独立业务服务提供工具实现，同时保持业务 schema、数据源与业务判断留在应用仓库。issue `Super-Sky/Athena#9` 为此提供通用 remote registry 与 HTTP execution surface。

Athena owns registration governance, runtime execution, safe trace and usage. The host app owns the tool schema implementation, business data and returned business content.

## 契约 / Contract

控制面接口 / Control-plane APIs:

- `GET /api/control-plane/remote-tools`
- `PUT /api/control-plane/remote-tools/:name`
- `DELETE /api/control-plane/remote-tools/:name`

执行 callback 使用 `remote_tool_execution.v1`：

- 请求包含 `request_id`、`tool_call_id`、`registration_id`、`app_id`、`tool_name`、JSON object `arguments`、`attempt` 和安全 metadata。
- 响应必须回传相同 `request_id` / `tool_call_id`，并返回 `status=ok` + `content`，或标准化 `error.code/message/retryable`。
- 注册不保存 credentials。MVP 面向同一受控部署网络；需要身份凭证的外部 callback 应在后续引入 secret reference，而不是把 token 写入注册文档。

## 执行顺序 / Execution Order

1. Control Plane 持久化注册，并在启动时恢复 enabled registrations。
2. Runtime resolver 与 Eino executor 对每次 operation 读取同一动态 catalog 快照。
3. Remote adapter 在任何网络请求前执行 tool governance。
4. `deny`、`allow_with_redaction`、`require_sandbox_ref` 与 `allow` 在 adapter 边界落实；未知 decision fail closed。
5. HTTP callback 受 exact-origin allowlist、30 秒最大超时、1 MiB 默认响应预算、最多三次重试和 redirect 禁止约束。
6. Side-effecting non-idempotent tools 不允许配置重试。
7. Canonical tool transcript 记录 call/result/error；remote observer 额外记录 endpoint origin、attempt、duration、decision ID、status 和 normalized error code，不记录 raw arguments/results。

## 配置 / Configuration

- `REMOTE_TOOL_ALLOWED_ORIGINS`: comma-separated exact origins，例如 `http://fund-api:8081,http://127.0.0.1:8081`。
- `REMOTE_TOOL_MAX_RESPONSE_BYTES`: 单次 callback 最大响应字节数，默认 `1048576`。

默认 allowlist 为空，因此未显式配置时所有 remote endpoint 注册都会失败。The default is deny-all.

## 边界 / Boundaries

Athena core 负责通用 registry、execution envelope、governance、network policy、trace 和 usage。基金行情、持仓、交易日记、建议策略以及数据供应商适配继续属于 `athena-fund-assistant`。

Remote tools do not grant permission to move money or perform automatic trading. Side effects remain governed and must be declared explicitly.

## 验证 / Verification

```bash
env -u APP_ENV go test ./...
go test -race ./internal/tools ./internal/runtime ./internal/app ./internal/server
go vet ./internal/tools ./internal/app ./internal/server
python3 scripts/check_no_absolute_paths.py
```

`internal/server/remote_tools_test.go` 使用本地 fake business service 验证 authenticated surface 下的注册、callback 执行、治理 decision、safe trace、重启恢复与删除。`internal/tools/remote_test.go` 覆盖 redaction、timeout、retry、correlation、response budget、redirect 与 deny-before-network。

## Skill 结论 / Skill Decision

暂不新增独立 feature skill。该能力已由 `eino-agent`、`eino-component`、code style 和仓库交付 skill 覆盖；稳定维护入口是本 feature 文档与 API/architecture/implementation/module README。
