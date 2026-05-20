package session

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestPostgresStoreIntegrationRoundTrip verifies the real PostgreSQL-backed store can migrate, persist, and update one session aggregate.
// TestPostgresStoreIntegrationRoundTrip 用于验证真实 PostgreSQL store 可以完成 migrate、持久化和单聚合更新闭环。
func TestPostgresStoreIntegrationRoundTrip(t *testing.T) {
	t.Parallel()

	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := NewPostgresDB(dsn)
	if err != nil {
		t.Fatalf("NewPostgresDB() error = %v", err)
	}

	store := NewPostgresStoreWithOptions(db, 2, time.Minute, 3)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	sessionID := "itest-sess-postgres-roundtrip"
	if err := db.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&PostgresDeferredMessageModel{}).Error; err != nil {
		t.Fatalf("cleanup deferred rows error = %v", err)
	}
	if err := db.WithContext(ctx).Where("id = ?", sessionID).Delete(&PostgresSessionModel{}).Error; err != nil {
		t.Fatalf("cleanup session row error = %v", err)
	}
	_, err = store.Update(ctx, sessionID, func(current *Session) error {
		current.Messages = append(current.Messages, Message{Role: "user", Content: "hello"})
		current.Pending = &PendingState{
			Stage:        "capability_resolution",
			Status:       "waiting_for_information",
			ActionType:   "information_request",
			ResumeToken:  "resume-itest",
			ModelID:      "model-pending",
			TimeoutAfter: 30 * time.Second,
			TimeoutAt:    time.Now().Add(30 * time.Second),
		}
		current.DeferredQueue = append(current.DeferredQueue,
			DeferredMessage{Query: "queued-1", ModelID: "model-1", ReceivedAt: time.Unix(1, 0)},
			DeferredMessage{Query: "queued-2", ModelID: "model-2", ReceivedAt: time.Unix(2, 0)},
			DeferredMessage{Query: "queued-3", ModelID: "model-3", ReceivedAt: time.Unix(3, 0)},
		)
		current.ClosedTokens = append(current.ClosedTokens,
			ClosedResumeToken{Token: "expired", Reason: "timeout_expired", ClosedAt: time.Now().Add(-2 * time.Minute)},
			ClosedResumeToken{Token: "fresh", Reason: "unable_to_provide", ClosedAt: time.Now()},
		)
		return nil
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}

	got, ok := store.Get(ctx, sessionID)
	if !ok {
		t.Fatalf("Get() expected stored session")
	}
	if len(got.Messages) != 1 {
		t.Fatalf("messages len = %d, want 1", len(got.Messages))
	}
	if got.Pending == nil || got.Pending.ResumeToken != "resume-itest" {
		t.Fatalf("unexpected pending = %#v", got.Pending)
	}
	if got.Pending.ModelID != "model-pending" {
		t.Fatalf("pending model id = %q, want model-pending", got.Pending.ModelID)
	}
	if len(got.DeferredQueue) != 2 {
		t.Fatalf("deferred queue len = %d, want 2", len(got.DeferredQueue))
	}
	if got.DeferredQueue[0].Query != "queued-2" {
		t.Fatalf("unexpected deferred queue head = %#v", got.DeferredQueue[0])
	}
	if got.DeferredQueue[0].ModelID != "model-2" || got.DeferredQueue[1].ModelID != "model-3" {
		t.Fatalf("unexpected deferred queue model ids = %#v", got.DeferredQueue)
	}
	if len(got.ClosedTokens) != 1 || got.ClosedTokens[0].Token != "fresh" {
		t.Fatalf("unexpected closed tokens = %#v", got.ClosedTokens)
	}
	var queuedRows int64
	if err := db.WithContext(ctx).Model(&PostgresDeferredMessageModel{}).Where("session_id = ?", sessionID).Count(&queuedRows).Error; err != nil {
		t.Fatalf("count deferred rows error = %v", err)
	}
	if queuedRows != 2 {
		t.Fatalf("deferred rows = %d, want 2", queuedRows)
	}

	updated, err := store.Update(ctx, sessionID, func(current *Session) error {
		current.Pending = nil
		current.DeferredQueue = nil
		current.Messages = append(current.Messages, Message{Role: "assistant", Content: "done"})
		return nil
	})
	if err != nil {
		t.Fatalf("Update(second) error = %v", err)
	}
	if updated.Pending != nil {
		t.Fatalf("updated.Pending = %#v, want nil", updated.Pending)
	}
	if len(updated.Messages) != 2 {
		t.Fatalf("updated messages len = %d, want 2", len(updated.Messages))
	}
	if len(updated.DeferredQueue) != 0 {
		t.Fatalf("updated deferred queue len = %d, want 0", len(updated.DeferredQueue))
	}
	if err := db.WithContext(ctx).Model(&PostgresDeferredMessageModel{}).Where("session_id = ?", sessionID).Count(&queuedRows).Error; err != nil {
		t.Fatalf("count deferred rows after clear error = %v", err)
	}
	if queuedRows != 0 {
		t.Fatalf("deferred rows after clear = %d, want 0", queuedRows)
	}
}
