# 仓库工作流规范

## 目标

- 让仓库入口、共享正文和双技能体系保持单一来源。
- 降低“规则写了两份但不一致”的维护成本。
- 让功能改动和治理改动在提交与文档上都更容易追溯。
- 统一版本目录、问题流转和仓库级开发记录方式。

## 当前协作链

当前默认链路已经收敛为：

`athena issue -> athena branch/commit -> athena MR`

含义：

- `athena issue`
  - 承载当前仓库的正式需求、版本范围和问题真相。
- `athena branch/commit`
  - 承载本仓实现过程中的开发分支、提交记录和 issue 回链语义。
- `athena MR`
  - 承载本仓代码审阅、门禁确认和合并动作。

因此，本仓当前采用“本仓 GitLab issue 驱动 + 本仓 MR 承接”的模型。

## 版本管理

本仓库默认采用“稳定主分支 + 文档版本目录”的轻量版本模型：

- Git 主分支：`master`
- 功能开发分支：
  - `feat/<topic>-<issue-id>`
  - `bugfix/<topic>-<issue-id>`
  - `hotfix/<topic>-<issue-id>`
- 文档版本目录建议：
  - `docs/vX.Y.Z/`
  - `docs/vX.Y.Z/plan/`
  - `docs/vX.Y.Z/plan/<topic>/`
  - `docs/vX.Y.Z/features/`
  - `docs/vX.Y.Z/checklists/`
- 版本 master plan 必须维护一份可勾选的具体 feat checklist：
  - checklist 应列出本版本计划内的每个具体功能项，而不是只写阶段标题或宽泛目标
  - 形成版本 plan 时同步创建或更新 checklist
  - 每个功能完成后同步勾选，并记录可追溯证据
  - 任务收尾时必须逐项回顾 checklist，确认计划功能项已经完成、明确延期或明确取消
  - checklist 未创建、过于笼统、未回顾或未与实际完成情况对账时，版本化任务不得进入 `completed`

当前仓库还处于脚手架阶段，若功能规模较小，可以先继续使用项目级正文：

- `docs/api.md`
- `docs/architecture.md`
- `docs/implementation.md`
- `docs/TODO.md`

满足以下任一条件时，建议开始启用版本目录：

- 连续迭代多个版本，单一 `docs/*` 已难以承载变更历史
- 同一阶段存在多组独立 feature 文档和 checklist
- 需要按版本沉淀回归范围和发布说明

## 本仓 Issue 规则

本仓 GitLab issue 是当前默认的需求真源，也是 commit 回链与 MR 回链的默认依据。

当前默认要求：

- 分支命名默认带本仓 issue 标识
- commit message 默认回链本仓 issue
- MR 描述默认回链本仓 issue
- 进入 plan / implementation 前，若用户已给出 issue URL 或 `group/project#iid`，优先通过仓库内置 `issue-intake` skill 读取 issue 内容，而不是手工复制网页内容
- 需要新建本仓 GitLab issue 时，优先通过 `gitlab-issue-create` skill 生成统一模板草稿或实际提单

## Issue 时间线同步

当任务存在阶段性交付、长周期开发或需要让 reviewer 直接在 issue 时间线看到进度时，默认采用以下规则：

### 1. 开发前必须读取 issue 正文

- issue 编号只是入口，不是需求真相
- 开发前至少要确认：
  - 本仓负责什么
  - 本仓明确不做什么
  - 是否仍依赖后续动作
  - 完成判定标准是什么

### 2. `Refs` 与 `Closes` 不能混用

- `Refs group/project#iid`
  - 用于部分完成、阶段性交付或当前提交还不是最终收口
- `Closes group/project#iid`
  - 仅用于当前提交已经完成该 issue 在本仓内的全部要求，不再依赖后续动作

默认规则：

- 只要 issue 仍依赖后续动作，优先使用 `Refs`
- 不允许把“部分完成”写成 `Closes`

### 3. MR 必须说明本仓完成范围

MR 描述不能只写关联 issue，还必须明确：

- 本仓已完成什么
- 本仓明确不做什么
- 哪些内容仍未完成
- 是否还需要额外验证或后续动作

### 4. 本仓部分完成后要回 issue 评论

以下三个节点都应检查并默认执行 issue 评论同步：

1. 分支推送远端后
2. MR 创建后
3. 合并完成后

原因：

- `Refs` 只是提交关联，不是 issue 时间线里的可见进度
- 审查人员需要在 issue 中直接看到当前仓做到哪一步

若满足以下任一条件，以上三个节点都不应省略评论同步：

