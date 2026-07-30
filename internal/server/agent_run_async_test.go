package server

import (
	"bytes"
	"context"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/sse"
	appcore "moss/internal/app"
	"moss/internal/asyncjob"
	"moss/internal/config"
)

type memoryAsyncJobStore struct {
	asyncjob.Store
	mu         sync.Mutex
	jobs       map[string]asyncjob.Job
	byIdentity map[string]string
	events     map[string][]asyncjob.Event
	nextCursor uint64
}

func newMemoryAsyncJobStore() *memoryAsyncJobStore {
	return &memoryAsyncJobStore{jobs: map[string]asyncjob.Job{}, byIdentity: map[string]string{}, events: map[string][]asyncjob.Event{}}
}

func (s *memoryAsyncJobStore) Create(_ context.Context, input asyncjob.CreateInput) (asyncjob.Job, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	identity := input.IdempotencyScope + "\x00" + input.IdempotencyKey
	if id := s.byIdentity[identity]; id != "" {
		job := s.jobs[id]
		if job.RequestSHA256 != input.RequestSHA256 {
			return asyncjob.Job{}, false, asyncjob.ErrIdempotencyConflict
		}
		return job, true, nil
	}
	now := time.Now().UTC()
	job := asyncjob.Job{ID: input.ID, RunID: input.RunID, AppID: input.AppID, WorkspaceID: input.WorkspaceID, AppInstanceID: input.AppInstanceID,
		Kind: input.Kind, Status: asyncjob.StatusDeliveryPending, RequestSHA256: input.RequestSHA256, EncryptedRequest: input.EncryptedRequest,
		IdempotencyScope: input.IdempotencyScope, IdempotencyKey: input.IdempotencyKey, MaxAttempts: input.MaxAttempts, CreatedAt: now, UpdatedAt: now}
	s.jobs[job.ID], s.byIdentity[identity] = job, job.ID
	s.appendEvent(job, asyncjob.EventTypeCreated, "", asyncjob.StatusDeliveryPending, "")
	return job, false, nil
}

func (s *memoryAsyncJobStore) Get(_ context.Context, id string) (asyncjob.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[strings.TrimSpace(id)]
	if !ok {
		return asyncjob.Job{}, asyncjob.ErrNotFound
	}
	return job, nil
}

func (s *memoryAsyncJobStore) ListEvents(_ context.Context, id string, after uint64, limit int) ([]asyncjob.Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []asyncjob.Event
	for _, event := range s.events[id] {
		if event.Cursor <= after {
			continue
		}
		result = append(result, event)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result, nil
}

func (s *memoryAsyncJobStore) RequestCancel(_ context.Context, id, reason string) (asyncjob.Job, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return asyncjob.Job{}, asyncjob.ErrNotFound
	}
	if job.Terminal() {
		return job, asyncjob.ErrAlreadyTerminal
	}
	from := job.Status
	job.Status, job.CancelReason, job.CancelRequestedAt, job.UpdatedAt = asyncjob.StatusCancelled, reason, time.Now().UTC(), time.Now().UTC()
	s.jobs[id] = job
	s.appendEvent(job, asyncjob.EventTypeCancelled, from, job.Status, reason)
	return job, nil
}

func (s *memoryAsyncJobStore) appendEvent(job asyncjob.Job, eventType asyncjob.EventType, from, to asyncjob.Status, reason string) {
	s.nextCursor++
	s.events[job.ID] = append(s.events[job.ID], asyncjob.Event{Cursor: s.nextCursor, JobID: job.ID, RunID: job.RunID,
		Type: eventType, FromStatus: from, ToStatus: to, Reason: reason, OccurredAt: time.Now().UTC()})
}

