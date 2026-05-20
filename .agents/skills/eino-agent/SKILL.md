---
name: eino-agent
description: Use when building or reviewing Eino Agent Development Kit flows: ChatModelAgent, Runner, events, middleware, agent-as-tool, filesystem, human-in-the-loop, interrupt/resume, or DeepAgent patterns.
---

# Eino Agent

Use this skill for Eino ADK agent and runner behavior.

## Read First

- `_eino/eino-ext/skills/eino-agent/SKILL.md`
- `_eino/eino-ext/skills/eino-agent/reference/chat-model-agent.md`
- `_eino/eino-ext/skills/eino-agent/reference/runner-and-events.md`
- `_eino/eino-ext/skills/eino-agent/reference/middleware.md`
- `_eino/eino-ext/skills/eino-agent/reference/human-in-the-loop.md`
- `_eino/eino-ext/skills/eino-agent/reference/agent-as-tool.md`
- `_eino/eino-ext/skills/eino-agent/reference/filesystem.md`
- `_eino/eino-ext/skills/eino-agent/reference/deep-agents.md`
- `_eino/eino/adk/`

## Athena Rule

- Use Eino Runner and events for agent execution visibility where practical.
- Human-in-the-loop and interrupt/resume should align with Athena runtime lifecycle, waiting, and persistence contracts.
- Agent event streams must be redacted before persistence; never persist raw credentials or sensitive tool payloads.