- 当前仓只完成 issue 的一部分
- issue 会跨多个提交或多个 MR 完成
- 当前提交使用了 `Refs`
- 当前仓完成后仍需要后续动作

推荐评论内容至少包括：

- 当前仓已完成内容
- 对应提交或 MR
- 当前仓明确未完成内容
- 后续动作

推荐工具化方式：

- 新建 issue 使用 `gitlab-issue-create`
- 读取 issue 正文使用 `issue-intake`
- issue 进度评论使用 `gitlab-issue-progress-sync`
- 关闭 issue 使用 `gitlab-issue-close`；关闭前必须先读取 issue 并将原始要求与交付证据对账
- 评论事件统一使用：
  - `push`
  - `mr`
  - `merge`

## 入口模型

本仓库当前采用双入口、单正文模型：

- Codex 入口：`AGENTS.md`
- Claude 入口：`CLAUDE.md`
- Issue 模板入口：`.gitlab/issue_templates/统一问题.md`
- MR 模板入口：`.gitlab/merge_request_templates/`
- 共享正文：`docs/*`
- 主 skill：`.agents/skills/`
- Claude 镜像 skill：`.claude/skills/`

约束：

- 入口文件只保留导航和工作规则摘要。
- 规则正文只维护在 `docs/*`。
- 主 skill 与镜像 skill 只保留入口和执行提示，不重复维护正文。
- 仓库正文、skill 和模板默认只写仓库内相对路径，不写个人机器绝对路径。

## 任务拆分原则

- 功能任务：修改代码、接口、运行时行为、tool 能力。
- 治理任务：修改入口文件、共享规范、skills、目录约定。
- 默认不要把大规模功能改动和长期治理改动混在同一个任务中。
- 若功能改动顺带暴露了通用规则缺口，可以做最小必要的治理补充，但不要借题发挥做大范围重构。

## MR 与提交规则

- branch / commit / 本地交付默认以本仓 GitLab issue 为主驱动
- MR 描述必须明确填写：
  - 对应 issue
  - 如有，额外记录关联依赖或外部协同信息
- 本仓默认使用：
  - `.gitlab/merge_request_templates/标准变更.md`
  - `.gitlab/merge_request_templates/合并确认.md`
- commit message 默认使用：

```text
feat: add test

Refs Super-Sky/Athena#1
```

说明：

- 日常开发提交默认使用：
  - `Refs Super-Sky/Athena#编号`
- 只有在明确完成并应关闭 issue 时，才使用：
  - `Closes Super-Sky/Athena#编号`
- `Closes` 不应用在中间态开发提交中，避免过早关闭本仓 issue
- 对于阶段性或部分完成任务，`Refs` 不是结束条件；合并后还需回 issue 评论同步真实进度
- 对于阶段性或部分完成任务，分支推送、MR 创建、合并完成三次都应检查是否已回 issue 评论
- 一笔提交默认只承载一个本仓 issue 主语义；不要把多个 issue 的改动压成同一笔提交
- 提交标题必须写当前这一笔的真实改动，不应用泛化标题掩盖多类内容，例如：
  - 不推荐：`chore: align issue-driven workflow and GitLab skills`
  - 推荐：`docs: tighten issue-driven workflow rules`
  - 推荐：`ci: add absolute path guard`
  - 推荐：`feat: add gitlab issue progress sync skill`

## GitLab CI

本仓库当前默认使用 GitLab CI，配置入口为：

- `.gitlab-ci.yml`

当前最小阻断流水线覆盖：

- `Go 测试`
  - 执行 `go test ./...`
  - PostgreSQL 集成测试默认通过 `ATHENA_PG_TEST_DSN` 显式启用；在 CI 未注入该变量时会自动 `skip`
- `绝对路径校验`
  - 执行 `python3 scripts/check_no_absolute_paths.py`
  - 阻断文档、skill、模板和脚本说明中再次引入个人机器绝对路径
- `Web 构建`
  - 在 `web/` 执行 `npm ci` 和 `npm run build`
- `部署配置校验`
  - 校验 `deploy/docker-compose.cloud.yml`
  - 使用 `deploy/athena.env.example` 临时生成 env 文件，再执行 `docker compose config`
- `发布包构建`
  - 校验 `scripts/build_release_bundle.sh` 与 `scripts/deploy_cloud.sh` 的 shell 语法
  - 执行 `scripts/build_release_bundle.sh`
  - 输出 `output/release/` 作为 artifact

当前触发策略：

- MR pipeline 优先
- 当分支已经存在打开中的 MR 时，普通 branch pipeline 默认不再重复触发
- 目标是避免同一份提交同时生成：
  - 一条 merge request pipeline
  - 一条普通 branch pipeline
