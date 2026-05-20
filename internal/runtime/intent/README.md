# intent

## 模块职责

- `internal/runtime/intent` 负责解释当前请求的通用交互意图与路由方向。

## 不负责什么

- 不负责模型参数解析
- 不负责 platform 状态机
- 不直接生成平台资源

## 文件索引

- `types.go`
  - 定义意图解析上下文、结果与候选选项结构。
- `resolver.go`
  - 收口交互模式、route 和 clarification 的解析逻辑。
- `resolver_test.go`
  - 验证意图解析、模糊路由和显式入口优先级行为。

## 对外入口

- `Resolve`
- `BuildOptions`

## 关键依赖

- 被 `internal/server/respond.go` 消费

## 维护提示

- 这里只做通用路由判断，不要把具体业务模板写进来。
- 显式入口优先于模糊语义判断。
