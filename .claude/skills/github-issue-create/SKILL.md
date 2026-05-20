---
name: github-issue-create
description: Use when the user wants to create, draft, preview, or post a GitHub issue for this repository workflow. Trigger for requests like “新建 issue”, “帮我提 issue”, “create a GitHub issue”, or when work requires opening an issue before planning because it changes feature boundaries, cross-service contracts, schema, local runtime boundaries, version freeze scope, or module-level defects.
---

# GitHub Issue Create

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Main skill implementation: `.agents/skills/github-issue-create/SKILL.md`

Use the main skill for the actual issue draft/create workflow. Shared rules live in `docs/*`.
