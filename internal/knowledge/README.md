# knowledge

## 模块职责

- `internal/knowledge` 负责知识检索命中、候选更新和运行时计数契约。
- 上游当前把这组能力命名为 `PM5`，但 Athena 内部不把它作为长期骨架概念继续扩散。

## 文件索引

- `pipeline.go`
  - 定义 query rewrite、recall、rerank 和 context pack 的结构化检索管线契约。
- `pipeline_test.go`
  - 验证检索管线结构输出行为。
- `types.go`
  - 定义知识检索命中、候选和计数结构。
- `types_test.go`
  - 验证知识结构最小行为。
