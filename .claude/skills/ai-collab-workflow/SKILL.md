---
name: ai-collab-workflow
description: Use when the task needs the repository's standard Claude-to-Codex collaboration flow, centered on Claude planning, one full Codex execution phase, Claude review, and one full Codex finish phase.
---

# AI Collaboration Workflow

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- Default chain: `Claude plan -> Codex execute -> Claude review -> Codex finish`
- Treat `planned`, `implementing`, `reviewing`, `ready_for_delivery`, and `completed` as evidence-backed states, not just labels.
- During `Codex finish`, run `doc-index-sync` when file layout or module boundaries changed, so module READMEs, full file indexes, and file-header guidance stay aligned.
- During `Codex finish`, run `feature-doc-skill-sync` as the stable closeout step for module-boundary clarification, final feature-doc consolidation, and feature-skill creation or update.
- Each completed feature should converge into a maintainable `feature-<topic>.md`, then be evaluated for extraction into a dedicated maintenance skill.
- If protected backbone surfaces were touched and merge into `master` is planned, also run `master-merge-gate`.
- Version master plan lives in `docs/vX.Y.Z/plan/`, and topic process docs live in `docs/vX.Y.Z/plan/<topic>/`.
- Versioned work must maintain a concrete feat checklist in the version master plan, and final delivery must review and reconcile that checklist before `completed`.
- During major version planning and feature refinement, classify each capability as `Core`, `Validation`, `Enhancement`, or `Application / Business Truth` before accepting it into the plan; do not let application enhancement or business truth inflate core runtime scope.
- Claude should stop after producing and saving the current plan artifact; the user then invokes one full Codex execution phase.
- Do not use `~/.claude/plans/` as the formal handoff surface for this repository; treat it only as Claude internal state.
- Reusable stage-doc templates can live in `docs/vX.Y.Z/plan/_template/`.
- Stage docs: Claude uses `-claude`, Codex uses `-codex`, and each phase should read the previous phase doc before continuing.
- After verification, Codex should update the master plan status, run `doc-index-sync`, run `feature-doc-skill-sync`, generate the canonical current feature doc in `docs/features/feature-<topic>.md`, optionally record a version snapshot in `docs/vX.Y.Z/features/`, decide whether the feature needs a dedicated skill, merge the current task result back into the current main branch, delete process docs, and clean unused task worktrees or temporary directories.
- State machine: `planned -> implementing -> reviewing -> ready_for_delivery -> completed`, with `blocked` as an exception state.
- Block forward progress when required planning, version feat checklist creation/reconciliation, correction, worktree isolation, phase-doc handoff, review artifact, delivery gate, `master-merge-gate` for protected-surface changes into `master`, master-plan update, merge-back-into-current-main-branch, process-doc cleanup, or worktree/temp-dir cleanup is missing.
- Remind the user with `current_state`, `missing_step`, `why_now`, and one highest-priority `next_action`.
- When ownership changes, proactively emit a directly reusable handoff block.
- Symmetric rule:
  - next owner `Codex` => current owner `Claude` emits `send_to_codex`
  - next owner `Claude` => current owner `Codex` emits `send_to_claude`
- The handoff block should be directly reusable, prefer the default short format, and continue to use `context / goal / inputs / required_output / constraints`.
- The `inputs` block should include the concrete execution anchors the next owner needs: topic, stage, worktree path, branch, stage-doc paths, exact ask, and explicit constraints.

Keep this skill lightweight. Shared rules live in `docs/*`.
