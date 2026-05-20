# System Truth Baseline Recovery

Use this skill after one active truth dir change has been validated in the running Athena instance and must be recovered back into the Git-managed baseline.

Primary rule sources:

- `docs/REPO_WORKFLOW_GUIDE.md`
- `docs/TASK_DELIVERY_GUIDE.md`
- `docs/参考资料/工具书/system-objects-management-and-iteration.md`
- `docs/features/feature-上下文资产注入与结构化管理.md`

## Goal

Recover validated system resource changes from the active truth dir into repository-managed baseline files without introducing one-click uncontrolled Git writes from the control-plane UI.

## Use When

- A control-plane `system-resources` change was validated in runtime.
- One exported truth-dir snapshot or one downloaded resource source must be reviewed and applied into repo baseline.
- The task needs an explicit operator workflow instead of direct UI-to-Git write.

## Workflow

1. Confirm the source of truth for this recovery task.
   - Prefer one exported truth-dir snapshot.
   - Otherwise use one downloaded resource source plus compile result and pipeline record.
2. Read the relevant baseline files in repo.
   - Typical baseline area: `config/system/truth/`
   - Related docs and skills may also need updates when the object affects AGENTS/specs/skills.
3. Produce a recovery analysis:
   - changed asset ids
   - current truth-dir version
   - candidate repo targets
   - risks if applied
4. Apply the repo changes explicitly.
   - No blind replace of the whole export bundle.
   - Keep one commit focused on one upstream issue semantics.
5. Update stable docs if the recovered baseline changes long-term behavior.
6. Run verification:
   - backend tests
   - frontend build if control-plane/Web changed
   - absolute-path guard if docs/skills changed
7. Follow issue-driven workflow:
   - commit with `Refs` or `Closes`
   - sync progress back to the canonical issue when required

## Guardrails

- Do not treat the active truth dir as automatically trusted Git baseline.
- Do not overwrite unrelated baseline files from one bulk export.
- Do not skip doc truth updates when one recovered asset changes runtime behavior.
- Do not create one-click Git submission behavior in control-plane.
