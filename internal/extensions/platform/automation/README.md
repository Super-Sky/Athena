# automation

## 模块职责

- `internal/extensions/platform/automation` 负责 platform 自动化草案相关的结构化增强输出。

## 不负责什么

- 不持有自动化主数据
- 不执行创建自动化动作
- 不负责 platform 状态机

## 文件索引

- `payload.go`
  - 构建 platform 可直接消费的自动化创建 payload。
- `routing.go`
  - 收敛 platform 专项的自动化草案路由判断、`choice_required` 增强和非流式规划进度生成。
- `payload_test.go`
  - 验证创建 payload 与 planning progress 描述的稳定结构。
- `routing_test.go`
  - 验证 platform 专项自动化路由增强与二选一样本命中保持稳定。

## 对外入口

- `BuildCreatePayload`
- `BuildPlanningProgressSteps`

## 维护提示

- 这里输出的对象必须以结构化字段为主，不能依赖自然语言被 platform 反解析。
