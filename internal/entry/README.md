# entry

## 模块职责

- `internal/entry` 负责进程启动、依赖装配和命令入口接线。

## 不负责什么

- 不负责具体业务编排
- 不负责 HTTP 协议实现

## 子目录索引

- 当前无子目录

## 文件索引

- `entry.go`
  - 负责创建 app、server、store、model 等根依赖并暴露启动入口。
- `entry_test.go`
  - 验证根依赖装配和启动相关行为。
- `postgres_logger.go`
  - 提供 PostgreSQL 相关日志适配和辅助能力。

## 对外入口

- 进程命令装配
- 服务启动依赖创建

## 关键依赖

- 依赖 `config/`、`app/`、`server/`、`session/`、`model/`

## 维护提示

- 新系统级依赖优先接入这里，不要把装配逻辑散落到业务模块。
