---
name: issue-intake
description: Use when one task starts from one GitLab issue URL or one `group/project#iid` issue reference and the agent must fetch the issue content before planning or implementation.
---

# Issue Intake

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- Feature doc: `docs/features/feature-issue读取与分析技能.md`
- Main skill implementation: `.agents/skills/issue-intake/SKILL.md`

Use the main skill for the actual fetch-and-analyze workflow. Shared rules live in `docs/*`.
