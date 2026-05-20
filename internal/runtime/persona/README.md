# persona

## 模块职责

- `internal/runtime/persona` 负责解析用户级 `persona_context`，并把它收敛成 runtime 可消费的表达风格上下文。

## 不负责什么

- 不负责 App 专家模式的 `app_context.persona`
- 不负责 persona 主数据持久化
- 不负责让 persona 影响事实判断、证据门槛或风险标准

## 文件索引

- `context.go`
  - 解析 `global_context.persona_context`，并生成 guidance 与日志锚点所需的最小结构。
- `context_test.go`
  - 验证 persona context 解析、风格规则裁剪和 guidance 生成行为。

## 对外入口

- `BuildContext`

## 关键依赖

- 被 `internal/runtime/service.go` 消费

## 维护提示

- 这里的 persona 语义是“用户表达风格偏好”，不要和 App 场景 persona 混用。
- 任何新增字段都应继续保持“影响表达、不影响事实”的边界。
