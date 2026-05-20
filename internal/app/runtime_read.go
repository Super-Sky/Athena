// runtime_read.go exposes app-layer read access to persisted runtime records.
// runtime_read.go 暴露 app 层对持久化 runtime 记录的读取入口。
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"moss/internal/runtime"
)

const (
	defaultRuntimeReadLimit = 50
	maxRuntimeReadLimit     = 200
)

// ErrRuntimeStoreNotConfigured marks a runtime read request without a persistence store.
// ErrRuntimeStoreNotConfigured 表示 runtime read 请求缺少持久化 store。
var ErrRuntimeStoreNotConfigured = errors.New("runtime persistence store is not configured")

// RuntimeRunReadQuery filters persisted runtime task runs for Control Plane reads.
// RuntimeRunReadQuery 用于筛选 Control Plane 读取的持久化 runtime task run。
type RuntimeRunReadQuery struct {
	WorkspaceID string
	Status      string
	Limit       int
}

// RuntimeRecordReadQuery filters persisted records attached to one runtime run.
// RuntimeRecordReadQuery 用于筛选挂在一个 runtime run 下的持久化记录。
type RuntimeRecordReadQuery struct {
	RunID  string
	StepID string
	Limit  int
}

// RuntimeContractFoundationReadout groups the v2.1 contract foundation read surface.
// RuntimeContractFoundationReadout 汇总 v2.1 contract foundation 的只读视图。
type RuntimeContractFoundationReadout struct {
	Contracts           []runtime.RuntimeContract
	TaskTypes           []runtime.TaskTypeRegistration
	HookBindings        []runtime.HookBinding
	ActiveSystemTruths  []runtime.SystemTruthActiveVersion
	StoreCapabilities   []string
	UnavailableSurfaces []string
}

// ListRuntimeRuns returns persisted runtime runs through the app-layer read boundary.
// ListRuntimeRuns 通过 app 层读取边界返回持久化 runtime run。
func (s *Service) ListRuntimeRuns(ctx context.Context, query RuntimeRunReadQuery) ([]runtime.TaskRun, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	return store.ListTaskRuns(ctx, runtime.TaskRunListFilter{
		WorkspaceID: strings.TrimSpace(query.WorkspaceID),
		Status:      strings.TrimSpace(query.Status),
		Limit:       normalizeRuntimeReadLimit(query.Limit),
	})
}

// GetRuntimeRun loads one persisted runtime run by ID.
// GetRuntimeRun 按 ID 读取一条持久化 runtime run。
func (s *Service) GetRuntimeRun(ctx context.Context, runID string) (runtime.TaskRun, bool, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return runtime.TaskRun{}, false, err
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return runtime.TaskRun{}, false, fmt.Errorf("runtime run id is required")
	}
	return store.GetTaskRun(ctx, runID)
}

// ListRuntimeSteps returns persisted steps for one runtime run.
// ListRuntimeSteps 返回一个 runtime run 下的持久化 step。
func (s *Service) ListRuntimeSteps(ctx context.Context, runID string) ([]runtime.TaskStep, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("runtime run id is required")
	}
	return store.ListTaskSteps(ctx, runID)
}

// ListRuntimeLifecycleEvents returns persisted lifecycle events for one runtime run.
// ListRuntimeLifecycleEvents 返回一个 runtime run 下的持久化生命周期事件。
func (s *Service) ListRuntimeLifecycleEvents(ctx context.Context, runID string) ([]runtime.TaskRunLifecycleEvent, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("runtime run id is required")
	}
	return store.ListLifecycleEventsByRun(ctx, runID)
}

// ListRuntimeTraces returns persisted trace summaries for one runtime run.
// ListRuntimeTraces 返回一个 runtime run 下的持久化 trace 摘要。
func (s *Service) ListRuntimeTraces(ctx context.Context, query RuntimeRecordReadQuery) ([]runtime.RuntimeTrace, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	query.RunID = strings.TrimSpace(query.RunID)
	if query.RunID == "" {
		return nil, fmt.Errorf("runtime run id is required")
	}
	return store.ListRuntimeTraces(ctx, runtime.RuntimeTraceListFilter{
		RunID:  query.RunID,
		StepID: strings.TrimSpace(query.StepID),
		Limit:  normalizeRuntimeReadLimit(query.Limit),
	})
}

