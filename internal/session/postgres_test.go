package session

import (
	"testing"
	"time"
)

// TestPostgresSessionAggregateRoundTrip verifies the aggregate can be encoded in the session row and rehydrated together with queue rows.
// TestPostgresSessionAggregateRoundTrip 用于验证 session 主表和 queue 表可以一起正确编码并回填聚合。
func TestPostgresSessionAggregateRoundTrip(t *testing.T) {
	t.Parallel()

	store := NewPostgresStoreWithOptions(nil, 2, time.Minute, 2)
	source := &Session{
		ID: "sess-pg-1",
		Messages: []Message{
			{Role: "user", Content: "hello"},
			{Role: "assistant", Content: "world"},
		},
		Pending: &PendingState{
			Stage:       "capability_resolution",
			Status:      "waiting_for_information",
			ActionType:  "information_request",
			ResumeToken: "resume-1",
		},
		DeferredQueue: []DeferredMessage{
			{Query: "q-1", ReceivedAt: time.Unix(1, 0)},
			{Query: "q-2", ReceivedAt: time.Unix(2, 0)},
			{Query: "q-3", ReceivedAt: time.Unix(3, 0)},
		},
		ClosedTokens: []ClosedResumeToken{
			{Token: "expired", ClosedAt: time.Now().Add(-2 * time.Minute)},
			{Token: "fresh", ClosedAt: time.Now()},
		},
	}

	record, err := store.encodeSession(source, 3, time.Unix(10, 0))
	if err != nil {
		t.Fatalf("encodeSession() error = %v", err)
	}
	if record.Version != 3 {
		t.Fatalf("record.Version = %d, want 3", record.Version)
	}
	if record.StateSchemaVersion != SessionStateSchemaVersion {
		t.Fatalf("record.StateSchemaVersion = %d, want %d", record.StateSchemaVersion, SessionStateSchemaVersion)
	}

	queueRows, err := buildDeferredQueueRows(source.ID, source.DeferredQueue)
	if err != nil {
		t.Fatalf("buildDeferredQueueRows() error = %v", err)
	}

	decoded, err := store.decodeAggregate(record, queueRows)
	if err != nil {
		t.Fatalf("decodeAggregate() error = %v", err)
	}
	if decoded.ID != source.ID {
		t.Fatalf("decoded.ID = %q, want %q", decoded.ID, source.ID)
	}
	if len(decoded.Messages) != 2 {
		t.Fatalf("messages len = %d, want 2", len(decoded.Messages))
	}
	if decoded.Pending == nil || decoded.Pending.ResumeToken != "resume-1" {
		t.Fatalf("unexpected pending = %#v", decoded.Pending)
	}
	if len(decoded.DeferredQueue) != 2 {
		t.Fatalf("deferred queue len = %d, want 2 after normalization", len(decoded.DeferredQueue))
	}
	if decoded.DeferredQueue[0].Query != "q-2" {
		t.Fatalf("unexpected oldest retained deferred item = %#v", decoded.DeferredQueue[0])
	}
	if len(decoded.ClosedTokens) != 1 || decoded.ClosedTokens[0].Token != "fresh" {
		t.Fatalf("unexpected closed tokens = %#v", decoded.ClosedTokens)
	}
}

// TestDecodeDeferredQueueRowsAcceptsObjectDefaults verifies migrated queued rows use JSON defaults that match their Go field shapes.
// TestDecodeDeferredQueueRowsAcceptsObjectDefaults 用于验证迁移后的 queued row JSON 默认值与 Go 字段形态匹配。
func TestDecodeDeferredQueueRowsAcceptsObjectDefaults(t *testing.T) {
	rows := []PostgresDeferredMessageModel{
		{
			SessionID:              "sess-pg-migrated",
			Sequence:               1,
			Query:                  "queued query",
			EnabledSkills:          []byte("[]"),
			EnabledTools:           []byte("[]"),
			ContextAssetOverrides:  []byte("[]"),
			DisabledAssetTypes:     []byte("[]"),
			AssetPriorityOverrides: []byte("{}"),
			ContextAssets:          []byte("[]"),
			ContextBindings:        []byte("[]"),
			CompiledRefs:           []byte("[]"),
			Status:                 DeferredMessageStatusQueued,
			ReceivedAt:             time.Unix(10, 0),
		},
	}

	queue, err := decodeDeferredQueueRows(rows)
	if err != nil {
		t.Fatalf("decodeDeferredQueueRows() error = %v", err)
	}
	if len(queue) != 1 {
		t.Fatalf("queue len = %d, want 1", len(queue))
	}
	if queue[0].AssetPriorityOverrides == nil || len(queue[0].AssetPriorityOverrides) != 0 {
		t.Fatalf("asset priority overrides = %#v, want empty map", queue[0].AssetPriorityOverrides)
	}
}
