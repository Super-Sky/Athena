---
name: eino-guide
description: Use when working with Eino framework concepts, repository navigation, high-level architecture, or deciding whether to use Eino components, compose, or agent APIs.
---

# Eino Guide

Use this skill when Eino framework context is needed before implementation or review.

## Read First

- `_eino/llms.txt`
- `_eino/eino-ext/skills/eino-guide/SKILL.md`
- `_eino/eino-ext/skills/eino-guide/reference/quick-start.md`
- `_eino/eino-ext/skills/eino-guide/reference/runnable.md`
- `_eino/eino-ext/skills/eino-guide/reference/schema.md`

## Athena Rule

- Eino is Athena's runtime execution framework.
- Athena owns runtime truth, public contracts, persistence schema, and Control Plane read models.
- Eino owns execution orchestration where its Graph, Workflow, Chain, Agent, callback, checkpoint, middleware, or component APIs can express the behavior.
- Do not expose Eino private/internal types directly in Athena public API or persistence schema.
