// pipeline.go defines the structured retrieval pipeline contracts used to make RAG execution more explainable.
// pipeline.go 定义结构化检索管线契约，用于让 RAG 执行过程更可解释。
package knowledge

import (
	"fmt"
	"sort"
	"strings"
)

// QueryRewrite captures the normalized retrieval query derived from the original user query.
// QueryRewrite 描述从原始用户问题派生出的归一化检索查询。
type QueryRewrite struct {
	OriginalQuery   string `json:"original_query,omitempty"`
	RewrittenQuery  string `json:"rewritten_query,omitempty"`
	RewriteStrategy string `json:"rewrite_strategy,omitempty"`
}

// RecallPlan captures the bounded recall configuration for one retrieval attempt.
// RecallPlan 描述一次检索尝试的受限召回配置。
type RecallPlan struct {
	Mode       string   `json:"mode,omitempty"`
	RequestedK int      `json:"requested_k,omitempty"`
	Labels     []string `json:"labels,omitempty"`
}

// RerankPlan captures the rerank configuration used after recall.
// RerankPlan 描述召回之后使用的重排配置。
type RerankPlan struct {
	Enabled bool `json:"enabled,omitempty"`
	TopN    int  `json:"top_n,omitempty"`
}

// ContextPackPlan captures the final context packing budget used before model generation.
// ContextPackPlan 描述模型生成前最终上下文拼装所使用的预算。
type ContextPackPlan struct {
	TokenBudget   int      `json:"token_budget,omitempty"`
	SelectedIDs   []string `json:"selected_ids,omitempty"`
	DroppedIDs    []string `json:"dropped_ids,omitempty"`
	SelectedCount int      `json:"selected_count,omitempty"`
}

// RetrievalPipeline captures the structured retrieval pipeline outcome Athena can surface to platform.
// RetrievalPipeline 描述 Athena 可返回给 platform 的结构化检索管线结果。
type RetrievalPipeline struct {
	QueryRewrite QueryRewrite    `json:"query_rewrite,omitempty"`
	RecallPlan   RecallPlan      `json:"recall_plan,omitempty"`
	RerankPlan   RerankPlan      `json:"rerank_plan,omitempty"`
	ContextPack  ContextPackPlan `json:"context_pack,omitempty"`
	Summary      string          `json:"summary,omitempty"`
}

// BuildRetrievalPipeline returns a minimal structured retrieval pipeline from existing hits and budgets.
// BuildRetrievalPipeline 会根据已有命中和预算构建最小结构化检索管线结果。
func BuildRetrievalPipeline(query string, hits []RetrievalHit, tokenBudget int) *RetrievalPipeline {
	if strings.TrimSpace(query) == "" {
		return nil
	}
	if tokenBudget <= 0 {
		tokenBudget = 1800
	}
	ids := make([]string, 0, len(hits))
	for _, hit := range hits {
		if strings.TrimSpace(hit.EntryID) != "" {
			ids = append(ids, strings.TrimSpace(hit.EntryID))
		}
	}
	sort.Strings(ids)
	selected := append([]string(nil), ids...)
	return &RetrievalPipeline{
		QueryRewrite: QueryRewrite{
			OriginalQuery:   strings.TrimSpace(query),
			RewrittenQuery:  strings.TrimSpace(query),
			RewriteStrategy: "identity",
		},
		RecallPlan: RecallPlan{
			Mode:       "hybrid_candidate",
			RequestedK: len(hits),
		},
		RerankPlan: RerankPlan{
			Enabled: len(hits) > 1,
			TopN:    len(hits),
		},
		ContextPack: ContextPackPlan{
			TokenBudget:   tokenBudget,
			SelectedIDs:   selected,
			SelectedCount: len(selected),
		},
		Summary: fmt.Sprintf("retrieval pipeline prepared %d candidate hits with token budget %d", len(selected), tokenBudget),
	}
}
