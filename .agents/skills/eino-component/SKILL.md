---
name: eino-component
description: Use when wiring or reviewing Eino components such as ChatModel, Tool, Retriever, Indexer, Embedding, Prompt, callback handlers, or provider implementations from eino-ext.
---

# Eino Component

Use this skill for Eino component selection, configuration, provider wiring, and component-level tests.

## Read First

- `_eino/eino-ext/skills/eino-component/SKILL.md`
- `_eino/eino-ext/skills/eino-component/reference/prompt.md`
- `_eino/eino/components/`
- `_eino/eino-ext/components/`

## Athena Rule

- Prefer Eino component interfaces and `eino-ext` implementations before adding project-local provider abstractions.
- Keep provider secrets out of persisted runtime trace payloads.
- Project component outputs into Athena runtime contracts instead of storing Eino-private objects.
