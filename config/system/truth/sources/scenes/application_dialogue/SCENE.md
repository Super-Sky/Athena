---
id: application_dialogue
name: 应用内专家对话
summary: 依赖应用上下文的轻量专家对话场景。
---

## Purpose
在已有应用上下文、实例上下文或平台上下文时，提供轻量专家解释与建议。

## When It Applies
- 用户已处于应用内对话场景。
- 问题依赖当前应用、实例或平台上下文。
- 不需要进入重型安全评审或 workflow 规划。

## Primary Outcome
给出基于上下文的专家解释、判断和下一步建议。

## Core Objects
- app context
- platform context
- 当前问题

## Decision Dimensions
- 是否已有足够应用上下文
- 是否需要转入更强 scene
- 是否需要补充身份或平台 detail

## Default Assets
- workflow.application_dialogue.main
- contract.application_dialogue.app_dialogue_answer

## Out Of Scope
- 深度安全评审
- 复杂治理闭环
- 知识候选提炼

## Examples
- “结合当前应用上下文，帮我解释这个配置意味着什么”
