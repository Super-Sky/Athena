# 任务交付文档规范

## 目标

- 让每次任务除了代码改动外，都留下可复用的项目文档资产。
- 避免接口契约、SSE 事件和运行时约束只停留在聊天记录里。
- 避免功能任务和仓库治理任务混在同一轮改动中难以追溯。

## 入库门槛

以下门槛未满足前，不应把代码作为“可正式入库”的结果交付：

- 版本化任务已回顾版本 master plan 的具体 feat checklist：
  - 已完成项已勾选，并能指向实现、测试或文档证据。
  - 未完成项已明确标记延期或取消，并写清原因。
  - checklist 缺失、过于笼统或未对账时，不得声明任务完成。
- 核心代码已补齐双语注释：
  - 英文在前，中文在后。
  - 双语注释必须分两行，不接受单行混排。
  - 至少覆盖核心方法、核心类型、核心变量、核心常量和关键状态转换。
- 已完成必要单测：
  - 不接受仅靠手工描述替代基础自动化验证。
- 已完成必要系统测试或等价集成验证：
  - 尤其是 API、SSE、等待态、补数回流、队列状态机等跨层链路。
- 已完成性能验证：
  - 关键链路默认带 benchmark 或已有基线复测。
- 已完成必要内存观察：
  - 涉及 session、queue、memory、长等待状态、stream buffer 时必须关注内存滞留风险。
  - 具体检查步骤统一参考 `docs/v0.1.0/features/feature-可观测验证指南.md`。

## 流程收口门禁

当任务进入交付收尾时，默认执行顺序如下：

1. 若本轮改动新增、删除、重命名文件，或明显改变模块边界，先执行 `doc-index-sync`。
2. 再执行 `feature-doc-skill-sync`，把完成的 feature 收敛为 `docs/features/feature-<topic>.md` 并判断 skill 去向。
3. 最后执行 `repo-task-delivery`，在正式提交或推送前做最终门禁检查。

门禁默认 fail-closed：

- 缺少对应证据时，不能宣称已通过。
- 缺少文档、测试、issue 回链信息、worktree 清理、master-plan 回写或版本 feat checklist 对账时，必须先补齐再交付。
- 若阶段文档、最终文档或验证证据缺失，应先回到对应阶段，不可直接跳过。

## 阶段证据口径

- `planned`：有效计划、目标 worktree、分支、版本 feat checklist 对应项、阶段文档可追溯。
- `implementing`：`*-codex.md`、实现结果、测试证据可追溯。
- `reviewing`：`review-claude.md` 可追溯。
- `ready_for_delivery`：review 结论、修复说明、门禁待执行项可追溯。
- `completed`：最终 feature 文档、master-plan 回写、版本 feat checklist 回顾与勾选、过程文档清理、worktree / 临时目录清理可追溯。

## 仓库工作模型

本仓库默认按 3 层模型工作：

### 1. 功能层

- 面向单个需求、缺陷或 runtime 行为变更。
- 典型产物：
  - `internal/*`
  - `main.go`
  - `docs/features/feature-*.md`
  - `docs/vX.Y.Z/checklists/*-review-checklist.md`

### 2. 治理层

- 面向跨任务复用的规则、入口和技能体系。
- 典型产物：
  - `AGENTS.md`
  - `CLAUDE.md`
  - `.agents/skills/`
  - `.claude/skills/`
  - `docs/README.md`
  - `docs/CODE_STYLE_GUIDE.md`
  - `docs/TASK_DELIVERY_GUIDE.md`
  - `docs/REPO_WORKFLOW_GUIDE.md`
  - `develop.log`

### 3. 记忆层

- 面向后续追溯与经验沉淀。
- 典型产物：
  - `develop.log`
  - `docs/features/feature-*.md` 中的经验补充
  - `docs/vX.Y.Z/checklists/*-review-checklist.md` 中的回归范围
  - `docs/TODO.md`

## 默认要求

只要任务满足以下任一条件，完成时都应同步产出或更新文档：

