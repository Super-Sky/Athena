# Docker Runtime Profile / Docker 运行 Profile

## Background / 背景

Issue `Super-Sky/Athena#12` introduces a local Docker Compose profile for Athena as the generic Agent Runtime consumed by business applications. It provides infrastructure only and never introduces fund-specific tables or business objects into Athena core.

Issue `Super-Sky/Athena#12` 为作为业务应用通用 Agent Runtime 的 Athena 提供本地 Docker Compose profile。它只提供基础设施，不会把基金专属表或业务对象引入 Athena core。

## Runtime Shape / 运行形态

`deploy/docker-compose.runtime.yml` contains:

- `postgres`: Athena session/model/runtime persistence dependency.
- `redis`: cache, rate-limit, and async-work topology dependency; not business truth.
- `athena-api`: waits for PostgreSQL and Redis, runs migration, then starts the API healthcheck.
- `athena-web`: optional `control-plane` profile, enabled only when the Web UI is needed.

`deploy/docker-compose.runtime.yml` 包含 PostgreSQL、Redis、Athena API 和可选 Control Plane Web。API 在两个依赖健康后先执行迁移再启动；Redis 只预留给缓存、限流和异步任务，不保存业务真相。

## Integration Contract / 对接契约

An app on the Compose network uses `ATHENA_BASE_URL=http://athena-api:8080`. A host-run app uses `ATHENA_EXTERNAL_BASE_URL` (default `http://127.0.0.1:8080`). `ATHENA_AUTH_TOKEN` remains optional and is supplied only when Athena enables authentication middleware.

同一 Compose 网络的应用使用 `ATHENA_BASE_URL=http://athena-api:8080`；宿主机应用使用 `ATHENA_EXTERNAL_BASE_URL`（默认 `http://127.0.0.1:8080`）。只有 Athena 启用认证 middleware 时才设置可选 `ATHENA_AUTH_TOKEN`，不得提交 token。

The runtime foundation bootstrap also registers the generic `chat` task type and its validator contract. This keeps the default `/api/agent/runs` task type executable when PostgreSQL persistence is enabled; it does not add any business-specific runtime contract.

runtime foundation bootstrap 还会注册通用 `chat` task type 及其 validator contract，保证启用 PostgreSQL persistence 后 `/api/agent/runs` 的默认任务类型可执行；该 contract 不包含任何业务领域语义。

## Verification / 验证

```bash
bash -n scripts/smoke_runtime_compose.sh
docker compose --env-file deploy/athena.runtime.env.example -f deploy/docker-compose.runtime.yml config
ATHENA_RUNTIME_ENV_FILE=deploy/athena.runtime.env.example ./scripts/smoke_runtime_compose.sh
```

Static configuration and the live Compose smoke pass. The validated profile starts PostgreSQL, Redis, and Athena API; migration completes before the API health check returns successfully. An isolated dual-service smoke also completed an Agent Run and called the fund assistant's remote `account_overview` tool. The first local Athena image build took about 259 seconds with limited BuildKit output, while later runs can reuse cache.

静态配置和实时 Compose smoke 均已通过。已验证 profile 会启动 PostgreSQL、Redis 和 Athena API；迁移完成后 API health check 成功返回。隔离双服务 smoke 还完成了一次 Agent Run，并调用基金助手提供的远程 `account_overview` tool。首次本机 Athena 镜像构建约耗时 259 秒，BuildKit 中间输出有限；后续运行可复用缓存。

## Skill Decision / Skill 结论

No feature-specific skill is added. `repo-task-delivery`, `doc-index-sync`, `feature-doc-skill-sync`, and the local runtime documents cover the recurring deployment checks.

当前不新增 feature 专属 skill。`repo-task-delivery`、`doc-index-sync`、`feature-doc-skill-sync` 与本地运行文档已覆盖重复部署检查。
