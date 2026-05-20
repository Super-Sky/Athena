# scripts

## 文件索引

- `post_gitlab_issue_note.py`
  - 生成并可选地回写 GitLab issue 进度评论，覆盖 `push`、`mr`、`merge` 三类事件。
- `test_post_gitlab_issue_note.py`
  - 验证 issue 进度评论脚本的引用解析、条目拆分和评论渲染逻辑。
