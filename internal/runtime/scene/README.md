# scene

## 模块职责

- `internal/runtime/scene` 负责场景命中、场景切换建议和相关内部契约。

## 文件索引

- `catalog.go`
  - 定义内置 scene catalog，并为控制面 override 提供稳定目录形态。
- `context.go`
  - 负责抽取 app 场景上下文并生成 guide questions 候选。
- `context_test.go`
  - 验证场景上下文抽取和 guide questions 解析行为。
- `matcher.go`
  - 实现基于 scene catalog 的严格场景命中逻辑。
- `scene.go`
  - 定义场景评分分段、命中强度和切换建议结构。
- `scene_test.go`
  - 验证场景评分阈值、显式 task 优先级和切换建议行为。
