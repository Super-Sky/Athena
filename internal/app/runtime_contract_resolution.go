// runtime_contract_resolution.go resolves task types into active runtime contracts.
// runtime_contract_resolution.go 负责把 task type 解析为 active runtime contract。
package app

import (
	"context"
	"strings"

	"moss/internal/runtime"
	runtimetask "moss/internal/runtime/task"
)

type runtimeContractResolverStore interface {
	runtime.RuntimeContractStore
	runtime.TaskTypeRegistryStore
}

type runtimeContractResolution struct {
	TaskType runtime.TaskTypeRegistration
	Contract runtime.RuntimeContract
}

func (s *Service) resolveRuntimeContractResolution(ctx context.Context, taskType string) (*runtimeContractResolution, error) {
	if s == nil || s.RuntimeStore == nil {
		return nil, nil
	}
	store, ok := s.RuntimeStore.(runtimeContractResolverStore)
	if !ok {
		return nil, nil
	}
	taskType = defaultString(strings.TrimSpace(taskType), runtimetask.InputKindChat)
	registration, found, err := store.GetTaskTypeRegistrationByKey(ctx, taskType)
	if err != nil {
		return nil, err
	}
	if !found || !strings.EqualFold(defaultString(registration.Status, runtime.TaskTypeStatusDraft), runtime.TaskTypeStatusActive) {
		return nil, &InvalidTaskRequestError{
			TaskType: taskType,
			Reason:   "unsupported_task_type",
		}
	}
	contractID := strings.TrimSpace(registration.DefaultContractID)
	if contractID == "" {
		return nil, &InvalidTaskRequestError{
			TaskType:      taskType,
			Reason:        "missing_required_fields",
			MissingFields: []string{"default_contract_id"},
		}
	}
	contract, found, err := store.GetRuntimeContract(ctx, contractID)
	if err != nil {
		return nil, err
	}
	if !found || !strings.EqualFold(defaultString(contract.Status, runtime.RuntimeContractStatusDraft), runtime.RuntimeContractStatusActive) {
		return nil, &InvalidTaskRequestError{
			TaskType: taskType,
			Reason:   "unsupported_task_type",
		}
	}
	return &runtimeContractResolution{
		TaskType: registration,
		Contract: contract,
	}, nil
}

func applyRuntimeContractResolutionToTask(task *runtimetask.RuntimeTask, resolved *runtimeContractResolution) {
	if task == nil || resolved == nil {
		return
	}
	if task.Constraints == nil {
		task.Constraints = map[string]string{}
	}
	task.Constraints["runtime_contract_id"] = strings.TrimSpace(resolved.Contract.ID)
	task.Constraints["contract_id"] = strings.TrimSpace(resolved.Contract.ID)
	task.Constraints["runtime_contract_version"] = strings.TrimSpace(resolved.Contract.Version)
	task.Constraints["runtime_contract_status"] = strings.TrimSpace(resolved.Contract.Status)
	task.Constraints["runtime_contract_name"] = strings.TrimSpace(resolved.Contract.Name)
	task.Constraints["runtime_task_type_registration_id"] = strings.TrimSpace(resolved.TaskType.ID)
	task.Constraints["runtime_task_type_status"] = strings.TrimSpace(resolved.TaskType.Status)
}

func resolvedRuntimeContract(resolved *runtimeContractResolution) *runtime.RuntimeContract {
	if resolved == nil {
		return nil
	}
	contract := resolved.Contract
	return &contract
}

func resolvedTaskTypeRegistration(resolved *runtimeContractResolution) *runtime.TaskTypeRegistration {
	if resolved == nil {
		return nil
	}
	taskType := resolved.TaskType
	return &taskType
}
