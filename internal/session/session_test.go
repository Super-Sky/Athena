package session

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func TestSessionNormalizeTrimsQueueAndPrunesClosedTokens(t *testing.T) {
	sess := &Session{
		ID: "sess-normalize",
		DeferredQueue: []DeferredMessage{
			{Query: "q1", ModelID: "model-1", ReceivedAt: time.Unix(1, 0)},
			{Query: "q2", ModelID: "model-2", ReceivedAt: time.Unix(2, 0)},
			{Query: "q3", ModelID: "model-3", ReceivedAt: time.Unix(3, 0)},
		},
		ClosedTokens: []ClosedResumeToken{
			{Token: "old", ClosedAt: time.Unix(0, 0)},
			{Token: "fresh", ClosedAt: time.Unix(100, 0)},
		},
	}

	sess.Normalize(time.Unix(110, 0), 2, 15*time.Second)

	if len(sess.DeferredQueue) != 2 {
		t.Fatalf("deferred queue length = %d, want 2", len(sess.DeferredQueue))
	}
	if sess.DeferredQueue[0].Query != "q2" || sess.DeferredQueue[1].Query != "q3" {
		t.Fatalf("unexpected deferred queue order = %#v", sess.DeferredQueue)
	}
	if sess.DeferredQueue[0].ModelID != "model-2" || sess.DeferredQueue[1].ModelID != "model-3" {
		t.Fatalf("unexpected deferred queue model ids = %#v", sess.DeferredQueue)
	}
	if len(sess.ClosedTokens) != 1 || sess.ClosedTokens[0].Token != "fresh" {
		t.Fatalf("unexpected closed tokens = %#v", sess.ClosedTokens)
	}
}

func TestSessionClonePreservesDeferredModelID(t *testing.T) {
	sess := &Session{
		ID: "sess-clone",
		Pending: &PendingState{
			ResumeToken: "resume-clone",
			ModelID:     "model-pending",
			Preserved: &PreservedContext{
				Goal:          "show user profile",
				MissingFields: []string{"user_id"},
				Facts: map[string]string{
					"case_id": "case-1",
				},
			},
		},
		DeferredQueue: []DeferredMessage{{
			Query:      "queued",
			ModelID:    "model-deferred",
			ReceivedAt: time.Unix(1, 0),
		}},
	}

	cloned := sess.Clone()
	if cloned.Pending == nil || cloned.Pending.ModelID != "model-pending" {
		t.Fatalf("cloned pending = %#v", cloned.Pending)
	}
	if cloned.Pending.Preserved == nil || cloned.Pending.Preserved.Goal != "show user profile" {
		t.Fatalf("cloned preserved context = %#v", cloned.Pending.Preserved)
	}
	if cloned.Pending.Preserved.Facts["case_id"] != "case-1" {
		t.Fatalf("cloned preserved facts = %#v", cloned.Pending.Preserved.Facts)
	}
	if len(cloned.DeferredQueue) != 1 || cloned.DeferredQueue[0].ModelID != "model-deferred" {
		t.Fatalf("cloned deferred queue = %#v", cloned.DeferredQueue)
	}
}

func TestMemoryStorePutGetNormalizesSessionState(t *testing.T) {
	store := NewMemoryStore()
	sess := &Session{
		ID: "sess-store",
	}
	for idx := 0; idx < DefaultDeferredQueueLimit+3; idx++ {
		sess.DeferredQueue = append(sess.DeferredQueue, DeferredMessage{
			Query:      fmt.Sprintf("q-%d", idx),
			ModelID:    fmt.Sprintf("model-%d", idx),
			ReceivedAt: time.Unix(int64(idx), 0),
		})
	}
	sess.ClosedTokens = []ClosedResumeToken{
		{Token: "expired", ClosedAt: time.Now().Add(-DefaultClosedResumeTokenTTL - time.Minute)},
		{Token: "alive", ClosedAt: time.Now()},
	}

	if err := store.Put(context.Background(), sess); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	got, ok := store.Get(context.Background(), sess.ID)
	if !ok {
		t.Fatalf("expected session to exist")
	}
	if len(got.DeferredQueue) != DefaultDeferredQueueLimit {
		t.Fatalf("deferred queue length = %d, want %d", len(got.DeferredQueue), DefaultDeferredQueueLimit)
	}
	if got.DeferredQueue[0].Query != "q-3" {
		t.Fatalf("unexpected oldest retained deferred message = %#v", got.DeferredQueue[0])
	}
	if got.DeferredQueue[0].ModelID != "model-3" {
		t.Fatalf("unexpected retained deferred model id = %#v", got.DeferredQueue[0])
	}
	if len(got.ClosedTokens) != 1 || got.ClosedTokens[0].Token != "alive" {
		t.Fatalf("unexpected closed tokens = %#v", got.ClosedTokens)
	}
}

