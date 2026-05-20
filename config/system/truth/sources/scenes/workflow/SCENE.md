---
id: workflow
name: 工作流规划
summary: 面向工作流计划、治理契约、验证闭环和自动化草案的场景。
---

## Purpose
把建议、任务或需求转成可执行的 workflow plan、governance contract、verification result 或 automation payload。

## When It Applies
- 用户要求 workflow、automation、计划、草案、步骤流或验证闭环。
- 当前任务属于 workflow_step_request。

## Primary Outcome
输出 workflow_plan、governance_contract、verification_result 或 automation_create_payload。

## Core Objects
- goal
- stages
- contract
- verification

## Decision Dimensions
- 是否需要人工确认
- 哪些步骤可自动化
- 哪些完成条件必须有 evidence

## Default Assets
- workflow.workflow.main
- contract.workflow.workflow_plan
- contract.workflow.governance_contract
- contract.workflow.verification_result
- contract.workflow.automation_create_payload
- skill.workflow.contract_generation
- skill.workflow.verification_loop

## Out Of Scope
- 长篇安全分析本身

## Examples
- “帮我把这个方案整理成工作流”
- “根据当前结果生成下一步治理契约”
