# runtimeassets

## 模块职责

- `internal/runtimeassets` 管理运行时任务资产和可见 skill 元数据 registry。
- 当前仅作为 `/api/runtime/scenario/respond` 的 legacy scenario compatibility island，承接既有 mosi/OpenClaw 兼容资产。
- 新的 core runtime truth、Eino Graph runtime foundation 或 system truth 资产不得继续加到这里。

## 不负责什么

- 不负责具体业务执行
- 不负责 HTTP 层协议映射

## 子目录索引

- `builtin/`
  - 内置 task bundle 与 runtime skill 元数据静态资源目录。

## 文件索引

- `assets.go`
  - 加载内置 task bundle 和 runtime skill metadata，并提供 registry 查询能力。
- `assets_test.go`
  - 验证 registry 加载、过滤和 allowlist 行为。
- `types.go`
  - 定义 task asset、skill metadata 和 filter 的核心结构。

## 对外入口

- `NewRegistry`
- `SelectTaskBundle`
- `ListSkills`
- `ResolveVisibleSkills`

## 关键依赖

- 被 `app/runtime_scenarios.go` 直接消费

## 维护提示

- 当前阶段不要新增任务子类型；如必须新增，应先把该包移动或重命名到 legacy/platform extension 边界。
- 后续 system truth phase 接管资产面后，再决定是否迁移这些兼容资产。
