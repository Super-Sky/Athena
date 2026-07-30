// redis_delivery.go implements at-least-once reference delivery with Redis Streams.
// redis_delivery.go 使用 Redis Streams 实现至少一次的引用投递。
package asyncjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisDeliveryConfig defines the Redis Streams endpoint and consumer group.
// RedisDeliveryConfig 定义 Redis Streams 端点和 consumer group。
type RedisDeliveryConfig struct {
	URL    string
	Stream string
	Group  string
}

// RedisDeliveryQueue delivers job references through Redis Streams while payloads stay in PostgreSQL.
// RedisDeliveryQueue 通过 Redis Streams 投递 job 引用，payload 保留在 PostgreSQL。
type RedisDeliveryQueue struct {
	client *redis.Client
	stream string
	group  string
}

// NewRedisDeliveryQueue builds a Redis-backed delivery queue from a redis:// URL.
// NewRedisDeliveryQueue 基于 redis:// URL 构建 Redis 投递队列。
func NewRedisDeliveryQueue(cfg RedisDeliveryConfig) (*RedisDeliveryQueue, error) {
	options, err := redis.ParseURL(strings.TrimSpace(cfg.URL))
	if err != nil {
		return nil, fmt.Errorf("parse async Redis URL: %w", err)
	}
	stream := strings.TrimSpace(cfg.Stream)
	if stream == "" {
		stream = "athena:{async_jobs}:stream"
	}
	group := strings.TrimSpace(cfg.Group)
	if group == "" {
		group = "athena-agent-workers"
	}
	return &RedisDeliveryQueue{client: redis.NewClient(options), stream: stream, group: group}, nil
}

// Ping checks Redis connectivity without creating stream metadata.
// Ping 检查 Redis 连通性，但不创建 stream 元数据。
func (q *RedisDeliveryQueue) Ping(ctx context.Context) error {
	if q == nil || q.client == nil {
		return ErrQueueDisabled
	}
	return q.client.Ping(ctx).Err()
}

// Ensure creates the stream consumer group idempotently.
// Ensure 幂等创建 stream consumer group。
func (q *RedisDeliveryQueue) Ensure(ctx context.Context) error {
	if err := q.Ping(ctx); err != nil {
		return err
	}
	err := q.client.XGroupCreateMkStream(ctx, q.stream, q.group, "0").Err()
	if err != nil && !strings.Contains(err.Error(), "BUSYGROUP") {
		return err
	}
	return nil
}

// Publish appends only outbox/job/run references to the Redis stream.
// Publish 只向 Redis stream 写入 outbox/job/run 引用。
func (q *RedisDeliveryQueue) Publish(ctx context.Context, outbox Outbox) (string, error) {
	return q.client.XAdd(ctx, &redis.XAddArgs{Stream: q.stream, Values: map[string]any{
		"outbox_id": outbox.ID, "job_id": outbox.JobID, "run_id": outbox.RunID,
	}}).Result()
}

// Claim reclaims stale pending messages before reading new messages.
// Claim 会先回收过期 pending 消息，再读取新消息。
func (q *RedisDeliveryQueue) Claim(ctx context.Context, consumer string, block, minIdle time.Duration) (Delivery, error) {
	if minIdle > 0 {
		messages, _, err := q.client.XAutoClaim(ctx, &redis.XAutoClaimArgs{
			Stream: q.stream, Group: q.group, Consumer: consumer, MinIdle: minIdle, Start: "0-0", Count: 1,
		}).Result()
		if err != nil && !errors.Is(err, redis.Nil) {
			return Delivery{}, err
		}
		if len(messages) > 0 {
			return deliveryFromMessage(messages[0]), nil
		}
	}
	streams, err := q.client.XReadGroup(ctx, &redis.XReadGroupArgs{
		Group: q.group, Consumer: consumer, Streams: []string{q.stream, ">"}, Count: 1, Block: block,
	}).Result()
	if errors.Is(err, redis.Nil) {
		return Delivery{}, context.DeadlineExceeded
	}
	if err != nil {
		return Delivery{}, err
	}
	if len(streams) == 0 || len(streams[0].Messages) == 0 {
		return Delivery{}, context.DeadlineExceeded
	}
	return deliveryFromMessage(streams[0].Messages[0]), nil
}

// Ack confirms one delivery after PostgreSQL reaches a stable state.
// Ack 在 PostgreSQL 进入稳定状态后确认一条投递。
func (q *RedisDeliveryQueue) Ack(ctx context.Context, delivery Delivery) error {
	return q.client.XAck(ctx, q.stream, q.group, delivery.MessageID).Err()
}

// Renew resets one pending delivery's idle timer while the PostgreSQL lease remains authoritative.
// Renew 在 PostgreSQL lease 仍为权威边界时重置一条 pending 投递的空闲计时。
func (q *RedisDeliveryQueue) Renew(ctx context.Context, delivery Delivery, consumer string) error {
	ids, err := q.client.XClaimJustID(ctx, &redis.XClaimArgs{
		Stream: q.stream, Group: q.group, Consumer: strings.TrimSpace(consumer), MinIdle: 0, Messages: []string{delivery.MessageID},
	}).Result()
	if err != nil {
		return err
	}
	if len(ids) != 1 || ids[0] != delivery.MessageID {
		return ErrLeaseUnavailable
	}
	return nil
}

func (q *RedisDeliveryQueue) Close() error {
	if q == nil || q.client == nil {
		return nil
	}
	return q.client.Close()
}

func deliveryFromMessage(message redis.XMessage) Delivery {
	return Delivery{MessageID: message.ID, OutboxID: fmt.Sprint(message.Values["outbox_id"]),
		JobID: fmt.Sprint(message.Values["job_id"]), RunID: fmt.Sprint(message.Values["run_id"])}
}
