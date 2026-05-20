---
name: github-issue-close
description: Use when the user explicitly asks to close a GitHub issue, mark an issue done, or perform merge closeout that may close the canonical issue. This skill is intentionally strict: it should read the issue, reconcile original requirements against delivery evidence, preview the final note first, and only set the GitHub issue state to closed when the user explicitly requested remote closure and completion is confirmed.
---

# GitHub Issue Close

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Main skill implementation: `.agents/skills/github-issue-close/SKILL.md`

Use the main skill for the actual issue closeout workflow. Shared rules live in `docs/*`.
