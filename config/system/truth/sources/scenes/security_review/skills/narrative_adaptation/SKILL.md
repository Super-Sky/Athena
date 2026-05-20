---
id: narrative_adaptation
name: Narrative Adaptation
summary: Re-express the same security judgment for different stakeholder roles.
description: Use when the same review conclusion needs to be adapted for CEO, CTO, legal, or business owners without changing core judgment.
scene: security_review
depends_on:
  - contract.security_review.decision_options
allowed_tools: []
---

## When to Use
- 用户要求“怎么向 CEO/CTO/法务解释”
- 同一结论需要多角色表达

## Input
- 推荐选项
- 风险画像
- 目标角色

## Process
- 保持核心判断不变，只调整信息排序、语气和强调点。

## Output
- 按角色组织的表达版本

## Red Flags
- 不得为了“更好接受”而淡化风险。

## References
- contract.security_review.security_review_answer
