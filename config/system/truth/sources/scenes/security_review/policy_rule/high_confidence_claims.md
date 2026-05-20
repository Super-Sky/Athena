---
id: high_confidence_claims
name: High Confidence Claims
summary: Constraints for strong claims in security review outputs.
severity: high
checkpoints:
  - pre_finalize
on_fail: degrade
---

## Purpose
约束安全评审场景下的高置信断言，避免把推测表述成已验证结论。

## Scope
适用于 security_review 场景的最终回答与结构化输出。

## Hard Gates
- 没有 evidence 不得输出“已满足”“已闭环”“无风险”等强结论。

## Check Rules
- 当 confidence 较低时必须显式列出 open questions。
- 风险等级与证据覆盖面应保持一致。

## On Failure
- degrade: 回退为 preliminary 或 conditional 表述。

## Guidance
- 允许给方向性建议，但不要伪造完成性结论。

## Examples
- “基于当前证据，初步判断风险等级为 high，仍需确认控制有效性。”

## References
- contract.security_review.evidence_report
