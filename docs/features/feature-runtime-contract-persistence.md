# Runtime Contract Foundation

## 背景

`v2.0.0` 已完成 core runtime records、Eino graph-backed validation path 和 Control Plane 验证闭环。`v2.1.0` 的第一批目标不是继续堆 demo，而是把 runtime execution 的稳定 contract foundation 真正落为 Athena 自己可迁移、可读写、可验证的核心对象。

当前这份功能文档收口的是第一批 foundation：

- `RuntimeContract`
- `TaskTypeRegistration`
- `HookBinding`
- `SystemTruthSource`
- `SystemTruthDraft`
- `SystemTruthCompileResult`
- `SystemTruthActiveVersion`
- semantic `ProjectionCandidate`
- internal Eino hook bridge
- Control Plane foundation readout

## 范围

本轮完成：

- 新增 Athena-owned core tables 和 Postgres store implementation。
- 继续沿用现有 migration 机制，不新增专用 write API 主入口。
- 通过 `internal/runtime` 的 deterministic writer / graph projection 写入安全 runtime truth。
- 启动阶段与 `SyncSystemResources` 后会基于 Control Plane active truth 自动补齐一份 Athena-owned foundation snapshot。
- 新增 `GET /api/control-plane/runtime/contracts/foundation` 只读聚合面。
- 新增 `PUT /api/control-plane/runtime/contracts/{contractID}`、`PUT /api/control-plane/runtime/task-types/{typeKey}`、`PUT /api/control-plane/runtime/hook-bindings/{bindingID}` 最小 foundation write path。
- 在 System Validation `Runtime Persistence Readout` 显示 contract foundation 摘要与快照。
- 在 System Validation 暴露 foundation JSON 编辑和保存入口。

本轮不做：

- arbitrary user code hook execution。
- business `EvidenceRecord` truth。
- cleanup jobs / replay conflict resolution。
- external MCP registry 或 remote sandbox platform。

## Core Objects

### 1. RuntimeContract

当前 `RuntimeContract` 覆盖：

- typed fields：`id`、`name`、`version`、`status`、`task_type`、`idempotency_scope`、`idempotency_key`
- JSONB fields：`input_schema`、`execution_profile`、`exit_policy`、`capability_profile`、`governance_policy_refs`、`hook_bindings`、`projection_policy`、`system_truth_refs`、`metadata`
- timestamps：`created_at`、`updated_at`

### 2. TaskTypeRegistry

当前 `TaskTypeRegistration` 负责：

- `type_key` 稳定任务键
- `default_contract_id` 默认 contract 关联
- `validator_refs` validator 引用
- `compatibility` legacy alias / migration hint

当前不把业务 task catalog 放进 core truth。

### 3. HookBinding

当前 `HookBinding` 负责：

- `hook_point`
- `binding_kind`
- `binding_ref`
- `order_index`
- `enabled`
- `failure_policy`

第一批只允许 internal allowlist：

- `runtime_contract_guard`
- `runtime_trace_recorder`
- `system_truth_guard`
- `projection_boundary_guard`

### 4. System Truth Lifecycle

当前 lifecycle 已具备 append-only write/edit/readout 闭环：

- `SystemTruthSource`
- `SystemTruthDraft`
- `SystemTruthCompileResult`
- `SystemTruthActiveVersion`

控制面 API：

- `GET /api/control-plane/runtime/system-truth/lifecycle`
- `POST /api/control-plane/runtime/system-truth/sources`
- `POST /api/control-plane/runtime/system-truth/drafts`
- `POST /api/control-plane/runtime/system-truth/drafts/:draftID/compile`
- `POST /api/control-plane/runtime/system-truth/compile-results/:compileID/activate`
- `POST /api/control-plane/runtime/system-truth/active-versions/:activeID/rollback`

关键约束：

- compile failure 不能 activate
- active pointer 切换追加新记录，不改写历史 source / draft 内容
- rollback 只追加新的 active pointer，并把当前 active 记录为 `rollback_from_id`
- lifecycle payload / metadata 不保存 raw credentials

### 5. Projection Boundary

当前 `ProjectionCandidate` 在最小 candidate 基础上补齐：

- `schema_version`
- `semantic_payload`
- `artifact_refs`
- `ui_hints`
- `materialization_target`

当前已知 candidate kinds 会自动补齐 deterministic `schema_version`，例如：

- `minimal_output` -> `runtime_projection.minimal_output.v1`
- `prepared_execution` -> `runtime_projection.prepared_execution.v1`
- `terminal_output` -> `runtime_projection.terminal_output.v1`
- `validation_mcp_result` -> `runtime_projection.validation_mcp_result.v1`
- `external_sandbox_ref` -> `runtime_projection.external_sandbox_ref.v1`
- `assistant_message` -> `runtime_projection.assistant_message.v1`

这些字段仍然只是 core candidate / ref / hint，不升级成业务最终真相。

## Eino 接线

当前 Eino bridge 策略已经真正落在代码里：

- `internal/runtime/eino_graph.go`
  - graph-backed validation path 会从 constraints 读取 `runtime_contract_id`
- `internal/runtime/hook_bridge.go`
  - 根据 contract + hook point 读取 enabled internal bindings
  - 投影 safe lifecycle event、trace 和 generic usage
