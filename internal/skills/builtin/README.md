# builtin

## 模块职责

- 保存内置 skill 定义资源。

## 不负责什么

- 不负责 loader 或治理逻辑

## 子目录索引

- 当前无子目录

## 文件索引

- `cso_review.json`
  - 内置的 `cso_review` 安全分析 skill 定义资源。
- `user_overview.json`
  - 当前内置的 `user_overview` skill 定义资源。

## 对外入口

- 被 `skills/loader` 和内置 source 读取

## 维护提示

- 新增内置 skill 时，注意名称冲突和运行时可见性规则。
