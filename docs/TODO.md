# TODO

本文件只记录 Athena 当前仓库还未完成、但已经明确的后续工作。

不在这里记录：

- 已经完成的架构决策
- 其他项目的 API 设计
- 聊天记录里的临时想法

## P0 当前优先级

### 1. Waiting / 补数异常协议收口

- [x] 补一份客户端 outcome 使用约定，明确 `provided / unable_to_provide / abandon_and_continue / pending_human / timeout_expired` 的适用边界
- [x] 把 `resume_token`、`gap_closed`、`invalid_resume_token`、deferred queue 的使用规则收成统一协议说明
- [x] 在 `api.md` 中补足“单 session 同时只允许一个 active waiting gap”的硬约束表达
- [x] 统一 `tombstone expired / policy reject / timeout closed` 的 detail 字段约定
- [x] 明确 queue overflow、gap closed、invalid resume token 的客户端处理建议
- [x] 将 SSE 生命周期结束事件统一改为 `done`
- [x] 明确 `action / notification / error / result / done` 的事件边界与出现规则

### 2. Waiting / deferred queue 持久化评估

- [x] 评估 waiting gap、deferred queue、closed token tombstone 是否需要外部持久化
- [x] 明确多实例、进程重启、长等待场景下的状态恢复边界
- [x] 落实 deferred queue 上限
- [x] 落实 closed token tombstone 的 TTL 与清理策略
- [x] 明确 queue 超限与 tombstone 过期后的对外观测口径
- [x] 决定外部持久化仍以 `session.Store` 为聚合根升级
- [x] 为 `session.Store` 设计 PostgreSQL 版本的最小 schema 与原子更新策略
- [x] 预埋 PostgreSQL 版 `session.Store` 的真实实现与 optimistic locking 重试策略
- [x] 补 PostgreSQL 集成测试入口，支持真实库环境下的显式验证
- [x] 将 `deferred_queue` 从 session 主表设计中拆出，改为独立队列表并保留 session 聚合语义
- [x] 基于 `sessions + session_deferred_messages` 落地 PostgreSQL 物理表拆分与原子消费实现

## P1 脚手架增强

### 3. Observability 做实

- [x] 增加结构化 trace 关联
- [x] 增加 metrics 输出
- [x] 增加 audit 持久化接口
- [x] 把内存观察方式收成可重复执行的检查步骤
- [x] 完成 metrics / audit / trace 外部导出与持久化评估
- [x] 明确 observability 日志默认采用 `debug / info / warn / error` 四级，并区分摘要日志与调试细节
- [x] 为关键模块和关键动作补统一结构化动作日志
- [ ] 如需接入真实后端，优先通过入口层注入自定义 observability manager
- [x] 评估 PostgreSQL session store 的真实接入与 migrate 发布流程
- [x] 明确 observability 接真实后端时的最小导出契约和推荐实现顺序

### 4. Skill 上传链路预埋完善

- [x] 定义 `SkillLoader`
- [x] 定义 `SkillStore`
- [x] 定义 `SkillSource`
- [x] 明确 builtin skill 与上传 skill 的统一装配入口
- [x] 为上传 skill 补具体 API / transport 方案，保持官方 skill 包形态
- [x] 决定上传 skill store 如何保存官方 skill 包，而不是把上传 skill 编译进二进制
- [x] 为 uploaded skill package 补运行时适配链，实现 package -> declaration -> loader
- [x] 为上传 skill package 增加正式 transport handler 与最小校验
- [x] 为 uploaded skill package 补更完整的上传校验、启用/禁用和治理能力
- [x] 明确 uploaded skill package 的命名冲突策略：创建不允许重名，治理主键使用服务端生成的 `id`
- [x] 明确 uploaded skill package 的 replace 策略和治理审计输出
- [x] 明确 uploaded skill package 的 replace 审批、版本留存和回滚策略
- [x] 明确 uploaded skill package revision 的保留上限与清理策略

### 5. Fast Path 业务化接入

- [ ] 选择一个真实 fast path 场景
- [ ] 明确 fast path 与标准 runtime path 的切换条件
- [ ] 补齐 fast path 命中/禁用/回退的最小验证用例

## P1 业务接入准备

### 6. 第一个真实分析场景

- [ ] 选择一个最小真实分析场景接入当前骨架
- [ ] 验证 skill 选择、补数请求、补数回流、deferred queue、超时与降级
- [ ] 形成一份独立 feature 文档和 review checklist

## P2 后续演进

### 7. Session / Memory 深化

- [ ] 评估 memory 策略是否需要区分短期历史与长期记忆
- [ ] 评估补数信息是否需要结构化并入历史
- [ ] 评估是否允许一个 session 在未来支持多个 gap

### 8. 文档持续收敛

- [ ] 保持一类信息只有一个主文档
- [ ] 把跨版本稳定规则逐步上移到共享正文
- [ ] 避免 `v0.1.0` 目录里出现重复解释同一规则的文档

## 已完成但需持续维护

- [x] `internal/runtime` / `internal/server` 首批测试已补齐
- [x] POST SSE、`resume_token`、`information_request`、`pending_human`、`abandon_and_continue`、`gap_closed`、`invalid_resume_token` 已进入主链
- [x] waiting 期间普通新消息会进入 deferred queue，并在 gap 关闭后自动消费一条
- [x] fast path 已预埋接口、禁用开关与可见性，但尚未接真实业务
- [x] waiting state / deferred queue / tombstone 的持久化边界已完成第一轮评估
