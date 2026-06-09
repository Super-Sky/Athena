package app

import (
	"context"
	"errors"
	"testing"

	"moss/internal/runtime"
)

func TestResolveRuntimeContractResolutionReturnsActiveContract(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	store.contracts["contract-chat"] = runtime.RuntimeContract{
		ID:       "contract-chat",
		Name:     "Chat Contract",
		Version:  "v1",
		Status:   runtime.RuntimeContractStatusActive,
		TaskType: "chat",
	}
	store.taskTypesByKey["chat"] = runtime.TaskTypeRegistration{
		ID:                "task-type-chat",
		TypeKey:           "chat",
		Status:            runtime.TaskTypeStatusActive,
		DefaultContractID: "contract-chat",
	}

	service := &Service{RuntimeStore: store}
	resolved, err := service.resolveRuntimeContractResolution(context.Background(), "chat")
	if err != nil {
		t.Fatalf("resolveRuntimeContractResolution() error = %v", err)
	}
	if resolved == nil {
		t.Fatalf("resolveRuntimeContractResolution() returned nil")
	}
	if resolved.Contract.ID != "contract-chat" || resolved.TaskType.TypeKey != "chat" {
		t.Fatalf("resolved = %#v", resolved)
	}
}

func TestResolveRuntimeContractResolutionRejectsUnknownTaskType(t *testing.T) {
	t.Parallel()

	store := newRuntimeFoundationWriteTestStore()
	service := &Service{RuntimeStore: store}
	_, err := service.resolveRuntimeContractResolution(context.Background(), "custom_runtime_task")
	if err == nil {
		t.Fatalf("resolveRuntimeContractResolution() expected unsupported_task_type error")
	}
	var invalid *InvalidTaskRequestError
	if !errors.As(err, &invalid) {
		t.Fatalf("error = %T, want *InvalidTaskRequestError", err)
	}
	if invalid.Reason != "unsupported_task_type" {
		t.Fatalf("reason = %q, want unsupported_task_type", invalid.Reason)
	}
	if invalid.TaskType != "custom_runtime_task" {
		t.Fatalf("task_type = %q, want custom_runtime_task", invalid.TaskType)
	}
}
