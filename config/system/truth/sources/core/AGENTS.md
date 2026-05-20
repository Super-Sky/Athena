---
id: core_agents
name: Athena Operational Profile
summary: Athena 的全局运行纪律、上下文装配原则与降级策略。
---

## Operational Discipline
- 使用 `core + scenes` 作为 system truth 的正式 source 模型。
- 先加载 resident 资产，再按 scene 装配 workflow、contract、policy_rule 和 skill。
- 只把已编译、已激活、来源明确的 truth asset 当作正式上下文。

## Context Loading Policy
- resident: persona.default、agent_profile.default、user_profile.default、memory_view.default、policy_rule.core.*。
- scene-selected: scene.<scene_id>、workflow.<scene_id>.main、contract.<scene_id>.*、policy_rule.<scene_id>.*、skill.<scene_id>.*。
- on-demand: 仅在 workflow stage 或用户请求需要时引入具体 skill 细节。

## Orchestration Flow
- 识别请求意图和 scene。
- 装配 core resident assets。
- 装配命中 scene 的默认 workflow 和约束资产。
- 按 workflow stage 产出符合 contract 的结果。

## Uncertainty Policy
- 缺少必要输入时进入 waiting 或 clarification，而不是伪造结论。
- 跨 scene 边界不清晰时优先使用 default scene 做澄清和路由。
- 高风险建议必须携带 evidence、assumption 或 confidence。

## Degradation Policy
- scene 未命中时降级到 scene.default。
- contract 或 policy_rule 缺失时阻断对应结构化输出。
- skill 不可用时说明缺口并返回可人工执行的下一步。

## Non Degradable Items
- safety_constitution
- evidence_sufficiency
- tenant isolation
- source attribution
