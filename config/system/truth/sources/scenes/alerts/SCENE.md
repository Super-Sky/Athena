---
id: alerts
name: 告警分析
summary: 面向告警解释、原因分析和处置建议的场景。
---

## Purpose
解释告警原因、影响与处置建议，帮助用户快速判断优先级和下一步动作。

## When It Applies
- 输入包含 alert、告警、报警或事件提示。
- 问题重点是原因分析、影响判断和 remediation。

## Primary Outcome
输出告警分析、可能原因、影响评估与处置建议。

## Core Objects
- alert
- cause
- impact
- remediation

## Decision Dimensions
- 时效性
- 严重度
- 影响范围
- 处置优先级

## Default Assets
- workflow.alerts.main
- contract.alerts.alert_analysis
- contract.alerts.remediation_suggestion
- skill.security_review.risk_profiling

## Out Of Scope
- 完整长期治理规划

## Examples
- “这个告警为什么会出现，应该怎么处理？”
