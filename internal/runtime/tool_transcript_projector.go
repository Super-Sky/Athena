// tool_transcript_projector.go persists safe, correlatable tool-call timeline entries.
// tool_transcript_projector.go 持久化安全且可关联的工具调用时间线记录。
package runtime

import (
	"context"
	"fmt"
	"time"
	"unicode/utf8"
)

// RuntimeToolTranscriptProjector projects canonical calls without persisting raw arguments or results.
// RuntimeToolTranscriptProjector 投影标准工具调用，但不持久化原始参数或结果。
type RuntimeToolTranscriptProjector struct {
	Store      RuntimePersistenceStore
	Now        func() time.Time
	RecordSet  *MinimalPersistenceRecordSet
	Transcript *ToolCallTranscript
}

// Project writes one safe trace per canonical tool call.
// Project 为每条标准工具调用写入一条安全 trace。
func (p RuntimeToolTranscriptProjector) Project(ctx context.Context) error {
	if p.Store == nil || p.RecordSet == nil || p.Transcript == nil || p.RecordSet.Run.ID == "" {
		return nil
	}
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	for index, call := range p.Transcript.Snapshot() {
		if _, err := p.Store.CreateRuntimeTrace(ctx, RuntimeTrace{
			RunID:     p.RecordSet.Run.ID,
			StepID:    p.RecordSet.Step.ID,
			TraceType: "tool_call",
			Summary:   fmt.Sprintf("tool %s call %s finished with status %s", call.Name, call.ID, call.Status),
			SafeLabels: map[string]string{
				"component":    "tool",
				"source":       "canonical_tool_transcript",
				"status":       string(call.Status),
				"tool_name":    call.Name,
				"tool_call_id": call.ID,
			},
			RedactedPayload: toolCallRedactedPayload(call),
			Metadata: map[string]any{
				"projection_source": "runtime_tool_transcript_projector",
				"redaction_policy":  "whitelist_summary",
			},
			CreatedAt: now.Add(time.Duration(index) * time.Millisecond),
		}); err != nil {
			return err
		}
	}
	return nil
}

func toolCallRedactedPayload(call ToolCall) map[string]any {
	payload := map[string]any{
		"index":          call.Index,
		"round":          call.Round,
		"tool_call_id":   call.ID,
		"tool_type":      call.Type,
		"tool_name":      call.Name,
		"status":         call.Status,
		"argument_keys":  safeJSONArgumentKeys(call.Arguments),
		"argument_runes": utf8.RuneCountInString(call.Arguments),
		"duration_ms":    call.DurationMS,
	}
	if call.StartedAt != nil {
		payload["started_at"] = call.StartedAt.UTC().Format(time.RFC3339Nano)
	}
	if call.CompletedAt != nil {
		payload["completed_at"] = call.CompletedAt.UTC().Format(time.RFC3339Nano)
	}
	if call.Result != nil {
		payload["result_runes"] = utf8.RuneCountInString(call.Result.Content)
		payload["result_is_error"] = call.Result.IsError
		payload["error_runes"] = utf8.RuneCountInString(call.Result.Error)
		payload["result_tool_call_id"] = call.Result.ToolCallID
	}
	return payload
}
