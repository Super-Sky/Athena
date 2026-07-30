# Privileged Trace Payload / 特权 Trace 载荷

## Background / 背景

Issue `Super-Sky/Athena#30` adds an opt-in detail plane for system administrators who must reconstruct what an Agent actually sent to a model, what the model returned, and which Skill, context, or Tool inputs affected the run. The existing timeline remains a safe summary and is not widened into a raw transcript API.

Issue `Super-Sky/Athena#30` 为需要复盘 Agent 实际模型请求、模型返回以及 Skill、Context、Tool 输入影响的系统管理员增加显式开启的明细平面。既有 timeline 继续保持安全摘要契约，不扩张为原始 transcript API。

## Security Rules / 安全规则

- Capture and read are disabled by default and require a dedicated encryption secret, key ID, and Control Plane authentication.
- Model messages/responses, Tool arguments/results, and Skill/context assembly summaries are recursively redacted before AES-256-GCM encryption.
- Credential-like values, Authorization, cookies, account/user identifiers, brokerage account fields, attachment/file bodies, and hidden reasoning fields are always replaced before persistence. Structured multimodal attachment bodies are not projected into this channel; defense-in-depth content patterns reject data URIs, private keys, JWTs, and labeled brokerage accounts even under generic text fields. Deployments may add denylisted field names.
- App-facing Agent Run trace and timeline endpoints never expose `payload_ref`; only the authenticated Control Plane timeline can return the opaque reference and capture status.
- Every payload read attempt is durably audited, including denied, missing, expired, failed, and successful outcomes. If the audit write fails, content is not returned.

- 采集和读取默认关闭，启用时必须配置独立加密密钥、key ID 与 Control Plane 鉴权。
- 模型 messages/response、Tool 参数/结果和 Skill/Context assembly 摘要会先递归脱敏，再使用 AES-256-GCM 加密。
- 凭据、Authorization、Cookie、账户/用户标识、券商账号、附件/文件正文和隐式推理字段永不落盘；结构化多模态附件正文不会投影到该通道，通用文本字段中的 data URI、私钥、JWT 和带标签券商账号还会被内容模式再次拦截；部署方可以追加字段 denylist。
- app-facing Agent Run trace/timeline 永不返回 `payload_ref`；只有已认证的 Control Plane timeline 可返回 opaque 引用与采集状态。
- 拒绝、未找到、过期、失败和成功读取都会写入持久审计；审计写入失败时不返回内容。

## Data Flow / 数据链路

`RuntimeCallbackRecorder` captures request-local model and Tool callback detail. `RuntimeToolTranscriptProjector` captures the canonical Tool call/result pair, while `RuntimeTerminalProjector` emits a separate Skill/context assembly summary. `projectPrivilegedTracePayload` applies workspace/component policy, deterministic sampling, recursive redaction, record/run size limits, retention, and encryption before `PostgresRuntimeStore` stores ciphertext in `runtime_privileged_trace_payloads`.

`RuntimeCallbackRecorder` 采集请求级模型与 Tool callback 明细；`RuntimeToolTranscriptProjector` 采集 canonical Tool call/result；`RuntimeTerminalProjector` 另行生成 Skill/Context assembly 摘要。`projectPrivilegedTracePayload` 在 `PostgresRuntimeStore` 写入 `runtime_privileged_trace_payloads` 密文前依次执行 workspace/component 策略、确定性采样、递归脱敏、单条/单 run 容量限制、保留期和加密。

The safe `RuntimeTrace` stores only `payload_status`, an unavailable reason, and, for recorded payloads, the opaque `payload_ref` and expiry. The app-facing projection strips references. The Control Plane detail endpoint binds lookup to both `runID` and `payloadRef`, decrypts with the dedicated key boundary, checks retention, records an immutable access audit, and returns `Cache-Control: no-store`. AES-GCM associated data authenticates the correlation coordinates, schema/key identity, digest, size/redaction counters, and microsecond-normalized creation/expiry timestamps, so retention metadata cannot be extended independently of the ciphertext.

安全 `RuntimeTrace` 只保存 `payload_status`、不可用原因，以及成功采集时的 opaque `payload_ref` 与过期时间。app-facing 投影会移除引用。Control Plane 明细接口同时绑定 `runID` 和 `payloadRef`，通过独立密钥边界解密、检查保留期、写入不可变访问审计，并返回 `Cache-Control: no-store`。AES-GCM 关联数据会认证关联坐标、schema/key 标识、digest、容量/脱敏计数及微秒归一化的创建/过期时间，因此不能脱离密文单独延长保留期。

## Configuration / 配置

The primary environment variables are:

- `PRIVILEGED_TRACE_PAYLOAD_CAPTURE_ENABLED`
- `PRIVILEGED_TRACE_PAYLOAD_READ_ENABLED`
- `PRIVILEGED_TRACE_PAYLOAD_SAMPLE_RATE`
- `PRIVILEGED_TRACE_PAYLOAD_RETENTION_HOURS`
- `PRIVILEGED_TRACE_PAYLOAD_MAX_RECORD_BYTES`
- `PRIVILEGED_TRACE_PAYLOAD_MAX_RUN_BYTES`
- `PRIVILEGED_TRACE_PAYLOAD_CAPTURE_MODEL`
- `PRIVILEGED_TRACE_PAYLOAD_CAPTURE_TOOLS`
- `PRIVILEGED_TRACE_PAYLOAD_CAPTURE_CONTEXT_SUMMARY`
- `PRIVILEGED_TRACE_PAYLOAD_DISABLED_WORKSPACE_IDS`
- `PRIVILEGED_TRACE_PAYLOAD_EXTRA_REDACTED_FIELDS`
- `TRACE_PAYLOAD_ENCRYPTION_KEY`
- `TRACE_PAYLOAD_ENCRYPTION_KEY_ID`

