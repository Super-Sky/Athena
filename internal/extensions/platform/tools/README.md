# tools

## 模块职责

- `internal/extensions/platform/tools` 负责承载 platform 提供的结构化 tool contract 与调用提示适配。

## 不负责什么

- 不直接执行 platform tool
- 不持有 platform 主数据
- 不把具体业务动作硬编码进 Athena 主线

## 文件索引

- `descriptor.go`
  - 定义平台 tool descriptor、参数 schema 和调用提示结构。
- `descriptor_test.go`
  - 验证 descriptor 和调用提示输出的稳定性。

## 对外入口

- `DefaultDescriptors`
- `BuildAutomationHints`

## 维护提示

- 这里只输出结构化 contract 和 hints，不输出自然语言给 platform 反解析。
