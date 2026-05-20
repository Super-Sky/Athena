// persistence_writer.go provides a deterministic internal writer for minimal runtime persistence.
// persistence_writer.go 提供最小 runtime 持久化的内部确定性写入器。
package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	runtimetask "moss/internal/runtime/task"
)

// PersistenceWriter writes a deterministic minimal record set through the runtime store boundary.
// PersistenceWriter 通过 runtime store 边界写入一组确定性的最小记录。
type PersistenceWriter struct {
	Store RuntimePersistenceStore
	Now   func() time.Time
}

// MinimalPersistenceInput contains the universal runtime fields needed for one deterministic write.
// MinimalPersistenceInput 保存一次确定性写入所需的通用 runtime 字段。
type MinimalPersistenceInput struct {
	Task             *runtimetask.RuntimeTask
	IdempotencyScope string
	IdempotencyKey   string
	RetentionPolicy  string
	Metadata         map[string]any
}

// MinimalPersistenceRecordSet returns the IDs and records written by one deterministic write.
// MinimalPersistenceRecordSet 返回一次确定性写入落库的 ID 和记录。
type MinimalPersistenceRecordSet struct {
	Run        TaskRun
	Step       TaskStep
	Events     []TaskRunLifecycleEvent
	Trace      RuntimeTrace
	Usage      Usage
	Projection ProjectionCandidate
}

// WriteMinimalRun writes one run, one step, lifecycle events, trace, usage, and candidate output.
// WriteMinimalRun 写入一个 run、一个 step、生命周期事件、trace、usage 和候选输出。
func (w PersistenceWriter) WriteMinimalRun(ctx context.Context, input MinimalPersistenceInput) (MinimalPersistenceRecordSet, error) {
	if w.Store == nil {
		return MinimalPersistenceRecordSet{}, fmt.Errorf("runtime persistence store is required")
	}
	if transactor, ok := w.Store.(RuntimePersistenceTransactor); ok {
		var recordSet MinimalPersistenceRecordSet
		err := transactor.WithTransaction(ctx, func(txStore RuntimePersistenceStore) error {
			txWriter := PersistenceWriter{Store: txStore, Now: w.Now}
			written, err := txWriter.writeMinimalRun(ctx, input)
			if err != nil {
				return err
			}
			recordSet = written
			return nil
		})
		return recordSet, err
	}
	return w.writeMinimalRun(ctx, input)
}

