// trace_timeline.go projects persisted runtime records into one safe agent timeline.
// trace_timeline.go 将已持久化的 runtime 记录投影为一条安全的 agent 时间线。
package server

import (
	"context"
	"sort"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
)

// agentTraceTimelineItem is a stable, app-readable view over existing safe runtime records.
// agentTraceTimelineItem 是既有安全 runtime 记录的稳定、面向应用的视图。
type agentTraceTimelineItem struct {
	ID         string         `json:"id"`
	Kind       string         `json:"kind"`
	Timestamp  time.Time      `json:"timestamp"`
	DurationMS *int64         `json:"duration_ms,omitempty"`
	Status     string         `json:"status,omitempty"`
	Source     string         `json:"source"`
	Summary    string         `json:"summary"`
	StepID     string         `json:"step_id,omitempty"`
	Error      map[string]any `json:"error,omitempty"`
	Detail     map[string]any `json:"detail,omitempty"`
}

// agentTraceTimelineSummary provides list-level debug counters without duplicating runtime data.
// agentTraceTimelineSummary 提供列表级调试计数，不复制 runtime 数据。
type agentTraceTimelineSummary struct {
	RunID        string     `json:"run_id"`
	ItemCount    int        `json:"item_count"`
	FailureCount int        `json:"failure_count"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

type agentTraceTimelineResponse struct {
	Run     runtimeRunDTO             `json:"run"`
	Items   []agentTraceTimelineItem  `json:"items"`
	Summary agentTraceTimelineSummary `json:"summary"`
}

// handleGetAgentRunTimeline exposes a business-app-readable safe trace timeline.
// handleGetAgentRunTimeline 暴露面向业务应用可读的安全 trace 时间线。
func handleGetAgentRunTimeline(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	response, err := loadAgentRunTimeline(ctx, application, string(c.Param("runID")))
	if err != nil {
		writeAgentRunReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, response)
}

// handleGetControlPlaneRuntimeTimeline exposes the same projection behind Control Plane auth.
// handleGetControlPlaneRuntimeTimeline 在 Control Plane 认证后暴露同一份投影。
func handleGetControlPlaneRuntimeTimeline(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	response, err := loadAgentRunTimeline(ctx, application, string(c.Param("runID")))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, response)
}

func loadAgentRunTimeline(ctx context.Context, application *appcore.Service, runID string) (agentTraceTimelineResponse, error) {
	readout, err := readAgentRunTrace(ctx, application, runID)
	if err != nil {
		return agentTraceTimelineResponse{}, err
	}
	items := projectAgentTraceTimeline(readout)
	return agentTraceTimelineResponse{
		Run:   readout.Run,
		Items: items,
		Summary: agentTraceTimelineSummary{
			RunID:        readout.Run.ID,
			ItemCount:    len(items),
			FailureCount: countTimelineFailures(items),
			StartedAt:    readout.Run.StartedAt,
			CompletedAt:  readout.Run.CompletedAt,
		},
	}, nil
}

func projectAgentTraceTimeline(readout agentRunTraceReadout) []agentTraceTimelineItem {
	items := make([]agentTraceTimelineItem, 0, len(readout.Steps)+len(readout.Events)+len(readout.Traces)+len(readout.Usage)+len(readout.Projections))
	for _, step := range readout.Steps {
		items = append(items, agentTraceTimelineItem{
			ID:         "step:" + step.ID,
			Kind:       "loop_step",
			Timestamp:  firstRuntimeTime(step.StartedAt, step.CreatedAt),
			DurationMS: runtimeDurationMilliseconds(step.StartedAt, step.CompletedAt),
			Status:     step.Status,
			Source:     "runtime",
			Summary:    firstTimelineText(step.Name, step.StepType, step.ID),
			StepID:     step.ID,
			Error:      timelineError(step.Status, step.Metadata),
			Detail: map[string]any{
				"sequence":  step.Sequence,
				"step_type": step.StepType,
				"metadata":  step.Metadata,
			},
		})
	}
	for _, event := range readout.Events {
		items = append(items, agentTraceTimelineItem{
			ID:        "event:" + event.ID,
			Kind:      "lifecycle",
			Timestamp: event.OccurredAt,
			Status:    event.ToStatus,
			Source:    "runtime",
			Summary:   firstTimelineText(event.Reason, event.EventType, event.SubjectID),
			StepID:    event.StepID,
			Error:     timelineError(event.ToStatus, event.Metadata),
			Detail: map[string]any{
				"event_type":   event.EventType,
				"subject_type": event.SubjectType,
				"subject_id":   event.SubjectID,
				"from_status":  event.FromStatus,
				"metadata":     event.Metadata,
			},
		})
	}
	for _, trace := range readout.Traces {
		items = append(items, agentTraceTimelineItem{
			ID:        "trace:" + trace.ID,
			Kind:      timelineTraceKind(trace.TraceType),
			Timestamp: trace.CreatedAt,
			Status:    timelineTraceStatus(trace.Metadata),
			Source:    timelineTraceSource(trace),
			Summary:   trace.Summary,
			StepID:    trace.StepID,
			Error:     timelineError(timelineTraceStatus(trace.Metadata), trace.Metadata),
			Detail: map[string]any{
				"trace_type":       trace.TraceType,
				"safe_labels":      trace.SafeLabels,
				"redacted_payload": trace.RedactedPayload,
				"metadata":         trace.Metadata,
			},
		})
	}
	for _, usage := range readout.Usage {
		items = append(items, agentTraceTimelineItem{
			ID:        "usage:" + usage.ID,
			Kind:      "usage",
			Timestamp: usage.CreatedAt,
			Status:    "recorded",
			Source:    firstTimelineText(usage.Provider, usage.ResourceType, "runtime"),
			Summary:   firstTimelineText(usage.ResourceName, usage.ResourceType, usage.ID),
			StepID:    usage.StepID,
			Detail: map[string]any{
				"resource_type": usage.ResourceType,
				"resource_name": usage.ResourceName,
				"unit":          usage.Unit,
				"amount":        usage.Amount,
				"currency":      usage.Currency,
				"metadata":      usage.Metadata,
			},
		})
	}
	for _, projection := range readout.Projections {
		items = append(items, agentTraceTimelineItem{
			ID:        "projection:" + projection.ID,
			Kind:      "delivery",
			Timestamp: projection.CreatedAt,
			Status:    projection.Status,
			Source:    "runtime",
			Summary:   firstTimelineText(projection.Summary, projection.CandidateKind, projection.ID),
			StepID:    projection.StepID,
			Error:     timelineError(projection.Status, projection.Metadata),
			Detail: map[string]any{
				"candidate_kind":   projection.CandidateKind,
				"schema_version":   projection.SchemaVersion,
				"redacted_payload": projection.RedactedPayload,
				"metadata":         projection.Metadata,
			},
		})
	}
	sort.SliceStable(items, func(left, right int) bool {
		if items[left].Timestamp.Equal(items[right].Timestamp) {
			return items[left].ID < items[right].ID
		}
		return items[left].Timestamp.Before(items[right].Timestamp)
	})
	return items
}

func firstRuntimeTime(preferred *time.Time, fallback time.Time) time.Time {
	if preferred != nil {
		return *preferred
	}
	return fallback
}

func runtimeDurationMilliseconds(start, end *time.Time) *int64 {
	if start == nil || end == nil || end.Before(*start) {
		return nil
	}
	value := end.Sub(*start).Milliseconds()
	return &value
}

func firstTimelineText(values ...string) string {
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			return value
		}
	}
	return "runtime record"
}

func timelineTraceKind(traceType string) string {
	traceType = strings.ToLower(strings.TrimSpace(traceType))
	switch {
	case strings.Contains(traceType, "model"):
		return "model_call"
	case strings.Contains(traceType, "governance"):
		return "governance"
	case strings.Contains(traceType, "context") || strings.Contains(traceType, "memory"):
		return "context"
	case strings.Contains(traceType, "tool"):
		return "tool_call"
	default:
		return "trace"
	}
}

func timelineTraceSource(trace runtimeTraceDTO) string {
	if trace.SafeLabels != nil {
		if source := strings.TrimSpace(trace.SafeLabels["provider"]); source != "" {
			return source
		}
		if source := strings.TrimSpace(trace.SafeLabels["component"]); source != "" {
			return source
		}
	}
	return "runtime"
}

func timelineTraceStatus(metadata map[string]any) string {
	if metadata != nil {
		if status, ok := metadata["status"].(string); ok && strings.TrimSpace(status) != "" {
			return strings.TrimSpace(status)
		}
	}
	return "recorded"
}

func timelineError(status string, metadata map[string]any) map[string]any {
	status = strings.ToLower(strings.TrimSpace(status))
	if !strings.Contains(status, "fail") && !strings.Contains(status, "error") && status != "deny" && status != "denied" {
		return nil
	}
	result := map[string]any{"status": status}
	if metadata != nil {
		for _, key := range []string{"error", "error_code", "reason", "decision_id"} {
			if value, ok := metadata[key]; ok {
				result[key] = value
			}
		}
	}
	return result
}

func countTimelineFailures(items []agentTraceTimelineItem) int {
	count := 0
	for _, item := range items {
		if len(item.Error) > 0 {
			count++
		}
	}
	return count
}
