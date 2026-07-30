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
- 注册只保存 `auth.type`、`auth.secret_ref` 和可选 `auth.header_name`，不保存 credential value。The persisted contract contains only a provider reference; the credential is resolved at the outbound network boundary.
- `bearer` 固定注入 `Authorization: Bearer <resolved value>`；`header` 只允许注入有效 `X-*` header，防止覆盖 Host、Cookie 等协议头。

## 执行顺序 / Execution Order

1. Control Plane 持久化注册，并在启动时恢复 enabled registrations。
2. Runtime resolver 与 Eino executor 对每次 operation 读取同一动态 catalog 快照。
3. Remote adapter 在任何网络请求前执行 tool governance。
4. `deny`、`allow_with_redaction`、`require_sandbox_ref` 与 `allow` 在 adapter 边界落实；未知 decision fail closed。
5. 通过治理后，runtime resolver 按 `secret_ref` 即时解析凭据并校验 revoked / expired 状态；解析失败或无效凭据 fail closed，且不会触发 callback。
6. HTTP callback 受 exact-origin allowlist、30 秒最大超时、1 MiB 默认响应预算、最多三次重试和 redirect 禁止约束。
7. Side-effecting non-idempotent tools 不允许配置重试。
8. Canonical tool transcript 记录 call/result/error；remote observer 额外记录 origin、attempt、duration、decision、`auth_type`、`secret_ref`、`auth_result` 和 normalized error code，不记录 credential、raw arguments/results。

## 配置 / Configuration

- `REMOTE_TOOL_ALLOWED_ORIGINS`: comma-separated exact origins，例如 `http://fund-api:8081,http://127.0.0.1:8081`。
- `REMOTE_TOOL_MAX_RESPONSE_BYTES`: 单次 callback 最大响应字节数，默认 `1048576`。

MVP secret provider 使用 `env://VARIABLE_NAME`。Athena 每次调用都重新读取 `VARIABLE_NAME`，因此可即时轮换；可选 `VARIABLE_NAME_REVOKED=true` 与 RFC3339 `VARIABLE_NAME_EXPIRES_AT` 用于撤销和过期。These values belong to the runtime secret environment and must not be written into registration JSON, logs, traces, or committed config.

注册示例 / Registration example:

```json
{
  "auth": {
    "type": "bearer",
    "secret_ref": "env://REMOTE_TOOL_CALLBACK_TOKEN"
  }
}
```

稳定错误 / Stable errors:

- `remote_auth_secret_unavailable`
- `remote_auth_secret_revoked`
- `remote_auth_secret_expired`
- `remote_auth_secret_invalid`

错误 credential 已发送到 callback 时，由业务服务以 `remote_tool_execution.v1` error response 返回业务中立的拒绝码，例如 `callback_auth_rejected`。

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

`internal/server/remote_tools_test.go` 使用本地 fake business service 验证注册、鉴权 callback、轮换、撤销、safe trace、重启恢复与删除。`internal/tools/remote_test.go` 覆盖 bearer/custom header 注入、missing/revoked/expired/invalid secret、callback rejection、redaction、timeout、retry、correlation、response budget、redirect 与 deny-before-network。

## Skill 结论 / Skill Decision

暂不新增独立 feature skill。该能力已由 `eino-agent`、`eino-component`、code style 和仓库交付 skill 覆盖；稳定维护入口是本 feature 文档与 API/architecture/implementation/module README。
