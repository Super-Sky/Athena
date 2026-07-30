package server

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"

	miniredis "github.com/alicebob/miniredis/v2"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	appcore "moss/internal/app"
	"moss/internal/asyncjob"
	"moss/internal/config"
)

func TestAsyncAgentRunHTTPDispatcherWorkerAndRead(t *testing.T) {
	dsn := os.Getenv("ATHENA_ASYNC_JOB_PG_TEST_DSN")
	if dsn == "" {
		t.Skip("ATHENA_ASYNC_JOB_PG_TEST_DSN is not set")
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	store := asyncjob.NewPostgresStore(db)
	if err := store.AutoMigrate(context.Background()); err != nil {
		t.Fatal(err)
	}
	redisServer := miniredis.RunT(t)
	queue, err := asyncjob.NewRedisDeliveryQueue(asyncjob.RedisDeliveryConfig{URL: "redis://" + redisServer.Addr(), Stream: "test:{agent-run}:stream", Group: "workers"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = queue.Close() })

	cfg := config.Config{Server: config.ServerConfig{HTTPPort: 8080}, Runtime: config.RuntimeConfig{
		MaxConcurrentRequests: 4, MaxConcurrentTools: 2, RequestTimeoutSeconds: 30, DeferredQueueLimit: 4, ClosedTokenTTLSecs: 3600,
	}, AsyncJobs: config.AsyncJobConfig{Enabled: true, MaxAttempts: 2}}
	runtimeStore := &testRuntimeReadStore{}
	application := appcore.NewServiceWithRuntimeStore(cfg, nil, nil, nil, runtimeStore)
	application.FastPath = &agentRunTestFastPath{store: runtimeStore}
	codec, err := asyncjob.NewCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAgentRunAsyncService(cfg, application, store, codec)
	if err != nil {
		t.Fatal(err)
	}
	httpServer := NewHTTPServerWithAsyncJobs(cfg, application, service)
	created := performAsyncAgentRunRequest(httpServer, `{"goal":"analyze portfolio","workspace_id":"workspace-a","app_instance_id":"fund-web"}`, "integration-key-"+time.Now().Format("150405.000000000"))
	if created.Code != consts.StatusAccepted {
		t.Fatalf("create status=%d body=%s", created.Code, created.Body.String())
	}
	var accepted asyncAgentRunHTTPResponse
	if err := json.Unmarshal(created.Body.Bytes(), &accepted); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		db.Exec("DELETE FROM runtime_async_job_events WHERE job_id = ?", accepted.JobID)
		db.Exec("DELETE FROM runtime_job_outbox WHERE job_id = ?", accepted.JobID)
		db.Exec("DELETE FROM runtime_async_jobs WHERE id = ?", accepted.JobID)
	})
	dispatcher, err := asyncjob.NewDispatcher(store, queue, asyncjob.DispatcherConfig{Owner: "dispatcher-test", Lease: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if delivered, err := dispatcher.DispatchOnce(context.Background(), time.Now().UTC().Add(time.Second)); err != nil || delivered != 1 {
		t.Fatalf("dispatch delivered=%d err=%v", delivered, err)
	}
	worker, err := asyncjob.NewWorker(store, queue, service.Handler(), asyncjob.WorkerConfig{
		Consumer: "worker-test", Block: 10 * time.Millisecond, Visibility: time.Second, CancelPoll: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- worker.Run(workerCtx) }()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		job, getErr := store.Get(context.Background(), accepted.JobID)
		if getErr == nil && job.Terminal() {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	cancelWorker()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	read := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, accepted.EventsURL[:len(accepted.EventsURL)-len("/events")], nil)
	if read.Code != consts.StatusOK {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	var completed asyncAgentRunHTTPResponse
	if err := json.Unmarshal(read.Body.Bytes(), &completed); err != nil {
		t.Fatal(err)
	}
	if completed.Status != asyncjob.StatusCompleted || completed.Result == nil || completed.RuntimeRunID == "" || completed.Result.Output == "" {
		t.Fatalf("completed response=%#v body=%s", completed, read.Body.String())
	}
}