- 新增或修改 HTTP 接口
- 新增或修改 SSE 事件类型、字段或输出顺序
- 新增模型提供方、tool 能力或 session/memory 行为
- 修改运行时能力裁剪、并发策略、权限或 customization 规则
- 增加一组容易复发的约束、回归点或排查经验
- 新建版本目录、切换开发版本或调整 issue 归属规则

## 文档放置规则

### 1. 当前系统共享正文

以下内容优先更新现有正文，而不是新建重复文档：

- 当前功能项总览：`docs/当前能力总览.md`
- 当前功能主文档区：`docs/features/`
- API 契约：`docs/api.md`
- 架构与时序：`docs/architecture.md`
- 模块实现细节：`docs/implementation.md`
- 通用实现规范：`docs/CODE_STYLE_GUIDE.md`
- 任务交付规则：本文件
- 仓库工作流：`docs/REPO_WORKFLOW_GUIDE.md`
- 仓库变更记录：`develop.log`

### 2. 具体功能文档

- 单个需求、缺陷或专项改造，优先放：
  - `docs/features/feature-<功能名>.md`
- 若当前任务仍处于计划或开发过程阶段，优先放：
  - `docs/vX.Y.Z/plan/`
  - `docs/vX.Y.Z/plan/<功能名>/`
- 每个功能都应有一份可持续维护的详细说明文档；若当前任务尚未结束，可先在 `plan/` 目录维护，完成后必须收敛到 `docs/features/feature-<topic>.md`
- `docs/vX.Y.Z/features/` 用于保存版本快照、阶段性收口和版本差异，不应作为“当前真相”主落点。
- 建议记录：
  - 背景
  - 规则
  - 技术方案
  - 关键模块与入口
  - 配置 / 开关 / 依赖
  - 对外契约或关键交互链路
  - 风险点
  - 验证方式

### 2.1 功能抽象为 Skill

- 当一个功能已经形成稳定流程、固定判断标准，或后续大概率需要独立调整方案 / 定向修 bug 时，应在任务完成后判断是否沉淀为对应 skill。
- skill 的作用是承接“如何继续维护这个功能”，而不是替代功能说明文档本身。
- 这一步默认应视为稳定交付动作，优先通过 `feature-doc-skill-sync` skill 执行，而不是临时决定是否补文档或补 skill。
- 默认要求：
  - 先补齐 `docs/features/feature-<topic>.md`
  - 明确当前功能或模块边界是否已经足够清晰，可被后续维护者独立定位
  - 再判断是否需要新增或更新对应 skill
  - 若新增或更新主 skill，默认同步镜像到 `.claude/skills/`
- skill 建议包含：
  - 适用场景
  - 必读文档入口
  - 推荐检查顺序
  - 常见风险 / 不要做什么
- 不要把 feature 文档整段复制进 skill；skill 只保留入口、流程和判断标准。

### 3. 联调 / 回归清单

- 多接口、多事件链路或高风险改动，建议补到版本目录：
  - `docs/vX.Y.Z/checklists/<task>-review-checklist.md`
- 建议记录：
  - 请求示例
  - 关键事件顺序
  - 需要重点验证的字段
  - 测试命令

### 4. 模块可读性与 AI 导航

- 若一个目录承载稳定模块职责、存在多个源码文件，或会被 AI / 人反复进入阅读和修改，则该目录应补 `README.md`。
- 目录 `README.md` 默认应视为模块导航层，而不是可选补充说明。
- 目录 README 至少应包含：
  - 模块职责
  - 不负责什么
  - 子目录索引
  - 文件索引
  - 对外入口
  - 关键依赖
  - 维护提示
- 文件索引默认覆盖该目录下全部有效源码、测试、配置、模板和内置资源文件，而不是只列“关键文件”。
- 每条文件索引使用 1-2 句话说明“这个文件负责什么”，目标是帮助 AI 快速建立目录地图，而不是重复源码实现。
- 对生成产物、缓存、临时文件和明显无说明价值的系统文件，可以不进入文件索引。

