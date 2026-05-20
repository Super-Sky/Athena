---
id: knowledge
name: 知识治理
summary: 面向知识候选、知识提炼与知识复用判断的场景。
---

## Purpose
从已完成案例中提炼可治理知识对象，或判断历史知识是否可复用到当前上下文。

## When It Applies
- 需要做知识候选、知识复用、画像补充、候选更新判断。
- 当前任务目标不是直接回答用户，而是形成或使用可治理知识对象。

## Primary Outcome
输出 knowledge object、knowledge candidate 或 reuse decision。

## Core Objects
- knowledge object
- candidate update
- reuse decision

## Decision Dimensions
- 是否可复用
- 是否需要脱敏
- 是否可跨租户共享
- 是否满足适用条件

## Default Assets
- workflow.knowledge.main
- contract.knowledge.knowledge_object
- contract.knowledge.knowledge_candidate
- contract.knowledge.reuse_decision
- skill.knowledge.reuse_judgment
- skill.knowledge.knowledge_extraction
- policy_rule.core.knowledge_governance

## Out Of Scope
- 通用闲聊
- 直接安全评审

## Examples
- “这个案例里有哪些可复用的知识？”
- “历史知识能不能直接套用到当前情境？”
