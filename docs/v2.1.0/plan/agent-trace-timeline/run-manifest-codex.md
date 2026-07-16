# Run Manifest Implementation Evidence / 运行清单实现证据

## Scope / 范围

- Freeze model/provider, final assembled prompt, skill, tool schema, governance, context asset, evaluator, system-truth and runtime-contract revision references at TaskRun creation.
- 在 TaskRun 创建时冻结 model/provider、最终 assembled prompt、skill、tool schema、governance、context asset、evaluator、system truth 与 runtime contract 版本引用。
- Expose one typed top-level manifest in both timeline APIs; preserve legacy runs without inference.
- 在两条 timeline API 顶层只暴露一份 typed manifest；旧 run 不做推断。

## Workflow Note / 流程说明

The local Claude CLI was unavailable in this worktree. Two independent read-only subagents were used for source inventory and security/backward-compatibility review before the implementation was finalized. Their findings drove the immutable write boundary, digest-only data model, legacy status, and explicit authorization follow-up.

本 worktree 未提供 Claude CLI。本轮使用两个独立只读子 agent 分别完成版本来源审计与安全/兼容性审查，再收口实现；审查结论直接决定了不可变写入边界、仅摘要数据模型、历史状态和授权后续项。

## Verification / 验证

- `go test ./internal/runtime ./internal/server -count=1`
- `cd web && npm run build`
- Manifest unit tests verify deterministic hashes and absence of raw sensitive content.
- Eino graph tests verify capture at persistence creation.
- Timeline tests verify one top-level source and `legacy_unavailable` compatibility.
- `BenchmarkBuildRunManifestWithOneHundredTools`: 177-229 us/op, about 27.1 KB/op, 41 allocs/op across three isolated runs on the local Intel development machine.

## Independent Review Resolution / 独立审查修复

- Recomputed tool revisions from the execution-time dynamic catalog snapshot and actual model-binding schema.
- 使用执行时动态 catalog 快照与实际模型绑定 schema 重算 tool revision。
- Merged resolved and requested context assets instead of dropping unresolved siblings.
- 合并 resolved/requested context assets，不再漏掉同批未解析资产。
- Added endpoint identity to the model digest only, tightened `partial` completeness checks, added PostgreSQL metadata JSON round-trip coverage, and typed `unsupported_schema` in OpenAPI and TypeScript.
- 仅在 model digest 中加入 endpoint identity，收紧 `partial` 判定，补充 PostgreSQL metadata JSON round-trip，并在 OpenAPI/TypeScript 中对齐 `unsupported_schema`。
- The existing app-facing cross-workspace authorization gap remains a separate production gate and is not hidden by this delivery.
- 既有 app-facing 跨 workspace 授权缺口保留为独立生产门禁，本交付不将其隐藏。