- 若后续确需同时保留两类 pipeline，必须先说明：
  - 为什么现有 MR-only 策略不够
  - 哪些 job 只应在 branch 跑
  - 是否会引入重复构建成本

命名约束：

- GitLab UI 展示层使用中文：
  - stage 例如 `校验`、`构建`
  - job 例如 `Go 测试`、`Web 构建`
- 执行模板层使用英文：
  - 隐藏模板例如 `.go_test_template`
  - 这样既方便团队识别，也方便后续复用与维护

当前约束：

- 新增会影响构建、测试、部署脚本或 Web 前端的改动时，应优先复用现有 CI job，而不是平行发明第二套校验路径
- 若新增 job 依赖外部密钥、数据库、镜像仓库或远端主机，必须显式说明：
  - 依赖的 CI 变量
  - 失败时是否阻断 MR
  - 是否只在默认分支或手动触发
- 若某项检查只能在本地或手工环境执行，不应伪装成默认 CI 阻断项；应先以文档或非阻断 job 形式引入

## 骨架保护原则

本仓库后续会持续引入 AI 辅助开发与快速试错，因此需要显式保护 Athena 骨架，避免单一业务场景的试验性逻辑把底座改得面目全非。

默认应把以下区域视为“不可随意变形区”：

- `main.go`
- `internal/entry/*`
- `internal/app/*`
- `internal/runtime/*`
- `internal/runtime/task/*`
- `internal/policy/*`
- `internal/session/*`
- `internal/model/*`
- `internal/skills/*`
- `internal/tools/*`
- `internal/server/*` 中的公共 transport / SSE / OpenAPI 契约部分
- `docs/project.md`
- `docs/architecture.md`
- `docs/implementation.md`
- `docs/api.md`

这些区域的含义是：

- 它们承载 Athena 的通用运行时骨架、公共契约和长期演进方向。
- 它们可以演进，但不应因为某一个场景、某一个客户或某一轮快速试错而直接长出强耦合业务逻辑。

默认约束：

- 单场景需求优先落在 adapter、mapping、Workspace、Plug 或版本化 feature 文档，不要优先改骨架。
- 若某项需求确实需要修改骨架，应先证明“现有扩展点不足”，而不是因为直接修改更省事。
- 进入这些区域的改动，目标应是让骨架更抽象、更稳定、更可扩展，而不是更贴合当前场景。

### 不可随意变形区的准入条件

当改动触及上述区域时，提交到当前主分支前至少应明确回答：

1. 为什么不能通过新增 adapter、mapper、Workspace / Plug 侧逻辑解决
2. 这次改动抽象出的能力是什么，未来至少还能服务哪一类非当前场景
3. 这次改动引入了哪些新边界、状态或契约
4. 如果未来推翻当前场景，这段改动是否仍然成立

若无法清晰回答上述问题，默认应继续把改动留在业务层或实验层，而不是进入骨架。

## 版本计划与能力分层决策

制定大的版本计划或细化具体功能时，必须先判断每个能力属于哪一层，避免 Athena core 因场景便利而无限膨胀。

默认分层：

- `Core`：不做就不是通用 AI agent 平台的跨应用基础能力，例如 runtime、task/step、tool execution、governance、trace、usage、sandbox boundary、system truth、projection 基础能力。
- `Validation`：不做就无法独立验证 core 是否真实工作，例如 System Validation、验证型 MCP server、deterministic validation flow。
- `Enhancement`：不做不影响平台成立，但能让应用更快搭建或更完整，例如用户 skill 接管、应用知识库、业务 workflow、场景包、provider adapter、应用 runtime 判断逻辑。
- `Application / Business Truth`：业务对象、业务规则、业务状态、最终业务真相，默认属于应用层；Athena 可通过 enhancement 支持 Athena-managed 或 app-managed 两种模式。

进入版本 master plan 或功能细化文档前，至少应写清：

1. 该能力属于哪一层，以及为什么。
2. 是否会修改 core runtime 或骨架保护区。
3. 如果进入 core，为什么不能通过 enhancement、adapter、configuration 或 app-managed ref 解决。
4. 如果属于 enhancement，如何保证不反向污染 core runtime。
5. 是否需要独立验证路径，以及验收证据在哪里。

若无法完成上述定性，默认不应直接进入实现阶段；应先继续拆分能力边界。

## AI 协作流程

本仓库默认采用一条收敛后的 AI 协作链：

1. `Claude plan`
2. `Codex execute`
3. `Claude review`
4. `Codex finish`

其中：