func (w PersistenceWriter) writeMinimalRun(ctx context.Context, input MinimalPersistenceInput) (MinimalPersistenceRecordSet, error) {
	now := time.Now().UTC()
	if w.Now != nil {
		now = w.Now().UTC()
	}
	task := input.Task
	if task == nil {
		task = &runtimetask.RuntimeTask{
			TaskID:    "runtime-task-" + uuid.NewString(),
			TaskType:  "runtime_task",
			InputKind: "runtime_task",
			UserGoal:  "persist minimal runtime records",
		}
	}

	run, err := w.Store.CreateTaskRun(ctx, TaskRun{
		ID:               uuid.NewString(),
		TaskID:           strings.TrimSpace(task.TaskID),
		TaskType:         strings.TrimSpace(task.TaskType),
		TaskSubtype:      strings.TrimSpace(task.TaskSubtype),
		InputKind:        strings.TrimSpace(task.InputKind),
		Scene:            strings.TrimSpace(task.Scene),
		WorkspaceID:      strings.TrimSpace(task.WorkspaceID),
		AppInstanceID:    strings.TrimSpace(task.AppInstanceID),
		Status:           TaskRunStatusCompleted,
		IdempotencyScope: defaultString(input.IdempotencyScope, defaultIdempotencyScope(task)),
		IdempotencyKey:   strings.TrimSpace(input.IdempotencyKey),
		RetentionPolicy:  strings.TrimSpace(input.RetentionPolicy),
		Metadata: mergeAnyMaps(input.Metadata, map[string]any{
			"writer":      "runtime_minimal_persistence",
			"input_kind":  strings.TrimSpace(task.InputKind),
			"output_mode": strings.TrimSpace(task.OutputMode),
		}),
		StartedAt:   &now,
		CompletedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return MinimalPersistenceRecordSet{}, err
	}

	step, err := w.Store.CreateTaskStep(ctx, TaskStep{
		ID:          uuid.NewString(),
		RunID:       run.ID,
		Sequence:    1,
		StepType:    "deterministic_writer",
		Name:        "persist_minimal_runtime_records",
		Status:      TaskStepStatusSuccess,
		Metadata:    map[string]any{"safe_label": "minimal_runtime_persistence"},
		StartedAt:   &now,
		CompletedAt: &now,
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		return MinimalPersistenceRecordSet{}, err
	}

	events := []TaskRunLifecycleEvent{
		{RunID: run.ID, EventType: "run_created", SubjectType: LifecycleSubjectRun, SubjectID: run.ID, FromStatus: "", ToStatus: TaskRunStatusCreated, Reason: "deterministic_writer_started", Metadata: map[string]any{"safe_label": "run_created"}, OccurredAt: now},
		{RunID: run.ID, EventType: "run_running", SubjectType: LifecycleSubjectRun, SubjectID: run.ID, FromStatus: TaskRunStatusCreated, ToStatus: TaskRunStatusRunning, Reason: "minimal_step_started", Metadata: map[string]any{"safe_label": "run_running"}, OccurredAt: now.Add(time.Millisecond)},
		{RunID: run.ID, StepID: step.ID, EventType: "step_started", SubjectType: LifecycleSubjectStep, SubjectID: step.ID, FromStatus: TaskStepStatusPending, ToStatus: TaskStepStatusRunning, Reason: "deterministic_step_started", Metadata: map[string]any{"safe_label": "step_started"}, OccurredAt: now.Add(2 * time.Millisecond)},
		{RunID: run.ID, StepID: step.ID, EventType: "step_completed", SubjectType: LifecycleSubjectStep, SubjectID: step.ID, FromStatus: TaskStepStatusRunning, ToStatus: TaskStepStatusSuccess, Reason: "minimal_records_written", Metadata: map[string]any{"safe_label": "step_completed"}, OccurredAt: now.Add(3 * time.Millisecond)},
		{RunID: run.ID, EventType: "run_completed", SubjectType: LifecycleSubjectRun, SubjectID: run.ID, FromStatus: TaskRunStatusRunning, ToStatus: TaskRunStatusCompleted, Reason: "minimal_persistence_complete", Metadata: map[string]any{"safe_label": "run_completed"}, OccurredAt: now.Add(4 * time.Millisecond)},
	}
	writtenEvents := make([]TaskRunLifecycleEvent, 0, len(events))
	for _, event := range events {
		written, err := w.Store.CreateLifecycleEvent(ctx, event)
		if err != nil {
			return MinimalPersistenceRecordSet{}, err
		}
		writtenEvents = append(writtenEvents, written)
	}

	trace, err := w.Store.CreateRuntimeTrace(ctx, RuntimeTrace{
		RunID:     run.ID,
		StepID:    step.ID,
		TraceType: "writer_summary",
		Summary:   "minimal runtime persistence writer stored a safe task-run record set",
		SafeLabels: map[string]string{
			"component": "internal/runtime",
			"writer":    "minimal",
		},
		RedactedPayload: map[string]any{
			"task_type":      strings.TrimSpace(task.TaskType),
			"status":         TaskRunStatusCompleted,
			"credential_raw": "[redacted]",
		},
		Metadata:  map[string]any{"redaction_policy": "whitelist_summary"},
		CreatedAt: now,
	})
	if err != nil {
		return MinimalPersistenceRecordSet{}, err
	}

	usage, err := w.Store.CreateUsage(ctx, Usage{
		RunID:        run.ID,
		StepID:       step.ID,
		ResourceType: "runtime_writer",
		Provider:     "athena",
		ResourceName: "minimal_persistence_write",
		Unit:         "operation",
		Amount:       1,
		Metadata:     map[string]any{"generic_usage": true},
		CreatedAt:    now,
	})
	if err != nil {
		return MinimalPersistenceRecordSet{}, err
	}

	projection, err := w.Store.CreateProjectionCandidate(ctx, ProjectionCandidate{
		RunID:         run.ID,
		StepID:        step.ID,
		CandidateKind: "minimal_output",
		Status:        "ready",
		Summary:       "minimal runtime candidate output",
		RedactedPayload: map[string]any{
			"output_mode": defaultString(task.OutputMode, "text"),
			"summary":     "candidate output stored without business evidence semantics",
		},
		Metadata:  map[string]any{"projection_scope": "candidate_output"},
		CreatedAt: now,
	})
	if err != nil {
		return MinimalPersistenceRecordSet{}, err
	}

	return MinimalPersistenceRecordSet{
		Run:        run,
		Step:       step,
		Events:     writtenEvents,
		Trace:      trace,
		Usage:      usage,
		Projection: projection,
	}, nil
}

func defaultIdempotencyScope(task *runtimetask.RuntimeTask) string {
	workspaceID := defaultString(task.WorkspaceID, "default_workspace")
	appInstanceID := defaultString(task.AppInstanceID, "default_app")
	sourceKind := defaultString(task.InputKind, "runtime_task")
	scene := defaultString(task.Scene, "default_scene")
	return workspaceID + ":" + appInstanceID + ":" + sourceKind + ":" + scene
}

func mergeAnyMaps(left map[string]any, right map[string]any) map[string]any {
	output := map[string]any{}
	for key, value := range left {
		output[key] = value
	}
	for key, value := range right {
		output[key] = value
	}
	return output
}
