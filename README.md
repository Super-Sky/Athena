# Athena

Athena 是一个通用 AI Agent Platform Runtime。

## 先看哪里

当前仓库文档已经按 5 个入口收口，默认从这里开始：

- 本地启动：
  - `docs/本地开发与服务启动.md`
- 外部集成：
  - `docs/platform-依赖与增强.md`
- 模块结构：
  - `docs/目录结构与模块说明.md`
- 云端部署：
  - `docs/云端部署与交付.md`
- 当前真相：
  - `docs/当前能力总览.md`

## Athena 现在是什么

Athena 不是通用聊天机器人脚手架，也不只是模型调用封装层。

它当前主要负责：

- 统一任务归一化
- 上下文资产装配与 system truth 管理
- `chat/stream` 的用户可见步骤流与结构化结果生成
- workflow / automation 草案生成
- session / waiting / resume 连续性
- 控制面治理与 Web 控制台

它的能力分层默认按：

- `Core`：跨应用通用 runtime、tool、governance、trace、usage、sandbox、system truth、projection 基础能力
- `Validation`：独立验证 core 的 System Validation、验证型 MCP、deterministic validation flow
- `Enhancement`：用户 skill、应用知识库、业务 workflow、场景包、provider adapter 等非侵入增强
- `Application / Business Truth`：业务对象、业务规则、最终业务状态和业务真相

Athena 管理最终 system content；用户层和业务层 truth 支持 Athena-managed 与 app-managed 两种模式。

## 主要命令

在仓库根目录执行：

```bash
go run . api-server
go run . migrate
go run . healthcheck
go run . version
```

说明：

- `api-server`
  - 启动 HTTP + SSE 服务
- `migrate`
  - 显式执行数据库迁移
- `healthcheck`
  - 用当前配置探测本地 `/healthz`，供容器健康检查复用
- `version`
  - 输出构建版本信息

## 最快启动

如果你只是想先把服务起起来：

```bash
export SESSION_STORE_DRIVER=memory
export MODEL_STORE_DRIVER=memory
export SECURITY_ENCRYPTION_KEY=athena-local-dev-key
go run . api-server
```

验证：

```bash
curl http://127.0.0.1:8090/healthz
curl http://127.0.0.1:8090/swagger/openapi.json
```

完整本地启动和 Web 控制台联调，见：

- `docs/本地开发与服务启动.md`

## 云端交付

当前仓库已经提供：

- Docker 镜像构建
- 单仓 Docker Compose 部署样例
- 一键远端部署脚本
- Linux 二进制打包脚本
- systemd 部署模板

入口见：

- `docs/云端部署与交付.md`
- `deploy/`
- `scripts/build_release_bundle.sh`
- `scripts/deploy_cloud.sh`

## CI 建议

公开仓默认建议的最小校验：

- `go test ./...`
- `cd web && npm ci && npm run build`
- `docker compose -f deploy/docker-compose.cloud.yml config`

## 仓库结构

- `main.go`
  - 进程命令分发入口
- `internal/entry`
  - 根依赖装配和进程级入口
- `internal/server`
  - HTTP、SSE、OpenAPI 和 transport
- `internal/app`
  - 请求编排、waiting、queue 和治理 use case
- `internal/runtime`
  - 运行时主链
- `internal/extensions/platform`
  - platform 专项增强
- `internal/controlplane`
  - 控制面治理
- `web/`
  - 独立 React 控制台
- `docs/`
  - 当前真相、启动、部署和架构文档
- `deploy/`
  - 部署样例和模板
- `scripts/`
  - 联调、打包和部署脚本

详细目录说明见：

- `docs/目录结构与模块说明.md`

## 当前状态

当前代码已经具备：

- `POST /api/chat/stream` 的用户可见 `progress_step` 过程流
- `done.detail.step_flow` 终态步骤流摘要
- waiting / supplement / resume 主链
- control-plane 与 Web 控制台
- platform context 协同
- automation create payload 和 workflow plan 的最小结构

完整项目真相见：

- `docs/当前能力总览.md`
