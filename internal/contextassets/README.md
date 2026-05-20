# contextassets

## 模块职责

- `internal/contextassets` 负责统一请求级 `context_assets` 的解析、resolved 视图转换、runtime usage trace 和 candidate update 产物生成。

## 不负责什么

- 不负责 active truth dir 中文件的持久化
- 不负责 platform detail HTTP 请求
- 不负责 session store 的持久化实现

## 文件索引

- `context_assets.go`
  - 定义 `Asset / ResolvedAsset / Bundle / UsageTrace / CandidateInput`，并实现 bundle 解析、override 应用、usage trace 与 candidate update 生成。
- `effective_views.go`
  - 基于 resolved assets 与 usage trace 构建 runtime 统一生效视图，产出 `effective_persona / effective_agent_profile / effective_user_profile / effective_memory_view / effective_scene / effective_workflow / effective_policy_rules / effective_contracts / effective_skills`。
- `context_assets_test.go`
  - 验证 ref-first 解析、override、生效 trace、candidate update 以及 effective views 的恢复与构建。

## 对外入口

- `BuildBundle`
- `ApplyOverrides`
- `ResolveUsage`
- `BuildCandidateTrace`
- `BuildEffectiveViews`
- `BuildEffectiveViewsFromGlobalContext`
- `AssetMaps`
- `ResolvedMaps`

## 维护提示

- 这里的 candidate update 是候选产物，不等于正式写回 truth dir 或 Git baseline。
- request-scoped override 只影响当前轮次，不应在这里直接变成长周期主源。
- 修改 trace 或 effective view 字段时，需要同步检查 `docs/api.md`、`docs/implementation.md`、`docs/features/feature-上下文资产注入与结构化管理.md` 和 transport 输出接线。
