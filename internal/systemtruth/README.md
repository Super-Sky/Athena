# systemtruth

## 模块职责

- `internal/systemtruth/` 负责 Athena 仓库内单一 system truth 主源的基础解析能力。
- 它为 `config/system/truth/sources/` 提供统一的 Markdown frontmatter、section 与 YAML 读取辅助。

## 不负责什么

- 不负责 control-plane 资源编排
- 不负责 runtime context 装配
- 不负责 workflow 执行
- 不负责 truth dir 的 parse / compile / activate 生命周期管理

## 子目录索引

- 当前无子目录

## 文件索引

- `source.go`
  - 提供单一真相主源的路径解析、ID 归一化、Markdown frontmatter 解析、section 提取和 YAML 读取辅助。

## 对外入口

- `DefaultTruthDir`
- `SourcesRoot`
- `NormalizeID`
- `ReadMarkdownDocument`
- `ParseMarkdownSections`
- `ReadYAMLMap`

## 关键依赖

- 依赖标准库文件系统与路径处理能力
- 依赖 `gopkg.in/yaml.v3` 解析 YAML / frontmatter
- 被 `internal/controlplane`、`internal/runtime/scene`、`internal/workflow`、`internal/skills` 等模块消费

## 常见修改路径

- 新增 source 文件格式规则时，通常改 `source.go`
- truth 根目录解析规则变化时，通常改 `DefaultTruthDir`
- Markdown 或 YAML 解析口径变化时，需要同步看 `internal/controlplane/system_resources.go` 和相关测试

## 维护提示

- 这里是单一真相主源解析的共享底座，改动会同时影响 control-plane、runtime、scene、workflow 和 skill 装配。
- 解析规则一旦变化，优先补测试，再跑全量 `go test ./...` 与 truth 同步脚本验证。
