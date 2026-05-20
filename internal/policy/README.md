# policy

## 模块职责

- `internal/policy` 承载能力治理和策略判断的基础抽象。

## 不负责什么

- 不负责 transport 层协议映射

## 子目录索引

- 当前无子目录

## 文件索引

- `policy.go`
  - 定义策略相关基础结构和判断入口。

## 对外入口

- policy 抽象

## 关键依赖

- 被 `runtime/`、`app/`、`skills/` 间接消费

## 维护提示

- 引入新的治理判断时，先检查是否应作为 policy 抽象进入这里。
