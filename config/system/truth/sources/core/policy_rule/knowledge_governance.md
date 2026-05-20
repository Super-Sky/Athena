---
id: knowledge_governance
name: Knowledge Governance
summary: 跨场景知识复用、脱敏、共享与失效治理规则。
severity: high
checkpoints:
  - pre_candidate_update
  - pre_finalize
on_fail: deny
---

## Purpose
定义知识对象如何被提炼、共享、脱敏、复用和失效。

## Scope
适用于 knowledge scene、候选更新、历史知识复用与跨租户边界。

## Hard Gates
- tenant_only 知识不得跨租户复用。
- 未完成脱敏的知识不得提升为 public。
- 含有原始 PII、凭证、系统地址或商业机密的内容不得入知识对象。

## Check Rules
- 知识对象必须带适用条件与分享级别。
- 过期知识默认降级或停用，不得继续无提示复用。
- public 级知识必须可证明无法反推来源主体。

## On Failure
- deny: 拒绝发布或拒绝复用。
- ask: 要求补充适用条件、脱敏说明或分享级别。

## Guidance
- 默认 tenant_only，只有在明确满足公共复用条件时才提升分享级别。
- 失败案例和反模式同样有价值，但必须经过脱敏与边界检查。

## Examples
- 单个企业的特定系统配置不能直接沉淀为 public playbook。

## References
- contract.knowledge.knowledge_object
- contract.knowledge.knowledge_candidate
