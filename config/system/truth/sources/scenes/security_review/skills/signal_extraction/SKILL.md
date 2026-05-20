---
id: signal_extraction
name: Signal Extraction
summary: Extract business-security signals from the current request and context.
description: Use when security_review needs to extract structured security-relevant signals before building a risk profile.
scene: security_review
depends_on:
  - policy_rule.core.safety_constitution
allowed_tools: []
---

## When to Use
- 安全评审开始阶段
- 需要从输入中提取业务、安全、约束和对象信号

## Input
- 当前请求文本
- 已知上下文

## Process
- 提取 intent、risk_theme、object_mention、constraint。
- 仅抽取能从现有文本直接推断的信号。

## Output
- 可供后续风险建模与风险画像消费的信号集合

## Red Flags
- 不要过度推断。
- 不要把 PII 或敏感值写进 signal。

## References
- policy_rule.core.safety_constitution
