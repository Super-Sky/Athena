---
id: cso_review
name: CSO Review
summary: Run a CSO-style security review across architecture, controls, evidence, and governance options.
description: Use when the user explicitly asks for a CSO-style security review or when the security_review scene should coordinate the full review flow.
scene: security_review
depends_on:
  - contract.security_review.risk_profile
  - contract.security_review.decision_options
  - contract.security_review.security_review_answer
  - policy_rule.core.safety_constitution
allowed_tools: []
---

## When to Use
- 用户要求 CSO 风格审查、全局安全评估或综合风险判断。
- 需要协调多项子 skill 完成完整 security review。

## Input
- 当前请求
- 已加载上下文
- scene.security_review 默认 contract

## Process
- 组织 signal_extraction、threat_modeling、risk_profiling、decision_planning、evidence_reporting。
- 保证输出围绕风险、证据、建议和下一步动作展开。

## Output
- 完整 security review answer

## Red Flags
- 不要把 umbrella skill 退化成空洞摘要。
- 不要跳过 evidence 与边界说明。

## References
- 需要时读取同 scene 下其他 skill 与 contract。
