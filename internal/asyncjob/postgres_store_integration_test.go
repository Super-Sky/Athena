package asyncjob

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func TestPostgresStoreJobOutboxLeaseRetryAndCancel(t *testing.T) {
	dsn := os.Getenv("ATHENA_ASYNC_JOB_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("ATHENA_ASYNC_JOB_PG_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := uuid.NewString()
	jobID, runID := "job-"+suffix, "run-"+suffix
	t.Cleanup(func() {
		db.Exec("DELETE FROM runtime_async_job_events WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_job_outbox WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_async_jobs WHERE id = ?", jobID)
	})
	input := CreateInput{ID: jobID, RunID: runID, Kind: "agent_run", RequestSHA256: "hash-a", EncryptedRequest: "encrypted",
		AppID: "fund-assistant", WorkspaceID: "workspace-1", AppInstanceID: "fund-instance-1",
		IdempotencyScope: "workspace/app", IdempotencyKey: "idem-" + suffix, MaxAttempts: 3}
	created, duplicate, err := store.Create(ctx, input)
	if err != nil || duplicate || created.Status != StatusDeliveryPending {
		t.Fatalf("create job=%#v duplicate=%v err=%v", created, duplicate, err)
	}
	if created.AppID != input.AppID || created.WorkspaceID != input.WorkspaceID || created.AppInstanceID != input.AppInstanceID {
		t.Fatalf("ownership round-trip job=%#v input=%#v", created, input)
	}
	loaded, err := store.Get(ctx, jobID)
	if err != nil || loaded.AppID != input.AppID || loaded.WorkspaceID != input.WorkspaceID || loaded.AppInstanceID != input.AppInstanceID {
		t.Fatalf("loaded ownership job=%#v err=%v", loaded, err)
	}
	reused, duplicate, err := store.Create(ctx, CreateInput{ID: "other-" + suffix, RunID: "other-run-" + suffix, Kind: input.Kind,
		RequestSHA256: input.RequestSHA256, EncryptedRequest: input.EncryptedRequest, IdempotencyScope: input.IdempotencyScope, IdempotencyKey: input.IdempotencyKey})
	if err != nil || !duplicate || reused.ID != jobID {
		t.Fatalf("reused=%#v duplicate=%v err=%v", reused, duplicate, err)
	}
	_, _, err = store.Create(ctx, CreateInput{ID: "conflict-" + suffix, RunID: "conflict-run-" + suffix, Kind: input.Kind,
		RequestSHA256: "hash-b", EncryptedRequest: input.EncryptedRequest, IdempotencyScope: input.IdempotencyScope, IdempotencyKey: input.IdempotencyKey})
	if !errors.Is(err, ErrIdempotencyConflict) {
		t.Fatalf("conflict error=%v", err)
	}
	now := time.Now().UTC()
	outboxes, err := store.ClaimOutbox(ctx, "dispatcher-1", now.Add(time.Second), 5*time.Second, 10)
	if err != nil || len(outboxes) != 1 {
		t.Fatalf("outboxes=%#v err=%v", outboxes, err)
	}
	if err := store.MarkOutboxDelivered(ctx, outboxes[0], "1-0", now); err != nil {
		t.Fatal(err)
	}
	leased, err := store.AcquireLease(ctx, jobID, "worker-1", 5*time.Second)
	if err != nil || leased.Status != StatusRunning || leased.Attempt != 1 || leased.LeaseToken == "" {
		t.Fatalf("leased=%#v err=%v", leased, err)
	}
	if err := store.ScheduleRetry(ctx, leased, Failure{Code: "upstream_unavailable", Message: "retry", Retryable: true}, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	retryOutboxes, err := store.ClaimOutbox(ctx, "dispatcher-1", now.Add(2*time.Second), 5*time.Second, 10)
	if err != nil || len(retryOutboxes) != 1 || retryOutboxes[0].Operation != "retry" {
		t.Fatalf("retry outboxes=%#v err=%v", retryOutboxes, err)
	}
	if err := store.MarkOutboxDelivered(ctx, retryOutboxes[0], "2-0", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	leased, err = store.AcquireLease(ctx, jobID, "worker-2", 5*time.Second)
	if err != nil || leased.Attempt != 2 {
		t.Fatalf("retry lease=%#v err=%v", leased, err)
	}
	cancelled, err := store.RequestCancel(ctx, jobID, "user requested")
	if err != nil || cancelled.CancelRequestedAt.IsZero() || cancelled.Status != StatusRunning {
		t.Fatalf("cancelled=%#v err=%v", cancelled, err)
	}
	if err := store.Fail(ctx, leased, Failure{Code: "cancelled", Message: "job cancelled"}); err != nil {
		t.Fatal(err)
	}
	terminal, err := store.Get(ctx, jobID)
	if err != nil || terminal.Status != StatusCancelled {
		t.Fatalf("terminal=%#v err=%v", terminal, err)
	}
	events, err := store.ListEvents(ctx, jobID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertEventTypes(t, events, EventTypeCreated, EventTypeQueued, EventTypeRunning, EventTypeRetryScheduled,
		EventTypeQueued, EventTypeRunning, EventTypeCancelRequested, EventTypeCancelled)
	assertEventIdentity(t, events, jobID, runID)
	assertStrictlyIncreasingCursors(t, events)
	after := events[3].Cursor
	remaining, err := store.ListEvents(ctx, jobID, after, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(remaining) != len(events)-4 || remaining[0].Cursor != events[4].Cursor {
		t.Fatalf("after cursor=%d events=%#v", after, remaining)
	}
	if err := store.Fail(ctx, leased, Failure{Code: "cancelled", Message: "duplicate terminal write"}); !errors.Is(err, ErrLeaseUnavailable) {
		t.Fatalf("stale finish error=%v", err)
	}
	afterCAS, err := store.ListEvents(ctx, jobID, 0, 100)
	if err != nil || len(afterCAS) != len(events) {
		t.Fatalf("CAS failure wrote event count=%d want=%d err=%v", len(afterCAS), len(events), err)
	}
}

func TestDispatcherAndWorkerCompleteDurableJob(t *testing.T) {
	dsn := os.Getenv("ATHENA_ASYNC_JOB_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("ATHENA_ASYNC_JOB_PG_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	redisServer := miniredis.RunT(t)
	queue, err := NewRedisDeliveryQueue(RedisDeliveryConfig{URL: "redis://" + redisServer.Addr(), Stream: "test:{durable}:stream", Group: "workers"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Close() })
	if err := queue.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := uuid.NewString()
	jobID := "worker-job-" + suffix
	t.Cleanup(func() {
		db.Exec("DELETE FROM runtime_async_job_events WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_job_outbox WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_async_jobs WHERE id = ?", jobID)
	})
	job, duplicate, err := store.Create(ctx, CreateInput{ID: jobID, RunID: "worker-run-" + suffix, Kind: "agent_run",
		RequestSHA256: "hash", EncryptedRequest: "encrypted", MaxAttempts: 2})
	if err != nil || duplicate {
		t.Fatalf("create job=%#v duplicate=%v err=%v", job, duplicate, err)
	}
	dispatcher, err := NewDispatcher(store, queue, DispatcherConfig{Owner: "dispatcher-test", Lease: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	delivered, err := dispatcher.DispatchOnce(ctx, time.Now().UTC().Add(time.Second))
	if err != nil || delivered != 1 {
		t.Fatalf("delivered=%d err=%v", delivered, err)
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	worker, err := NewWorker(store, queue, func(_ context.Context, claimed Job) (Completion, *Failure) {
		if claimed.ID != jobID || claimed.Attempt != 1 {
			t.Fatalf("claimed=%#v", claimed)
		}
		cancelWorker()
		return Completion{EncryptedResult: "encrypted-result"}, nil
	}, WorkerConfig{Consumer: "worker-test", Block: 10 * time.Millisecond, Visibility: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Run(workerCtx); err != nil {
		t.Fatal(err)
	}
	completed, err := store.Get(ctx, jobID)
	if err != nil || completed.Status != StatusCompleted || completed.EncryptedResult != "encrypted-result" {
		t.Fatalf("completed=%#v err=%v", completed, err)
	}
	events, err := store.ListEvents(ctx, jobID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertEventTypes(t, events, EventTypeCreated, EventTypeQueued, EventTypeRunning, EventTypeCompleted)
	assertStrictlyIncreasingCursors(t, events)
}

func TestPostgresStoreWaitingResumeAndFailureEvents(t *testing.T) {
	dsn := os.Getenv("ATHENA_ASYNC_JOB_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("ATHENA_ASYNC_JOB_PG_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := uuid.NewString()
	jobID := "waiting-job-" + suffix
	t.Cleanup(func() {
		db.Exec("DELETE FROM runtime_async_job_events WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_job_outbox WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_async_jobs WHERE id = ?", jobID)
	})
	now := time.Now().UTC()
	_, duplicate, err := store.Create(ctx, CreateInput{ID: jobID, RunID: "waiting-run-" + suffix, Kind: "agent_run",
		RequestSHA256: "hash", EncryptedRequest: "encrypted", MaxAttempts: 2})
	if err != nil || duplicate {
		t.Fatalf("create duplicate=%v err=%v", duplicate, err)
	}
	outboxes, err := store.ClaimOutbox(ctx, "dispatcher-wait", now.Add(time.Second), time.Second, 10)
	if err != nil || len(outboxes) != 1 {
		t.Fatalf("claim outboxes=%#v err=%v", outboxes, err)
	}
	if err := store.MarkOutboxDelivered(ctx, outboxes[0], "wait-1", now); err != nil {
		t.Fatal(err)
	}
	leased, err := store.AcquireLease(ctx, jobID, "worker-wait", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Wait(ctx, leased, Completion{EncryptedResult: "checkpoint", WaitingReason: "approval required"}); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resume(ctx, jobID, "resumed-request", "resumed-hash", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	outboxes, err = store.ClaimOutbox(ctx, "dispatcher-wait", now.Add(3*time.Second), time.Second, 10)
	if err != nil || len(outboxes) != 1 {
		t.Fatalf("claim resumed outboxes=%#v err=%v", outboxes, err)
	}
	if err := store.MarkOutboxDelivered(ctx, outboxes[0], "wait-2", now.Add(2*time.Second)); err != nil {
		t.Fatal(err)
	}
	leased, err = store.AcquireLease(ctx, jobID, "worker-fail", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Fail(ctx, leased, Failure{Code: "invalid_request", Message: "request rejected"}); err != nil {
		t.Fatal(err)
	}
	events, err := store.ListEvents(ctx, jobID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertEventTypes(t, events, EventTypeCreated, EventTypeQueued, EventTypeRunning, EventTypeWaiting,
		EventTypeResumed, EventTypeQueued, EventTypeRunning, EventTypeFailed)
	assertStrictlyIncreasingCursors(t, events)
	if events[3].Reason != "approval required" || events[7].Reason != "request rejected" {
		t.Fatalf("event reasons=%#v", events)
	}
}

func TestPostgresStoreAcquireLeaseFinalizesRecoveredCancel(t *testing.T) {
	dsn := os.Getenv("ATHENA_ASYNC_JOB_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("ATHENA_ASYNC_JOB_PG_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := NewPostgresStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatal(err)
	}
	suffix := uuid.NewString()
	jobID := "cancel-recovery-job-" + suffix
	t.Cleanup(func() {
		db.Exec("DELETE FROM runtime_async_job_events WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_job_outbox WHERE job_id = ?", jobID)
		db.Exec("DELETE FROM runtime_async_jobs WHERE id = ?", jobID)
	})
	now := time.Now().UTC()
	_, duplicate, err := store.Create(ctx, CreateInput{ID: jobID, RunID: "cancel-recovery-run-" + suffix, Kind: "agent_run",
		RequestSHA256: "hash", EncryptedRequest: "encrypted", MaxAttempts: 2})
	if err != nil || duplicate {
		t.Fatalf("create duplicate=%v err=%v", duplicate, err)
	}
	outboxes, err := store.ClaimOutbox(ctx, "dispatcher-cancel-recovery", now.Add(time.Second), time.Second, 10)
	if err != nil || len(outboxes) != 1 {
		t.Fatalf("claim outboxes=%#v err=%v", outboxes, err)
	}
	if err := store.MarkOutboxDelivered(ctx, outboxes[0], "cancel-recovery-1", now); err != nil {
		t.Fatal(err)
	}
	leased, err := store.AcquireLease(ctx, jobID, "worker-crashed", time.Hour)
	if err != nil || leased.Status != StatusRunning {
		t.Fatalf("lease before cancel=%#v err=%v", leased, err)
	}
	cancelRequested, err := store.RequestCancel(ctx, jobID, "user cancelled")
	if err != nil || cancelRequested.Status != StatusRunning || cancelRequested.CancelRequestedAt.IsZero() {
		t.Fatalf("cancel request=%#v err=%v", cancelRequested, err)
	}
	recovered, err := store.AcquireLease(ctx, jobID, "worker-reclaimer", time.Second)
	if !errors.Is(err, ErrAlreadyTerminal) || recovered.Status != StatusCancelled {
		t.Fatalf("recovered=%#v err=%v", recovered, err)
	}
	events, err := store.ListEvents(ctx, jobID, 0, 100)
	if err != nil {
		t.Fatal(err)
	}
	assertEventTypes(t, events, EventTypeCreated, EventTypeQueued, EventTypeRunning, EventTypeCancelRequested, EventTypeCancelled)
	assertStrictlyIncreasingCursors(t, events)
}

func assertEventTypes(t *testing.T, events []Event, want ...EventType) {
	t.Helper()
	if len(events) != len(want) {
		t.Fatalf("event count=%d want=%d events=%#v", len(events), len(want), events)
	}
	for index := range want {
		if events[index].Type != want[index] {
			t.Fatalf("event[%d].type=%q want=%q events=%#v", index, events[index].Type, want[index], events)
		}
	}
}

func assertStrictlyIncreasingCursors(t *testing.T, events []Event) {
	t.Helper()
	for index := range events {
		if events[index].Cursor == 0 {
			t.Fatalf("event[%d] has zero cursor: %#v", index, events[index])
		}
		if index > 0 && events[index].Cursor <= events[index-1].Cursor {
			t.Fatalf("event cursors are not strictly increasing: %#v", events)
		}
	}
}

func assertEventIdentity(t *testing.T, events []Event, jobID, runID string) {
	t.Helper()
	for index, event := range events {
		if event.JobID != jobID || event.RunID != runID || event.OccurredAt.IsZero() {
			t.Fatalf("event[%d] identity or timestamp mismatch: %#v", index, event)
		}
	}
}
