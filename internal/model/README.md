# model

## 模块职责

- `internal/model` 负责模型供应商治理、模型端点存储、密钥处理和模型调用适配。

## 不负责什么

- 不负责业务级 prompt 编排

## 子目录索引

- `parameters/`
  - 模型参数策略中心，负责参数模板、上下文、解析器与 fail-closed 校验。

## 文件索引

- `chat_model.go`
  - 定义聊天模型相关适配和运行时调用入口。
- `governance.go`
  - 承载 provider/model 治理相关逻辑。
- `memory_store.go`
  - 提供内存态模型存储实现。
- `memory_store_test.go`
  - 验证内存态模型存储行为。
- `postgres_integration_test.go`
  - 验证 PostgreSQL 模型存储集成行为。
- `postgres_store.go`
  - 提供 PostgreSQL 模型存储实现。
- `provider.go`
  - 定义 provider 抽象和供应商层入口。
- `secret.go`
  - 处理模型密钥加解密与敏感信息边界。
- `secret_test.go`
  - 验证密钥处理行为。

## 对外入口

- provider/model CRUD
- model store
- secret handling

## 关键依赖

- 被 `app/`、`runtime/`、`entry/` 消费

## 维护提示

- provider/model 结构变化时，需同步检查 API、存储和密钥边界。
