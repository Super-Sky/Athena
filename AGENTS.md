# Repository Working Notes

## Codex Entry

- `AGENTS.md` 是 Codex 在本仓库的入口文件。
- 共享规则正文统一维护在 `docs/*`，不要在入口文件和 skill 中重复维护。
- 本仓库引入双技能体系：
  - `.agents/skills/` 是主 skill 目录，面向 Codex，内容最完整。
  - `.claude/skills/` 是 Claude 可见的轻量镜像入口。

## Shared Docs

- `docs/README.md`
- `docs/CODE_STYLE_GUIDE.md`
- `docs/TASK_DELIVERY_GUIDE.md`
- `docs/REPO_WORKFLOW_GUIDE.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/implementation.md`

## Eino Framework Reference

- 本项目使用 Eino 作为 runtime execution 基础框架。
- 涉及 Eino API、Graph、Workflow、Chain、Agent、callback、checkpoint、中间件或组件接线时，优先查看本地只读参考 `_eino/`；若 `_eino/` 不完整，再查看 Go module cache 中的 `github.com/cloudwego/eino` / `eino-ext` 源码。
- `_eino/` 是 AI / 开发者参考资料目录，已加入 `.gitignore`，不要提交。

## Working Rules

- 代码实现规范只维护在 `docs/CODE_STYLE_GUIDE.md`。
- 任务完成后需要补哪些文档，统一看 `docs/TASK_DELIVERY_GUIDE.md`。
- 分支、提交、文档落点和技能主从同步，统一看 `docs/REPO_WORKFLOW_GUIDE.md`。
- 本仓任务推进只使用 GitHub issue，不再使用 Jira 作为流程入口或状态真相。
- 前端页面读取、SPA 检查、本地后台页面 smoke、截图或交互验证，优先使用 Codex 页面内置 `Browser` 插件；只有该插件不可用或不足以完成验证时，再使用 Dokobot / Playwright 等回退方案。
- `chrome-devtools-mcp` 只用于用户明确要求的低层浏览器调试，例如 console、network、performance、protocol 或 DOM internals，不作为常规页面验收默认工具。
- 版本目录、issue 归属和开发日志也统一看 `docs/REPO_WORKFLOW_GUIDE.md`。
- 对版本化的非微小任务，默认进入 `ai-collab-workflow`；由 AI 主动判断 plan、correction、worktree、review、delivery 和 cleanup 阶段，而不是等待开发者逐步指挥。
- 提交或推送前，必须先执行 `repo-task-delivery` skill 做质量门禁检查。
- 若要提交或推送，默认应先确认对应的上游 issue 标识；提交说明与 PR 回链规则统一看 `docs/REPO_WORKFLOW_GUIDE.md`。
- 一笔提交默认只承载一个上游 issue 主语义，提交标题必须准确描述这一次改动，不要用泛化标题覆盖多类内容。
- 修改 API、SSE 事件、运行时装配、模型接入或 tool 契约时，默认同步检查 `docs/api.md`、`docs/architecture.md`、`docs/implementation.md` 是否需要更新。
- skill 只保留入口、流程和判断标准；共享正文统一指向 `docs/*`。
