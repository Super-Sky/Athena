package store

import (
	"testing"

	"moss/internal/knowledge"
)

// TestCounterSnapshotStoresValue verifies the counter snapshot contract preserves numeric values.
// TestCounterSnapshotStoresValue 用于验证计数快照契约会保留数值。
func TestCounterSnapshotStoresValue(t *testing.T) {
	counter := CounterSnapshot{CounterName: "knowledge.adoption", SubjectID: "entry-1", Value: 2}
	if counter.Value != 2 {
		t.Fatalf("Value = %d, want 2", counter.Value)
	}
}

// TestKnowledgeStoreRecordsExposeExplicitExecutionSurface verifies runtime store records carry the canonical knowledge execution surface.
// TestKnowledgeStoreRecordsExposeExplicitExecutionSurface 用于验证 runtime store 记录会携带标准知识执行面对象。
func TestKnowledgeStoreRecordsExposeExplicitExecutionSurface(t *testing.T) {
	cache := KnowledgeCompileCacheRecord{
		BlobRef: knowledge.KnowledgeBlobRef{BlobRefID: "blob-1"},
	}
	view := RetrievalViewRecord{
		View: knowledge.RetrievalView{ViewID: "view-1"},
	}
	if cache.BlobRef.BlobRefID != "blob-1" {
		t.Fatalf("BlobRefID = %q, want blob-1", cache.BlobRef.BlobRefID)
	}
	if view.View.ViewID != "view-1" {
		t.Fatalf("ViewID = %q, want view-1", view.View.ViewID)
	}
}
