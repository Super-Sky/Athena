# plugins

## 模块职责

- 保存仓库内置的 repo-local Codex plugins。

## 不负责什么

- 不负责 `.agents/skills/` 主技能规则正文
- 不负责 Athena 运行时代码

## 子目录索引

- `send-to-claude/`
  - 提供将 handoff 发送给 Claude 的本地插件定义。
- `send-to-codex/`
  - 提供将 handoff 发送给 Codex 的本地插件定义。

## 文件索引

- 当前以子目录为主，顶层不直接承载插件源码文件。

## 对外入口

- 各插件目录中的 `.codex-plugin/plugin.json`

## 关键依赖

- 与 `.agents/skills/send_to_*` 以及 handoff workflow 配套使用

## 维护提示

- 插件目录结构变化时，应同步更新这里和对应插件子目录的 README。
