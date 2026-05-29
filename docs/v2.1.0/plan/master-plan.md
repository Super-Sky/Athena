# v2.1.0 Master Plan

## Issue

- canonical issue: `Super-Sky/Athena#1`
- title: `v2.1.0 RuntimeContract foundation 收口与 Batch 2 计划冻结`
- current state: `implementing`
- branch: `codex/v2.1-runtime-contract-batch2-issue-1`

## Scope Guard

v2.1.0 只接受能强化通用 agent runtime foundation，或能证明 foundation 真实可用的工作。任何业务真相、业务 evidence 或应用专属判断逻辑都不能借本期进入 core。

- `Core`: RuntimeContract, registered task type, hook binding, System Truth lifecycle, semantic projection boundary, checkpoint-backed waiting read surface.
- `Validation`: System Validation readout, deterministic validation run, Validation MCP, sandbox boundary, repeatable API and DOM smoke checks.
- `Enhancement`: scenario packages, app-specific skills, app knowledge, provider adapters.
- `Application / Business Truth`: business evidence, formal business object state, and app-owned rules.

Supplement、Compaction、Memory、Context ownership、Capability Studio、external MCP registry、cleanup jobs 和业务 evidence ownership 继续保持 deferred，除非后续 issue 明确拉入。

## Batch 2 Checklist

- [x] 恢复与 `Super-Sky/Athena#1` 绑定的 v2.1.0 master plan 和可勾选 checklist。
- [x] 增加可重复运行的 RuntimeContract foundation API smoke，覆盖 read/write/readout 相关 surface。
- [x] 为 System Validation 页面增加稳定 DOM anchors，支持严格浏览器自动化。
- [x] 在脚本索引和 feature 文档中记录可重复 smoke 路径。
- [x] 对运行中的 Web 控制台执行严格 Codex in-app Browser DOM 验收。
- [x] 产品化 checkpoint-backed waiting run readout，不暴露 Eino private checkpoint payload。
- [x] 为 `inspection_task`、`integration_event`、`scheduled_job`、`workflow_step_request` 定义 registered task type validator contract。
- [ ] 深化 System Truth `source -> draft -> compile -> active -> rollback` write/edit path 规划。
- [ ] 收口 semantic projection boundary，确保 projection candidate 不升级为业务 `EvidenceRecord`。
- [ ] 将剩余 direct respond rich delivery 兼容拼装从 transport 收敛到 app/runtime graph node 或 Batch 2 read model。

## Acceptance Gates

- 在启用 runtime persistence 的真实后端上通过 API smoke：
  - `/api/control-plane/runtime/contracts/foundation`
  - `/api/control-plane/runtime/validation-runs`
  - runtime run detail, steps, lifecycle, traces, usage, projections, checkpoint safe metadata
- Web build 通过。
- runtime read/write 与 validation 的聚焦 Go 测试通过。
- 关闭 Browser checklist 项前，System Validation tab 的 DOM smoke 必须通过。
- 任何 commit 或 push 前必须执行 `repo-task-delivery`。

## Current Verification Commands

```bash
go test ./internal/controlplane ./internal/server ./internal/app ./internal/runtime
go test ./internal/runtime ./internal/app ./internal/server
cd web && npm run build
PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build
python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py
python3 scripts/check_no_absolute_paths.py
python3 scripts/control_plane_runtime_foundation_smoke.py --base-url http://127.0.0.1:8090
python3 scripts/control_plane_runtime_foundation_smoke.py --base-url http://127.0.0.1:8090 --web-url http://127.0.0.1:5173
```

API smoke 需要一个已启用 runtime persistence 的运行中后端。`--web-url` 路径还需要 Python Playwright 与运行中的 Web 控制台。

## Current Evidence

- 2026-05-26 本机临时 PostgreSQL `55432` 环境通过 `go run . migrate`。
- 2026-05-26 API smoke 通过，foundation 返回 `contracts=1`、`task_types=1`、`hook_bindings=3`、`active_system_truths=54`。
- 2026-05-26 Web DOM smoke 通过，System Validation 页面新增锚点均可定位。
- 2026-05-26 Codex in-app Browser 切到 System Validation tab 后确认新增锚点 `missing=[]`。
- 2026-05-26 self-review 后收紧 DOM smoke：所有新增 `data-testid` 必须唯一且可见，并确保浏览器失败时也会关闭。
- 2026-05-26 checkpoint-backed waiting readout 已接入 `GET /api/control-plane/runtime/runs/:runID/checkpoints`、OpenAPI、System Validation 与 smoke 脚本；API 只暴露 `payload_size` / `payload_sha256` 等安全摘要，不返回 raw payload 或 resume token。
- 2026-05-26 聚焦验证通过：`go test ./internal/runtime ./internal/app ./internal/server`、`cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`。
- 2026-05-26 真实 API + DOM smoke 通过，返回 `run_id=163b71fa-8c18-4a03-bdb8-6de12f168e0e`、`records.steps=1`、`records.lifecycle=9`、`records.traces=5`、`records.usage=5`、`records.projections=3`、`records.checkpoints=0`、`web.dom=ok`。
- 2026-05-29 registered task type validator contract 已接入四个 legacy task type，active task type 写入校验要求 `default_contract_id`、`input_schema` 和 `validator_refs.validators`；draft 仍可暂存。
- 2026-05-29 聚焦验证通过：`go test ./internal/runtime ./internal/app ./internal/server`、`cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`、`python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py`。
- 2026-05-29 真实 API + DOM smoke 通过，返回 `run_id=f09ad491-9df4-47c4-b92c-97adf7098c24`、`contracts=5`、`task_types=5`、`hook_bindings=3`、`active_system_truths=54`、`web.dom=ok`。

## Next Iteration Plan

下一轮优先处理 System Truth `source -> draft -> compile -> active -> rollback` write/edit path，继续保持 core 只承载通用生命周期与校验面，不引入业务 truth ownership。

执行清单：

1. 盘点现有 System Truth source、draft、compiled、active pointer 与 rollback 相关 store/API 能力。
2. 定义最小 write/edit/readout 路径，确保 source ingest、draft edit、compile 和 active pointer 切换可追踪。
3. 补 Go 单测覆盖状态转换、非法回退、active pointer 冲突和 compiled artifact 引用。
4. 在 System Validation 中展示 lifecycle readout 与失败原因摘要。
5. 更新 feature 文档、真实场景测试用例和 smoke 断言。
