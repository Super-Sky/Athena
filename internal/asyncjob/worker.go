// worker.go consumes Redis references while PostgreSQL leases guard authoritative execution.
// worker.go 消费 Redis 引用，并使用 PostgreSQL lease 保护权威执行。
package asyncjob

import (
	"context"
	"errors"
	"fmt"
	"time"
)

type WorkerConfig struct {
	Consumer       string
	Block          time.Duration
	Visibility     time.Duration
	CancelPoll     time.Duration
	RetryBase      time.Duration
	RetryMax       time.Duration
	CleanupTimeout time.Duration
}

type Worker struct {
	store   Store
	queue   DeliveryQueue
	handler Handler
	config  WorkerConfig
}

func NewWorker(store Store, queue DeliveryQueue, handler Handler, cfg WorkerConfig) (*Worker, error) {
	if store == nil || queue == nil || handler == nil {
		return nil, fmt.Errorf("async job store, delivery queue, and handler are required")
	}
	if cfg.Consumer == "" {
		cfg.Consumer = fmt.Sprintf("worker-%d", time.Now().UnixNano())
	}
	if cfg.Block <= 0 {
		cfg.Block = time.Second
	}
	if cfg.Visibility <= 0 {
		cfg.Visibility = 30 * time.Second
	}
	if cfg.CancelPoll <= 0 {
		cfg.CancelPoll = 250 * time.Millisecond
	}
	if cfg.RetryBase <= 0 {
		cfg.RetryBase = time.Second
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = time.Minute
	}
	if cfg.CleanupTimeout <= 0 {
		cfg.CleanupTimeout = 5 * time.Second
	}
	return &Worker{store: store, queue: queue, handler: handler, config: cfg}, nil
}

func (w *Worker) Run(ctx context.Context) error {
	if err := w.queue.Ensure(ctx); err != nil {
		return err
	}
	for ctx.Err() == nil {
		delivery, err := w.queue.Claim(ctx, w.config.Consumer, w.config.Block, w.config.Visibility)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			timer := time.NewTimer(min(w.config.Block, 250*time.Millisecond))
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil
			case <-timer.C:
			}
			continue
		}
		w.execute(ctx, delivery)
	}
	return nil
}

func (w *Worker) execute(parent context.Context, delivery Delivery) {
	job, err := w.store.AcquireLease(parent, delivery.JobID, w.config.Consumer, w.config.Visibility)
	if err != nil {
		if errors.Is(err, ErrAlreadyTerminal) {
			_ = w.queue.Ack(parent, delivery)
		}
		return
	}
	ctx, cancel := context.WithCancel(parent)
	monitorDone := make(chan struct{})
	go w.monitor(ctx, cancel, job, delivery, monitorDone)
	completion, failure := w.handler(ctx, job)
	cancel()
	<-monitorDone
	cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(parent), w.config.CleanupTimeout)
	defer cleanupCancel()
	cancelRequested, _ := w.store.CancelRequested(cleanupCtx, job.ID)
	if cancelRequested {
		failure = &Failure{Code: "cancelled", Message: "job cancelled"}
	}
	var persistErr error
	switch {
	case failure == nil && completion.Waiting:
		persistErr = w.store.Wait(cleanupCtx, job, completion)
	case failure == nil:
		persistErr = w.store.Complete(cleanupCtx, job, completion)
	case failure.Retryable && job.Attempt < job.MaxAttempts:
		persistErr = w.store.ScheduleRetry(cleanupCtx, job, *failure, time.Now().UTC().Add(w.retryDelay(job.Attempt)))
	default:
		persistErr = w.store.Fail(cleanupCtx, job, *failure)
	}
	if persistErr == nil {
		_ = w.queue.Ack(cleanupCtx, delivery)
	}
}

func (w *Worker) monitor(ctx context.Context, cancel context.CancelFunc, job Job, delivery Delivery, done chan<- struct{}) {
	defer close(done)
	interval := min(w.config.CancelPoll, w.config.Visibility/3)
	if interval <= 0 {
		interval = 100 * time.Millisecond
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			requested, err := w.store.CancelRequested(ctx, job.ID)
			if err == nil && requested {
				cancel()
				return
			}
			if err := w.store.RenewLease(ctx, job, w.config.Visibility); err != nil {
				cancel()
				return
			}
			if err := w.queue.Renew(ctx, delivery, w.config.Consumer); err != nil {
				cancel()
				return
			}
		}
	}
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := w.config.RetryBase
	for index := 1; index < attempt; index++ {
		delay *= 2
		if delay >= w.config.RetryMax {
			return w.config.RetryMax
		}
	}
	return delay
}
