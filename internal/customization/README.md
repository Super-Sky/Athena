# customization

## 模块职责

- `internal/customization` 保存请求级 customization 的基础数据结构和解析入口。

## 不负责什么

- 不负责主编排逻辑

## 子目录索引

- 当前无子目录

## 文件索引

- `customization.go`
  - 定义 customization 的核心结构和基础处理逻辑。

## 对外入口

- customization 结构定义

## 关键依赖

- 被 `app/` 和 `runtime/` 间接消费

## 维护提示

- customization 字段变化时，需同步检查 API 请求体和 runtime 解析逻辑。
