# Agent Trace Timeline 真实场景测试用例

# Agent Trace Timeline Real-World Test Cases

## 前置条件 / Prerequisites

启动带 runtime persistence 的 Athena，并使用一次 Agent Run 或 Control Plane runtime validation run 生成 persisted run。Start Athena with runtime persistence and create one persisted Agent Run or Control Plane runtime validation run.

## 1. 业务应用读取 / App Read

```bash
curl -sS http://127.0.0.1:8080/api/agent/runs/<run-id>/timeline
```

预期：返回 `run`、按 `timestamp` 升序的 `items` 和 summary。Expected: the response contains `run`, timestamp-ascending `items`, and a summary.

## 2. 后台安全详情 / Admin Safe Detail

登录 Control Plane 后打开 System Validation，选择该 run 并展开 Agent Trace Timeline 任一条目。确认能看到 status、source、duration（如有）、safe labels、redacted payload 和 metadata；不得出现 raw prompt、raw tool args/results、凭据或基金/持仓对象。

After Control Plane login, open System Validation, select the run, and expand any Agent Trace Timeline item. Confirm status, source, duration when available, safe labels, redacted payload, and metadata are visible; raw prompts, raw tool args/results, credentials, and fund/holding objects must never appear.

## 3. 治理失败 / Governance Failure

使用一条被拒绝的 tool governance decision 触发 run 或 validation run。预期：对应 `governance` 条目包含失败 status、reason/decision metadata，且 summary 中 `failure_count` 增加。

Trigger a run or validation run with a denied tool governance decision. Expected: the corresponding `governance` item contains failure status plus reason/decision metadata, and summary `failure_count` increases.
