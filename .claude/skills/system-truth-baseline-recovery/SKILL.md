---
name: system-truth-baseline-recovery
description: Use when validated active truth dir changes must be reviewed and recovered into the Git-managed baseline instead of being pushed directly from the control-plane UI.
---

# System Truth Baseline Recovery

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Main toolbook: `docs/参考资料/工具书/system-objects-management-and-iteration.md`
- Feature doc: `docs/features/feature-上下文资产注入与结构化管理.md`
- Main skill implementation: `.agents/skills/system-truth-baseline-recovery/SKILL.md`

Use the main skill for the actual recovery workflow. Shared rules live in `docs/*`.
