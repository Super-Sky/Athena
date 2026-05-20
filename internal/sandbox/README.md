# internal/sandbox

## 模块职责

- 定义 Athena core runtime 与外部 sandbox 执行边界之间的最小稳定契约。
- 当前阶段只落地 `external_sandbox_ref` validation result：引用、结构化结果、审计摘要和安全 projection payload。

## 不负责什么

- 不实现远程 sandbox 平台、sandbox fleet 或浏览器插件执行。
- 不保存 raw credential、raw tool input 或外部平台私有 payload。
- 不承接业务 EvidenceRecord 语义。

## 文件索引

- `types.go`
  - 定义 `ExternalSandboxRef`、审计摘要、结构化结果和 validation request/result。
- `external_ref.go`
  - 构造 deterministic `external_sandbox_ref` validation result，并提供安全持久化 payload。
- `external_ref_test.go`
  - 覆盖结构化结果和 credential-like 字段脱敏。

## 对外入口

- `BuildExternalSandboxValidationResult`
- `ExternalSandboxValidationResult.RedactedPayload`

## 常见修改路径

- 新增 sandbox boundary 字段：先改 `types.go`，再补 API / OpenAPI / System Validation 展示。
- 接入真实远程 sandbox：新增独立 adapter，不要把 provider 私有字段塞进 core public contract。
