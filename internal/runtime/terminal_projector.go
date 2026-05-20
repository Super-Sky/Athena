// terminal_projector.go projects final runtime outcomes into safe persistence records.
// terminal_projector.go 将 runtime 最终结果投影为安全的持久化记录。
package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// RuntimeTerminalStatusCompleted names a completed terminal outcome.
	// RuntimeTerminalStatusCompleted 表示一次已完成的终态结果。
	RuntimeTerminalStatusCompleted = "completed"
	RuntimeTerminalStatusFailed    = "failed"
)

// RuntimeTerminalOutcome is the safe terminal signal emitted after the runner finishes.
// RuntimeTerminalOutcome 表示 runner 完成后向 runtime persistence 回传的安全终态信号。
type RuntimeTerminalOutcome struct {
	Status          string
	Content         string
	Error           error
	ToolSideEffects bool
	Metadata        map[string]any
}

// RuntimeTerminalProjector writes terminal output summaries through the runtime persistence boundary.
// RuntimeTerminalProjector 通过 runtime persistence 边界写入最终输出摘要。
type RuntimeTerminalProjector struct {
	Store     RuntimePersistenceStore
	Now       func() time.Time
	RecordSet *MinimalPersistenceRecordSet
	Metadata  map[string]any
	Callbacks *RuntimeCallbackRecorder
}

// ProjectTerminalOutcome records a final runner outcome when terminal projection is configured.
// ProjectTerminalOutcome 会在配置了终态投影器时记录最终 runner 结果。
func (p *PreparedExecution) ProjectTerminalOutcome(ctx context.Context, outcome RuntimeTerminalOutcome) error {
	if p == nil || p.TerminalProjector == nil {
		return nil
	}
	return p.TerminalProjector.Project(ctx, outcome)
}

// Project writes terminal lifecycle, trace, usage, and candidate-output records.
// Project 写入终态 lifecycle、trace、usage 和 candidate-output 记录。
func (p *RuntimeTerminalProjector) Project(ctx context.Context, outcome RuntimeTerminalOutcome) error {
	if p == nil || p.Store == nil || p.RecordSet == nil || strings.TrimSpace(p.RecordSet.Run.ID) == "" {
		return nil
	}
	if transactor, ok := p.Store.(RuntimePersistenceTransactor); ok {
		return transactor.WithTransaction(ctx, func(txStore RuntimePersistenceStore) error {
			txProjector := *p
			txProjector.Store = txStore
			return txProjector.project(ctx, outcome)
		})
	}
	return p.project(ctx, outcome)
}