- `internal/app/runtime_validation.go`
  - Control Plane validation trigger 会显式绑定默认 `RuntimeContract`，因此新的 validation run 会带出 contract-aware hook records

当前已覆盖的 hook 投影点是 `before_run`。后续若继续扩展，默认仍优先走 Eino middleware / callback / graph node，而不是在 Athena 内再造第二套编排框架。

## 安全规则

foundation 写入前会拒绝 credential-like plaintext：

- credential-like key，例如 `authorization`、`api_key`、`access_token`、`password`、`secret`、`credential`
- credential-like value，例如 `Bearer ...`、`sk-...`、`AKIA...`

适用范围包括：

- RuntimeContract JSONB profile / policy refs / metadata
- TaskType input schema / validator refs / compatibility / metadata
- HookBinding config / metadata
- System Truth payload / metadata
- Projection semantic maps

当前策略是 fail-closed rejection，不做“先写进去再补 redaction”。

## 控制面读面

新增：

- `GET /api/control-plane/runtime/contracts/foundation`

当前只读返回：

- `contracts`
- `task_types`
- `hook_bindings`
- `active_system_truths`
- `system_truth_sources`
- `system_truth_drafts`
- `system_truth_compile_results`
- `store_capabilities`
- `unavailable_surfaces`

System Validation `Runtime Persistence Readout` 当前会：

- 显示 RuntimeContract / TaskType / HookBinding / active System Truth / lifecycle 摘要卡片
- 显示 foundation snapshot 和 capability surface
- 继续显示 runtime runs、steps、trace、usage、projection
- 对 projection 展示 schema version / semantic boundary 标签
- 在 foundation active 后，新生成的 validation run 会额外出现 `runtime_hook_binding` traces 和 `runtime_hook` usage

## 关键代码

- `internal/runtime/persistence.go`
  - 定义 `RuntimeContract`、Projection boundary 扩展和 validation
- `internal/runtime/contract_foundation.go`
  - 定义 TaskTypeRegistry / HookBinding / System Truth lifecycle contracts
- `internal/runtime/postgres_persistence.go`
  - 定义 core tables 和 Postgres create/read/list implementation
- `internal/app/runtime_system_truth.go`
  - 编排 System Truth source、draft、compile、activate 和 rollback 的 append-only 写入路径
- `internal/runtime/hook_bridge.go`
  - 定义 internal hook read/project bridge
- `internal/runtime/eino_graph.go`
  - 在 graph-backed validation path 中接入 contract-aware hook bridge
- `internal/app/runtime_read.go`
  - 提供 foundation readout app-layer boundary
- `internal/app/runtime_contract_write.go`
  - 提供 RuntimeContract / TaskType / HookBinding 的 app-layer write boundary 与统一前置校验
- `internal/app/runtime_contract_bootstrap.go`
  - 基于 active control-plane truth 同步 RuntimeContract / TaskType / HookBinding / SystemTruth lifecycle foundation
- `internal/server/runtime_read.go`
  - 提供 foundation read/write API DTO 和 HTTP handler
- `web/src/App.tsx`
  - 提供 System Validation foundation readout

## 验证

本轮已通过：

```sh
go test ./internal/runtime ./internal/app ./internal/server
cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build
python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py
python3 scripts/check_no_absolute_paths.py
git diff --check
go test -count=1 ./internal/app ./internal/runtime ./internal/server ./internal/entry
go test ./...
npm --prefix web run build
python3 -B scripts/test_check_no_absolute_paths.py
git diff --check
```

本地 PostgreSQL-focused verification 已通过：

```sh
ATHENA_PG_TEST_DSN=... go test -count=1 -v ./internal/runtime ./internal/server ./internal/entry -run 'Test(PostgresRuntimeContractStoreIntegrationRoundTrip|TestPostgresRuntimeContractFoundationRoundTrip|TestPostgresRuntimeStoreIntegrationRoundTrip|TestPostgresRuntimeStoreRejectsInvalidInputs|MigrateStoresCreatesRuntimePersistenceTables|ControlPlaneRuntimeReadEndpointPostgresIntegration|ControlPlaneRuntimeValidationRunEndpointPostgresIntegration)'
```

live backend / proxy smoke 已通过：

- `HTTP_PORT=8090 go run . api-server`
- `npm --prefix web run dev -- --host 127.0.0.1 --port 5173`
- `curl http://127.0.0.1:5173/api/control-plane/runtime/contracts/foundation`
- `curl -H 'Content-Type: application/json' -d '{...}' http://127.0.0.1:8090/api/control-plane/runtime/validation-runs`

Codex in-app Browser 页面级验收已完成：在 `http://127.0.0.1:5173/` 的 `System Validation` 中确认 foundation cards 已非空（`CONTRACTS=1 / TASK TYPES=1 / HOOK BINDINGS=3 / SYSTEM TRUTH=54`），并通过页面按钮新增一条 runtime validation record；最新 run 已展示 `5 traces / 5 usage`，其中包含 `runtime_hook_binding` traces 与 `runtime_hook` usage。

## Skill 维护结论

暂不新增独立 skill。

原因是 System Truth lifecycle 写入闭环已经稳定到 feature 文档和测试，但 Batch 2 仍剩 semantic projection boundary 与 direct respond rich delivery 收口。等 v2.1.0 Batch 2 完整收口后，再判断是否需要单独的 `runtime-contract-foundation` 维护 skill。
