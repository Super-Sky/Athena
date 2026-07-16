# Agent Run App Auth 真实场景测试用例

## 前置配置

1. 设置 `APP_AUTH_REQUIRED=true`。
2. 在 `APP_AUTH_IDENTITIES_JSON` 中配置两个不同应用身份，并为每个身份分配不同的 workspace/app-instance scope。
3. 使用与 app token 不同的 `CONTROL_PLANE_AUTH_TOKEN` 启动 Athena。

## 核心用例

| 场景 | 操作 | 预期 |
| --- | --- | --- |
| 身份缺失 | 不带四个应用身份 header 请求任一 Agent Run 路由 | 返回 `401` |
| 正常创建 | 使用合法身份创建 run，body 不填写 ownership | run 自动记录 header 中的 workspace/app-instance |
| body 越权 | 合法身份创建 run，但 body 填写其他 workspace | 返回 `403 app_scope_mismatch` |
| 同租户读取 | 使用创建者身份读取 run、trace、timeline | 返回 `200`，只包含安全字段 |
| 跨租户读取 | 使用另一合法 scope 读取已存在 run | 返回 `404 resource_not_found` |
| 不存在读取 | 使用创建者身份读取不存在 run | 与跨租户读取的状态码和 body 完全一致 |
| 历史空归属 | app 身份读取 ownership 为空的历史 run | 返回通用 `404` |
| 续跑归属 | 合法身份 resume 原 run | 新 run 继承原 run 的 workspace/app-instance |
| Token 隔离 | 只在 `Authorization` 中发送 app token | 返回 `401`，token 不被当作 app 身份 |
| Trace 脱敏 | metadata/payload 写入嵌套 API key、authorization、raw input | 响应中对应值为 `[redacted]`，app trace 无 semantic payload |
| 后台排障 | 使用 Control Plane 管理员登录查看同一 run | 可查看安全 trace 与性能数据，不暴露凭据值 |

## 自动化回归

```bash
go test ./internal/config ./internal/server -count=1
go test ./...
go vet ./internal/server
python3 scripts/check_no_absolute_paths.py
docker compose --env-file deploy/athena.env.example -f deploy/docker-compose.cloud.yml config
```
