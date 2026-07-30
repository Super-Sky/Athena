# Athena 本地 Docker Runtime

## 目标

`deploy/docker-compose.runtime.yml` 是 Athena 面向业务应用的本地运行 profile：启动 API、PostgreSQL、Redis，并可通过 `control-plane` profile 启动 Web 控制台。它不包含任何基金、持仓、交易或业务证据表。

## 启动

```bash
cp deploy/athena.runtime.env.example deploy/athena.runtime.env
docker compose --env-file deploy/athena.runtime.env -f deploy/docker-compose.runtime.yml up --build -d
curl -fsS http://127.0.0.1:8080/healthz
```

若本地或企业网络要求构建期代理，请在未提交的 `deploy/athena.runtime.env` 中设置：

```dotenv
DOCKER_BUILD_HTTP_PROXY=http://host.docker.internal:PORT
DOCKER_BUILD_HTTPS_PROXY=http://host.docker.internal:PORT
DOCKER_BUILD_NO_PROXY=localhost,127.0.0.1,postgres,redis,athena-api
```

它们是 Docker build args，不是 Athena 运行时设置。直连时保持为空；代理 URL 若带凭据，绝不能提交。

需要 Control Plane 页面时：

```bash
COMPOSE_PROFILES=control-plane \
docker compose --env-file deploy/athena.runtime.env -f deploy/docker-compose.runtime.yml up --build -d
```

## 服务与健康检查

- `postgres`：`pg_isready`。
- `redis`：`redis-cli ping`。
- `athena-api`：先执行 `/app/athena migrate`，启动 `api-server`，再由 `/app/athena healthcheck` 探测服务。
- `athena-web`：可选 profile，检查 Nginx 根路径。

API 只有在 PostgreSQL 与 Redis 健康后才启动；迁移失败不会被掩盖成健康状态。

## 基金助手对接

同一 Docker 网络中的基金助手应设置：

```dotenv
ATHENA_BASE_URL=http://athena-api:8080
ATHENA_AUTH_TOKEN=
```

宿主机运行基金助手时使用 `ATHENA_EXTERNAL_BASE_URL`（默认 `http://127.0.0.1:8080`）。`ATHENA_AUTH_TOKEN` 仅在 Athena 明确启用认证 middleware 后设置；不要把 token 写入版本库。

Redis 已纳入拓扑，用于后续缓存、限流和异步任务；当前 Athena core 不将 Redis 作为业务对象真相库。
