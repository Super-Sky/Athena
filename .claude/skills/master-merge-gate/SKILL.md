---
name: master-merge-gate
description: Use before merging changes into master when the task touches Athena backbone or shared contract surfaces, to verify the changes remain general, extensible, and not scene-coupled.
---

# Master Merge Gate

- Primary workflow source: `docs/REPO_WORKFLOW_GUIDE.md`
- Delivery gate source: `docs/TASK_DELIVERY_GUIDE.md`
- Docs index: `docs/README.md`
- Use this skill before merging into `master` when protected backbone surfaces were touched.
- Require both:
  - AI review confirming the change stays general and extensible
  - explicit human confirmation before merge into `master`
- Gate questions:
  - why this cannot stay in adapters, mappers, Workspace, Plug, or versioned feature code
  - what reusable capability is entering the backbone
  - how scene-specific logic was kept outside the backbone
  - whether the change would still make sense if the current business direction were later discarded

Keep this skill lightweight. Shared rules live in `docs/*`.
