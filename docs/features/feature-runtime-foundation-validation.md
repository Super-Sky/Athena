# Runtime Foundation Validation

## 背景

v2.1.0 需要把 RuntimeContract foundation 与 Batch 2 计划冻结成可追踪、可重复验证的工作面。本功能补齐两件事：

- 一条可运行的 Control Plane runtime foundation smoke 路径。
- 一组稳定的 System Validation DOM anchors，供 Codex in-app Browser 或 Playwright 做严格页面验收。

## 范围

- canonical checklist 维护在 `docs/v2.1.0/plan/master-plan.md`。
- smoke 脚本验证 RuntimeContract、registered task type、hook binding、active System Truth pointer、deterministic validation run 和 runtime persistence readout。
- runtime persistence readout 已包含 checkpoint-backed waiting run 安全摘要，仅展示 checkpoint ID、stage、resume token 是否存在、payload size/hash 与时间戳。
- 前端提供稳定 `data-testid` 自动化锚点，并在 System Validation 中展示 checkpoint 安全摘要；不展示私有 checkpoint payload。

## 不做什么

- 不接管业务 evidence ownership。
- 不暴露 Eino checkpoint private payload。
- 不在本轮实现完整 registered task validator。
- 不在本轮实现完整 System Truth write/edit lifecycle。

## 验证

在启用 runtime persistence 的后端上运行：

```bash
python3 scripts/control_plane_runtime_foundation_smoke.py --base-url http://127.0.0.1:8090
```

启动 Web 控制台并安装 Python Playwright 后，可运行 DOM 路径：

```bash
python3 scripts/control_plane_runtime_foundation_smoke.py --base-url http://127.0.0.1:8090 --web-url http://127.0.0.1:5173
```

DOM 路径会通过稳定 `data-testid` 检查 System Validation tab、Runtime Persistence Readout、Runtime foundation snapshot、capability surface、编辑器和验证触发按钮。
本轮还会检查 `runtime-checkpoint-readout`，并通过 API smoke 校验 checkpoint readout 不含 raw `payload` 或 `resume_token` 字段。

本轮已完成的验证证据：

- `go test ./internal/runtime ./internal/app ./internal/server`
- `cd web && PATH=$HOME/.cache/codex-runtimes/codex-primary-runtime/dependencies/node/bin:$PATH npm run build`
- `go test ./internal/controlplane ./internal/server ./internal/app ./internal/runtime`
- `cd web && npm run build`
- `python3 -m py_compile scripts/control_plane_runtime_foundation_smoke.py`
- `python3 scripts/check_no_absolute_paths.py`
- API smoke 返回 `contracts=1`、`task_types=1`、`hook_bindings=3`、`active_system_truths=54`
- checkpoint readout smoke 返回 `records.checkpoints=0`，确认无等待态 checkpoint 时为安全空列表 fallback
- Web DOM smoke 返回 `web.dom=ok`
- Codex in-app Browser 在 System Validation tab 复查新增 DOM anchors，结果为 `missing=[]`
- self-review 后 DOM smoke 已收紧为所有目标 `data-testid` 必须唯一且可见，并在失败时关闭 Chromium。

## 维护结论

本功能暂不抽取独立 skill。当前维护入口足够小，脚本与 feature 文档即可承载；如果后续 Runtime foundation smoke 扩展到多环境、多浏览器或 CI gate，再抽取专门 skill。
