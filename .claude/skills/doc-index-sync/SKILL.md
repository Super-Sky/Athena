---
name: doc-index-sync
description: Use when a task changed module boundaries, file layout, or directory contents and the repository needs module README, full file index, and file-header guidance to stay accurate for AI readers.
---

# Doc Index Sync

- Primary source: `docs/TASK_DELIVERY_GUIDE.md`
- Workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Docs index: `docs/README.md`
- Template sources:
  - `docs/templates/模块README模板.md`
  - `docs/templates/文件头注释模板.md`
  - `docs/templates/文件级说明模板.md`
- Use this when module boundaries or file layout changed and the repository must keep directory README navigation, full file indexes, and file-header guidance aligned.
- Prefer one directory README plus file-header comments over one extra markdown file per source file.
- Only create file-level detail docs for especially complex protocol, state-machine, or orchestration files.
- Follow repository language rules:
  - system filenames stay English
  - code filenames stay English
  - non-system docs default to Chinese body text
  - explanatory code comments default to bilingual two-line format
- When layout or boundaries changed, run this before feature-doc-skill-sync and repo-task-delivery.

Keep this skill lightweight. Shared rules live in `docs/*`.
