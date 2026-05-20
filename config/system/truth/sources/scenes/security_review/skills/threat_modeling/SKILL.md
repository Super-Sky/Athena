---
id: threat_modeling
name: Threat Modeling
summary: Build structural threat hypotheses and trust boundaries for downstream risk profiling.
description: Use when architecture or system structure needs deeper decomposition before business-facing risk synthesis.
scene: security_review
depends_on:
  - contract.security_review.risk_profile
allowed_tools: []
---

## When to Use
- 输入包含架构、集成、数据流、权限边界等结构信息。
- 不做这一步会导致后续 risk profile 过于模糊。

## Input
- 信号、系统描述、数据流、角色和边界信息

## Process
- 识别系统对象、数据对象、角色与 trust boundaries。
- 构造高价值威胁路径，不做 checklist dumping。

## Output
- threat hypotheses
- trust boundaries
- 供 risk_profiling 使用的结构化 threat paths

## Red Flags
- 不要把这一步直接写成最终风险建议。

## References
- contract.security_review.risk_profile
