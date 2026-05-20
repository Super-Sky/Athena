// types.go defines knowledge retrieval, candidate, and counter contracts used by Athena.
// types.go 定义 Athena 使用的知识检索、候选和计数契约。
package knowledge

// KnowledgeBlobRef captures one rebuildable compiled knowledge blob reference owned by Athena runtime.
// KnowledgeBlobRef 描述一条由 Athena runtime 持有、可重建的知识编译结果引用。
type KnowledgeBlobRef struct {
	BlobRefID      string `json:"blob_ref_id,omitempty"`
	SourceRef      string `json:"source_ref,omitempty"`
	SourceVersion  string `json:"source_version,omitempty"`
	PayloadHash    string `json:"payload_hash,omitempty"`
	CompiledAt     string `json:"compiled_at,omitempty"`
	TokenBudgetKey string `json:"token_budget_key,omitempty"`
}

// WikiNode captures one runtime-owned knowledge node derived from compiled source material.
// WikiNode 描述一条由编译后知识主源衍生出来、由 runtime 持有的知识节点。
type WikiNode struct {
	NodeID       string   `json:"node_id,omitempty"`
	BlobRefID    string   `json:"blob_ref_id,omitempty"`
	Title        string   `json:"title,omitempty"`
	Summary      string   `json:"summary,omitempty"`
	ParentNodeID string   `json:"parent_node_id,omitempty"`
	ChildNodeIDs []string `json:"child_node_ids,omitempty"`
	Leaf         bool     `json:"leaf,omitempty"`
}

// WikiPathSummary captures one rebuildable path-level summary assembled from runtime wiki nodes.
// WikiPathSummary 描述一条由 runtime wiki 节点组装得到、可重建的路径级摘要。
type WikiPathSummary struct {
	PathID    string   `json:"path_id,omitempty"`
	NodeIDs   []string `json:"node_ids,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Title     string   `json:"title,omitempty"`
	SourceRef string   `json:"source_ref,omitempty"`
}

// RetrievalView captures one platform-consumable retrieval projection built from runtime execution assets.
// RetrievalView 描述一条由 runtime 执行面构建的平台可消费检索视图。
type RetrievalView struct {
	ViewID         string   `json:"view_id,omitempty"`
	Query          string   `json:"query,omitempty"`
	MatchedNodeIDs []string `json:"matched_node_ids,omitempty"`
	MatchedPathIDs []string `json:"matched_path_ids,omitempty"`
	Summary        string   `json:"summary,omitempty"`
}

// RetrievalHit captures one knowledge retrieval result returned to runtime consumers.
// RetrievalHit 描述返回给 runtime 消费侧的单条知识检索命中结果。
type RetrievalHit struct {
	EntryID   string   `json:"entry_id,omitempty"`
	Title     string   `json:"title,omitempty"`
	Summary   string   `json:"summary,omitempty"`
	Labels    []string `json:"labels,omitempty"`
	SourceRef string   `json:"source_ref,omitempty"`
}

// Candidate captures one knowledge-derived candidate update emitted by Athena.
// Candidate 描述 Athena 发出的一条知识衍生候选更新。
type Candidate struct {
	CandidateID string         `json:"candidate_id,omitempty"`
	Kind        string         `json:"kind,omitempty"`
	Title       string         `json:"title,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// Counters captures knowledge retrieval runtime counters that can be rebuilt from runtime events.
// Counters 描述可从 runtime 事件重建的知识检索运行时计数。
type Counters struct {
	AdoptionCount int `json:"adoption_count,omitempty"`
	SuccessCount  int `json:"success_count,omitempty"`
}

// BuildCandidate returns a minimal knowledge-candidate update contract from summary inputs.
// BuildCandidate 会根据摘要输入构建最小知识候选更新契约。
func BuildCandidate(kind, title, summary string, payload map[string]any) Candidate {
	return Candidate{
		CandidateID: "candidate-summary",
		Kind:        kind,
		Title:       title,
		Summary:     summary,
		Payload:     payload,
	}
}
