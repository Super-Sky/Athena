# scripts

## 模块职责

- 保存辅助验证、发布打包和部署脚本。

## 不负责什么

- 不负责正式运行时主链
- 不负责平台级编排系统

## 子目录索引

- 当前无子目录

## 当前工作流文件索引

- `chat_stream_reliability.py`
  - 用于验证 `/api/chat/stream` 在真实调用中的可靠性和场景覆盖情况。
- `chat_respond_reliability.py`
  - 用于验证 `/api/chat/respond` 或结构化响应路径的可靠性表现。
- `check_no_absolute_paths.py`
  - 扫描文档、skill、模板和脚本说明中的个人机器绝对路径，作为 issue-driven 文档治理门禁。
- `test_check_no_absolute_paths.py`
  - 验证绝对路径扫描脚本的命中与放行逻辑。
- `build_release_bundle.sh`
  - 构建 Linux 二进制发布包，并把部署模板和配置一起打进 tar 包。
- `deploy_cloud.sh`
  - 本地构建 Docker 镜像，上传到远端服务器并执行 `docker compose up -d`。

## 对外入口

- 回归与交付文档中的测试命令引用
- 云端打包与部署入口

## 遗留兼容工具

- 旧外部平台适配样例 smoke 脚本仍保留在 `scripts/` 中，仅作历史兼容参考。
- 旧外部跟踪器辅助脚本和示例文件仍保留在 `scripts/` 中，仅作历史兼容参考，不属于当前仓库工作流入口。

## 关键依赖

- 验证脚本依赖当前启动中的 Athena 服务
- 部署脚本依赖本地 `docker`、`ssh`、`scp`

## 维护提示

- 接口协议或鉴权方式变化时，优先同步更新这里的脚本。
- 旧外部跟踪器辅助脚本仅保留作兼容参考，不应作为当前仓库流程入口继续扩展。
- 部署脚本默认以单机云服务器为目标，不负责 Kubernetes 或 Helm。
