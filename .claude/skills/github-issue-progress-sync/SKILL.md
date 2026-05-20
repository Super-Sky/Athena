---
name: github-issue-progress-sync
description: Use when a branch is pushed, a PR is created, or a merge is completed for a GitHub-backed task and the agent must check and usually sync progress back to the canonical issue.
---

# GitHub Issue Progress Sync

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Main skill implementation: `.agents/skills/github-issue-progress-sync/SKILL.md`

Use the main skill for the actual progress-comment workflow. Shared rules live in `docs/*`.