- `Codex execute` 统一承接：
  - `correction`
  - `git worktree`
  - `implementation`
  - `tests`
  - `docs`
- `Codex finish` 统一承接：
  - `fix`
  - `repo-task-delivery`
  - 最终文档收敛
  - 将当前任务 worktree 的结果合入当前主分支
  - 过程文档清理
  - worktree 与过程里产生的临时目录清理
  - `git commit / git push`

目标：

- 让 `Claude` 负责需求理解、方案拆解和 review。
- 让 `Codex` 负责一整段实现责任，而不是在实现中途频繁来回切换。
- 让 `git worktree` 仍负责任务隔离，避免多个任务共享一个脏工作区。
- 让 AI 在流程缺步时能主动提醒，而不是依赖开发者记忆。
- 让每个阶段只有一个明确 owner，减少交接成本。

### Master Merge Gate

本仓库后续默认要求：

- 普通开发可在 feature / bugfix / hotfix 分支或独立 worktree 中快速推进
- 当结果准备合入 `master` 时，若改动触及“不可随意变形区”，必须额外执行一次显式的 `master-merge-gate`

`master-merge-gate` 的目的不是拖慢开发，而是确保：

- 场景试错不会无审查地改写骨架
- AI 辅助开发产生的抽象变更已被重新审视
- 进入 `master` 的是可复用能力，而不是阶段性业务形状

该 gate 默认要求两类确认同时存在：

- `AI review`
  - 明确指出这次改动是否仍保持通用性、扩展性和场景隔离
- `human review`
  - 当前使用者或指定 reviewer 明确确认该改动可以进入骨架主线

这里的 `human review` 不要求复杂审批系统，但必须有明确结论，而不是默认跳过。

### AI 执行原则

本节优先服务 AI，而不是要求开发者手工记忆流程。

默认要求：

- 开发者只需要描述目标，AI 负责判断当前缺少哪个阶段。
- AI 应根据当前状态主动提醒“现在该做什么、下一步该做什么”。
- 当缺失关键前置步骤时，AI 不应直接跳过到后续阶段。
- 当进入提交、推送或收尾阶段时，AI 应重新检查前置阶段是否闭合。
- AI 默认应遵守仓库语言约定：
  - 系统文件名保持英文
  - 代码目录和代码文件名保持英文
  - 非系统功能文档正文默认中文
  - 代码说明性注释默认双语两行制
- AI 默认应先读取当前代码真相文档和当前功能主文档，再决定是否进入版本文档：
  - `docs/当前能力总览.md`
  - `docs/features/README.md`
  - `docs/project.md`
  - `docs/runtime-in-product-architecture.md`
  - `docs/implementation.md`
  - `docs/api.md`
- 只有在需要理解历史漂移、版本边界、回归范围或迁移关系时，才系统性查阅旧版本目录。

### AI 状态机

AI 在处理一个版本化功能任务时，应将该任务视为以下有限状态之一：

- `planned`
- `implementing`
- `reviewing`
- `ready_for_delivery`
- `completed`
- `blocked`

状态含义：

- `planned`
  - 已有 Claude 计划，尚未进入 Codex 一整段执行
  - 版本 master plan 已包含可勾选的具体 feat checklist，且本任务对应项可定位
- `implementing`
  - Codex 正在执行一整段实现责任，包括 correction、worktree、编码、验证和文档同步
- `reviewing`
  - 已有一轮实现，等待 Claude review 或正在 review
- `ready_for_delivery`
  - 已收到 review 结论，Codex 正在 finish 阶段或准备进入 finish 阶段
- `completed`
  - 已完成最终文档收敛、过程文档清理和 worktree 清理
  - 已回顾版本 master plan 的 feat checklist，并完成本任务相关功能项的勾选、延期或取消标记
- `blocked`
  - 因缺少关键信息、环境、权限或外部依赖而无法推进

### 状态推进规则

- 无计划时，不应直接进入 `implementing` 或 `reviewing`
- 已有 `plan-claude.md` 但还没有进入 Codex 一整段执行时，状态应视为 `planned`
- 已有 `plan-claude.md` 但版本 master plan 没有可勾选的具体 feat checklist 时，不应进入 `implementing`；应先补齐 checklist
- `planned -> implementing` 需要能指向当前有效计划、目标 worktree、分支和本任务对应的 feat checklist 项
- Codex 已开始 correction / worktree / implementation 任一子阶段后，状态应视为 `implementing`
- `implementing -> reviewing` 需要能指向当前 `*-codex.md`、实现结果和测试证据
- 已完成一轮实现且需要独立审查时，状态应切到 `reviewing`
- `reviewing -> ready_for_delivery` 需要能指向当前 `review-claude.md`
- 已有 review findings 或 review 结论，且准备进入 Codex finish 时，状态应切到 `ready_for_delivery`
- 已完成最终 feature 文档、更新 master plan、回顾并对账 feat checklist、将当前任务结果合入当前主分支、删除过程文档并清理无用 worktree/临时目录后，状态才可视为 `completed`
- `completed` 只在上述收尾证据都可追溯时成立

