# parameters

## 模块职责

- `internal/model/parameters` 是 Athena 的模型参数策略中心，负责参数上下文、内部模板、单点解析和 fail-closed 校验。

## 不负责什么

- 不负责 provider / model CRUD
- 不负责密钥存储
- 不直接负责 HTTP transport
- 不负责 capability / wait / supplement 等治理语义

## 子目录索引

- 当前无子目录

## 文件索引

- `types.go`
  - 定义 `ModelPolicyContext`、`ResolvedModelParameters`、受控 override 以及相关枚举。
- `templates.go`
  - 定义 Athena 内部模型参数模板集合。
- `resolver.go`
  - 负责按固定优先级解析最终模型参数，并执行 fail-closed 校验。
- `resolver_test.go`
  - 验证模板命中、loop 阶段覆盖、specific tool 校验和拒绝原始参数直传。

## 对外入口

- `ResolveModelParameters`
- `ParseControlledOverride`
- `DefaultTemplateCatalog`

## 关键依赖

- 被 `internal/runtime`、`internal/server`、`internal/model` 消费
- 依赖上层传入 `allowed_tools`、任务场景和 loop 上下文

## 维护提示

- 参数策略只归这个目录，不要再回流到 `internal/policy`
- 若新增模板或 override 规则，先补测试再改 resolver
- 若 provider 参数映射发生变化，优先改调用方，不要在这里掺入 provider 细节
