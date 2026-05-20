# tools

## 模块职责

- `internal/tools` 提供 tool registry、tool 元数据和 tool middleware。

## 不负责什么

- 不负责高层 skill 路由

## 子目录索引

- 当前无子目录

## 文件索引

- `demo.go`
  - 提供示例或演示性质的 tool 定义。
- `middleware.go`
  - 定义 tool middleware 能力。
- `registry.go`
  - 提供 tool registry 入口、稳定元数据和控制面可消费的定义列表。

## 对外入口

- tool registry
- tool middleware

## 关键依赖

- 被 `runtime/`、`app/` 间接消费

## 维护提示

- 新 tool 加入时，优先看 registry 和 middleware 是否需要同步调整。
- 若 tool 元数据被控制面消费，应同步维护 scope、side effect、confirmation 和 schema summary 字段。
