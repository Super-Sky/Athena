---
name: gitlab-issue-create
description: Use when the user wants to create, draft, preview, or post a GitLab issue for this repository workflow. Trigger for requests like “新建 issue”, “帮我提 issue”, “create a GitLab issue”, or when work requires opening an issue before planning because it changes feature boundaries, cross-service contracts, schema, local runtime boundaries, version freeze scope, or module-level defects.
---

# GitLab Issue Create

Use this skill when a task needs a new canonical GitLab issue, either as a preview draft or an actual posted issue.

This skill complements:

- `issue-intake`: read an existing issue before planning or implementation.
- `gitlab-issue-progress-sync`: sync progress comments after push, MR, or merge milestones.

## Read First

- `docs/REPO_WORKFLOW_GUIDE.md`
- `docs/TASK_DELIVERY_GUIDE.md`
- `scripts/create_gitlab_issue.py`
- Repository issue template: `.gitlab/issue_templates/统一问题.md`

## When to create an issue

Create or draft an issue before implementation when the work affects any of these:

- new feature boundaries
- cross-service changes
- schema or shared contract changes
- local runtime boundary changes
- version freeze or version scope changes
- delivery-unit work
- integration work
- module-level defects

Do not create issues for tiny local implementation details unless the user explicitly asks. Small same-module bugfixes, tests, styling, or documentation refinements can usually go straight to MR, while still following review and delivery rules.

## Required intake fields

Before posting, make sure the issue draft has enough user-facing context. Ask only for missing information that changes the issue meaning; otherwise draft with explicit `待补充` placeholders.

Required submitter fields:

- title
- issue type, one of:
  - `需要新增能力`
  - `需要增强现有能力`
  - `当前功能有问题`
  - `本地运行或环境有问题`
  - `文档或方向需要调整`
- scenario or business background
- current problem
- requested improvement
- current result
- expected result
- urgency, one of `高`, `中`, `低`
- supplemental material if available

Maintainer triage fields are optional at issue creation and should not be invented:

- handling mode
- target version
- delivery unit
- primary repository
- collaborating repositories
- owner
- collaborators
- next action

## Workflow

1. Decide whether a new issue is actually required by the repository workflow.
2. Confirm the target project:
   - use a full GitLab project URL, or
   - use `group/project` with `GITLAB_BASE_URL` set.
3. Build a draft using the unified issue body shape.
4. Preview first unless the user explicitly asked to post.
5. If posting:
   - prefer `glab` when local GitLab auth exists;
   - otherwise use `GITLAB_TOKEN` or `GITLAB_PRIVATE_TOKEN`;
   - never invent a GitLab URL or project path.
6. Return the issue URL or the previewed title/body.

## Script usage

Preview a draft:

```bash
python3 .agents/skills/gitlab-issue-create/scripts/create_gitlab_issue.py \
  --project "example-org/athena" \
  --title "Skill 管理支持完整包编辑" \
  --issue-type "需要增强现有能力" \
  --background "控制台需要调试 uploaded skill 和 builtin skill 覆盖关系" \
  --problem "当前只能编辑轻量 skill 配置，不能查看完整包文件和 revision" \
  --request "补齐 skill package 详情、文件编辑、上传替换和覆盖确认" \
  --current-result "调试人员无法在控制台完成完整 skill 包维护" \
  --expected-result "控制台可以查看、上传、替换、编辑并验证完整 skill package" \
  --urgency "中"
```

Post the issue:

```bash
python3 .agents/skills/gitlab-issue-create/scripts/create_gitlab_issue.py \
  --project-url "https://git.example.com/example-org/athena" \
  --title "Skill 管理支持完整包编辑" \
  --issue-type "需要增强现有能力" \
  --background "控制台需要调试 uploaded skill 和 builtin skill 覆盖关系" \
  --problem "当前只能编辑轻量 skill 配置，不能查看完整包文件和 revision" \
  --request "补齐 skill package 详情、文件编辑、上传替换和覆盖确认" \
  --current-result "调试人员无法在控制台完成完整 skill 包维护" \
  --expected-result "控制台可以查看、上传、替换、编辑并验证完整 skill package" \
  --urgency "中" \
  --labels "需求,athena" \
  --post
```

## Output standard

When previewing, return:

- target project
- title
- generated body
- labels / assignee / milestone if provided
- exact command needed to post, if useful

When posting, return:

- issue URL
- issue IID if available
- project path
- title

## Notes

- This skill creates the issue; it does not replace `issue-intake` for reading the issue before implementation.
- If the issue is posted, future branch / commit / MR work should use that canonical issue reference.
- If the work is only a local fix and does not require a new issue, say so and recommend continuing with the existing issue or direct MR path.
