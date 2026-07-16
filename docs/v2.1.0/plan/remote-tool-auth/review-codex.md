# Remote Tool 出站鉴权独立审查

## 审查方式

- command: `codex exec review --uncommitted --ephemeral`
- reviewer session: `019f6925-8639-72f1-ab7f-e2936ab25624`
- scope: issue #24 uncommitted diff

## Findings

1. `[P2]` `env://TOKEN/path` 与 `env://TOKEN:1` 可通过通用 URI 校验，但 MVP env resolver 无法执行。
2. `[P2]` `http.CanonicalHeaderKey` 不验证 field-name，换行、冒号或空格可进入 `X-*` header registration。
3. `[P2]` core 接受通用 provider reference，但默认 Service 只装配 env resolver，`vault://` 可被发布后永久执行失败。

## Resolution

- `env` provider 在注册阶段额外要求 exact `env://UPPER_CASE_VARIABLE_NAME`，拒绝 path、port 和非法变量名。
- custom header 使用显式 `X-` token validator，仅允许字母、数字和连字符，并拒绝空后缀。
- app 装配层在 upsert 与 restart restore 时校验 provider 是否已安装；当前默认仅允许 `env://`，core resolver contract 继续保持 provider-neutral。
- 三类 finding 均新增回归用例；修复后重新执行聚焦测试、全仓测试与独立 review。
- final reviewer session `019f692c-b842-71a0-b035-9a365d7594b0` 结论：无可执行 correctness finding。

## Review Environment Note

临时 reviewer sandbox 禁止监听本地端口，因此其 `internal/server` 全包测试在既有 `httptest.NewServer` 用例处失败；宿主环境的同一测试命令可正常运行。This is a reviewer sandbox limitation, not a product test failure.

交付前全包 race 复测还暴露既有 `scene.SetSourcesRoot` / `workflow.SetSourcesRoot` 全局变量并行写竞争；该位置不在 issue #24 diff。`internal/tools` 与 `internal/server` 全包 race 通过，remote auth 相关 `tools/app/server` 定向 race 通过，因此将该基线竞争留作独立治理任务，不混入本次鉴权改动。
