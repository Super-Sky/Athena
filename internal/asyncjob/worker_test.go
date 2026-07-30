package asyncjob

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

type workerStoreStub struct {
	Store
	acquireErr error
	renewErr   error
}

func (s *workerStoreStub) AcquireLease(context.Context, string, string, time.Duration) (Job, error) {
	return Job{}, s.acquireErr
}

func (s *workerStoreStub) RenewLease(context.Context, Job, time.Duration) error {
	return s.renewErr
}

func (s *workerStoreStub) CancelRequested(context.Context, string) (bool, error) {
	return false, nil
}

type workerQueueStub struct {
	ackCount atomic.Int32
	renewErr error
}

func (*workerQueueStub) Ping(context.Context) error                      { return nil }
func (*workerQueueStub) Ensure(context.Context) error                    { return nil }
func (*workerQueueStub) Publish(context.Context, Outbox) (string, error) { return "", nil }
func (*workerQueueStub) Claim(context.Context, string, time.Duration, time.Duration) (Delivery, error) {
	return Delivery{}, context.DeadlineExceeded
}
func (q *workerQueueStub) Ack(context.Context, Delivery) error {
	q.ackCount.Add(1)
	return nil
}
func (q *workerQueueStub) Renew(context.Context, Delivery, string) error {
	return q.renewErr
}
func (*workerQueueStub) Close() error { return nil }

func TestWorkerDoesNotAckDeliveryWhileAnotherLeaseMayOwnIt(t *testing.T) {
	store := &workerStoreStub{acquireErr: ErrLeaseUnavailable}
	queue := &workerQueueStub{}
	worker, err := NewWorker(store, queue, func(context.Context, Job) (Completion, *Failure) {
		t.Fatal("handler must not run without a PostgreSQL lease")
		return Completion{}, nil
	}, WorkerConfig{Consumer: "worker-a", Visibility: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	worker.execute(context.Background(), Delivery{MessageID: "1-0", JobID: "job-1"})
	if got := queue.ackCount.Load(); got != 0 {
		t.Fatalf("ack count=%d, want delivery left pending for reclaim", got)
	}

	store.acquireErr = ErrAlreadyTerminal
	worker.execute(context.Background(), Delivery{MessageID: "2-0", JobID: "job-1"})
	if got := queue.ackCount.Load(); got != 1 {
		t.Fatalf("ack count=%d, want terminal duplicate acknowledged", got)
	}
}

func TestWorkerMonitorCancelsExecutionWhenLeaseRenewalFails(t *testing.T) {
	store := &workerStoreStub{renewErr: errors.New("postgres unavailable")}
	worker, err := NewWorker(store, &workerQueueStub{}, func(context.Context, Job) (Completion, *Failure) {
		return Completion{}, nil
	}, WorkerConfig{Consumer: "worker-a", Visibility: 30 * time.Millisecond, CancelPoll: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()
	executionCtx, cancelExecution := context.WithCancel(ctx)
	done := make(chan struct{})
	go worker.monitor(executionCtx, cancelExecution, Job{ID: "job-1"}, Delivery{MessageID: "1-0", JobID: "job-1"}, done)
	select {
	case <-executionCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("execution context was not cancelled after lease renewal failed")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lease monitor did not exit")
	}
}

func TestWorkerMonitorCancelsExecutionWhenQueueRenewalFails(t *testing.T) {
	queue := &workerQueueStub{renewErr: errors.New("redis unavailable")}
	worker, err := NewWorker(&workerStoreStub{}, queue, func(context.Context, Job) (Completion, *Failure) {
		return Completion{}, nil
	}, WorkerConfig{Consumer: "worker-a", Visibility: 30 * time.Millisecond, CancelPoll: time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancelContext := context.WithCancel(context.Background())
	defer cancelContext()
	executionCtx, cancelExecution := context.WithCancel(ctx)
	done := make(chan struct{})
	go worker.monitor(executionCtx, cancelExecution, Job{ID: "job-1"}, Delivery{MessageID: "1-0", JobID: "job-1"}, done)
	select {
	case <-executionCtx.Done():
	case <-time.After(time.Second):
		t.Fatal("execution context was not cancelled after queue renewal failed")
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("lease monitor did not exit")
	}
}
