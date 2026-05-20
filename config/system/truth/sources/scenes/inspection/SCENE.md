---
id: inspection
name: 巡检解释
summary: 面向巡检、体检和扫描结果解释的场景。
---

## Purpose
对巡检、扫描或体检结果做风险归纳、优先级梳理和后续动作建议。

## When It Applies
- 用户提供 inspection、scan、体检、巡检结果。
- 问题重点是解释结果，而不是完整安全咨询。

## Primary Outcome
输出发现摘要、风险排序和后续动作建议。

## Core Objects
- findings
- evidence
- 风险优先级

## Decision Dimensions
- 发现严重度
- 证据覆盖
- 后续动作优先级

## Default Assets
- workflow.inspection.main
- contract.inspection.inspection_findings
- contract.inspection.inspection_summary
- skill.security_review.risk_profiling
- skill.security_review.evidence_reporting

## Out Of Scope
- 完整 CSO 风格评审

## Examples
- “帮我解释这份巡检结果”
