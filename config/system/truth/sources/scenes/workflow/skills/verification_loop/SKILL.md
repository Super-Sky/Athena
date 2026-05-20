---
id: verification_loop
name: Verification Loop
summary: Judge whether execution can close, requires supplement, or should reopen.
description: Use when execution results and evidence arrive and the system must decide close, supplement, or reopen.
scene: workflow
depends_on:
  - contract.workflow.verification_result
  - policy_rule.core.evidence_sufficiency
allowed_tools: []
---

## When to Use
- 任务声称完成，需要验证闭环。

## Input
- execution results
- evidence objects
- acceptance criteria

## Process
- 按 action 检查 evidence 与 acceptance criteria。
- 计算 overall verdict。

## Output
- contract.workflow.verification_result

## Red Flags
- attestation alone 不能视为 sufficient。
- 证据缺口不能“差不多就通过”。

## References
- contract.workflow.verification_result