### AI 阻断规则

当满足下面任一条件时，AI 不应静默跳过，而应先提醒并优先补齐前置步骤：

- 任务是新需求或跨模块任务，但没有 `plan-claude.md`
- 版本化任务已经有计划，但版本 master plan 缺少具体 feat checklist，或本任务对应项不可定位
- 已进入 Codex 执行阶段，但没有独立 worktree，且不满足例外条件
- 需要进入 review 阶段，但没有当前有效的 Codex 执行结果或 feature/diff 上下文
- 准备进入 Codex finish 阶段，但没有当前有效的 `review-claude.md`
- 准备提交或推送，但没有执行 `repo-task-delivery`
- 准备提交或推送，但没有明确本仓 issue 标识
- 准备合入 `master`，且本轮改动触及“不可随意变形区”，但没有执行 `master-merge-gate`
- 准备结束任务，但 master plan 状态未更新
- 准备结束任务，但本任务相关 feat checklist 未逐项回顾、勾选或标记延期 / 取消
- 准备结束任务，但当前任务结果尚未合入当前主分支
- 准备结束任务，但过程文档未收敛删除
- 准备结束任务，但无用 worktree 或过程里产生的临时目录未清理

阻断后的默认动作：

1. 明确指出当前状态
2. 明确指出缺失步骤
3. 明确指出下一步唯一优先动作
4. 补齐后再进入后续阶段

### 合入 `master` 的额外要求

当本轮任务准备合入 `master` 时，除了常规 delivery gate 外，还应额外检查：

- 是否触及“不可随意变形区”
- 若触及，是否已经执行 `master-merge-gate`
- 是否已有 AI 对通用性与扩展性的明确审查结论
- 是否已有人工确认该改动可以进入骨架主线
- 是否已说明哪些逻辑仍留在业务层、哪些逻辑被提升为骨架能力

默认不要把“准备以后再收敛”的试验性骨架改动直接并入 `master`。

### 1. `Claude plan`

适用时机：

- 新需求
- 跨模块改动
- 风险不清楚的缺陷
- 需要先做方案比较的任务

输出要求：

- 任务目标
- 影响模块
- 版本 master plan 中对应的具体 feat checklist 项
- 风险点
- 最小测试清单
- 完成定义

约束：

- 这一阶段不默认开始写代码。
- Claude 在产出并保存当前有效计划后，应停在 planning 边界，由当前使用者手动调用 Codex 进入下一阶段；不要默认由 Claude 继续实现。
- 当前有效的计划与阶段文档应放在仓库内 `docs/vX.Y.Z/plan/` 或 `docs/vX.Y.Z/plan/<topic>/`，不要把 `~/.claude/plans/` 作为正式交接面。
- 计划应尽量收敛到可执行清单，而不是长篇背景讨论。
- 若任务归属某个版本，计划必须同步创建或更新版本 master plan 的 feat checklist，并确保本任务对应项可被后续阶段引用和勾选。
- 若输出为阶段文档，文件名应带 `-claude` 后缀，明确表示该文档由 Claude 在当前阶段生成。
### 2. `Codex execute`

适用时机：

- Claude 已给出初版计划，准备进入实现

职责：

- 工程化 correction
- 创建或切换独立 worktree
- 落代码
- 跑受影响验证
- 同步共享文档
- 追加 `develop.log`

输出要求：

- 一份 `correction-codex.md` 或等价的工程化执行清单
- 一轮完整实现结果与测试结果

约束：

- 这一阶段应尽量由 Codex 一次性完成，不要在 correction、worktree、implementation 之间频繁切回 Claude。
- 若 Claude 已产出阶段文档，Codex 应先读取对应 `-claude` 文档，再生成当前阶段的 `-codex` 文档。
- `-codex` 文档应明确记录：
  - 修正后的执行清单
  - 分层归属
  - 必补测试
  - 必补文档
  - 当前阶段结论

### 2.2 真实测试用例

当某个版本主题已经进入“可供真实使用者验证”的阶段时，Codex 不应只停在自动化测试或 review checklist，而应额外生成一份面向真实使用者的测试用例文档。

