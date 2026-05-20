# controlplane

## 模块职责

- `internal/controlplane` 负责 Athena 独立控制面的文件化 override、版本快照、有效配置视图和读写管理逻辑。

## 不负责什么

- 不负责前端页面渲染
- 不负责平台主数据和业务状态机
- 不负责直接修改核心结构化 contract

## 子目录索引

- 当前无子目录

## 文件索引

- `types.go`
  - 定义 scene、skill、tool、governance、control-plane auth、system resources、配置版本和 bootstrap payload 的控制面契约。
- `store.go`
  - 定义 JSON 文件化 store，负责加载和写回控制面 override 及版本快照。
- `manager.go`
  - 基于内置 scene/skill/tool 定义和持久化 override 计算有效控制面视图，并规范 bootstrap payload 的稳定空数组返回。
- `auth.go`
  - 实现控制面的最小 token 登录、cookie session 与 IP 锁定状态。
- `auth_store.go`
  - 抽象控制面认证状态持久化；共享 PostgreSQL 已配置时落数据库表，本地无库环境回退文件态。
- `system_resources.go`
  - 管理 active truth dir 中的 file-backed system resources，以及 `sources/**/*.md` 主源同步、source 保存后自动 `parse / compile / activate`、typed compile、versions / audit / rollback、debug payload / export 流水线。
- `tool_governance.go`
  - 编译并聚合 `tool_governance_policy` system truth，提供 effective policy、validation tool request 决策和 persisted decision log。
- `manager_test.go`
  - 验证场景、skill、tool、governance 与版本回滚的合成行为。
- `system_resources_test.go`
  - 验证 system resource 的创建、自动 pipeline、versions / audit / rollback、debug payload 和 truth dir 导出。
- `auth_test.go`
  - 验证认证锁定状态持久化以及“人工释放锁定后恢复登录”的主链行为。

## 对外入口

- `NewManager`
- `NewFileStore`
- `ListVersions / LoadVersion / SaveVersion`
- `SaveSystemResourceSource / PutSystemResourceSource`
- `CompileSystemResource / ActivateSystemResource`
- `ListSystemResourceVersions / GetSystemResourceVersion / RollbackSystemResourceVersion`
- `ListSystemResourceAuditEntries`
- `EffectiveToolGovernancePolicy / EvaluateToolGovernance / ListToolGovernanceDecisions`
- `Login / Logout / AuthStatus`

## 关键依赖

- 依赖 `internal/runtime/scene`
- 依赖 `internal/skills`
- 依赖 `internal/tools`
- 依赖 `config/system/truth/` 作为 Git baseline 目录
- 依赖 `config/system/truth/sources/` 作为 system markdown 主源目录

## 维护提示

- 这里只开放白名单内的可调项，不接受把模型原始参数或治理底线直接放进控制面。
- `runtime` 当前作为 `governance` 的兼容别名保留，后续消费方应优先切到 `governance`。
- control-plane 修改的是当前实例的 active truth dir，不直接写 Git baseline。
- `sources/` 下的 Markdown 是人维护的主源；其余 JSON 文件由系统生成和维护。
- 修改控制面契约时，需同步检查 `docs/api.md`、`docs/architecture.md`、`docs/implementation.md` 和 `web/` 前端。
