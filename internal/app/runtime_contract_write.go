// runtime_contract_write.go exposes app-layer write access to runtime foundation records.
// runtime_contract_write.go 暴露 app 层对 runtime foundation 记录的写入入口。
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"moss/internal/runtime"
)

// ErrRuntimeFoundationWriteUnsupported marks a runtime store that cannot edit foundation records.
// ErrRuntimeFoundationWriteUnsupported 表示当前 runtime store 不支持 foundation 编辑。
var ErrRuntimeFoundationWriteUnsupported = errors.New("runtime foundation write is not supported by the configured store")

// UpdateRuntimeContract creates or replaces one runtime contract by stable ID.
// UpdateRuntimeContract 按稳定 ID 创建或替换一条 runtime contract 记录。
func (s *Service) UpdateRuntimeContract(ctx context.Context, id string, input runtime.RuntimeContract) (runtime.RuntimeContract, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return runtime.RuntimeContract{}, err
	}
	contractStore, ok := store.(runtime.RuntimeContractStore)
	if !ok {
		return runtime.RuntimeContract{}, ErrRuntimeFoundationWriteUnsupported
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return runtime.RuntimeContract{}, fmt.Errorf("runtime contract id is required")
	}
	input.ID = id

	now := time.Now().UTC()
	if existing, found, err := contractStore.GetRuntimeContract(ctx, id); err != nil {
		return runtime.RuntimeContract{}, err
	} else if found {
		input.CreatedAt = preserveTimestamp(input.CreatedAt, existing.CreatedAt)
	} else if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	if err := runtime.ValidateRuntimeContract(input); err != nil {
		return runtime.RuntimeContract{}, err
	}
	return contractStore.PutRuntimeContract(ctx, input)
}

// UpdateRuntimeTaskTypeRegistration creates or replaces one task type registration by stable type key.
// UpdateRuntimeTaskTypeRegistration 按稳定 type key 创建或替换一条 task type registration 记录。
func (s *Service) UpdateRuntimeTaskTypeRegistration(ctx context.Context, typeKey string, input runtime.TaskTypeRegistration) (runtime.TaskTypeRegistration, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return runtime.TaskTypeRegistration{}, err
	}
	taskTypeStore, ok := store.(runtime.TaskTypeRegistryStore)
	if !ok {
		return runtime.TaskTypeRegistration{}, ErrRuntimeFoundationWriteUnsupported
	}

	typeKey = strings.TrimSpace(typeKey)
	if typeKey == "" {
		return runtime.TaskTypeRegistration{}, fmt.Errorf("runtime task type key is required")
	}
	input.TypeKey = typeKey

	now := time.Now().UTC()
	if existing, found, err := taskTypeStore.GetTaskTypeRegistrationByKey(ctx, typeKey); err != nil {
		return runtime.TaskTypeRegistration{}, err
	} else if found {
		input.ID = preserveString(input.ID, existing.ID)
		input.CreatedAt = preserveTimestamp(input.CreatedAt, existing.CreatedAt)
	} else if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	if err := runtime.ValidateTaskTypeRegistration(input); err != nil {
		return runtime.TaskTypeRegistration{}, err
	}
	return taskTypeStore.PutTaskTypeRegistration(ctx, input)
}

// UpdateRuntimeHookBinding creates or replaces one hook binding by stable ID.
// UpdateRuntimeHookBinding 按稳定 ID 创建或替换一条 hook binding 记录。
func (s *Service) UpdateRuntimeHookBinding(ctx context.Context, id string, input runtime.HookBinding) (runtime.HookBinding, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return runtime.HookBinding{}, err
	}
	hookStore, ok := store.(runtime.HookBindingStore)
	if !ok {
		return runtime.HookBinding{}, ErrRuntimeFoundationWriteUnsupported
	}

	id = strings.TrimSpace(id)
	if id == "" {
		return runtime.HookBinding{}, fmt.Errorf("runtime hook binding id is required")
	}
	input.ID = id

	now := time.Now().UTC()
	if existing, found, err := hookStore.GetHookBinding(ctx, id); err != nil {
		return runtime.HookBinding{}, err
	} else if found {
		input.CreatedAt = preserveTimestamp(input.CreatedAt, existing.CreatedAt)
	} else if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	if err := runtime.ValidateHookBinding(input); err != nil {
		return runtime.HookBinding{}, err
	}
	return hookStore.PutHookBinding(ctx, input)
}

func preserveTimestamp(current time.Time, fallback time.Time) time.Time {
	if !current.IsZero() {
		return current
	}
	return fallback
}

func preserveString(current string, fallback string) string {
	current = strings.TrimSpace(current)
	if current != "" {
		return current
	}
	return strings.TrimSpace(fallback)
}
