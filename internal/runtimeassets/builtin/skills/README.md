# skills

## 模块职责

- 保存内置 runtime skill metadata JSON 资源。

## 不负责什么

- 不负责具体 skill 执行逻辑

## 子目录索引

- 当前无子目录

## 文件索引

- `mosi_audit_operator.json`
  - 定义 `mosi` 审计操作相关 runtime skill 元数据。
- `mosi_email_sender.json`
  - 定义 `mosi` 邮件发送相关 runtime skill 元数据。

## 对外入口

- 被 `runtimeassets` registry 作为内置 skill metadata 读取

## 维护提示

- 新增 skill metadata 时，确保 task type、subtype 和 output mode 白名单正确。
