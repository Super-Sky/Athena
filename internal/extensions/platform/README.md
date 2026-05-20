# platform

## 模块职责

- `internal/extensions/platform` 负责承载 platform 专项增强的可插拔实现。

## 不负责什么

- 不负责 Athena 主线骨架
- 不持有 platform 主数据真源
- 不直接替代 runtime / server / policy

## 子目录索引

- `automation/`
  - 自动化创建 payload 和 planning progress 等平台专项增强。
- `context/`
  - platform 上下文 catalog、summary 与 detail 请求增强。
- `tools/`
  - platform 结构化 tool contract 与调用提示增强。

## 维护提示

- 这层必须可删除、可替换、可关闭。
- 删除这层后，Athena 主线服务仍应可以启动并处理通用请求。
