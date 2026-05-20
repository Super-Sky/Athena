---
name: gitlab-issue-close
description: Use when the user explicitly asks to close a GitLab issue, mark an issue done, or perform merge closeout that may close the canonical issue. This skill is intentionally strict: it should read the issue, reconcile original requirements against delivery evidence, preview the final note first, and only set the GitLab issue state to closed when the user explicitly requested remote closure and completion is confirmed.
---

# GitLab Issue Close

Use this skill for the final issue closeout step. Closing an issue changes remote project state, so this skill is stricter than `gitlab-issue-progress-sync`.

This skill complements:

- `gitlab-issue-create`: create or draft a new issue.
- `issue-intake`: read and analyze an existing issue before planning or implementation.
- `gitlab-issue-progress-sync`: write progress notes for push / MR / merge milestones.

## Read First

- `docs/REPO_WORKFLOW_GUIDE.md`
- `docs/TASK_DELIVERY_GUIDE.md`
- `scripts/close_gitlab_issue.py`

## When to use

Use this skill when:

- the user says to close a GitLab issue;
- the user asks for final issue closeout;
- an MR or merge has completed and the canonical issue may now be done;
- the workflow needs to decide whether `Closes group/project#iid` is appropriate.

Do not use this skill for routine progress updates. Use `gitlab-issue-progress-sync` for `push`, `mr`, or `merge` progress comments when the issue remains open or follow-up work remains.

## Required intake and reconciliation

Before closing, first read and interpret the canonical issue. Use `issue-intake` with notes when possible so the close decision is grounded in the issue body and follow-up discussion, not only in the current conversation.

Extract at least:

- original requested scope;
- completion criteria;
- explicit non-goals;
- dependencies or cross-repository work;
- reviewer / validation / follow-up notes.

Then reconcile the issue requirements against delivery evidence. A closeout note should map original issue requirements to concrete evidence such as commits, MR, tests, docs, or accepted deferrals.

Before closing, confirm the issue is truly complete in the repository scope:

- the canonical issue reference is known;
- issue body and relevant notes were read or their absence is explicit;
- original requirements and completion criteria have been extracted;
- each requirement is mapped to evidence, accepted deferral, or cancellation reason;
- the current repository completed its promised scope;
- no required follow-up work remains in this repository;
- no required cross-repository, validation, or reviewer action remains;
- the relevant MR is merged or the work is explicitly cancelled / no longer continuing;
- tests or agreed verification are complete or the missing verification is explicitly accepted;
- docs / feature docs / version plan / delivery notes are updated when applicable;
- `Refs` is not being used to represent partial completion as closure;
- the user explicitly asked to close the issue or approved the close action.

If any item is unknown, preview the close-readiness report and ask for confirmation instead of closing.

## Workflow

1. Run or follow `issue-intake` for the canonical issue; include notes when they may contain scope changes or reviewer decisions.
2. Extract original scope, completion criteria, non-goals, dependencies, and follow-up notes.
3. Reconcile each original requirement against delivery evidence or an explicit deferral / cancellation reason.
4. Run the close-readiness checklist.
5. Build a final closeout note that states:
   - branch / commit / MR if known;
   - issue original requirements / completion criteria;
   - requirement-to-evidence reconciliation;
   - completed scope;
   - verification evidence;
   - remaining scope, if any;
   - close decision.
6. Preview the closeout note first.
7. If the user explicitly requested remote closure and reconciliation passes, call the script with `--close`.
8. Return the issue URL, final state, and closeout note summary.

## Script usage

Preview a closeout note only:

```bash
python3 .agents/skills/gitlab-issue-close/scripts/close_gitlab_issue.py \
  --issue "example-org/athena#7" \
  --branch "feat/example-issue-7" \
  --commit "abc1234" \
  --mr-url "https://git.example.com/group/project/-/merge_requests/1" \
  --issue-requirement "控制台可维护完整 skill package;覆盖同名 builtin skill 有确认" \
  --reconciled "完整 package 管理 -> MR !1 + web smoke;builtin 覆盖确认 -> server tests + browser smoke" \
  --completed "实现完整 skill package 管理;补齐 API 与 UI 验证" \
  --verification "go test ./...;npm --prefix web run build" \
  --decision "关闭：本仓 scope 已完成且无后续动作"
```

Post the final note and close the issue:

```bash
python3 .agents/skills/gitlab-issue-close/scripts/close_gitlab_issue.py \
  --issue "https://git.example.com/example-org/athena/-/issues/7" \
  --branch "feat/example-issue-7" \
  --commit "abc1234" \
  --mr-url "https://git.example.com/group/project/-/merge_requests/1" \
  --issue-requirement "控制台可维护完整 skill package;覆盖同名 builtin skill 有确认" \
  --reconciled "完整 package 管理 -> MR !1 + web smoke;builtin 覆盖确认 -> server tests + browser smoke" \
  --completed "实现完整 skill package 管理;补齐 API 与 UI 验证" \
  --verification "go test ./...;npm --prefix web run build" \
  --decision "关闭：本仓 scope 已完成且无后续动作" \
  --post-note \
  --close
```

## Auth

- Prefer `glab` when local GitLab auth exists.
- HTTP fallback uses `GITLAB_TOKEN` or `GITLAB_PRIVATE_TOKEN`.
- For `group/project#iid` references, set `GITLAB_BASE_URL` when using HTTP fallback.

## Output standard

When previewing, return:

- issue reference;
- checklist result;
- final note body;
- whether remote closure would be allowed.

When closing, return:

- issue URL if available;
- issue IID;
- final state;
- whether the closeout note was posted.

## Notes

- This skill should not silently close an issue as a side effect of progress sync.
- The script refuses `--close` without `--issue-requirement` and `--reconciled` inputs, so run issue intake and completion reconciliation first.
- If remaining scope exists, use `gitlab-issue-progress-sync` instead and keep the issue open.
- `Closes` should only appear in commit/MR context when the issue is truly complete; otherwise use `Refs` and progress comments.
