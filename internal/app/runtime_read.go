// runtime_read.go exposes app-layer read access to persisted runtime records.
// runtime_read.go 暴露 app 层对持久化 runtime 记录的读取入口。
package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

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

// RuntimeCheckpointReadout is the Control Plane-safe checkpoint metadata view.
// RuntimeCheckpointReadout 是控制面可见的 checkpoint 安全元数据视图。
type RuntimeCheckpointReadout struct {
	CheckpointID       string
	RunID              string
	Stage              string
	ResumeTokenPresent bool
	PayloadSize        int
	PayloadSHA256      string
	CreatedAt          time.Time
	UpdatedAt          time.Time
	SnapshotAvailable  bool
	Source             string
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

// ListRuntimeCheckpointReadouts returns safe checkpoint metadata inferred from one run.
// ListRuntimeCheckpointReadouts 返回从一个 run 推导出的 checkpoint 安全元数据。
func (s *Service) ListRuntimeCheckpointReadouts(ctx context.Context, runID string) ([]RuntimeCheckpointReadout, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return nil, fmt.Errorf("runtime run id is required")
	}
	run, ok, err := store.GetTaskRun(ctx, runID)
	if err != nil || !ok {
		return nil, err
	}
	items := runtimeCheckpointReadoutsFromRun(run)
	snapshotStore, hasSnapshots := store.(runtime.RuntimeGraphCheckpointSnapshotStore)
	for idx := range items {
		if !hasSnapshots {
			continue
		}
		snapshot, ok, err := snapshotStore.GetCheckpointSnapshot(ctx, items[idx].CheckpointID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		items[idx].PayloadSize = snapshot.PayloadSize
		items[idx].PayloadSHA256 = snapshot.PayloadSHA256
		items[idx].CreatedAt = snapshot.CreatedAt
		items[idx].UpdatedAt = snapshot.UpdatedAt
		items[idx].SnapshotAvailable = true
		items[idx].Source = "checkpoint_store"
	}
	return items, nil
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

func runtimeCheckpointReadoutsFromRun(run runtime.TaskRun) []RuntimeCheckpointReadout {
	var out []RuntimeCheckpointReadout
	seen := map[string]struct{}{}
	add := func(item RuntimeCheckpointReadout) {
		item.CheckpointID = strings.TrimSpace(item.CheckpointID)
		if item.CheckpointID == "" {
			return
		}
		if _, exists := seen[item.CheckpointID]; exists {
			return
		}
		if strings.TrimSpace(item.RunID) == "" {
			item.RunID = run.ID
		}
		if strings.TrimSpace(item.Source) == "" {
			item.Source = "task_run_metadata"
		}
		seen[item.CheckpointID] = struct{}{}
		out = append(out, item)
	}
	for _, key := range []string{"checkpoint_id", "runtime_checkpoint_id"} {
		if checkpointID := stringFromMetadata(run.Metadata, key); checkpointID != "" {
			add(RuntimeCheckpointReadout{
				CheckpointID:       checkpointID,
				RunID:              run.ID,
				Stage:              stringFromMetadata(run.Metadata, "checkpoint_stage"),
				ResumeTokenPresent: stringFromMetadata(run.Metadata, "resume_token") != "",
				Source:             key,
			})
		}
	}
	for _, key := range []string{"checkpoint_ref", "runtime_checkpoint", "runtime_graph_checkpoint"} {
		if ref, ok := metadataObject(run.Metadata, key); ok {
			add(runtimeCheckpointReadoutFromObject(run.ID, key, ref))
		}
	}
	if refs, ok := metadataSlice(run.Metadata, "checkpoints"); ok {
		for _, raw := range refs {
			switch item := raw.(type) {
			case string:
				add(RuntimeCheckpointReadout{CheckpointID: item, RunID: run.ID, Source: "checkpoints"})
			case map[string]any:
				add(runtimeCheckpointReadoutFromObject(run.ID, "checkpoints", item))
			}
		}
	}
	return out
}

func runtimeCheckpointReadoutFromObject(runID string, source string, values map[string]any) RuntimeCheckpointReadout {
	return RuntimeCheckpointReadout{
		CheckpointID:       firstMetadataString(values, "checkpoint_id", "id"),
		RunID:              defaultRuntimeReadString(firstMetadataString(values, "run_id"), runID),
		Stage:              firstMetadataString(values, "stage", "checkpoint_stage"),
		ResumeTokenPresent: metadataBool(values, "resume_token_present") || firstMetadataString(values, "resume_token") != "",
		Source:             source,
	}
}

func stringFromMetadata(values map[string]any, key string) string {
	if values == nil {
		return ""
	}
	return stringFromAny(values[key])
}

func firstMetadataString(values map[string]any, keys ...string) string {
	for _, key := range keys {
		if value := stringFromMetadata(values, key); value != "" {
			return value
		}
	}
	return ""
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}

func metadataObject(values map[string]any, key string) (map[string]any, bool) {
	raw, ok := values[key]
	if !ok {
		return nil, false
	}
	typed, ok := raw.(map[string]any)
	return typed, ok
}

func metadataSlice(values map[string]any, key string) ([]any, bool) {
	raw, ok := values[key]
	if !ok {
		return nil, false
	}
	typed, ok := raw.([]any)
	return typed, ok
}

func metadataBool(values map[string]any, key string) bool {
	raw, ok := values[key]
	if !ok {
		return false
	}
	typed, ok := raw.(bool)
	return ok && typed
}

func defaultRuntimeReadString(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
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
