---
name: eino-compose
description: Use when building or reviewing Eino Graph, Workflow, Chain, Runnable execution, streaming, callbacks, checkpoint/state, call options, or orchestration design.
---

# Eino Compose

Use this skill when runtime execution should be expressed with Eino compose APIs.

## Read First

- `_eino/eino-ext/skills/eino-compose/SKILL.md`
- `_eino/eino-ext/skills/eino-compose/reference/graph.md`
- `_eino/eino-ext/skills/eino-compose/reference/workflow.md`
- `_eino/eino-ext/skills/eino-compose/reference/chain.md`
- `_eino/eino-ext/skills/eino-compose/reference/callback.md`
- `_eino/eino-ext/skills/eino-compose/reference/checkpoint-and-state.md`
- `_eino/eino-ext/skills/eino-compose/reference/stream.md`
- `_eino/eino-ext/skills/eino-compose/reference/call-option.md`
- `_eino/eino/compose/`

## Athena Rule

- New runtime orchestration defaults to Eino Graph / Workflow / Chain unless a documented platform boundary requires a project-owned abstraction.
- Graph callbacks and node outputs should project into Phase 1 runtime persistence objects: `TaskStep`, `RuntimeTrace`, generic `Usage`, `TaskRunLifecycleEvent`, and `ProjectionCandidate`.
- Deterministic validation must not bypass the Eino graph foundation once that phase is implemented, except for lower-level store or contract tests.
