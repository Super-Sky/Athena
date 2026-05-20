# deploy

## 模块职责

- 保存 Athena 单仓部署和交付所需的样例文件与模板。

## 不负责什么

- 不负责运行时代码
- 不负责平台级编排系统

## 子目录索引

- `systemd/`
  - 二进制部署使用的 systemd 单元模板。
- `bin/`
  - 二进制部署目录约定说明。

## 文件索引

- `docker-compose.cloud.yml`
  - 单仓云端 Docker Compose 部署样例，包含 PostgreSQL、Athena API 和可选的 Web 控制台。
- `athena.env.example`
  - Compose 和二进制部署共用的环境变量模板。
- `systemd/athena.service.example`
  - 二进制部署时的 systemd 服务定义模板。
- `bin/README.md`
  - 描述二进制发布包在服务器上的推荐目录约定。

## 对外入口

- `docs/云端部署与交付.md`
- `scripts/build_release_bundle.sh`
- `scripts/deploy_cloud.sh`

## 关键依赖

- 依赖仓库根目录 `Dockerfile`
- 可选依赖 `web/Dockerfile`

## 维护提示

- 这里的模板和文档必须与真实脚本保持一致。
- 端口、镜像名和启动命令变更时，优先同步这里。
