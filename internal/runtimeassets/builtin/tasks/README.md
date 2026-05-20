# tasks

## 模块职责

- 保存内置 task bundle JSON 资源。

## 不负责什么

- 不负责 runtime judgment 主链

## 子目录索引

- 当前无子目录

## 文件索引

- `openclaw_before_tool_call.json`
  - 定义 OpenClaw 工具调用前场景的 task bundle。
- `openclaw_message_sending.json`
  - 定义 OpenClaw 消息发送场景的 task bundle。
- `openclaw_runtime_explanation.json`
  - 定义 OpenClaw runtime explanation 场景的 task bundle。

## 对外入口

- 被 `runtimeassets` registry 作为 task bundle 读取

## 维护提示

- 新增 task bundle 时，需同步检查 API、feature 文档和测试。
