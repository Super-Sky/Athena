# config

## 模块职责

- 管理 Athena 的默认配置文件和测试配置文件。

## 不负责什么

- 不负责配置解析逻辑，解析逻辑在 `internal/config/`

## 子目录索引

- 当前无子目录

## 文件索引

- `config.yml`
  - 默认运行配置，提供本地开发和常规启动所需的配置基线。
- `config.test.yml`
  - 测试环境使用的配置覆盖文件，用于测试场景下的配置隔离。

## 对外入口

- 由 `internal/config` 读取和合并

## 关键依赖

- 被 `main.go` 和 `internal/entry` 间接消费

## 维护提示

- 新增配置项时，应同时检查 `internal/config/config.go` 和测试配置是否需要同步。