### 4.3 文档语言与命名

- 仓库通用系统文件名保持英文不变，例如：
  - `AGENTS.md`
  - `CLAUDE.md`
  - `README.md`
  - `SKILL.md`
  - plugin manifest 等系统约定文件
- 代码目录名、代码文件名、插件目录名和 skill 目录名默认保持英文。
- 除上述通用系统文件外，团队内部功能文档、模块说明、checklist 和版本 feature 文档正文默认使用中文。
- 若文档文件名不属于通用系统文件，默认也可使用中文名称；但在同一目录内应保持命名风格一致，避免中英混杂无规则漂移。
- 文件索引描述、模块职责、维护提示等导航性文案默认使用中文，以帮助团队和 AI 一眼识别作用。

### 4.4 路径引用规则

- 仓库内文档、skill、模板和脚本说明，默认使用仓库内相对路径或逻辑路径，例如：
  - `docs/REPO_WORKFLOW_GUIDE.md`
  - `.agents/skills/issue-intake/SKILL.md`
  - `.agents/skills/gitlab-issue-create/SKILL.md`
  - `.agents/skills/gitlab-issue-close/SKILL.md`
  - `scripts/check_no_absolute_paths.py`
- 不要在仓库正文中写个人机器绝对路径，例如：
  - `<user-home>/...`
  - `<drive>:\\Users\\<name>\\...`
- 只有在用户界面响应必须生成可点击文件链接时，才允许在对话输出中使用绝对路径；这条不反向要求写入仓库文档。
- 若历史快照文档保留了旧绝对路径，应优先在“当前真相”文档中改正；历史资料是否批量清理，可单独立项处理。

### 4.1 文件级说明

- 每个核心源码文件应有文件头说明，至少说明：
  - 这个文件负责什么
  - 为什么存在
  - 主要类型 / 入口函数有哪些
  - 修改时最容易影响哪里
- 默认优先使用文件头注释，而不是为每个文件额外创建独立 `xx.md`。
- 代码说明性注释默认使用双语两行制：
  - 英文在前
  - 中文在后
  - 不接受单行混排
- 这条要求适用于：
  - 文件头说明
  - 核心类型 / 方法注释
  - 关键逻辑说明注释
- 只有在以下场景之一成立时，才建议为单个文件补独立说明文档：
  - 状态机复杂
  - 对外协议复杂
  - 编排链路长
  - 历史上容易反复改坏
  - 文件本身承担跨模块协作语义

### 4.2 稳定同步动作

- 目录 README、文件索引和文件头说明的更新，默认通过 `doc-index-sync` skill 执行。
- 当任务新增、删除、重命名文件，或明显改变模块边界时，交付前应执行 `doc-index-sync`。
- 模板默认使用：
  - `docs/templates/模块README模板.md`
  - `docs/templates/文件头注释模板.md`
  - `docs/templates/文件级说明模板.md`

## Skill 使用原则

- `.agents/skills/` 为主 skill 目录。
- `.claude/skills/` 为 Claude 轻量镜像目录。
- skill 只写流程、判断标准和入口导航。
- 共享正文只维护在 `docs/*`，skill 中统一链接，不重复抄写正文。
- 更新主 skill 后，默认同步镜像 skill，避免双体系漂移。

## 最低交付标准

每次任务完成后，至少满足以下之一：

1. 更新现有共享正文中的相关章节。
2. 新建或更新对应 `docs/features/feature-*.md`。
3. 新建或更新对应 `docs/vX.Y.Z/checklists/*-review-checklist.md`。
4. 如果结论对多个任务通用，再补一处项目级规则正文。

补充强制要求：

- 若任务已进入“可供真实使用者验证”的阶段，必须同时生成一份“给真实使用者直接执行”的测试用例文档。
- 默认落点：
  - `docs/vX.Y.Z/checklists/真实场景测试用例.md`
- 这份文档和 `review checklist` 一样，属于默认交付物，不需要使用者重复强调。

补充要求：

