---
id: decision_planning
name: Decision Planning
summary: Turn risk findings into differentiated governance options.
description: Use when a risk profile exists and the user needs conservative, balanced, and aggressive governance options.
scene: security_review
depends_on:
  - contract.security_review.risk_profile
  - contract.security_review.decision_options
allowed_tools: []
---

## When to Use
- 已有风险画像，用户需要治理选项或下一步建议

## Input
- risk profile
- 关键约束与治理目标

## Process
- 设计 conservative、balanced、aggressive 三档选项。
- 明确 tradeoffs 与 recommended default。

## Output
- contract.security_review.decision_options

## Red Flags
- 三档选项不能只是换一种说法。
- 不要在无证据时给出伪确定性推荐。

## References
- contract.security_review.decision_options