// ListRuntimeUsage returns persisted generic resource usage for one runtime run.
// ListRuntimeUsage 返回一个 runtime run 下的持久化通用资源用量。
func (s *Service) ListRuntimeUsage(ctx context.Context, query RuntimeRecordReadQuery) ([]runtime.Usage, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	query.RunID = strings.TrimSpace(query.RunID)
	if query.RunID == "" {
		return nil, fmt.Errorf("runtime run id is required")
	}
	return store.ListUsage(ctx, runtime.UsageListFilter{
		RunID:  query.RunID,
		StepID: strings.TrimSpace(query.StepID),
		Limit:  normalizeRuntimeReadLimit(query.Limit),
	})
}

// ListRuntimeProjectionCandidates returns persisted candidate outputs for one runtime run.
// ListRuntimeProjectionCandidates 返回一个 runtime run 下的持久化候选输出。
func (s *Service) ListRuntimeProjectionCandidates(ctx context.Context, query RuntimeRecordReadQuery) ([]runtime.ProjectionCandidate, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	query.RunID = strings.TrimSpace(query.RunID)
	if query.RunID == "" {
		return nil, fmt.Errorf("runtime run id is required")
	}
	return store.ListProjectionCandidates(ctx, runtime.ProjectionCandidateListFilter{
		RunID:  query.RunID,
		StepID: strings.TrimSpace(query.StepID),
		Limit:  normalizeRuntimeReadLimit(query.Limit),
	})
}

// GetRuntimeContractFoundation returns v2.1 contract foundation records for Control Plane reads.
// GetRuntimeContractFoundation 返回供控制面读取的 v2.1 contract foundation 记录。
func (s *Service) GetRuntimeContractFoundation(ctx context.Context) (RuntimeContractFoundationReadout, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return RuntimeContractFoundationReadout{}, err
	}
	readout := RuntimeContractFoundationReadout{}
	if contractStore, ok := store.(runtime.RuntimeContractStore); ok {
		readout.StoreCapabilities = append(readout.StoreCapabilities, "runtime_contracts")
		contracts, err := contractStore.ListRuntimeContracts(ctx, runtime.RuntimeContractListFilter{
			Limit: normalizeRuntimeReadLimit(50),
		})
		if err != nil {
			return RuntimeContractFoundationReadout{}, err
		}
		readout.Contracts = contracts
	} else {
		readout.UnavailableSurfaces = append(readout.UnavailableSurfaces, "runtime_contracts")
	}
	if taskTypeStore, ok := store.(runtime.TaskTypeRegistryStore); ok {
		readout.StoreCapabilities = append(readout.StoreCapabilities, "task_type_registry")
		taskTypes, err := taskTypeStore.ListTaskTypeRegistrations(ctx, runtime.TaskTypeRegistrationListFilter{
			Limit: normalizeRuntimeReadLimit(100),
		})
		if err != nil {
			return RuntimeContractFoundationReadout{}, err
		}
		readout.TaskTypes = taskTypes
	} else {
		readout.UnavailableSurfaces = append(readout.UnavailableSurfaces, "task_type_registry")
	}
	if hookStore, ok := store.(runtime.HookBindingStore); ok {
		readout.StoreCapabilities = append(readout.StoreCapabilities, "hook_bindings")
		hooks, err := hookStore.ListHookBindings(ctx, runtime.HookBindingListFilter{
			Limit: normalizeRuntimeReadLimit(100),
		})
		if err != nil {
			return RuntimeContractFoundationReadout{}, err
		}
		readout.HookBindings = hooks
	} else {
		readout.UnavailableSurfaces = append(readout.UnavailableSurfaces, "hook_bindings")
	}
	if truthStore, ok := store.(runtime.SystemTruthLifecycleStore); ok {
		readout.StoreCapabilities = append(readout.StoreCapabilities, "system_truth_lifecycle")
		active, err := truthStore.ListSystemTruthActiveVersions(ctx, "")
		if err != nil {
			return RuntimeContractFoundationReadout{}, err
		}
		readout.ActiveSystemTruths = active
	} else {
		readout.UnavailableSurfaces = append(readout.UnavailableSurfaces, "system_truth_lifecycle")
	}
	return readout, nil
}

func (s *Service) runtimePersistenceStore() (runtime.RuntimePersistenceStore, error) {
	if s == nil || s.RuntimeStore == nil {
		return nil, ErrRuntimeStoreNotConfigured
	}
	return s.RuntimeStore, nil
}

func normalizeRuntimeReadLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultRuntimeReadLimit
	case limit > maxRuntimeReadLimit:
		return maxRuntimeReadLimit
	default:
		return limit
	}
}
