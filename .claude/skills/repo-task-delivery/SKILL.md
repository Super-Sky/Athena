---
name: repo-task-delivery
description: Use when finishing a task in this repository and you need to decide what documentation, workflow notes, or mirrored skill updates are required before closing the task.
---

# Repo Task Delivery

- Primary source: `docs/TASK_DELIVERY_GUIDE.md`
- Workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Docs index: `docs/README.md`
- This skill is the required pre-commit and pre-push gate for this repository.
- Commit gate: bilingual comments must use two lines, required tests and benchmarks must pass, API/SSE/state-machine changes need system coverage, memory risks must be checked when queue/session/memory or long-wait logic changes, `doc-index-sync` should already have run when module structure or file layout changed, `feature-doc-skill-sync` should already have run for completed feature work, affected module directories need `README.md` with full file indexes, newly added or heavily changed core source files need file-header guidance, every completed feature needs a detailed final `feature-<topic>.md`, feature-level maintenance skills should be added or explicitly deferred, docs plus `develop.log` must be updated, stage `-claude` / `-codex` docs must be consolidated and cleaned if this workflow was used, version `master-plan.md` must reflect the final state and its concrete feat checklist must be reviewed/reconciled, completed items checked with evidence, deferred or cancelled items marked with reasons, obsolete task worktrees must be cleaned or explicitly retained, the current repository issue must be confirmed before commit/push, and for staged or multi-step tasks the gate must explicitly decide whether issue progress comments are required at push, PR, and merge milestones.
- If the task touched protected backbone surfaces and is preparing to merge into `master`, `master-merge-gate` must also be executed.
- The gate is fail-closed: missing evidence means do not mark the task ready for commit or push.

Keep this skill lightweight. Shared rules live in `docs/*`.