func (p RuntimeTerminalProjector) project(ctx context.Context, outcome RuntimeTerminalOutcome) error {
	now := time.Now().UTC()
	if p.Now != nil {
		now = p.Now().UTC()
	}
	run := p.RecordSet.Run
	step := p.RecordSet.Step
	status := normalizeRuntimeTerminalStatus(outcome.Status, outcome.Error)
	runToStatus := TaskRunStatusCompleted
	stepToStatus := TaskStepStatusSuccess
	if status == RuntimeTerminalStatusFailed {
		runToStatus = TaskRunStatusFailed
		stepToStatus = TaskStepStatusFailed
	}
	summary := safeTerminalContentSummary(outcome.Content, outcome.Error)
	metadata := mergeAnyMaps(p.Metadata, outcome.Metadata)
	metadata = mergeAnyMaps(metadata, map[string]any{
		"projection_source": "runtime_terminal_projector",
		"redaction_policy":  "whitelist_summary",
	})

	if _, err := p.Store.CreateLifecycleEvent(ctx, TaskRunLifecycleEvent{
		RunID:       run.ID,
		StepID:      step.ID,
		EventType:   "step_terminal_observed",
		SubjectType: LifecycleSubjectStep,
		SubjectID:   step.ID,
		FromStatus:  step.Status,
		ToStatus:    stepToStatus,
		Reason:      "runner_terminal_outcome_observed",
		Metadata:    map[string]any{"safe_label": "step_terminal_observed", "terminal_status": status},
		OccurredAt:  now,
	}); err != nil {
		return err
	}
	if _, err := p.Store.CreateLifecycleEvent(ctx, TaskRunLifecycleEvent{
		RunID:       run.ID,
		EventType:   "run_terminal_observed",
		SubjectType: LifecycleSubjectRun,
		SubjectID:   run.ID,
		FromStatus:  run.Status,
		ToStatus:    runToStatus,
		Reason:      "runner_terminal_outcome_observed",
		Metadata:    map[string]any{"safe_label": "run_terminal_observed", "terminal_status": status},
		OccurredAt:  now.Add(time.Millisecond),
	}); err != nil {
		return err
	}
	if _, err := p.Store.CreateRuntimeTrace(ctx, RuntimeTrace{
		RunID:     run.ID,
		StepID:    step.ID,
		TraceType: "terminal_output_summary",
		Summary:   summary,
		SafeLabels: map[string]string{
			"component": "internal/runtime",
			"source":    "eino_runtime_graph",
			"status":    status,
		},
		RedactedPayload: map[string]any{
			"content_summary":   summary,
			"content_runes":     utf8.RuneCountInString(outcome.Content),
			"tool_side_effects": outcome.ToolSideEffects,
			"error_summary":     safeTerminalErrorSummary(outcome.Error),
		},
		Metadata:  metadata,
		CreatedAt: now.Add(2 * time.Millisecond),
	}); err != nil {
		return err
	}
	if callbackCount := callbackEventCount(metadata); callbackCount > 0 {
		if _, err := p.Store.CreateRuntimeTrace(ctx, RuntimeTrace{
			RunID:     run.ID,
			StepID:    step.ID,
			TraceType: "callback_summary",
			Summary:   fmt.Sprintf("eino callback metadata observed (%d events)", callbackCount),
			SafeLabels: map[string]string{
				"component": "internal/runtime",
				"source":    "eino_callbacks",
			},
			RedactedPayload: map[string]any{
				"callback_event_count": callbackCount,
			},
			Metadata:  metadata,
			CreatedAt: now.Add(3 * time.Millisecond),
		}); err != nil {
			return err
		}
		if _, err := p.Store.CreateUsage(ctx, Usage{
			RunID:        run.ID,
			StepID:       step.ID,
			ResourceType: "eino_callback",
			Provider:     "eino",
			ResourceName: "graph_callback_events",
			Unit:         "event",
			Amount:       float64(callbackCount),
			Metadata:     map[string]any{"generic_usage": true},
			CreatedAt:    now.Add(4 * time.Millisecond),
		}); err != nil {
			return err
		}
	}
	if _, err := p.Store.CreateUsage(ctx, Usage{
		RunID:        run.ID,
		StepID:       step.ID,
		ResourceType: "runtime_output",
		Provider:     "athena",
		ResourceName: "assistant_output_summary",
		Unit:         "character",
		Amount:       float64(utf8.RuneCountInString(outcome.Content)),
		Metadata: map[string]any{
			"generic_usage":     true,
			"tool_side_effects": outcome.ToolSideEffects,
		},
		CreatedAt: now.Add(5 * time.Millisecond),
	}); err != nil {
		return err
	}
	if err := (RuntimeCallbackProjector{
		Store:     p.Store,
		Now:       p.Now,
		RecordSet: p.RecordSet,
		Recorder:  p.Callbacks,
	}).Project(ctx); err != nil {
		return err
	}
	_, err := p.Store.CreateProjectionCandidate(ctx, ProjectionCandidate{
		RunID:         run.ID,
		StepID:        step.ID,
		CandidateKind: "terminal_output",
		Status:        status,
		Summary:       summary,
		RedactedPayload: map[string]any{
			"summary":           summary,
			"content_runes":     utf8.RuneCountInString(outcome.Content),
			"tool_side_effects": outcome.ToolSideEffects,
		},
		Metadata:  metadata,
		CreatedAt: now.Add(6 * time.Millisecond),
	})
	return err
}

func normalizeRuntimeTerminalStatus(status string, err error) string {
	if err != nil {
		return RuntimeTerminalStatusFailed
	}
	switch strings.TrimSpace(status) {
	case RuntimeTerminalStatusFailed:
		return RuntimeTerminalStatusFailed
	default:
		return RuntimeTerminalStatusCompleted
	}
}

func safeTerminalContentSummary(content string, err error) string {
	if err != nil {
		return safeTerminalErrorSummary(err)
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "runtime terminal output was empty"
	}
	return fmt.Sprintf("runtime terminal output observed (%d characters)", utf8.RuneCountInString(trimmed))
}

func safeTerminalErrorSummary(err error) string {
	if err == nil {
		return ""
	}
	return fmt.Sprintf("runtime terminal error observed (%d characters)", utf8.RuneCountInString(fmt.Sprintf("%v", err)))
}

func callbackEventCount(metadata map[string]any) int {
	value := metadata["callback_events"]
	switch typed := value.(type) {
	case []map[string]string:
		return len(typed)
	case []map[string]any:
		return len(typed)
	case []any:
		return len(typed)
	default:
		return 0
	}
}
