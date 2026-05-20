# Public Mirror Notes

This repository snapshot was prepared for public publishing.

Sanitization actions applied:

- Removed local env files and runtime logs (`.env.local`, `.env.local.example`, `develop.log`).
- Removed legacy stage docs (`docs/v0.1.0`, `docs/v0.1.1`, `docs/v0.2.0`, `docs/v1.0.0`).
- Removed internal Jira helper scripts and internal plugin stubs.
- Removed staged version planning docs (`docs/v2.0.0`, `docs/v2.1.0`) and large feature-history docs (`docs/features`) for a lean public mirror.
- Kept collaboration assets (`.agents`, `.claude`, `.gitlab`, `.gitlab-ci.yml`, `AGENTS.md`) for shared contributor workflow, and replaced private-domain examples with public-safe placeholders.
- Redacted `config/config.yml` database password to `change-me-public`.

If you need full internal history, use the internal repository, not this public mirror.
