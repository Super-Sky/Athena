package app

import (
	"context"
	"testing"
	"time"

	"moss/internal/runtime"
)

func TestUpdateRuntimeContractPreservesCreatedAt(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	service := &Service{RuntimeStore: store}
	createdAt := time.Date(2026, 5, 19, 9, 0, 0, 0, time.UTC)
	store.contracts["contract-1"] = runtime.RuntimeContract{
		ID:        "contract-1",
		Name:      "Old Contract",
		Version:   "v1",
		Status:    runtime.RuntimeContractStatusDraft,
		TaskType:  "runtime_validation",
		CreatedAt: createdAt,
		UpdatedAt: createdAt,
	}

	item, err := service.UpdateRuntimeContract(context.Background(), "contract-1", runtime.RuntimeContract{
		Name:     "New Contract",
		Version:  "v2",
		Status:   runtime.RuntimeContractStatusActive,
		TaskType: "runtime_validation",
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeContract() error = %v", err)
	}
	if item.CreatedAt != createdAt {
		t.Fatalf("created_at = %s, want %s", item.CreatedAt, createdAt)
	}
	if !item.UpdatedAt.After(createdAt) {
		t.Fatalf("updated_at = %s, want after %s", item.UpdatedAt, createdAt)
	}
	if item.Name != "New Contract" || item.Version != "v2" {
		t.Fatalf("updated contract = %#v", item)
	}
}

func TestUpdateRuntimeTaskTypeRegistrationUsesPathKey(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	service := &Service{RuntimeStore: store}

	item, err := service.UpdateRuntimeTaskTypeRegistration(context.Background(), "runtime_validation", runtime.TaskTypeRegistration{
		DisplayName:       "Runtime Validation",
		Status:            runtime.TaskTypeStatusActive,
		InputSchema:       map[string]any{"type": "object"},
		ValidatorRefs:     map[string]any{"validators": []any{"runtime_contract_input"}, "status": "ready"},
		DefaultContractID: "contract-1",
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeTaskTypeRegistration() error = %v", err)
	}
	if item.TypeKey != "runtime_validation" {
		t.Fatalf("type_key = %q, want runtime_validation", item.TypeKey)
	}
	if item.ID == "" {
		t.Fatalf("id is empty after upsert")
	}
}

func TestUpdateRuntimeHookBindingPreservesCreatedAt(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	service := &Service{RuntimeStore: store}
	createdAt := time.Date(2026, 5, 19, 9, 30, 0, 0, time.UTC)
	store.hooks["hook-1"] = runtime.HookBinding{
		ID:            "hook-1",
		ContractID:    "contract-1",
		HookPoint:     runtime.HookPointBeforeRun,
		BindingKind:   runtime.HookBindingKindEinoMiddleware,
		BindingRef:    "runtime_contract_guard",
		Enabled:       true,
		FailurePolicy: runtime.HookFailurePolicyFailClosed,
		CreatedAt:     createdAt,
		UpdatedAt:     createdAt,
	}

	item, err := service.UpdateRuntimeHookBinding(context.Background(), "hook-1", runtime.HookBinding{
		ContractID:    "contract-1",
		HookPoint:     runtime.HookPointBeforeRun,
		BindingKind:   runtime.HookBindingKindEinoMiddleware,
		BindingRef:    "system_truth_guard",
		Enabled:       true,
		FailurePolicy: runtime.HookFailurePolicyRecordOnly,
	})
	if err != nil {
		t.Fatalf("UpdateRuntimeHookBinding() error = %v", err)
	}
	if item.CreatedAt != createdAt {
		t.Fatalf("created_at = %s, want %s", item.CreatedAt, createdAt)
	}
	if item.BindingRef != "system_truth_guard" || item.FailurePolicy != runtime.HookFailurePolicyRecordOnly {
		t.Fatalf("updated hook = %#v", item)
	}
}

type runtimeFoundationWriteTestStore struct {
	*runtimeFoundationMemoryStore
}

func newRuntimeFoundationWriteTestStore() *runtimeFoundationWriteTestStore {
	return &runtimeFoundationWriteTestStore{runtimeFoundationMemoryStore: newRuntimeFoundationMemoryStore()}
}

func (s *runtimeFoundationWriteTestStore) CreateTaskRun(context.Context, runtime.TaskRun) (runtime.TaskRun, error) {
	return runtime.TaskRun{}, nil
}

func (s *runtimeFoundationWriteTestStore) GetTaskRun(context.Context, string) (runtime.TaskRun, bool, error) {
	return runtime.TaskRun{}, false, nil
}

func (s *runtimeFoundationWriteTestStore) ListTaskRuns(context.Context, runtime.TaskRunListFilter) ([]runtime.TaskRun, error) {
	return nil, nil
}

func (s *runtimeFoundationWriteTestStore) CreateTaskStep(context.Context, runtime.TaskStep) (runtime.TaskStep, error) {
	return runtime.TaskStep{}, nil
}

func (s *runtimeFoundationWriteTestStore) GetTaskStep(context.Context, string) (runtime.TaskStep, bool, error) {
	return runtime.TaskStep{}, false, nil
}

func (s *runtimeFoundationWriteTestStore) ListTaskSteps(context.Context, string) ([]runtime.TaskStep, error) {
	return nil, nil
}

func (s *runtimeFoundationWriteTestStore) CreateLifecycleEvent(context.Context, runtime.TaskRunLifecycleEvent) (runtime.TaskRunLifecycleEvent, error) {
	return runtime.TaskRunLifecycleEvent{}, nil
}

func (s *runtimeFoundationWriteTestStore) GetLifecycleEvent(context.Context, string) (runtime.TaskRunLifecycleEvent, bool, error) {
	return runtime.TaskRunLifecycleEvent{}, false, nil
}

func (s *runtimeFoundationWriteTestStore) ListLifecycleEventsByRun(context.Context, string) ([]runtime.TaskRunLifecycleEvent, error) {
	return nil, nil
}

func (s *runtimeFoundationWriteTestStore) ListLifecycleEventsBySubject(context.Context, string, string) ([]runtime.TaskRunLifecycleEvent, error) {
	return nil, nil
}

func (s *runtimeFoundationWriteTestStore) CreateRuntimeTrace(context.Context, runtime.RuntimeTrace) (runtime.RuntimeTrace, error) {
	return runtime.RuntimeTrace{}, nil
}

func (s *runtimeFoundationWriteTestStore) GetRuntimeTrace(context.Context, string) (runtime.RuntimeTrace, bool, error) {
	return runtime.RuntimeTrace{}, false, nil
}

func (s *runtimeFoundationWriteTestStore) ListRuntimeTraces(context.Context, runtime.RuntimeTraceListFilter) ([]runtime.RuntimeTrace, error) {
	return nil, nil
}

func (s *runtimeFoundationWriteTestStore) CreateUsage(context.Context, runtime.Usage) (runtime.Usage, error) {
	return runtime.Usage{}, nil
}

func (s *runtimeFoundationWriteTestStore) GetUsage(context.Context, string) (runtime.Usage, bool, error) {
	return runtime.Usage{}, false, nil
}

func (s *runtimeFoundationWriteTestStore) ListUsage(context.Context, runtime.UsageListFilter) ([]runtime.Usage, error) {
	return nil, nil
}

func (s *runtimeFoundationWriteTestStore) CreateProjectionCandidate(context.Context, runtime.ProjectionCandidate) (runtime.ProjectionCandidate, error) {
	return runtime.ProjectionCandidate{}, nil
}

func (s *runtimeFoundationWriteTestStore) GetProjectionCandidate(context.Context, string) (runtime.ProjectionCandidate, bool, error) {
	return runtime.ProjectionCandidate{}, false, nil
}

func (s *runtimeFoundationWriteTestStore) ListProjectionCandidates(context.Context, runtime.ProjectionCandidateListFilter) ([]runtime.ProjectionCandidate, error) {
	return nil, nil
}
