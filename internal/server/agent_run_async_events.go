// agent_run_async_events.go replays durable job lifecycle events as resumable SSE.
// agent_run_async_events.go 将持久任务生命周期事件回放为可续传 SSE。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/sse"
	"moss/internal/asyncjob"
)

type asyncAgentRunEventPayload struct {
	Cursor     uint64          `json:"cursor"`
	JobID      string          `json:"job_id"`
	RunID      string          `json:"run_id"`
	Type       string          `json:"type"`
	FromStatus asyncjob.Status `json:"from_status,omitempty"`
	ToStatus   asyncjob.Status `json:"to_status,omitempty"`
	Reason     string          `json:"reason,omitempty"`
	OccurredAt time.Time       `json:"occurred_at"`
}

func (s *AgentRunAsyncService) streamEvents(ctx context.Context, c *app.RequestContext, id string) {
	if strings.TrimSpace(id) == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "run_id_required"})
		return
	}
	job, err := s.store.Get(ctx, id)
	if errors.Is(err, asyncjob.ErrNotFound) || (err == nil && !authorizeAsyncJob(ctx, job)) {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "resource_not_found"})
		return
	}
	if err != nil {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": "async_job_persistence_unavailable"})
		return
	}
	cursor, err := parseAsyncEventCursor(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid_event_cursor"})
		return
	}
	stream := sse.NewStream(c)
	heartbeat := time.NewTicker(asyncAgentRunEventHeartbeat)
	poll := time.NewTicker(asyncAgentRunEventPoll)
	defer heartbeat.Stop()
	defer poll.Stop()

	for {
		events, listErr := s.store.ListEvents(ctx, id, cursor, 100)
		if listErr != nil {
			return
		}
		if len(events) > 0 {
			cursor, err = publishAsyncAgentRunEvents(stream, events)
			if err != nil {
				return
			}
		}
		job, err = s.store.Get(ctx, id)
		if err != nil {
			return
		}
		if job.Terminal() && len(events) == 0 {
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-heartbeat.C:
			data, _ := json.Marshal(map[string]any{"job_id": job.ID, "run_id": job.RunID, "status": job.Status})
			if stream.Publish(&sse.Event{Event: "heartbeat", Data: data}) != nil {
				return
			}
		case <-poll.C:
		}
	}
}

func publishAsyncAgentRunEvents(stream *sse.Stream, events []asyncjob.Event) (uint64, error) {
	var cursor uint64
	for _, event := range events {
		payload := asyncAgentRunEventPayload{
			Cursor: event.Cursor, JobID: event.JobID, RunID: event.RunID, Type: string(event.Type),
			FromStatus: event.FromStatus, ToStatus: event.ToStatus, Reason: event.Reason, OccurredAt: event.OccurredAt,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			return cursor, err
		}
		if err := stream.Publish(&sse.Event{ID: strconv.FormatUint(event.Cursor, 10), Event: string(event.Type), Data: data}); err != nil {
			return cursor, err
		}
		cursor = event.Cursor
	}
	return cursor, nil
}
