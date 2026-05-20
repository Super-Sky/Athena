---
name: github-issue-progress-sync
description: Use when a branch is pushed, a PR is created, or a merge is completed for a GitHub-backed task and the agent must check and usually sync progress back to the canonical issue.
---

# GitHub Issue Progress Sync

Use this skill when a task is backed by a GitHub issue and one of these milestones is reached. The default is to check and sync a progress note, not to wait until merge closeout:

- branch pushed
- pull request created
- merge completed

## Read First

- `docs/REPO_WORKFLOW_GUIDE.md`
- `docs/TASK_DELIVERY_GUIDE.md`
- `scripts/post_github_issue_comment.py`

## Workflow

1. Confirm the canonical issue reference and current repository scope.
2. Decide which event is being reported:
   - `push`
   - `pr`
   - `merge`
3. Check whether issue comment sync is required. Do not skip it when any of these are true:
   - the current repository only completes part of the issue
   - the issue spans multiple commits or PRs
   - the current commit uses `Refs`
   - follow-up work remains after the current repository change
4. Summarize:
   - branch
   - commit
   - PR link if applicable
   - completed scope
   - remaining scope
   - next actions
5. Post the note back to the issue instead of relying only on commit footer linkage.

## Usage

Preview a push-progress note:

```bash
python3 .agents/skills/github-issue-progress-sync/scripts/post_github_issue_comment.py \
  --issue "Super-Sky/Athena#7" \
  --event push \
  --branch "feat/example-issue-4" \
  --commit "61e20ec" \
  --completed "收敛绝对路径规则;更新 PR 模板和交付 skill" \
  --pending "联调确认;等待 review" \
  --next "发起当前仓 PR"
```

Post the note:

```bash
python3 .agents/skills/github-issue-progress-sync/scripts/post_github_issue_comment.py \
  --issue "Super-Sky/Athena#7" \
  --event pr \
  --branch "feat/example-issue-4" \
  --commit "61e20ec" \
  --pr-url "https://github.com/Super-Sky/Athena/pull/1" \
  --completed "当前仓改动已入 PR，等待评审" \
  --pending "review comment 处理;本仓后续动作" \
  --next "评审通过后合并" \
  --post
```

## Notes

- Prefer `gh` when local GitHub auth exists.
- Fall back to `GITHUB_TOKEN`, `GH_TOKEN`, or `GITHUB_PAT` when needed.
- Always include current-repo scope and any remaining follow-up work in the note.
- `Refs` / `Closes` semantics are decided by the repository workflow and PR/commit context; this skill only syncs the real progress note.
- `push` / `pr` / `merge` 三类事件都应显式检查并默认回 issue 评论；不要只在 merge 后补一次。
