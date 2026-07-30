// dispatcher.go publishes durable PostgreSQL outbox references to Redis Streams.
// dispatcher.go 将 PostgreSQL 持久 outbox 引用发布到 Redis Streams。
package asyncjob

import (
	"context"
	"fmt"
	"time"
)

// DispatcherConfig controls outbox polling, leasing, and Redis publish retry timing.
// DispatcherConfig 控制 outbox 轮询、租约和 Redis 发布重试时间。
type DispatcherConfig struct {
	Owner        string
	PollInterval time.Duration
	Lease        time.Duration
	BatchSize    int
	RetryBase    time.Duration
	RetryMax     time.Duration
}

// Dispatcher moves durable PostgreSQL outbox records into Redis Streams as references.
// Dispatcher 将 PostgreSQL 持久 outbox 记录以引用形式投递到 Redis Streams。
type Dispatcher struct {
	store  Store
	queue  DeliveryQueue
	config DispatcherConfig
}

// NewDispatcher creates a dispatcher without starting the long-running loop.
// NewDispatcher 创建 dispatcher，但不启动长期运行循环。
func NewDispatcher(store Store, queue DeliveryQueue, cfg DispatcherConfig) (*Dispatcher, error) {
	if store == nil || queue == nil {
		return nil, fmt.Errorf("async job store and delivery queue are required")
	}
	if cfg.Owner == "" {
		cfg.Owner = fmt.Sprintf("dispatcher-%d", time.Now().UnixNano())
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 500 * time.Millisecond
	}
	if cfg.Lease <= 0 {
		cfg.Lease = 10 * time.Second
	}
	if cfg.BatchSize <= 0 {
		cfg.BatchSize = 100
	}
	if cfg.RetryBase <= 0 {
		cfg.RetryBase = time.Second
	}
	if cfg.RetryMax <= 0 {
		cfg.RetryMax = time.Minute
	}
	return &Dispatcher{store: store, queue: queue, config: cfg}, nil
}

// Run continuously dispatches available outbox records until the context is cancelled.
// Run 会持续投递可用 outbox 记录，直到 context 被取消。
func (d *Dispatcher) Run(ctx context.Context) {
	ticker := time.NewTicker(d.config.PollInterval)
	defer ticker.Stop()
	for {
		_, _ = d.DispatchOnce(ctx, time.Now().UTC())
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

// DispatchOnce claims one batch and publishes each record exactly once per won outbox lease.
// DispatchOnce 领取一个批次，并对每个赢得 outbox lease 的记录执行一次发布。
func (d *Dispatcher) DispatchOnce(ctx context.Context, now time.Time) (int, error) {
	outboxes, err := d.store.ClaimOutbox(ctx, d.config.Owner, now, d.config.Lease, d.config.BatchSize)
	if err != nil {
		return 0, err
	}
	delivered := 0
	for _, outbox := range outboxes {
		streamID, publishErr := d.queue.Publish(ctx, outbox)
		if publishErr != nil {
			availableAt := now.Add(d.retryDelay(outbox.DeliveryAttempts))
			_ = d.store.ReleaseOutbox(ctx, outbox, "redis_publish_failed", availableAt)
			continue
		}
		if err := d.store.MarkOutboxDelivered(ctx, outbox, streamID, now); err != nil {
			return delivered, err
		}
		delivered++
	}
	return delivered, nil
}

func (d *Dispatcher) retryDelay(attempt int) time.Duration {
	if attempt < 1 {
		attempt = 1
	}
	delay := d.config.RetryBase
	for index := 1; index < attempt; index++ {
		delay *= 2
		if delay >= d.config.RetryMax {
			return d.config.RetryMax
		}
	}
	return delay
}