默认要求：

- 这是默认交付物，不需要开发者重复提出
- 默认放在：
  - `docs/vX.Y.Z/checklists/真实场景测试用例.md`
- 目标读者是实际点接口、跑流程、验证业务链路的人，而不是代码实现者

文档至少应包含：

- 测试目标
- 操作步骤
- 示例请求
- 预期结果
- 失败时重点排查点

适用阶段：

- 版本已具备真实场景测试条件时
- 或某个子链路已经足够稳定，适合先做边缘功能验证时

### 2.1 `git worktree`

默认要求：

- 进入正式实施前，优先为当前任务创建独立 worktree。
- 一个 worktree 对应一个任务分支。
- 不要把多个独立任务堆在同一个脏工作区里并行推进。

推荐命名：

- 分支：`codex/<topic>`
- 目录：`../athena-<topic>`
- 阶段文档建议放在当前任务 worktree 内，并与任务主题同名

示例：

```bash
git worktree add ../athena-model-governance -b codex/model-governance
codex -C ../athena-model-governance
```

例外：

- 纯只读分析
- 极小改动且当前工作区干净
- 用户明确要求直接在当前工作区继续

若未满足以上例外，AI 应主动提醒当前使用者优先切到独立 worktree。

### 阶段文档接力规则

本仓库允许 Claude 和 Codex 通过文档接力，而不是只依赖聊天上下文。

目录规则：

- 本次版本的整体计划放在 `docs/vX.Y.Z/plan/`
- 单个功能的过程文档放在 `docs/vX.Y.Z/plan/<topic>/`
- 模板建议放在 `docs/vX.Y.Z/plan/_template/`

命名规则：

- Claude 生成的阶段文档使用 `*-claude.md`
- Codex 生成的阶段文档使用 `*-codex.md`

推荐阶段：

- `plan-claude.md`
- `correction-codex.md`
- `review-claude.md`
- `fix-codex.md`

要求：

- 同一阶段只保留一份当前有效文档，避免并行版本漂移。
- Codex 在进入下一阶段前，应优先读取对应 `-claude` 文档。
- Claude 在 review 时，应优先读取当前有效的 `-codex` 文档。
- 阶段文档要明确写清“这是哪个阶段产物”，不能只写结论不写阶段。
- `docs/vX.Y.Z/plan/` 下的整体计划应同步标记各功能当前开发状态。

这些阶段文档是过程文档，不是最终交付文档。

### 3. `Claude review`

适用时机：

- Codex 已完成一轮实现
- 准备收尾、提交或推送前

review 重点：

- bug
- 行为回归
- 边界错误
- 测试缺口
- 文档缺口

约束：

- review 以发现问题为主，不重复实现摘要。
- review 结果若写成文档，应输出为 `*-claude.md`，并明确标注为 review 阶段。

### 4. `Codex finish`

职责：

- 根据 review findings 做最小必要修正
- 补齐遗漏测试或文档
- 回顾版本 master plan 的 feat checklist，逐项确认本任务相关功能项是否已完成、延期或取消
- 汇总每条 finding 是否已关闭
- 执行 `doc-index-sync`
- 执行 `feature-doc-skill-sync`
- 执行 `repo-task-delivery`
- 收敛最终 feature 文档
- 判断当前功能是否应抽象为对应 skill，并完成必要的主从同步
- 清理过程文档
- 清理无后续用途的 worktree
- 在已确认本仓 issue 回链语义的前提下完成 `git commit / git push`

约束：

- 这一轮优先处理 review 已指出的问题，不重新发散方案。
- fix 阶段若写成文档，应输出为 `*-codex.md`，并明确列出已关闭和未关闭的问题。
- finish 阶段默认也由 Codex 一次性完成，不要在 fix、delivery、cleanup 之间频繁切回 Claude。

### 4.1 `repo-task-delivery`

这是正式提交和推送前的必走门禁。

要求：

- finish 阶段若新增、删除、重命名文件，或明显调整模块边界，先执行 `doc-index-sync`
- finish 阶段先执行 `feature-doc-skill-sync`，再执行 `repo-task-delivery`
- 提交或推送前必须执行 `repo-task-delivery` skill
- 版本化任务必须在交付前回顾版本 master plan 的 feat checklist，逐项勾选已完成项，或明确标记延期 / 取消项及原因
- 检查共享正文、skill 镜像、`develop.log`、测试、benchmark、内存风险说明是否齐全
- 未确认本仓 issue 时，不得由 AI 主动提交或主动推送
- 当前任务若使用了阶段文档，需先把 `*-claude.md` / `*-codex.md` 整理为一份描述功能、实现方案与当前状态的最终文档，再删除过程文档
- 该最终文档默认落在 `docs/features/feature-<topic>.md`
- 对已形成稳定维护面的功能，需在交付前判断是否沉淀为对应 skill，并同步 `.claude/skills/` 镜像或明确记录暂不沉淀的原因
- 当前任务完成测试后，Codex 负责清理本次任务产生的临时 worktree；只保留当前需要继续工作的 worktree

