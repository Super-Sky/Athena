# External Memory and Context Assets / 外部记忆与上下文资产

## Background / 背景

Issue `Super-Sky/Athena#10` gives business applications a generic way to store governed summaries and turn them into runtime context. It is intentionally not a business database: Athena never models funds, portfolios, trades, broker accounts, or application-specific evidence.

Issue `Super-Sky/Athena#10` 为业务应用提供保存受治理摘要并将其转为运行时上下文的通用能力。它有意不是业务数据库：Athena 不建模基金、持仓、交易、券商账号或应用专属 evidence。

## Contract / 契约

- `POST /api/memory/write` accepts `app_id`, `owner_id`, `scope`, `kind`, `summary`, optional `schema_version`, and string metadata.
- `POST /api/memory/query` requires the same ownership triple and returns only matching records.
- `POST /api/context-assets/resolve` turns scoped summaries into read-only `memory_view` assets with `source_kind=app_memory`.
- `POST /api/context-assets/assemble` combines those assets with caller-provided generic context assets and returns usage trace, effective views, and `external_context_compression.v1` statistics.

每次读写都必须提供 `app_id + owner_id + scope`。响应只保留操作、归属、作用域和 record IDs 的安全 trace；它不会复制原始业务对象载荷。

## Implementation / 实现

`internal/memory.ExternalStore` is thread-safe and owns generic records plus operation traces. `internal/app.Service` exposes the narrow write/query facade. `internal/server/external_memory.go` maps the four HTTP contracts and produces `contextassets.Asset` values without asking System Truth to resolve an application-owned asset.

`internal/memory.ExternalStore` 是线程安全的通用记录与操作 trace store。`internal/app.Service` 暴露窄写入/查询门面。`internal/server/external_memory.go` 映射四条 HTTP 契约，并在不请求 System Truth 解析应用资产的情况下生成 `contextassets.Asset`。

## Boundaries and Risks / 边界与风险

- The default store is process-local. It is appropriate for local/demo MVP wiring but is not durable across service restarts.
- A business application remains responsible for durable records, authorization, retention, deletion, and domain validation.
- The compression response is a truthful summary shape, not a lossy model-generated compaction algorithm; `truncated=false` until a governed compaction provider is added.
- The API does not authenticate callers by itself. Production deployment must bind `app_id` and `owner_id` to the host application's authenticated principal before invoking Athena.

- 默认 store 为进程内实现，适合本地/演示 MVP，服务重启后不持久。
- 业务应用仍负责持久记录、授权、留存、删除和领域校验。
- compression 响应是如实的摘要统计形状，不是模型生成的有损压缩算法；在接入受治理 compaction provider 前，`truncated=false`。
- API 本身不做调用者认证；生产部署必须在调用 Athena 前将 `app_id` 和 `owner_id` 绑定到宿主应用的已认证主体。

## Verification / 验证

```bash
go test ./internal/memory ./internal/app ./internal/server
go test ./internal/memory -run '^$' -bench '^BenchmarkExternalStoreQuery$' -benchmem -count=1
```

The handler test writes one profile summary, proves another owner receives no records, resolves an `app_memory` asset, assembles an effective memory view, and rejects an incomplete ownership tuple.

Handler 测试会写入一条画像摘要，证明另一 owner 无法读取记录，解析 `app_memory` 资产，组装 effective memory view，并拒绝不完整的 ownership 三元组。

## Skill Decision / Skill 结论

No feature-specific maintenance skill is added. This slice is maintained through this document, the three module READMEs, `code-style`, `doc-index-sync`, `feature-doc-skill-sync`, and `repo-task-delivery`; a dedicated skill would not remove meaningful complexity yet.

当前不新增 feature 专属维护 skill。本文、三个模块 README、`code-style`、`doc-index-sync`、`feature-doc-skill-sync` 与 `repo-task-delivery` 已覆盖稳定维护入口；现阶段新增专属 skill 不会实质降低复杂度。
