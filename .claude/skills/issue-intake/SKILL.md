---
name: issue-intake
description: Use when one task starts from one GitHub issue URL or one `owner/repo#number` issue reference and the agent must fetch the issue content before planning or implementation.
---

# Issue Intake

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- Main skill implementation: `.agents/skills/issue-intake/SKILL.md`

Use the main skill for the actual fetch-and-analyze workflow. Shared rules live in `docs/*`.
