# Agent Run App Authentication / Agent Run 应用认证

## Background / 背景

Issue `Super-Sky/Athena#26` closes the production authorization gap on app-facing Agent Run create/read/resume/cancel/events/trace/timeline routes. A run ID is a locator, never an authorization credential.

Issue `Super-Sky/Athena#26` 补齐面向应用的 Agent Run create/read/resume/cancel/events/trace/timeline 生产授权边界。run ID 只是定位符，不能作为授权凭据。

## Contract / 契约

When `APP_AUTH_REQUIRED=true`, all app-facing `/api/agent/runs` routes require:

- `X-Athena-App-Token`: application secret; a dedicated header prevents accidental Platform Context forwarding.
- `X-Athena-App-ID`: configured application identity.
- `X-Athena-Workspace-ID`: requested workspace scope.
- `X-Athena-App-Instance-ID`: requested application-instance scope.

`APP_AUTH_IDENTITIES_JSON` binds each app ID/token to exact workspace and app-instance pairs. Missing or invalid identity returns `401`. A valid identity outside the requested scope, a cross-tenant run, an unowned legacy run, and a nonexistent run all return the same `404 {"error":"resource_not_found"}` shape.

`APP_AUTH_IDENTITIES_JSON` 将 app ID/token 绑定到精确的 workspace 与 app-instance 组合。身份缺失或无效返回 `401`。合法身份访问 scope 外资源、跨租户 run、无法证明归属的历史 run 或不存在 run 时，统一返回同形的 `404 {"error":"resource_not_found"}`。

Create requests may omit workspace/app-instance in the body; Athena injects the authenticated scope. Conflicting body values return `403`. Resume first authorizes the original run and makes the follow-up run inherit its ownership.

创建请求可以省略 body 中的 workspace/app-instance，Athena 会注入已认证 scope；body 冲突返回 `403`。Resume 会先授权原 run，并让 follow-up run 继承原归属。

## Safety / 安全边界

- Application tokens never use `Authorization`, so they cannot enter the existing authorization-forwarding path for Platform Context.
- Tokens are compared in constant time and are excluded from JSON serialization.
- Runtime metadata, labels and payload maps recursively redact credential-like keys for both app and Control Plane DTOs.
- App-facing full trace additionally removes projection `semantic_payload`; timeline continues to expose only redacted payload and safe metadata.
- Existing authenticated Control Plane sessions are treated as `system-admin`. Workspace-operator roles are not simulated; adding role/workspace claims remains a separate RBAC evolution.

- 应用 token 不使用 `Authorization`，因此不会进入 Platform Context 的既有 authorization 转发链。
- Token 使用常量时间比较，并禁止出现在 JSON 序列化结果中。
- App 与 Control Plane 共用 DTO 会递归脱敏 metadata、labels 与 payload map 中的凭据型键。
- App-facing 完整 trace 额外移除 projection `semantic_payload`；timeline 只继续展示 redacted payload 与安全 metadata。
- 现有已认证 Control Plane session 明确定义为 `system-admin`。当前不伪造 workspace operator；角色与 workspace claims 属于后续 RBAC 演进。

## Rollout / 上线

`APP_AUTH_REQUIRED=false` is an explicit gray-deployment mode. Identities can be distributed and validated before switching the gate on. Production deployment examples enable the gate and require distinct app/admin secrets.

`APP_AUTH_REQUIRED=false` 是明确的灰度模式，可先下发并验证 identities，再开启门禁。生产部署样例默认开启门禁，并要求 app/admin 使用不同 secret。

Runs created before ownership fields were persisted are fail-closed for app APIs and remain visible only to authenticated Control Plane system administrators. Backfill ownership only when provenance can be proven; empty ownership must never act as a wildcard.

缺少 ownership 字段的历史 run 对 app API fail-closed，仅允许已认证的 Control Plane system-admin 查看。只有能证明来源时才可离线回填；空 ownership 不能作为 wildcard。

## Verification / 验证

```bash
go test ./internal/config ./internal/server -count=1
go test ./...
go test ./internal/server -run '^$' -bench '^BenchmarkAuthenticateAppRequestWithOneHundredScopes$' -benchmem -count=3
```

Tests cover Agent Run route authentication, dedicated token header, gray mode, body scope injection/conflict, workspace/app-instance/app-identity isolation, identical missing/cross-tenant 404 responses, recursive redaction, semantic payload removal, token serialization, config validation and OpenAPI security metadata.

On the local Intel validation host, the 100-instance scope benchmark measured approximately `1.5-1.7 us/op`, `48 B/op`, and `3 allocs/op`.

## Skill Decision / Skill 结论

No feature-specific skill is added. The stable maintenance entry is this document plus `internal/server/README.md`; existing code-style and delivery skills cover future changes.

当前不新增 feature 专属 skill。本文与 `internal/server/README.md` 是稳定维护入口；既有 code-style 与交付 skills 已覆盖后续变更。
