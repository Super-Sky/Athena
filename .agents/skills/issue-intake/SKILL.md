---
name: issue-intake
description: Use when a task starts from one GitHub issue URL or one `owner/repo#number` issue reference and the agent must fetch the issue content before planning or implementation.
---

# Issue Intake

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- Bundled fetch script: `scripts/fetch_github_issue.py`
- Bundled tests: `scripts/test_fetch_github_issue.py`

Use this skill before plan or implementation when the user gives:

- one GitHub issue URL
- one `owner/repo#number` style issue reference
- one request like “read the issue first, then analyze and plan”

Do not rely on manually opening the issue page in a browser tab and paraphrasing from memory. Always prefer the bundled fetch script first. Treat the issue number as an entry point, not as the requirement truth itself.

## Expected workflow

1. Run the bundled fetch script with one issue reference.
2. Prefer the script's `gh` path when local GitHub auth already exists.
3. If API auth is still missing, report the exact missing auth path instead of pretending the issue was read.
4. Once content is available, produce one structured analysis that covers:
   - requested change
   - constraints / non-goals
   - platform or upstream dependencies
   - likely touched modules
   - open questions
   - recommended implementation plan

## Script usage

Basic:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_github_issue.py \
  "https://github.com/Super-Sky/Athena/issues/7"
```

Project reference:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_github_issue.py \
  "Super-Sky/Athena#7"
```

Include notes:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_github_issue.py \
  "Super-Sky/Athena#7" \
  --include-comments
```

Markdown output:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_github_issue.py \
  "Super-Sky/Athena#7" \
  --format markdown
```

## Auth order

The script resolves issue content in this order:

1. `gh`
2. `GITHUB_TOKEN`
3. `GH_TOKEN`
4. `GITHUB_PAT`
5. `GITHUB_BEARER_TOKEN`

Notes:

- Preferred path is `gh` when the local machine already has GitHub auth.
- API token fallback prefers `GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_PAT`.
- If an `owner/repo#number` reference is used without a URL, set `GITHUB_BASE_URL`.
- If none of the above is present and the issue is not publicly readable, the script should fail clearly and say auth is required.

## Analysis output standard

After fetching the issue, summarize it into one compact structure with these headings:

- `Issue`
- `Scope`
- `Constraints`
- `Dependencies`
- `Touched Areas`
- `Open Questions`
- `Plan`

Keep the analysis grounded in the fetched issue content. Do not invent hidden requirements.
