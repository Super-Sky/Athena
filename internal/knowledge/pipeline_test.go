// pipeline_test.go verifies retrieval pipeline output stays structured and stable.
// pipeline_test.go 用于验证检索管线输出保持结构化且稳定。
package knowledge

import "testing"

// TestBuildRetrievalPipelineReturnsStructuredPlan verifies query rewrite, recall, rerank, and context pack stay visible.
// TestBuildRetrievalPipelineReturnsStructuredPlan 用于验证 query rewrite、recall、rerank 与 context pack 都能显式暴露。
func TestBuildRetrievalPipelineReturnsStructuredPlan(t *testing.T) {
	pipeline := BuildRetrievalPipeline("where is my order", []RetrievalHit{
		{EntryID: "doc-2"},
		{EntryID: "doc-1"},
	}, 1024)
	if pipeline == nil {
		t.Fatalf("BuildRetrievalPipeline() = nil")
	}
	if pipeline.QueryRewrite.RewrittenQuery != "where is my order" {
		t.Fatalf("rewritten_query = %q", pipeline.QueryRewrite.RewrittenQuery)
	}
	if pipeline.ContextPack.TokenBudget != 1024 {
		t.Fatalf("token_budget = %d", pipeline.ContextPack.TokenBudget)
	}
	if len(pipeline.ContextPack.SelectedIDs) != 2 || pipeline.ContextPack.SelectedIDs[0] != "doc-1" {
		t.Fatalf("selected_ids = %#v", pipeline.ContextPack.SelectedIDs)
	}
}
