# Agent Run App Auth Codex 实施记录

## Context

- Issue: `Super-Sky/Athena#26`
- Branch: `codex/agent-run-read-auth-issue-26`
- Base: `codex/agent-trace-timeline-issue-11`
- State: `ready_for_delivery`

## Implementation

- 为六条 app-facing Agent Run 路由增加独立应用身份门禁，不复用可能被 Platform Context 转发的 `Authorization`。
- 将身份绑定到精确 workspace/app-instance scope；create 注入权威归属，resume 继承已授权原 run 的归属。
- 在读取 TaskRun 后、读取任何子 trace record 前执行 ownership 校验；不存在与跨租户资源统一返回通用 `404`。
- 对共用 runtime DTO 递归脱敏凭据型键，app-facing trace 额外移除 projection semantic payload。
- 补齐配置校验、OpenAPI、部署样例、功能文档与系统级 HTTP 回归测试。

## Review

本次环境没有可用 Claude CLI，因此未伪造 `review-claude.md`。实现前使用独立审查 agent 检查安全边界，并逐项落实以下结论：

- app token 必须使用专用 header。
- 不存在与跨租户资源必须同形响应。
- 共用 DTO 必须递归脱敏。
- Control Plane 当前只定义 system-admin，不伪造 workspace operator。
- 灰度开关只由 `APP_AUTH_REQUIRED=true` 开启。
- ownership 为空的历史 run 对 app API fail-closed。

实现后再次执行独立代码审查和完整自动化门禁；最终证据记录在 PR 与 issue 时间线。

独立 Codex review 发现通用 `app_token`、`id_token` 与 cookie 类键未被首版规则覆盖。最终实现增加 `token` / `*_token` 与 `cookie` / `*_cookie` 规则，并补充对应泄漏回归样本。

100 个 app-instance scope 的认证基准连续三次为 `1.5-1.7 us/op`、`48 B/op`、`3 allocs/op`。完整 `go test ./...`、`go vet ./internal/server`、绝对路径检查与 Compose 配置解析均通过。

## Delivery Decision

- 当前功能边界已经收敛到 `docs/features/feature-agent-run-app-auth.md`。
- 不新增 feature skill；既有 `code-style`、`feature-doc-skill-sync` 与 `repo-task-delivery` 已覆盖维护路径。
- 本分支依赖 issue `#11` 的 immutable manifest，因此以叠加 PR 方式交付，不直接基于 `master`。
