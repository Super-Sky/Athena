---
id: security_review
name: 安全评审
summary: 面向结构化安全评审、风险分析与治理建议的核心场景。
---

## Purpose
提供结构化安全评审、风险画像、治理选项、证据说明和行动建议。

## When It Applies
- 安全审计、威胁建模、供应链、CI/CD、密钥、AI/LLM 安全相关请求。
- 用户要求 CSO 风格评审、STRIDE 或 OWASP 视角分析。
- 用户需要系统化地理解风险、控制和治理选项。

## Primary Outcome
输出风险画像、关键发现、治理选项、证据化说明与下一步动作。

## Core Objects
- 业务目标
- 核心资产
- 威胁
- 控制
- 缺口
- evidence

## Decision Dimensions
- 影响
- 可利用性
- 暴露面
- 控制成熟度
- 修复成本
- 业务约束

## Default Assets
- workflow.security_review.main
- contract.security_review.risk_profile
- contract.security_review.decision_options
- contract.security_review.evidence_report
- contract.security_review.security_review_answer
- policy_rule.core.safety_constitution
- policy_rule.core.evidence_sufficiency
- skill.security_review.cso_review
- skill.security_review.signal_extraction
- skill.security_review.threat_modeling
- skill.security_review.risk_profiling
- skill.security_review.decision_planning
- skill.security_review.narrative_adaptation
- skill.security_review.evidence_reporting

## Out Of Scope
- 不带任何上下文的纯闲聊
- 仅需轻量 app 解释的问题

## Examples
- “请做一份 CSO 风格的安全评审”
- “帮我按 STRIDE 看一下这套架构”
- “结合当前上下文给出风险和治理建议”
