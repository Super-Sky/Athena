---
name: code-style
description: Use when changing Go code in this repository and you need the project-specific layering, API, SSE, and runtime implementation rules. Read the shared code style guide instead of inferring local conventions.
---

# Code Style

Use this skill when you are about to implement or review code in this repository.

## Read First

- `docs/CODE_STYLE_GUIDE.md`
- `docs/api.md`
- `docs/architecture.md`
- `docs/implementation.md`

## Focus

- Keep `main.go` thin.
- Keep runtime assembly in `internal/app`.
- Keep HTTP and SSE mapping in `internal/server`.
- Keep model compatibility logic in `internal/model`.
- Update shared docs when API, SSE, or runtime behavior changes.
