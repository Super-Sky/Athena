---
id: default
name: 默认场景
summary: 未命中其他强场景时的保底场景。
---

## Purpose
当请求未明确命中任何专门 scene 时，提供稳妥回答、最小澄清或路由建议。

## When It Applies
- 没有明显命中安全评审、巡检、告警、workflow、knowledge 或应用内对话。
- 当前信息不足，先澄清比直接重型分析更合理。
- 用户只是泛问、试探或需要上下文总览。

## Primary Outcome
给出直接回答、提出最小澄清问题，或提示转入更具体的 scene。

## Core Objects
- 当前问题
- 已知上下文
- 缺失信息

## Decision Dimensions
- 是否需要澄清
- 是否已有足够上下文直接回答
- 是否应转入更具体场景

## Default Assets
- workflow.default.main
- contract.default.general_answer
- contract.default.clarification_request
- skill.default.user_overview

## Out Of Scope
- 深度安全评审
- 复杂治理执行闭环
- 知识候选提炼

## Examples
- 用户只问“你现在能帮我做什么？”
- 用户给出的信息太少，无法直接进入安全分析
