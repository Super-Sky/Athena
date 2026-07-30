# External Memory / Context 真实场景测试用例

# External Memory / Context Real-World Test Cases

## 前置条件

启动 Athena 本地服务，并确认 `GET /healthz` 返回 `200`。本清单使用 `fund-assistant` 作为应用标识，但不向 Athena 写入基金、持仓或交易对象。

## Prerequisites

Start Athena locally and confirm `GET /healthz` returns `200`. This checklist uses `fund-assistant` as an application identifier, but never writes fund, holding, or trade objects into Athena.

## 1. 写入画像摘要

```bash
curl -sS -X POST http://127.0.0.1:8080/api/memory/write \
  -H 'Content-Type: application/json' \
  -d '{"app_id":"fund-assistant","owner_id":"demo-user-1","scope":"investor-profile","kind":"profile-summary","summary":"Balanced risk preference; long investment horizon.","schema_version":"fund-profile.v1"}'
```

预期：返回 `201`，`item` 使用 snake_case 字段，`trace.operation=memory_write`，且 trace 只含归属和 record ID。

Expected: returns `201`; `item` uses snake_case fields; `trace.operation=memory_write`; the trace contains only ownership metadata and record IDs.

## 2. 查询同一 owner 的摘要

```bash
curl -sS -X POST http://127.0.0.1:8080/api/memory/query \
  -H 'Content-Type: application/json' \
  -d '{"app_id":"fund-assistant","owner_id":"demo-user-1","scope":"investor-profile"}'
```

预期：返回 `200`，`items` 包含上一步摘要，`trace.operation=memory_query`。

Expected: returns `200`; `items` includes the prior summary; `trace.operation=memory_query`.

## 3. 验证 owner 隔离

```bash
curl -sS -X POST http://127.0.0.1:8080/api/memory/query \
  -H 'Content-Type: application/json' \
  -d '{"app_id":"fund-assistant","owner_id":"demo-user-2","scope":"investor-profile"}'
```

预期：返回 `200` 和空 `items` 数组；绝不能看到 `demo-user-1` 的摘要。

Expected: returns `200` with an empty `items` array; it must never reveal `demo-user-1` summaries.

## 4. 解析并组装上下文

```bash
curl -sS -X POST http://127.0.0.1:8080/api/context-assets/assemble \
  -H 'Content-Type: application/json' \
  -d '{"app_id":"fund-assistant","owner_id":"demo-user-1","scope":"investor-profile","query":"Review my risk preferences"}'
```

预期：返回只读 `memory_view` asset，`source_kind=app_memory`，包含 `context_trace`、`effective_views.effective_memory_view` 和 `context_compression.schema_version=external_context_compression.v1`。

Expected: returns a read-only `memory_view` asset with `source_kind=app_memory`, plus `context_trace`, `effective_views.effective_memory_view`, and `context_compression.schema_version=external_context_compression.v1`.

## 5. 边界确认

检查四个响应：Athena 只收到摘要文本与应用定义 metadata，没有基金代码、持仓数量、交易指令、账户凭据或资金操作字段。进程重启后默认 in-process store 会清空；生产使用前须由业务应用提供已认证主体绑定与 durable persistence adapter。

Inspect all four responses: Athena receives only summary text and application-defined metadata, never fund symbols, holding quantities, trading instructions, account credentials, or money-operation fields. The default in-process store clears after restart; production use requires the business app to bind authenticated principals and provide a durable persistence adapter.
