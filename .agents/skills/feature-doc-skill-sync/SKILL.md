---
name: feature-doc-skill-sync
description: Use when a feature is being finished, refactored, or stabilized and the repository needs a consistent step for module-boundary clarification, feature-doc consolidation, and skill creation/update.
---

# Feature Doc Skill Sync

- Primary source: `docs/TASK_DELIVERY_GUIDE.md`
- Workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Docs index: `docs/README.md`
- Use this as the stable closeout step for module-boundary clarification, final `feature-<topic>.md` consolidation, and feature-skill creation or update.
- The canonical current feature doc should live in `docs/features/`; version directories are only for snapshots when needed.
- The final feature doc should cover background, rules, implementation approach, module boundaries, config/dependencies, contracts or key interaction chain, risks, and verification.
- The closeout must end with one explicit conclusion: feature skill created, updated, or intentionally deferred with a recorded reason.
- The skill does not replace the feature doc; it standardizes the maintenance entry and should point back to the doc.
- Use this before final delivery so module boundaries and maintenance ownership are closed with evidence.

Keep this skill lightweight. Shared rules live in `docs/*`.