func TestAsyncAgentRunIsIdempotentAcrossGeneratedRequestIDs(t *testing.T) {
	server, _ := newAsyncAgentRunHTTPServer(t)
	body := `{"goal":"analyze portfolio","workspace_id":"workspace-a","app_instance_id":"fund-web"}`
	first := performAsyncAgentRunRequest(server, body, "request-key-1")
	if first.Code != consts.StatusAccepted || strings.Contains(first.Body.String(), `"duplicate":true`) {
		t.Fatalf("first response status=%d body=%s", first.Code, first.Body.String())
	}
	second := performAsyncAgentRunRequest(server, body, "request-key-1")
	if second.Code != consts.StatusAccepted || !strings.Contains(second.Body.String(), `"duplicate":true`) {
		t.Fatalf("second response status=%d body=%s", second.Code, second.Body.String())
	}
	conflict := performAsyncAgentRunRequest(server, `{"goal":"different goal","workspace_id":"workspace-a","app_instance_id":"fund-web"}`, "request-key-1")
	if conflict.Code != consts.StatusConflict || !strings.Contains(conflict.Body.String(), "idempotency_conflict") {
		t.Fatalf("conflict status=%d body=%s", conflict.Code, conflict.Body.String())
	}
}

func TestAsyncAgentRunReadCancelAndEventCursor(t *testing.T) {
	server, store := newAsyncAgentRunHTTPServer(t)
	created := performAsyncAgentRunRequest(server, `{"goal":"analyze portfolio","workspace_id":"workspace-a","app_instance_id":"fund-web"}`, "request-key-2")
	location := string(created.Header().Peek("Location"))
	if location == "" {
		t.Fatalf("create response missing Location: %s", created.Body.String())
	}
	read := ut.PerformRequest(server.engine.Engine, http.MethodGet, location, nil)
	if read.Code != consts.StatusOK || !strings.Contains(read.Body.String(), `"status":"delivery_pending"`) {
		t.Fatalf("read status=%d body=%s", read.Code, read.Body.String())
	}
	cancelBody := bytes.NewBufferString(`{"reason":"user requested"}`)
	cancelled := ut.PerformRequest(server.engine.Engine, http.MethodPost, location+"/cancel", &ut.Body{Body: cancelBody, Len: cancelBody.Len()},
		ut.Header{Key: "Content-Type", Value: "application/json"})
	if cancelled.Code != consts.StatusAccepted || !strings.Contains(cancelled.Body.String(), `"status":"cancelled"`) {
		t.Fatalf("cancel status=%d body=%s", cancelled.Code, cancelled.Body.String())
	}
	store.mu.Lock()
	createdCursor := store.events[strings.TrimPrefix(location, "/api/agent/runs/")][0].Cursor
	store.mu.Unlock()
	jobID := strings.TrimPrefix(location, "/api/agent/runs/")
	events, err := store.ListEvents(context.Background(), jobID, createdCursor, 100)
	if err != nil {
		t.Fatal(err)
	}
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(hertzapp.NewContext(0), writer)
	lastCursor, err := publishAsyncAgentRunEvents(stream, events)
	if err != nil || lastCursor <= createdCursor || strings.Contains(writer.String(), "event:created") || !strings.Contains(writer.String(), "event:cancelled") {
		t.Fatalf("events cursor=%d err=%v body=%s", lastCursor, err, writer.String())
	}
}

func newAsyncAgentRunHTTPServer(t *testing.T) (*HTTPServer, *memoryAsyncJobStore) {
	t.Helper()
	cfg := config.Config{Server: config.ServerConfig{HTTPPort: 8080}, Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
		AsyncJobs: config.AsyncJobConfig{Enabled: true, MaxAttempts: 3}}
	application := &appcore.Service{}
	store := newMemoryAsyncJobStore()
	codec, err := asyncjob.NewCodec("0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	service, err := NewAgentRunAsyncService(cfg, application, store, codec)
	if err != nil {
		t.Fatal(err)
	}
	return NewHTTPServerWithAsyncJobs(cfg, application, service), store
}

func performAsyncAgentRunRequest(server *HTTPServer, body, key string) *ut.ResponseRecorder {
	payload := bytes.NewBufferString(body)
	return ut.PerformRequest(server.engine.Engine, http.MethodPost, "/api/agent/runs", &ut.Body{Body: payload, Len: payload.Len()},
		ut.Header{Key: "Content-Type", Value: "application/json"}, ut.Header{Key: "Prefer", Value: "respond-async"},
		ut.Header{Key: asyncAgentRunIdempotencyKey, Value: key})
}

var _ asyncjob.Store = (*memoryAsyncJobStore)(nil)
