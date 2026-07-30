# Async Job Module / 异步任务模块

## Module Role / 模块职责

`internal/asyncjob` owns Athena's durable asynchronous execution contract. PostgreSQL is the authoritative state, event, lease, idempotency, and result store; Redis Streams only carries lightweight delivery references.

`internal/asyncjob` 承接 Athena 的持久异步执行契约。PostgreSQL 是状态、事件、租约、幂等和结果的权威存储；Redis Streams 只承载轻量投递引用。

## Non-goals / 非职责

- It does not execute domain-specific business logic.
- It does not store fund, portfolio, trading, or application-owned objects.
- It does not expose HTTP routes directly; `internal/server` maps the contract to Agent Run APIs.
- It does not make Redis the source of truth for payloads or lifecycle state.

- 不执行具体业务领域逻辑。
- 不保存基金、组合、交易或应用拥有的业务对象。
- 不直接暴露 HTTP route；`internal/server` 负责映射到 Agent Run API。
- 不把 Redis 作为 payload 或生命周期状态真相。

## File Index / 文件索引

- `types.go`
  - Defines public statuses, lifecycle events, store boundaries, delivery boundaries, job inputs, completion and failure envelopes.
  - 定义公开状态、生命周期事件、store 边界、delivery 边界、job 输入、完成和失败包络。
- `crypto.go`
  - Encrypts async request/result payloads with AES-GCM and job-specific associated data before durable storage.
  - 在持久化前用 AES-GCM 和 job 相关附加认证数据加密异步请求/结果 payload。
- `postgres_store.go`
  - Persists authoritative jobs, outbox records, immutable lifecycle events, leases, retries, waiting/resume, cancellation and idempotency.
  - 持久化权威 job、outbox、不可变生命周期事件、租约、重试、等待/恢复、取消和幂等。
- `redis_delivery.go`
  - Implements Redis Streams reference delivery, pending reclaim, ACK and idle-time renewal.
  - 实现 Redis Streams 引用投递、pending 回收、ACK 和空闲时间续租。
- `dispatcher.go`
  - Polls PostgreSQL outbox rows and publishes delivery references to Redis after winning an outbox lease.
  - 轮询 PostgreSQL outbox，在赢得 outbox lease 后向 Redis 发布投递引用。
- `worker.go`
  - Claims Redis deliveries, acquires PostgreSQL job leases, renews both lease layers, executes handlers, and commits terminal/waiting/retry state.
  - 领取 Redis 投递、获取 PostgreSQL job lease、续租两层租约、执行 handler，并提交终态/等待/重试状态。
- `crypto_test.go`
  - Verifies authenticated encryption round-trips and associated-data rejection.
  - 验证认证加密往返和附加认证数据不匹配拒绝。
- `redis_delivery_test.go`
  - Verifies reference-only Redis payloads, ACK, real Redis pending reclaim and renewal behavior.
  - 验证 Redis 只写引用、ACK、真实 Redis pending 回收和续租行为。
- `postgres_store_integration_test.go`
  - Verifies PostgreSQL-backed create, idempotency, outbox, lease, retry, wait/resume, cancel and event cursor behavior.
  - 验证 PostgreSQL create、幂等、outbox、lease、retry、wait/resume、cancel 和 event cursor 行为。
- `worker_test.go`
  - Verifies worker ACK and monitor behavior when leases or Redis renewals fail.
  - 验证 lease 或 Redis 续租失败时的 worker ACK 和 monitor 行为。

## Maintenance Notes / 维护提示

- Any state transition that changes a job status must write its lifecycle event in the same PostgreSQL transaction.
- 任何修改 job 状态的状态转换都必须在同一个 PostgreSQL 事务中写入 lifecycle event。
- A Redis delivery may be duplicated or reclaimed; handler execution must still be guarded by `AcquireLease`.
- Redis delivery 可能重复或被回收；handler 执行必须仍由 `AcquireLease` 保护。
- Do not put raw request payloads, model responses, tool arguments, tool results, or business objects into Redis.
- 不要把原始请求 payload、模型响应、工具参数、工具结果或业务对象写入 Redis。
