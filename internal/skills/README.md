# skills

## 模块职责

- `internal/skills` 负责 skill registry、package、source、loader 和治理链。

## 不负责什么

- 不负责 tool registry
- 不负责业务编排层

## 子目录索引

- `builtin/`
  - 内置 skill 定义资源。

## 文件索引

- `adapter.go`
  - 把 skill 定义转换为运行时更易消费的视图。
- `loader.go`
  - 统一 skill 加载链入口。
- `loader_test.go`
  - 验证 loader 行为。
- `package.go`
  - 定义 uploaded skill package 及其治理相关能力。
- `package_source.go`
  - 处理 package source 侧加载逻辑。
- `package_source_test.go`
  - 验证 package source 行为。
- `package_test.go`
  - 验证 package 治理逻辑。
- `registry.go`
  - 维护 skill registry 基础能力。
- `source.go`
  - 定义 skill source 抽象。
- `source_test.go`
  - 验证 source 抽象行为。
- `store.go`
  - 定义 skill store 抽象。
- `validation.go`
  - 处理 uploaded skill 校验逻辑。
- `validation_test.go`
  - 验证 uploaded skill 校验行为。

## 对外入口

- loader
- registry
- source/store abstractions

## 关键依赖

- 被 `app/` 和 `runtime/` 消费

## 维护提示

- 新 skill 来源或 package 生命周期变化时，优先检查这里的抽象边界是否仍成立。
- 新增 builtin skill 时，注意是否会影响默认 skill 选择；必要时应同步调整 scene / runtime 默认路由。
