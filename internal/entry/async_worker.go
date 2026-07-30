// async_worker.go assembles the Redis delivery adapter, outbox dispatcher, and durable Agent Run worker.
// async_worker.go 组装 Redis 投递适配器、outbox dispatcher 与持久 Agent Run worker。
package entry

import (
	"context"
	"fmt"
	"time"

	"moss/internal/asyncjob"
)

// AsyncWorkerRuntime owns the long-running dispatcher and worker pair for one process.
// AsyncWorkerRuntime 持有单个进程中的长期 dispatcher 与 worker 组合。
type AsyncWorkerRuntime struct {
	dispatcher *asyncjob.Dispatcher
	worker     *asyncjob.Worker
	queue      asyncjob.DeliveryQueue
}

// NewAsyncWorkerRuntime builds and probes the queue before the worker process starts.
// NewAsyncWorkerRuntime 在 worker 进程启动前构建并探测队列。
func (b *Bootstrap) NewAsyncWorkerRuntime(ctx context.Context) (*AsyncWorkerRuntime, error) {
	if b == nil || !b.Config.AsyncJobs.Enabled || b.AsyncJobStore == nil || b.AsyncRuns == nil {
		return nil, fmt.Errorf("async jobs are not enabled")
	}
	queue, err := asyncjob.NewRedisDeliveryQueue(asyncjob.RedisDeliveryConfig{
		URL: b.Config.AsyncJobs.RedisURL, Stream: b.Config.AsyncJobs.Stream, Group: b.Config.AsyncJobs.Group,
	})
	if err != nil {
		return nil, err
	}
	if err := queue.Ensure(ctx); err != nil {
		_ = queue.Close()
		return nil, fmt.Errorf("async Redis readiness failed: %w", err)
	}
	dispatcher, err := asyncjob.NewDispatcher(b.AsyncJobStore, queue, asyncjob.DispatcherConfig{
		Owner: b.Config.AsyncJobs.Consumer + "-dispatcher", PollInterval: b.Config.AsyncJobDispatcherPoll(),
		Lease: b.Config.AsyncJobOutboxLease(), BatchSize: b.Config.AsyncJobs.BatchSize,
		RetryBase: b.Config.AsyncJobRetryBase(), RetryMax: b.Config.AsyncJobRetryMax(),
	})
	if err != nil {
		_ = queue.Close()
		return nil, err
	}
	worker, err := asyncjob.NewWorker(b.AsyncJobStore, queue, b.AsyncRuns.Handler(), asyncjob.WorkerConfig{
		Consumer: b.Config.AsyncJobs.Consumer, Block: time.Second, Visibility: b.Config.AsyncJobVisibilityTimeout(),
		CancelPoll: b.Config.AsyncJobCancelPoll(), RetryBase: b.Config.AsyncJobRetryBase(), RetryMax: b.Config.AsyncJobRetryMax(),
	})
	if err != nil {
		_ = queue.Close()
		return nil, err
	}
	return &AsyncWorkerRuntime{dispatcher: dispatcher, worker: worker, queue: queue}, nil
}

// Run starts outbox delivery and blocks while the Redis worker consumes references.
// Run 启动 outbox 投递，并在 Redis worker 消费引用期间阻塞。
func (r *AsyncWorkerRuntime) Run(ctx context.Context) error {
	if r == nil || r.dispatcher == nil || r.worker == nil {
		return fmt.Errorf("async worker runtime is not configured")
	}
	dispatcherCtx, cancelDispatcher := context.WithCancel(ctx)
	defer cancelDispatcher()
	go r.dispatcher.Run(dispatcherCtx)
	return r.worker.Run(ctx)
}

// Close releases the Redis connection owned by this worker runtime.
// Close 释放该 worker runtime 持有的 Redis 连接。
func (r *AsyncWorkerRuntime) Close() error {
	if r == nil || r.queue == nil {
		return nil
	}
	return r.queue.Close()
}
