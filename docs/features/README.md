# Features

本目录保存当前仍有维护价值的功能级说明。版本目录只保存计划或快照，稳定后的功能说明应回收到这里。

## 文件索引

- `feature-agent-run-api.md`
  - 说明 issues #7/#8 的 app-facing Agent Run API、OpenAI-compatible tool-call contract、通用 core 边界、resume/cancel 当前行为与验证方式。
- `feature-agent-run-app-auth.md`
  - 说明 issue #26 的应用身份认证、精确 workspace/app-instance 授权、租户安全响应、递归脱敏与灰度上线约束。
- `feature-agent-trace-timeline.md`
  - 说明 issue #11 的统一 trace 时间线、不可变 run manifest、安全详情与后台观测入口。
- `feature-remote-business-tools.md`
  - 说明 issue #9 的 app-owned remote tool registry、HTTP callback contract、执行治理、网络边界与验证方式。
- `feature-builtin-tools.md`
  - 说明 issue #14 的 calculator、current_time、JSON Schema 子集、Core 边界与验证方式。
- `feature-external-memory-context.md`
  - 说明 issue #10 的外部记忆写入/查询、context asset resolve/assemble 与 ownership 边界。
- `feature-runtime-foundation-validation.md`
  - 说明 v2.1.0 RuntimeContract foundation smoke、System Validation DOM anchors 和对应验证路径。
- `feature-runtime-contract-persistence.md`
  - 说明 v2.1.0 RuntimeContract persistence、task type registry、hook binding、System Truth lifecycle 和 runtime contract write/read path。
