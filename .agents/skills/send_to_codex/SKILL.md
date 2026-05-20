---
name: send_to_codex
description: Use when you want Claude to produce the next standardized Codex handoff for the current repository task.
---

# Send to Codex

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- This shortcut is for producing the next reusable `send_to_codex` handoff only.
- Reuse the repository's existing reminder fields and handoff structure; do not introduce a second protocol.
- Include worktree, branch, relevant doc paths, exact ask, and constraints when available.
- The `inputs` block should carry the concrete execution anchors the next Codex step needs.
- Stop after producing the handoff.

Keep this skill lightweight. Shared rules live in `docs/*`.
