---
id: reuse_judgment
name: Reuse Judgment
summary: Judge whether historical knowledge is safe and applicable to the current scenario.
description: Use when the system retrieves historical knowledge and must decide direct reuse, adapt_with_change, reference_only, or block.
scene: knowledge
depends_on:
  - contract.knowledge.reuse_decision
  - policy_rule.core.knowledge_governance
allowed_tools: []
---

## When to Use
- 需要判断历史知识是否可直接复用。

## Input
- current context
- candidate knowledge

## Process
- 先做 sharing level、freshness、confidence 硬阻断检查。
- 再判断 applicability 与 recompute_required_parts。

## Output
- contract.knowledge.reuse_decision

## Red Flags
- tenant_only 知识绝不跨租户。
- deprecated 知识不得复用。

## References
- policy_rule.core.knowledge_governance
