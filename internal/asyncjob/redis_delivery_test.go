package asyncjob

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
)

func TestRedisDeliveryPublishesReferencesOnlyAndAcknowledges(t *testing.T) {
	server := miniredis.RunT(t)
	queue, err := NewRedisDeliveryQueue(RedisDeliveryConfig{
		URL: "redis://" + server.Addr(), Stream: "test:{jobs}:stream", Group: "workers",
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Close() })
	ctx := context.Background()
	if err := queue.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	outbox := Outbox{ID: "outbox-1", JobID: "job-1", RunID: "run-1"}
	streamID, err := queue.Publish(ctx, outbox)
	if err != nil {
		t.Fatal(err)
	}
	delivery, err := queue.Claim(ctx, "worker-1", time.Millisecond, 0)
	if err != nil {
		t.Fatal(err)
	}
	if delivery.MessageID != streamID || delivery.OutboxID != outbox.ID || delivery.JobID != outbox.JobID || delivery.RunID != outbox.RunID {
		t.Fatalf("delivery=%#v", delivery)
	}
	entries, err := server.Stream("test:{jobs}:stream")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || len(entries[0].Values) != 6 {
		t.Fatalf("stream entries=%#v", entries)
	}
	if err := queue.Ack(ctx, delivery); err != nil {
		t.Fatal(err)
	}
}

func TestRedisDeliveryReclaimsAbandonedPendingMessage(t *testing.T) {
	redisURL := os.Getenv("ATHENA_ASYNC_JOB_REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("ATHENA_ASYNC_JOB_REDIS_TEST_URL is not set")
	}
	stream := "test:{reclaim}:" + time.Now().UTC().Format("20060102150405.000000000")
	queue, err := NewRedisDeliveryQueue(RedisDeliveryConfig{URL: redisURL, Stream: stream, Group: "workers"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Close() })
	ctx := context.Background()
	if err := queue.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	outbox := Outbox{ID: "outbox-reclaim", JobID: "job-reclaim", RunID: "run-reclaim"}
	messageID, err := queue.Publish(ctx, outbox)
	if err != nil {
		t.Fatal(err)
	}
	first, err := queue.Claim(ctx, "worker-a", 10*time.Millisecond, 0)
	if err != nil || first.MessageID != messageID {
		t.Fatalf("first claim=%#v err=%v", first, err)
	}
	time.Sleep(25 * time.Millisecond)
	reclaimed, err := queue.Claim(ctx, "worker-b", 10*time.Millisecond, 10*time.Millisecond)
	if err != nil || reclaimed.MessageID != messageID || reclaimed.JobID != outbox.JobID {
		t.Fatalf("reclaimed=%#v err=%v", reclaimed, err)
	}
	if err := queue.Ack(ctx, reclaimed); err != nil {
		t.Fatal(err)
	}
}

func TestRedisDeliveryRenewKeepsPendingMessageWithCurrentWorker(t *testing.T) {
	redisURL := os.Getenv("ATHENA_ASYNC_JOB_REDIS_TEST_URL")
	if redisURL == "" {
		t.Skip("ATHENA_ASYNC_JOB_REDIS_TEST_URL is not set")
	}
	stream := "test:{renew}:" + time.Now().UTC().Format("20060102150405.000000000")
	queue, err := NewRedisDeliveryQueue(RedisDeliveryConfig{URL: redisURL, Stream: stream, Group: "workers"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Close() })
	ctx := context.Background()
	if err := queue.Ensure(ctx); err != nil {
		t.Fatal(err)
	}
	outbox := Outbox{ID: "outbox-renew", JobID: "job-renew", RunID: "run-renew"}
	messageID, err := queue.Publish(ctx, outbox)
	if err != nil {
		t.Fatal(err)
	}
	first, err := queue.Claim(ctx, "worker-a", 10*time.Millisecond, 0)
	if err != nil || first.MessageID != messageID {
		t.Fatalf("first claim=%#v err=%v", first, err)
	}
	time.Sleep(15 * time.Millisecond)
	if err := queue.Renew(ctx, first, "worker-a"); err != nil {
		t.Fatal(err)
	}
	if _, err := queue.Claim(ctx, "worker-b", 10*time.Millisecond, 25*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("claim after renewal err=%v, want deadline exceeded", err)
	}
	time.Sleep(30 * time.Millisecond)
	reclaimed, err := queue.Claim(ctx, "worker-b", 10*time.Millisecond, 25*time.Millisecond)
	if err != nil || reclaimed.MessageID != messageID {
		t.Fatalf("reclaimed after renewed idle=%#v err=%v", reclaimed, err)
	}
	if err := queue.Ack(ctx, reclaimed); err != nil {
		t.Fatal(err)
	}
}
