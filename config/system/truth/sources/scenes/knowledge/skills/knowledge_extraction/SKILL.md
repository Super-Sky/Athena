---
id: knowledge_extraction
name: Knowledge Extraction
summary: Extract reusable knowledge objects and candidates from completed cases.
description: Use after a case or execution closes and the system needs sanitized reusable knowledge candidates.
scene: knowledge
depends_on:
  - contract.knowledge.knowledge_object
  - contract.knowledge.knowledge_candidate
  - policy_rule.core.knowledge_governance
allowed_tools: []
---

## When to Use
- 案例闭环后需要提炼可复用知识。

## Input
- case summary
- lessons learned
- evidence summary

## Process
- 判断是否值得提炼。
- 做抽象、脱敏、适用条件与分享级别判定。

## Output
- knowledge object
- knowledge candidate

## Red Flags
- 不得保留原始敏感识别信息。
- 不得把单个案例的强结论直接标为 public。

## References
- contract.knowledge.knowledge_object
- contract.knowledge.knowledge_candidate