### 最终文档收敛规则

当功能已完成实现并通过当前要求的验证后，Codex 应负责：

1. 读取本次任务相关的 `*-claude.md` 与 `*-codex.md`
2. 更新 `docs/vX.Y.Z/plan/` 下整体计划的开发状态，并回顾 / 对账版本 master plan 的 feat checklist
3. 通过 `doc-index-sync` 梳理目录 README、文件索引和文件头说明
4. 通过 `feature-doc-skill-sync` 梳理模块边界、文档收敛和 feature skill 结论
5. 合并整理为一份“当前状态文档”
6. 文档内容至少包含：
   - 功能目标
   - 实现方案
   - 关键模块与入口
   - 配置 / 开关 / 依赖
   - 对外契约或关键交互链路
   - 当前状态
   - 已完成项
   - 未完成项或剩余风险
7. 若本轮修改影响模块目录结构，应同步更新对应目录 README：
   - 子目录索引
   - 全文件索引
   - 对外入口
   - 维护提示
8. 最终文档默认写入 `docs/features/feature-<topic>.md`
9. 若本轮需要保留版本快照，可在 `docs/vX.Y.Z/features/` 额外记录版本范围或快照说明，但不替代当前功能主文档。
10. 根据最终文档判断该功能是否需要抽象为对应 skill：
   - 需要时，先更新 `.agents/skills/`
   - 再同步 `.claude/skills/`
   - 暂不抽象时，在最终文档中保留原因
11. 删除过程文档：
   - `*-claude.md`
   - `*-codex.md`
12. 清理本次任务产生且已不再需要的 worktree

约束：

- 最终文档应描述“当前有效状态”，而不是保留过程性对话痕迹。
- 若任务仍在进行中，不要提前删除当前仍在使用的阶段文档。
- 若 worktree 仍承载未合并或未交接的有效工作，不应误删；只清理已完成且确认无后续用途的任务 worktree。

### AI 提醒规则

当 AI 发现当前任务符合下面任一条件时，应主动提醒使用这条流程：

- 当前任务是新需求或跨模块任务，但还没有明确计划
- 当前准备开始实现，但还没有进入 `Codex execute`
- 当前工作区已有未提交改动，且又要开始另一个独立任务
- 当前准备正式实施，但还没有创建独立 worktree
- 当前阶段需要衔接上一阶段结果，但还没有读取对应的 `-claude` 或 `-codex` 文档
- 当前准备进入 finish / 提交阶段，但还没有执行 `repo-task-delivery`
- 当前准备提交或推送，但还没有明确本仓 issue 标识
- 当前任务已经完成验证，但过程文档和临时 worktree 还没有收敛清理

默认提醒顺序：

1. 先提醒是否需要 `Claude plan`
2. 再提醒是否需要进入 `Codex execute`
3. 再提醒是否需要切到独立 `worktree`
4. 再提醒是否需要读取对应阶段文档
5. 实现完成后提醒 `Claude review`
6. review 后提醒进入 `Codex finish`
7. 收尾时提醒 `repo-task-delivery`、最终文档收敛与 worktree 清理

### AI 提醒输出约定

当 AI 触发流程提醒时，建议输出以下最小结构：

- `current_state`
- `missing_step`
- `why_now`
- `next_action`

要求：

- 优先只给一个最高优先级的下一步，而不是同时给多个并行动作。
- 若当前阶段已经明确，不要重复回顾整条链路。
- 若当前阶段被阻断，先说明阻断原因，再给下一步。

### AI 交接输出约定

当下一步需要切换 owner 时，当前 owner 不能只提示“去找 Claude”或“去找 Codex”，而必须主动输出一段可直接转发的标准化 handoff。

双向规则：

- 若下一 owner 是 `Codex`，则当前 owner `Claude` 必须主动输出 `send_to_codex`
- 若下一 owner 是 `Claude`，则当前 owner `Codex` 必须主动输出 `send_to_claude`

这条规则适用于所有明确的 owner 切换点，包括但不限于：

- `Claude plan -> Codex correction`
- `Codex implementation -> Claude review`
- `Claude review -> Codex fix`

