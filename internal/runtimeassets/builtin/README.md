# builtin

## 模块职责

- 保存内置 runtime task bundle 和 runtime skill metadata 静态资源。
- 当前资源是 mosi/OpenClaw legacy scenario compatibility 数据，不是 Athena core runtime truth。

## 不负责什么

- 不负责 registry 加载逻辑，加载逻辑在上层 `runtimeassets/`。

## 子目录索引

- `skills/`
  - 内置 runtime skill metadata JSON。
- `tasks/`
  - 内置 task bundle JSON。

## 文件索引

- 当前以子目录资源为主，顶层不直接承载资源文件。

## 对外入口

- 被 `go:embed` 读取

## 维护提示

- 当前阶段不要在这里新增资源；新增 scenario / system truth 资产应先走 v2.0.0 架构边界确认。
