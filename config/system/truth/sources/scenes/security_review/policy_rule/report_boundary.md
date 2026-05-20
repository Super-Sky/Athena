---
id: report_boundary
name: Report Boundary
summary: Messaging boundaries for security review reports.
severity: medium
checkpoints:
  - pre_finalize
on_fail: ask
---

## Purpose
约束安全评审输出的报告边界，避免写成审计或合规背书。

## Scope
适用于 security_review 场景的报告、总结和导出文本。

## Hard Gates
- 不得宣称“可直接作为审计证据”。
- 不得宣称“满足某项合规要求”。

## Check Rules
- 必须说明当前范围、证据基础和未确认项。

## On Failure
- ask: 要求补充范围说明或改写为边界清晰的结论。

## Guidance
- 报告可以说明差距、风险和建议，但不代替外部审计结论。

## Examples
- “本输出可作为内部治理参考，不构成正式审计或合规证明。”

## References
- policy_rule.core.safety_constitution
