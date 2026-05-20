---
id: risk_profiling
name: Risk Profiling
summary: Build a structured risk profile from current business and security signals.
description: Use when the scene requires a structured risk profile grounded in actual context rather than generic advice.
scene: security_review
depends_on:
  - contract.security_review.risk_profile
  - policy_rule.core.evidence_sufficiency
allowed_tools: []
---

## When to Use
- 需要输出结构化风险画像
- 用户要求风险分析、控制缺口判断或安全现状理解

## Input
- 当前请求
- signal extraction 输出
- 可选 threat modeling 输出

## Process
- 从业务目标、资产、威胁、控制和缺口构建 risk profile。
- 明确 confidence 和 open questions。

## Output
- contract.security_review.risk_profile

## Red Flags
- 不要用模板化行业建议替代真实风险画像。
- 无法确认的控制状态应标为待确认，不要直接判定缺失。

## References
- contract.security_review.risk_profile
