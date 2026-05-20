---
id: evidence_reporting
name: Evidence Reporting
summary: Organize evidence into readable mappings, coverage, and gaps.
description: Use when security review outputs must explain what evidence exists, what it proves, and what remains unproven.
scene: security_review
depends_on:
  - contract.security_review.evidence_report
  - policy_rule.core.evidence_sufficiency
allowed_tools: []
---

## When to Use
- 需要组织 evidence 与 finding 的映射
- 需要解释 coverage 与仍未覆盖部分

## Input
- evidence
- findings
- current conclusions

## Process
- 映射 evidence 到 finding。
- 标出 gaps 与 non-claimable items。

## Output
- contract.security_review.evidence_report

## Red Flags
- 不要把 attestation 当作充分证据。
- 不要隐藏 coverage gaps。

## References
- contract.security_review.evidence_report
