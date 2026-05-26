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
- [ ] 产品化 checkpoint-backed waiting run readout，不暴露 Eino private checkpoint payload。
- [ ] 为 `inspection_task`、`integration_event`、`scheduled_job`、`workflow_step_request` 定义 registered task type validator contract。
- [ ] 深化 System Truth `source -> draft -> compile -> active -> rollback` write/edit path 规划。
- [ ] 收口 semantic projection boundary，确保 projection candidate 不升级为业务 `EvidenceRecord`。
- [ ] 将剩余 direct respond rich delivery 兼容拼装从 transport 收敛到 app/runtime graph node 或 Batch 2 read model。

## Acceptance Gates

- 在启用 runtime persistence 的真实后端上通过 API smoke：
  - `/api/control-plane/runtime/contracts/foundation`
  - `/api/control-plane/runtime/validation-runs`
  - runtime run detail, steps, lifecycle, traces, usage, projections
- Web build 通过。
- runtime read/write 与 validation 的聚焦 Go 测试通过。
- 关闭 Browser checklist 项前，System Validation tab 的 DOM smoke 必须通过。
- 任何 commit 或 push 前必须执行 `repo-task-delivery`。

## Current Verification Commands

```bash
go test ./internal/controlplane ./internal/server ./internal/app ./internal/runtime
cd web && npm run build
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

## Next Iteration Plan

下一轮优先处理 `checkpoint-backed waiting run readout`，目标是把等待态 runtime run 的读取面产品化，同时避免暴露 Eino private checkpoint payload。

执行清单：

1. 盘点当前 `runtime_graph_checkpoints`、task run、step timeline 与 projection read model 的可用字段。
2. 定义 control-plane 只读等待态 readout DTO，明确哪些字段可出现在 API / Web，哪些字段只能留在内部 checkpoint store。
3. 补 Go 单测覆盖 checkpoint 只读摘要、payload redaction 和空 checkpoint fallback。
4. 在 System Validation Runtime Persistence Readout 中展示等待态摘要，但不展示 checkpoint 原始 payload。
5. 更新 `docs/features/feature-runtime-foundation-validation.md` 与真实场景测试用例。
