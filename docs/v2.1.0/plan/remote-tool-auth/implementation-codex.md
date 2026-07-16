# Remote Tool 出站鉴权实现记录

## 任务

- issue: `Super-Sky/Athena#24`
- branch: `codex/remote-tool-auth-issue-24`
- base: `codex/remote-tool-registry-issue-9`
- current state: `ready_for_delivery`

## Core / Application Boundary

- Athena core 只定义通用 outbound auth reference、secret resolver、header injection、稳定错误和安全观测。
- `athena-fund-assistant` 等业务服务负责 callback 端身份校验和业务 error code，不把基金账户或业务权限语义写入 core。

## 实现

- `RemoteRegistration.auth` 仅保存 `type`、`secret_ref` 和可选 `header_name`。
- `env://VARIABLE_NAME` resolver 在每次调用时读取 runtime environment，并支持 `_REVOKED` 与 `_EXPIRES_AT` metadata。
- 凭据在治理通过、HTTP request 建立后才解析和注入；missing/revoked/expired/invalid secret 在 `client.Do` 前 fail closed。
- persisted registration、API readout 和 trace 只暴露 secret reference 与 safe auth result。

## 验证

- 聚焦测试覆盖 bearer/custom header、rotation、revocation、expiration、restart restoration、callback rejection 和 no-leak。
- `BenchmarkRemoteAuthResolution` 三次基线约为 `97.20-101.1 ns/op`、`24 B/op`、`1 allocs/op`。
- `env -u APP_ENV go test ./...` 通过。
- `go vet ./internal/tools ./internal/app ./internal/server` 通过。
- `python3 scripts/check_no_absolute_paths.py` 与 `git diff --check` 通过。
- 出站鉴权相关定向 race 测试通过：`go test -race ./internal/tools ./internal/app ./internal/server -run 'RemoteTool(RegistrationExecutionRestoreAndDelete|OpenAPIContractIsTyped|InjectsResolvedAuthenticationWithoutLeakingValue|RejectsInvalidSecretStatesBeforeNetwork|ValidatesAuthenticationContract|ReturnsCallbackAuthenticationRejection)|ResolveRemoteToolSecret|ValidateRemoteToolSecretProvider' -count=1 -parallel=1`。
- 全包 race 复测暴露既有 `scene.SetSourcesRoot` / `workflow.SetSourcesRoot` 全局变量并行写竞争；该位置不在本次 diff，`internal/tools` 与 `internal/server` 全包 race 均通过，本次鉴权链路定向 race 也通过。
- 独立 review 的三项 P2 finding 已修复；最终复审结论为无可执行 correctness finding。

## 后续

- Athena #24 合入后，由 fund service callback 增加匹配的 service token 校验，并执行双服务 deny/pass smoke。
- Vault、cloud secret manager 等 provider 通过 `RemoteSecretResolver` 扩展，不改变 registration 和 trace contract。