- 若任务归属某个版本，必须在交付前回顾版本 master plan 的具体 feat checklist：
  - 本任务相关项是否已全部完成
  - 已完成项是否已勾选并能关联证据
  - 未完成项是否已明确延期 / 取消原因

- 代码或治理发生关键变更时，应同步追加 `develop.log`。

- 若本轮任务改变了“当前代码已经具备什么”，应同步更新 `docs/当前能力总览.md`。
- 若任务归属到具体版本或 issue，交付说明中应明确写出对应编号。
- 若涉及 `git commit` 或 `git push`，必须先确认对应本仓 GitLab issue。
- 若任务属于阶段性或部分完成交付，交付前必须明确：
  - 本仓已完成什么
  - 本仓明确不做什么
  - 是否需要在分支推送、MR 创建、合并完成三个节点回当前 issue 评论同步真实进度
- 若任务新增或修改文档、skill、模板或脚本说明，交付前应执行：
  - `python3 scripts/check_no_absolute_paths.py`
  以确认没有再次引入个人机器绝对路径
- 交付前必须再确认一次提交归属：
  - 当前提交是否只承载一个本仓 issue 主语义
  - 当前提交标题是否准确描述这一笔改动，而不是使用泛化标题覆盖多类内容
- 若本轮任务新增、删除、重命名文件，或明显改变模块边界，应先执行 `doc-index-sync`，再进入 `feature-doc-skill-sync` / `repo-task-delivery`。
- 功能完成收尾时，应先执行 `feature-doc-skill-sync`，再进入 `repo-task-delivery`。
- 若本轮任务形成了独立功能能力，交付前应明确判断是否需要抽象为对应 skill；若决定暂不抽象，也应在功能文档中写明原因。
- 若任务采用独立 worktree 完成，声明任务完成前应确认：
  - 当前任务结果已合入当前主分支
  - 本轮 worktree 与过程里产生的临时目录已清理或明确保留原因

补充骨架保护要求：

- 若本轮改动触及 `docs/REPO_WORKFLOW_GUIDE.md` 定义的“不可随意变形区”，交付时必须补一段“为什么这次改动值得进入骨架”的说明。
- 若本轮改动准备合入 `master`，且触及“不可随意变形区”，必须额外执行 `master-merge-gate`，不能只依赖普通 delivery checklist。
- `master-merge-gate` 至少应检查：
  - 是否仍保持通用性
  - 是否仍保持扩展性
  - 是否把场景逻辑留在骨架之外
  - 是否已有 AI review 结论
  - 是否已有人工确认可以进入骨架主线

## 对本项目的当前约定

- 接口或事件变更，优先更新现有 `api.md`、`architecture.md`、`implementation.md`。
- 当前仓库仍处于脚手架阶段，通用规则优先沉淀到项目级共享正文。
- skill 体系以仓库流程治理为主，不把实现细节拆进 skill。
- `internal/*`、`config/`、`scripts/`、`plugins/` 这类长期维护目录，应逐步补齐目录 `README.md` 与全文件索引。
- 当前仓库的版本/issue/开发日志规则统一以 `docs/REPO_WORKFLOW_GUIDE.md` 为准。
- 版本化任务的整体计划默认放在 `docs/vX.Y.Z/plan/`。
- Claude/Codex 的过程文档默认放在 `docs/vX.Y.Z/plan/<topic>/`。
- 功能完成后，应优先收敛到 `docs/features/feature-<topic>.md`，并删除过程文档。
- 若需要保留版本快照，可在 `docs/vX.Y.Z/features/` 额外记录该版本的交付范围或快照说明，但不替代 `docs/features/`。
- 功能完成后的文档收敛、模块边界确认和 feature skill 同步，默认通过 `feature-doc-skill-sync` 执行。
- 目录导航、文件索引和文件头说明同步，默认通过 `doc-index-sync` 执行。
- 若功能已形成稳定维护入口，完成后应评估是否新增或更新对应 `.agents/skills/<topic>/SKILL.md`，并同步 `.claude/skills/<topic>/SKILL.md`。
