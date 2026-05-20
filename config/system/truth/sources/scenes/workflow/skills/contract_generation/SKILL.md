---
id: contract_generation
name: Contract Generation
summary: Convert a chosen option or plan into an actionable governance contract.
description: Use when recommendations or workflow plans need to become executable governance contracts with owners, deadlines, and evidence requirements.
scene: workflow
depends_on:
  - contract.workflow.governance_contract
  - policy_rule.core.safety_constitution
allowed_tools: []
---

## When to Use
- 需要把建议、方案或计划转成可执行 contract。

## Input
- recommended option
- actions
- constraints

## Process
- 为 action 明确 owner、deadline、acceptance_criteria 和 evidence_required。
- 检查 approval requirement 与依赖关系。

## Output
- contract.workflow.governance_contract

## Red Flags
- 没有 owner 或 due date 的 action 不可交付。
- acceptance criteria 不能依赖主观感受。

## References
- contract.workflow.governance_contract