Defaults are a 24-hour retention, 256 KiB per record, 2 MiB per run, full deterministic sampling, and all three component captures enabled once the overall capture switch is enabled. The dedicated encryption secret must contain at least 32 bytes of random material. Key rotation currently requires retaining the configured key that matches stored `key_id`; multi-key historical decryption is intentionally deferred.

默认保留 24 小时、单条 256 KiB、单 run 2 MiB、确定性全量采样；总采集开关开启后，model、tool、context summary 三类采集默认启用。独立加密 secret 必须包含至少 32 字节随机材料。当前密钥轮换要求保留与历史 `key_id` 匹配的配置密钥；多密钥历史解密暂不在本期范围。

## API And UI / API 与页面

`GET /api/control-plane/runtime/runs/:runID/trace-payloads/:payloadRef` requires a valid platform Control Plane administrator session and returns only the decrypted, already-redacted payload envelope. Control Plane administration is currently a platform-wide trust boundary rather than workspace-scoped RBAC; every attempt is attributed to a session hash and remote IP in the access audit. The `运行观测` inspector shows capture status and performs the audited read only after the administrator clicks `查看明细`; it does not preload payloads while browsing the timeline and discards stale responses after run/step switches.

`GET /api/control-plane/runtime/runs/:runID/trace-payloads/:payloadRef` 要求有效的平台级 Control Plane 管理员 session，只返回已完成脱敏后的解密载荷包络。当前 Control Plane 管理属于平台级信任边界，不是 workspace 级 RBAC；每次尝试都会把 session hash 与远端 IP 写入访问审计。后台 `运行观测` 检查器展示采集状态，仅在管理员点击 `查看明细` 后执行审计读取，浏览 timeline 时不会预加载明细，切换 run/步骤后会丢弃旧请求返回。

## Verification / 验证

Focused tests cover redaction/encryption/tamper rejection, deterministic sampling and size limits, context capture policy, PostgreSQL run-budget and expiry cleanup, control-plane authorization/audit outcomes, and app/control-plane reference isolation. The redactor also has an explicit nil-value regression test: a real model response exposed that an untyped nil previously re-entered JSON normalization recursively and could exhaust the process stack.

聚焦测试覆盖脱敏/加密/篡改拒绝、确定性采样与容量限制、Context 采集策略、PostgreSQL 单 run 预算与过期清理、控制面鉴权/审计结果，以及 app/control-plane 引用隔离。脱敏器还包含显式 nil 值回归测试：真实模型响应暴露出未类型化 nil 曾反复进入 JSON 归一化并可能耗尽进程栈的问题。

The PostgreSQL-backed end-to-end smoke completed run `3040a1a3-4b66-45e7-8ddd-9fe44ed43302` against an OpenAI-compatible fake provider. It recorded separate context and model payloads, returned no reference from the app timeline, kept the plaintext response out of PostgreSQL ciphertext, and wrote a successful `control_plane_trace_inspection` audit before returning the detail with `Cache-Control: no-store`. Browser verification confirmed that the detail is not preloaded, appears only after `查看明细`, contains the expected model response, hides the injected test secret, and causes no horizontal overflow at 1280 px.

PostgreSQL 真实端到端 smoke 使用 OpenAI 兼容假 provider 完成 run `3040a1a3-4b66-45e7-8ddd-9fe44ed43302`。该 run 分别记录 Context 与 Model 载荷，app timeline 不返回引用，PostgreSQL 密文中不存在响应明文，并在返回 `Cache-Control: no-store` 明细前成功写入 `control_plane_trace_inspection` 审计。浏览器验收确认明细不会预加载，仅在点击 `查看明细` 后出现，包含预期模型响应、不包含注入测试密钥，且 1280 px 下无横向溢出。

`BenchmarkBuildPrivilegedTracePayload` measured 18.3-40.6 ms/op, about 15.3 KiB/op, and 54 allocations/op over three 100-iteration samples on the local Intel i5-7Y54 machine after the final content-pattern and metadata-authentication hardening. The host was thermally throttled, so this is a conservative local baseline rather than a production SLO.

最终内容模式与元数据认证加固后，`BenchmarkBuildPrivilegedTracePayload` 在本机 Intel i5-7Y54 上执行三组各 100 次，结果为 18.3-40.6 ms/op、约 15.3 KiB/op、54 allocs/op。测试时机器存在热降频，因此该结果只作为偏保守的本地基线，不作为生产 SLO。

## Skill Decision / Skill 结论

No feature-specific skill is added. This document and the `internal/runtime`, `internal/app`, and `internal/server` module indexes are the stable maintenance entrypoints; existing repository delivery skills already enforce API, security, test, and documentation synchronization.

本功能不新增专属 Skill。本文与 `internal/runtime`、`internal/app`、`internal/server` 模块索引作为稳定维护入口；既有仓库交付 Skill 已覆盖 API、安全、测试与文档同步要求。
