// store.go defines the unified runtime-store boundary for counters, event records, and rebuildable indexes.
// store.go 定义统一的 runtime store 边界，用于承载计数、事件记录和可重建索引。
package store

import (
	"context"

	"moss/internal/knowledge"
)

// EventRecord captures one rebuildable runtime event stored for observability or later derivation.
// EventRecord 描述一条为可观测或后续派生而保存的可重建 runtime 事件记录。
type EventRecord struct {
	EventID    string         `json:"event_id,omitempty"`
	EventType  string         `json:"event_type,omitempty"`
	SubjectID  string         `json:"subject_id,omitempty"`
	OccurredAt string         `json:"occurred_at,omitempty"`
	Detail     map[string]any `json:"detail,omitempty"`
}

// CounterSnapshot captures one named runtime counter value.
// CounterSnapshot 描述一条具名 runtime 计数值快照。
type CounterSnapshot struct {
	CounterName string `json:"counter_name,omitempty"`
	SubjectID   string `json:"subject_id,omitempty"`
	Value       int64  `json:"value,omitempty"`
}

// KnowledgeCompileCacheRecord captures one rebuildable knowledge compile-cache snapshot.
// KnowledgeCompileCacheRecord 描述一条可重建的知识编译缓存快照。
type KnowledgeCompileCacheRecord struct {
	BlobRef       knowledge.KnowledgeBlobRef  `json:"blob_ref,omitempty"`
	Nodes         []knowledge.WikiNode        `json:"nodes,omitempty"`
	PathSummaries []knowledge.WikiPathSummary `json:"path_summaries,omitempty"`
}

// RetrievalViewRecord captures one runtime-owned retrieval view projection.
// RetrievalViewRecord 描述一条由 Athena runtime 持有的检索视图投影。
type RetrievalViewRecord struct {
	View knowledge.RetrievalView `json:"view,omitempty"`
}

// Store is the unified runtime-store abstraction for rebuildable state owned by Athena.
// Store 是 Athena 自有可重建运行时状态的统一存储抽象。
type Store interface {
	AppendEvent(context.Context, EventRecord) error
	UpsertCounter(context.Context, CounterSnapshot) error
	UpsertKnowledgeCompileCache(context.Context, KnowledgeCompileCacheRecord) error
	UpsertRetrievalView(context.Context, RetrievalViewRecord) error
}
