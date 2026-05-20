package knowledge

import "testing"

// TestCountersStructStoresCounts verifies knowledge counters preserve adoption and success counts.
// TestCountersStructStoresCounts 用于验证知识计数结构会保留 adoption 与 success 计数。
func TestCountersStructStoresCounts(t *testing.T) {
	counters := Counters{AdoptionCount: 2, SuccessCount: 1}
	if counters.AdoptionCount != 2 || counters.SuccessCount != 1 {
		t.Fatalf("Counters = %#v", counters)
	}
}

// TestBuildCandidateCreatesCandidate verifies BuildCandidate creates one knowledge candidate.
// TestBuildCandidateCreatesCandidate 用于验证 BuildCandidate 会创建知识候选对象。
func TestBuildCandidateCreatesCandidate(t *testing.T) {
	candidate := BuildCandidate("knowledge", "title", "summary", nil)
	if candidate.Kind != "knowledge" || candidate.Title != "title" {
		t.Fatalf("candidate = %#v", candidate)
	}
}

// TestExecutionSurfaceTypesPreserveCoreFields verifies the canonical knowledge execution-surface objects keep their identifiers.
// TestExecutionSurfaceTypesPreserveCoreFields 用于验证标准知识执行面对象会保留各自的核心标识字段。
func TestExecutionSurfaceTypesPreserveCoreFields(t *testing.T) {
	blob := KnowledgeBlobRef{BlobRefID: "blob-1", SourceRef: "wiki/page-a"}
	node := WikiNode{NodeID: "node-1", BlobRefID: "blob-1"}
	path := WikiPathSummary{PathID: "path-1", NodeIDs: []string{"node-1"}}
	view := RetrievalView{ViewID: "view-1", MatchedNodeIDs: []string{"node-1"}}
	if blob.BlobRefID != "blob-1" || node.NodeID != "node-1" || path.PathID != "path-1" || view.ViewID != "view-1" {
		t.Fatalf("unexpected execution surface values: %#v %#v %#v %#v", blob, node, path, view)
	}
}
