# github-issue-progress-sync

## 作用域

本 skill 用于在 GitHub issue 下同步当前仓的进度评论，覆盖：

- 分支推送远端后
- PR 创建后
- 合并完成后

它不负责：

- 读取 issue 正文
- 决定任务范围
- 替代 commit footer 中的 `Refs` 或 `Closes`

这些仍由：

- `issue-intake`
- 仓库工作流文档
- PR 模板

共同负责。

## 内容索引

- `README.md`
  - 本目录说明
- `SKILL.md`
  - skill 使用规则
- `scripts/`
  - issue 进度评论脚本
