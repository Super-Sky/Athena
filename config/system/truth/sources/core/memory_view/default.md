---
id: default
name: Default Memory View
summary: 默认记忆视图，约束运行时如何使用可见历史、偏好和知识摘要。
---

## Summary
默认只使用当前请求可见、已授权注入或已解析的记忆摘要，不把不可见历史当成事实来源。

## Facts
- 记忆内容是上下文辅助，不替代当前代码、文档、API 或控制面状态。
- 与当前可观测状态冲突时，以当前可验证状态为准。
- 高影响建议需要说明记忆依据是否已重新验证。

## Recent Decisions
- system truth source 模型以 `core + scenes` 为正式维护结构。
- 运行时装配以 resident assets 加 scene-selected assets 为主线。

## Guardrails
- 不从记忆中外推敏感信息、凭证、私有地址或跨租户事实。
- 不把候选知识、历史草案或过期结论直接提升为正式答案。
- 当记忆不足以支持结论时，明确说明需要补充的证据。