func TestMemoryStoreWithOptionsUsesConfiguredLimits(t *testing.T) {
	store := NewMemoryStoreWithOptions(2, time.Minute)
	sess := &Session{
		ID: "sess-configured",
		DeferredQueue: []DeferredMessage{
			{Query: "q-1", ModelID: "model-1", ReceivedAt: time.Unix(1, 0)},
			{Query: "q-2", ModelID: "model-2", ReceivedAt: time.Unix(2, 0)},
			{Query: "q-3", ModelID: "model-3", ReceivedAt: time.Unix(3, 0)},
		},
		ClosedTokens: []ClosedResumeToken{
			{Token: "expired", ClosedAt: time.Now().Add(-2 * time.Minute)},
			{Token: "fresh", ClosedAt: time.Now()},
		},
	}

	if err := store.Put(context.Background(), sess); err != nil {
		t.Fatalf("Put() error = %v", err)
	}
	got, ok := store.Get(context.Background(), sess.ID)
	if !ok {
		t.Fatalf("expected session to exist")
	}
	if len(got.DeferredQueue) != 2 {
		t.Fatalf("deferred queue length = %d, want 2", len(got.DeferredQueue))
	}
	if got.DeferredQueue[0].Query != "q-2" {
		t.Fatalf("unexpected oldest retained deferred message = %#v", got.DeferredQueue[0])
	}
	if got.DeferredQueue[0].ModelID != "model-2" {
		t.Fatalf("unexpected retained deferred model id = %#v", got.DeferredQueue[0])
	}
	if len(got.ClosedTokens) != 1 || got.ClosedTokens[0].Token != "fresh" {
		t.Fatalf("unexpected closed tokens = %#v", got.ClosedTokens)
	}
}

func TestMemoryStoreUpdateCreatesAndNormalizesSessionAtomically(t *testing.T) {
	store := NewMemoryStoreWithOptions(2, time.Minute)

	got, err := store.Update(context.Background(), "sess-update", func(current *Session) error {
		current.Messages = append(current.Messages, Message{Role: "user", Content: "hello"})
		current.DeferredQueue = append(current.DeferredQueue,
			DeferredMessage{Query: "q-1", ModelID: "model-1", ReceivedAt: time.Unix(1, 0)},
			DeferredMessage{Query: "q-2", ModelID: "model-2", ReceivedAt: time.Unix(2, 0)},
			DeferredMessage{Query: "q-3", ModelID: "model-3", ReceivedAt: time.Unix(3, 0)},
		)
		current.ClosedTokens = append(current.ClosedTokens,
			ClosedResumeToken{Token: "expired", ClosedAt: time.Now().Add(-2 * time.Minute)},
			ClosedResumeToken{Token: "fresh", ClosedAt: time.Now()},
		)
		return nil
	})
	if err != nil {
		t.Fatalf("Update() error = %v", err)
	}
	if got.ID != "sess-update" {
		t.Fatalf("session id = %q, want sess-update", got.ID)
	}
	if len(got.DeferredQueue) != 2 {
		t.Fatalf("deferred queue length = %d, want 2", len(got.DeferredQueue))
	}
	if got.DeferredQueue[0].Query != "q-2" {
		t.Fatalf("unexpected oldest retained deferred message = %#v", got.DeferredQueue[0])
	}
	if got.DeferredQueue[0].ModelID != "model-2" {
		t.Fatalf("unexpected retained deferred model id = %#v", got.DeferredQueue[0])
	}
	if len(got.ClosedTokens) != 1 || got.ClosedTokens[0].Token != "fresh" {
		t.Fatalf("unexpected closed tokens = %#v", got.ClosedTokens)
	}
}