交接文本默认继续沿用统一 block 结构：

- `context`
- `goal`
- `inputs`
- `required_output`
- `constraints`

约束：

- handoff 不能是被动的，不能等开发者追问后才补
- handoff 不能只是摘要，必须尽量做到“复制即可发送”
- 默认采用短版 handoff，只有复杂任务才扩展为长版
- handoff 至少应包含：
  - 当前任务 topic
  - 当前阶段
  - worktree 路径
  - 分支名
  - 相关阶段文档路径
  - 本轮任务目标
  - 明确要求对方做什么
  - 明确要求对方不要做什么
- 若 handoff 依赖阶段文档，必须写出文档路径
- 若 handoff 依赖 worktree，必须写出 worktree 路径与分支
- 默认不要把长期背景、已稳定沉淀在仓库文档中的大段说明再次整段复制进 handoff

## 文档更新顺序

1. 先改代码和真实行为。
2. 再核对 `docs/api.md`、`docs/architecture.md`、`docs/implementation.md` 是否需要更新。
3. 若结论可复用，补 `docs/CODE_STYLE_GUIDE.md`、`docs/TASK_DELIVERY_GUIDE.md` 或本文件。
4. 如需 skill 暴露新流程，先更新 `.agents/skills/`，再同步 `.claude/skills/`。

## Skill 主从约定

- `.agents/skills/` 是主版本。
- `.claude/skills/` 是轻量镜像。
- 两侧 skill 目录名应保持一一对应。
- `name` 和 `description` 建议保持一致。
- 若共享正文发生变化，优先更新正文，再检查 skill 是否仍然指向正确入口。
- `repo-task-delivery` 是本仓库提交前与推送前的必走 skill。
- 任何 `git commit` 或 `git push` 前，都应先按 `repo-task-delivery` 完成质量门禁检查。

## 提交建议

- 功能改动提交优先描述行为变化。
- 治理改动提交优先描述规范或入口变化。
- 提交信息建议格式：
  - 标题行：`type: 描述`
  - footer：`Refs group/project#iid` 或 `Closes group/project#iid`
- 只有在当前提交确实完成本仓 issue 的收尾语义时，才使用 `Closes`
- 若同一次提交同时改代码和文档，提交说明应明确：
  - 改了什么行为
  - 更新了哪些文档

## Issue 缺失时的强制规则

- 只要任务涉及 `git commit`、`git push` 或“由 AI 主动推送”，就必须先确认明确的本仓 issue 标识。
- 如果当前上下文没有 issue 标识，AI 必须先向当前使用者索取，不能自行假设、编造或跳过。
- 未确认 issue 前：
  - 可以继续本地分析、改代码、补文档、跑验证
  - 不可以完成正式提交
  - 不可以执行主动推送
- 当前使用者以本机 git 身份和当前对话中的实际操作者为准；如需确认，可优先参考 `git config user.name`。

## develop.log 约定

- 仓库级开发活动默认记录到根目录 `develop.log`。
- 记录人默认取 `git config user.name`。
- 每次关键变更完成后及时追加，不集中追记。

### 记录格式

- `[YYYY-MM-DD HH:mm] [人员] [类别] 描述内容 (涉及文件)`

### 类别建议

- `BE`：业务或核心功能代码
- `FIX`：问题修复
- `DOC`：文档更新
- `INFRA`：配置、目录、基础设施、仓库治理
- `SKILL`：skills、入口文件、AI 协作规则

### 记录要求

- 描述以“做了什么、影响了哪些文件”为主。
- 如果已知归属版本和 issue，可直接写在描述中。
- 功能任务和治理任务可以分别补记录。
- 入口、共享规范、skills 的调整属于应记录的治理变更。

## 提交前自检

- [ ] 提交前已执行 `repo-task-delivery` skill
- [ ] 入口文件没有重复维护正文
- [ ] 共享规则已收口到 `docs/*`
- [ ] API / SSE / runtime 改动已核对现有文档
- [ ] `.agents/skills/` 与 `.claude/skills/` 的对应 skill 已同步
- [ ] 已补充或确认 `develop.log`
- [ ] 核心代码已补齐双语注释
- [ ] 双语注释采用“两行制”：英文一行，中文一行
- [ ] 已完成必要单测、系统测试、性能验证
- [ ] 涉及 queue/session/memory/等待态时已观察内存行为或明确说明风险
- [ ] 已按 `docs/v0.1.0/features/feature-可观测验证指南.md` 执行或复用性能/内存检查步骤
- [ ] 若要提交或推送，已确认对应 issue 标识与 `Refs / Closes` 语义
