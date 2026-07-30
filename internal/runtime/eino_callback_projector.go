// eino_callback_projector.go projects safe Eino callback events into runtime persistence.
// eino_callback_projector.go 将安全 Eino callback event 投影到 runtime persistence。
package runtime

import (
	"context"
	"fmt"
	"time"
)

// RuntimeCallbackProjector persists fine-grained model/tool callback traces and usage records.
// RuntimeCallbackProjector 持久化细粒度 model/tool callback trace 与 usage 记录。
type RuntimeCallbackProjector struct {
	Store         RuntimePersistenceStore
	PayloadStore  PrivilegedTracePayloadStore
	PayloadPolicy PrivilegedTracePayloadPolicy
	Now           func() time.Time
	RecordSet     *MinimalPersistenceRecordSet
	Recorder      *RuntimeCallbackRecorder
}

// Project writes all currently recorded callback events as safe trace and generic usage records.
// Project 将当前已记录 callback event 写成安全 trace 和通用 usage 记录。
func (p RuntimeCallbackProjector) Project(ctx context.Context) error {
	if p.Store == nil || p.RecordSet == nil || p.Recorder == nil || p.RecordSet.Run.ID == "" {
		return nil
	}
	events := p.Recorder.Events()
	if len(events) == 0 {
		return nil
	}
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	for index, event := range events {
		createdAt := now.Add(time.Duration(index) * time.Millisecond)
		if err := p.projectEvent(ctx, event, createdAt); err != nil {
			return err
		}
	}
	return nil
}

func (p RuntimeCallbackProjector) projectEvent(ctx context.Context, event RuntimeComponentCallbackEvent, createdAt time.Time) error {
	traceType := "eino_" + event.Component + "_callback"
	traceID := defaultID("")
	payloadMetadata := projectPrivilegedTracePayload(
		ctx, p.PayloadStore, p.PayloadPolicy, p.RecordSet.Run.WorkspaceID, event.Component,
		p.RecordSet.Run.ID, p.RecordSet.Step.ID, traceID, traceType, "eino_callbacks", event.PrivilegedPayload, createdAt,
	)
	if _, err := p.Store.CreateRuntimeTrace(ctx, RuntimeTrace{
		ID:        traceID,
		RunID:     p.RecordSet.Run.ID,
		StepID:    p.RecordSet.Step.ID,
		TraceType: traceType,
		Summary:   callbackTraceSummary(event),
		SafeLabels: map[string]string{
			"component":     event.Component,
			"source":        "eino_callbacks",
			"status":        event.Status,
			"resource_name": event.ResourceName,
		},
		RedactedPayload: callbackRedactedPayload(event),
		Metadata:        mergeAnyMaps(mergeAnyMaps(event.Metadata, map[string]any{"projection_source": "runtime_callback_projector"}), payloadMetadata),
		CreatedAt:       createdAt,
	}); err != nil {
		return err
	}
	for _, usage := range callbackUsageRecords(p.RecordSet, event, createdAt.Add(time.Millisecond)) {
		if _, err := p.Store.CreateUsage(ctx, usage); err != nil {
			return err
		}
	}
	return nil
}

func callbackTraceSummary(event RuntimeComponentCallbackEvent) string {
	return fmt.Sprintf("eino %s callback observed for %s with status %s", event.Component, defaultString(event.ResourceName, "unknown"), event.Status)
}

func callbackRedactedPayload(event RuntimeComponentCallbackEvent) map[string]any {
	return map[string]any{
		"component":         event.Component,
		"node":              event.Node,
		"provider":          event.Provider,
		"resource_name":     event.ResourceName,
		"status":            event.Status,
		"duration_ms":       event.DurationMS,
		"input_count":       event.InputCount,
		"input_runes":       event.InputRuneCount,
		"output_runes":      event.OutputRuneCount,
		"tool_call_count":   event.ToolCallCount,
		"prompt_tokens":     event.PromptTokens,
		"completion_tokens": event.CompletionTokens,
		"total_tokens":      event.TotalTokens,
		"cached_tokens":     event.CachedTokens,
		"reasoning_tokens":  event.ReasoningTokens,
		"error_summary":     event.ErrorSummary,
	}
}

func callbackUsageRecords(recordSet *MinimalPersistenceRecordSet, event RuntimeComponentCallbackEvent, createdAt time.Time) []Usage {
	if recordSet == nil {
		return nil
	}
	base := Usage{
		RunID:        recordSet.Run.ID,
		StepID:       recordSet.Step.ID,
		ResourceType: event.Component,
		Provider:     defaultString(event.Provider, "eino"),
		ResourceName: defaultString(event.ResourceName, event.Component),
		Metadata: map[string]any{
			"generic_usage": true,
			"status":        event.Status,
			"duration_ms":   event.DurationMS,
		},
		CreatedAt: createdAt,
	}
	if event.Component == runtimeCallbackComponentModel && event.TotalTokens > 0 {
		total := base
		total.Unit = "token"
		total.Amount = float64(event.TotalTokens)
		total.Metadata = mergeAnyMaps(base.Metadata, map[string]any{
			"token_scope":       "total",
			"prompt_tokens":     event.PromptTokens,
			"completion_tokens": event.CompletionTokens,
			"cached_tokens":     event.CachedTokens,
			"reasoning_tokens":  event.ReasoningTokens,
		})
		return []Usage{total}
	}
	invocation := base
	invocation.Unit = "operation"
	invocation.Amount = 1
	return []Usage{invocation}
}
