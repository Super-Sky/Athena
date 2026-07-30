# Deterministic Built-in Tools / 确定性内置工具

## Background / 背景

Issue `Super-Sky/Athena#14` adds the first Core tool pack that a business application can enable through an Agent Run without registering an application callback. These tools remain generic: Athena never receives fund, portfolio, brokerage, or other business objects.

issue `Super-Sky/Athena#14` 增加第一组 Core 工具包。业务应用可以通过 Agent Run 启用它们，而无需注册应用回调。这些工具保持通用性：Athena 不接收基金、持仓、券商或其他业务对象。

## Included tools / 已包含工具

- `calculator`: evaluates a bounded arithmetic expression with numbers, `+`, `-`, `*`, `/`, `%`, parentheses, and unary signs. It does not evaluate scripts, variables, files, or network input.
- `current_time`: returns an RFC3339 timestamp and Unix seconds for an IANA timezone; the default timezone is `UTC`.
- `json_schema_validate`: validates a JSON-compatible value using the documented `json_schema_subset.v1` subset: `type`, `properties`, `required`, boolean `additionalProperties`, `items`, `enum`, `minimum`, `maximum`, `minLength`, and `maxLength`.

- `calculator`：计算受限算术表达式，支持数字、`+`、`-`、`*`、`/`、`%`、括号和一元正负号；不执行脚本、变量、文件或网络输入。
- `current_time`：按 IANA 时区返回 RFC3339 时间戳和 Unix 秒；默认时区为 `UTC`。
- `json_schema_validate`：使用已文档化的 `json_schema_subset.v1` 校验 JSON 兼容值，支持 `type`、`properties`、`required`、布尔 `additionalProperties`、`items`、`enum`、`minimum`、`maximum`、`minLength` 和 `maxLength`。

## Runtime contract / 运行时契约

All three definitions are registered in the existing live tool catalog with `tool_scope=builtin_deterministic`, `side_effect_level=none`, and `requires_confirmation=false`. They follow the same canonical tool-call ID, governance, ordered transcript, timing, safe trace, and Agent Run response rules as any other registered tool.

三条定义都注册到现有 live tool catalog，并标记为 `tool_scope=builtin_deterministic`、`side_effect_level=none` 和 `requires_confirmation=false`。它们与其他已注册工具一样，遵循相同的 canonical tool-call ID、治理、有序 transcript、时序、安全 trace 和 Agent Run 响应规则。

The local executor evaluates governance before invocation and records only decision metadata, duration, argument keys, and the canonical call ID. Any deny, redaction-only decision, or sandbox-required decision fails closed because these deterministic tools do not implement those transformations.

本地执行器会在调用前评估治理，并且只记录决策元数据、耗时、参数键和 canonical call ID。拒绝、仅脱敏或要求 sandbox 的决策都会 fail closed，因为这组确定性工具不实现相应转换。

`json_schema_validate` is intentionally not a complete JSON Schema implementation. Unsupported schema types, malformed keyword values, oversized input, and depth beyond 24 fail closed with a tool error. Schema mismatches are returned as a normal structured result with `valid=false` and safe path/keyword diagnostics.

`json_schema_validate` 有意不实现完整 JSON Schema。未支持的 schema 类型、畸形关键字值、超大输入和超过 24 层的深度都会 fail closed 并返回工具错误。普通 schema 不匹配则作为结构化结果返回，其中包含 `valid=false` 以及安全的 path/keyword 诊断。

## Boundaries / 边界

- This Core slice does not add HTTP fetch, web search, file import, CSV import, credentials, or data-provider behavior.
- HTTP/search/file capability remains an Enhancement and must be added later with explicit network, sandbox, size, redaction, and provider-governance controls.
- Business-specific tools continue to belong to application repositories and use the remote tool registry when needed.

- 这个 Core 切片不增加 HTTP fetch、web search、文件导入、CSV 导入、凭据或数据 provider 行为。
- HTTP/search/file 能力仍属于 Enhancement，后续必须在明确的网络、sandbox、大小、脱敏和 provider 治理控制下接入。
- 业务专属工具继续属于应用仓；需要时通过 remote tool registry 接入。

## Verification / 验证

```bash
go test ./...
go test -race ./internal/tools
go test ./internal/tools -run '^$' -bench '^BenchmarkCalculatorExpression$' -benchmem -count=1
```

The focused tests cover expression bounds, divide-by-zero rejection, timezone validation, schema mismatch reporting, malformed schema rejection, callable catalog registration, governance denial, canonical call-ID propagation, and runtime declaration admission.

聚焦测试覆盖表达式边界、除零拒绝、时区校验、schema 不匹配报告、畸形 schema 拒绝、可调用 catalog 注册、治理拒绝、canonical call ID 传递和 runtime 声明准入。

## Skill decision / Skill 结论

No separate maintenance skill is added yet. The stable maintenance entry is this feature document plus `internal/tools/README.md`; existing `eino-component`, `eino-agent`, code-style, and delivery skills cover the required runtime and contract checks.

当前不新增独立维护 skill。稳定维护入口是本文档和 `internal/tools/README.md`；现有 `eino-component`、`eino-agent`、code-style 和交付 skills 已覆盖必要的 runtime 与契约检查。
