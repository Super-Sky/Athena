---
name: issue-intake
description: Use when a task starts from one GitLab issue URL or one `group/project#iid` issue reference and the agent must fetch the issue content before planning or implementation.
---

# Issue Intake

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- Feature doc: `docs/features/feature-issue读取与分析技能.md`
- Bundled fetch script: `scripts/fetch_gitlab_issue.py`
- Bundled tests: `scripts/test_fetch_gitlab_issue.py`

Use this skill before plan or implementation when the user gives:

- one GitLab issue URL
- one `group/project#iid` style issue reference
- one request like “read the issue first, then analyze and plan”

Do not rely on manually opening the issue page in a browser tab and paraphrasing from memory. Always prefer the bundled fetch script first. Treat the issue number as an entry point, not as the requirement truth itself.

## Expected workflow

1. Run the bundled fetch script with one issue reference.
2. Prefer the script's `glab` path when local GitLab auth already exists.
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
python3 .agents/skills/issue-intake/scripts/fetch_gitlab_issue.py \
  "https://git.example.com/example-org/athena/-/issues/7"
```

Project reference:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_gitlab_issue.py \
  "example-org/athena#7"
```

Include notes:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_gitlab_issue.py \
  "example-org/athena#7" \
  --include-notes
```

Markdown output:

```bash
python3 .agents/skills/issue-intake/scripts/fetch_gitlab_issue.py \
  "example-org/athena#7" \
  --format markdown
```

## Auth order

The script resolves issue content in this order:

1. `glab`
2. `GITLAB_TOKEN`
3. `GITLAB_PRIVATE_TOKEN`
4. `GITLAB_BEARER_TOKEN`
5. `GITLAB_SESSION_COOKIE`

Notes:

- Preferred path is `glab` when the local machine already has GitLab auth.
- API token fallback prefers `GITLAB_TOKEN` or `GITLAB_PRIVATE_TOKEN`.
- `GITLAB_SESSION_COOKIE` is one best-effort fallback for intranet debugging; it should not be treated as the long-term default.
- If a `group/project#iid` reference is used without a URL, set `GITLAB_BASE_URL`.
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
