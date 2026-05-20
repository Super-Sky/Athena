---
name: master-merge-gate
description: Use before merging changes into master when the task touches Athena backbone or shared contract surfaces, to verify the changes remain general, extensible, and not scene-coupled.
---

# Master Merge Gate

Use this skill before merging into `master` when the current task touches the repository's protected backbone surfaces.

## Read First

- `docs/REPO_WORKFLOW_GUIDE.md`
- `docs/TASK_DELIVERY_GUIDE.md`
- `docs/README.md`

## Focus

- Check whether the current change touched the protected backbone surfaces.
- If yes, verify the change is entering the backbone because it adds reusable capability, not because it was the fastest way to support one scenario.
- Require both AI review and explicit human confirmation before merge into `master`.

## Protected Backbone Surfaces

The authoritative list lives in `docs/REPO_WORKFLOW_GUIDE.md`.

Typical examples include:

- `main.go`
- `internal/entry/*`
- `internal/app/*`
- `internal/runtime/*`
- `internal/runtime/task/*`
- `internal/policy/*`
- `internal/session/*`
- `internal/model/*`
- `internal/skills/*`
- `internal/tools/*`
- shared contract docs such as `docs/project.md`, `docs/architecture.md`, `docs/implementation.md`, `docs/api.md`

## Gate Questions

Before merge into `master`, explicitly answer:

1. Why can this not stay in adapters, mappers, Workspace, Plug, or versioned feature code?
2. What reusable capability is being added to the backbone?
3. How does this change keep the backbone more extensible rather than more scene-specific?
4. If the current business direction is later discarded, would this backbone change still make sense?
5. What tests, docs, and regression checks support that claim?

## Required Outcome

Do not pass the gate unless all of the following are true:

- the protected-surface touch is explicit, not accidental
- the reusable capability is clearly named
- the scene-specific logic remains outside the backbone where possible
- AI review has stated the change remains general and extensible
- human review has explicitly confirmed the change may enter `master`

Keep this skill lightweight. Shared rules live in `docs/*`.
