---
id: user_overview
name: User Overview
summary: Summarize the effective user, memory, and persona context already loaded for the current request.
description: Use when the current scene needs a concise overview of what is already known from context without inventing subject-specific data.
scene: default
depends_on:
  - policy_rule.core.safety_constitution
allowed_tools: []
---

## When to Use
- Need a concise summary of the current effective user, memory, or persona context.
- Need to explain what is already known before asking for more information.

## Input
- effective_persona
- effective_user_profile
- effective_memory_view

## Process
- Read only the currently effective context assets.
- Summarize known facts, known preferences, and obvious gaps.
- Do not fabricate missing profile or history information.

## Output
- Concise overview of what Athena currently knows.
- Explicit statement of missing context when continuity matters.

## Red Flags
- Do not invent user identity, business context, or historical decisions.
- Do not treat missing memory as real memory.

## References
- Use only currently effective context assets.
