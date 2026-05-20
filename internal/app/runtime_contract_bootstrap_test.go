package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"moss/internal/controlplane"
	"moss/internal/runtime"
)

func TestSyncRuntimeContractFoundationSnapshotSeedsActiveTruth(t *testing.T) {
	t.Parallel()

	manager := newRuntimeFoundationTestManager(t)
	store := newRuntimeFoundationMemoryStore()

	if err := syncRuntimeContractFoundationSnapshot(context.Background(), manager, store); err != nil {
		t.Fatalf("syncRuntimeContractFoundationSnapshot() error = %v", err)
	}
	if len(store.contracts) != 1 {
		t.Fatalf("contracts len = %d, want 1", len(store.contracts))
	}
	contract, ok := store.contracts[runtimeValidationContractID]
	if !ok || contract.TaskType != runtimeValidationTaskTypeKey {
		t.Fatalf("contract = %#v, want %q task type", contract, runtimeValidationTaskTypeKey)
	}
	taskType, ok := store.taskTypesByKey[runtimeValidationTaskTypeKey]
	if !ok || taskType.DefaultContractID != runtimeValidationContractID {
		t.Fatalf("task type = %#v, want default contract %q", taskType, runtimeValidationContractID)
	}
	if len(store.hooks) != len(runtimeValidationHookSeeds) {
		t.Fatalf("hooks len = %d, want %d", len(store.hooks), len(runtimeValidationHookSeeds))
	}
	if len(store.activeTruthsByAsset) == 0 {
		t.Fatalf("activeTruthsByAsset is empty, want at least one active truth")
	}
	if _, ok := store.activeTruthsByAsset["persona.default"]; !ok {
		t.Fatalf("activeTruthsByAsset missing persona.default: %#v", store.activeTruthsByAsset)
	}
}

func TestSyncRuntimeContractFoundationSnapshotIsIdempotent(t *testing.T) {
	t.Parallel()

	manager := newRuntimeFoundationTestManager(t)
	store := newRuntimeFoundationMemoryStore()

	if err := syncRuntimeContractFoundationSnapshot(context.Background(), manager, store); err != nil {
		t.Fatalf("first sync error = %v", err)
	}
	firstTruthCount := len(store.activeTruthsByAsset)
	if err := syncRuntimeContractFoundationSnapshot(context.Background(), manager, store); err != nil {
		t.Fatalf("second sync error = %v", err)
	}
	if len(store.contracts) != 1 || len(store.taskTypesByKey) != 1 {
		t.Fatalf("foundation counts changed after second sync: contracts=%d taskTypes=%d", len(store.contracts), len(store.taskTypesByKey))
	}
	if len(store.hooks) != len(runtimeValidationHookSeeds) {
		t.Fatalf("hook count changed after second sync: %d", len(store.hooks))
	}
	if len(store.activeTruthsByAsset) != firstTruthCount {
		t.Fatalf("active truth count = %d after second sync, want %d", len(store.activeTruthsByAsset), firstTruthCount)
	}
}

func newRuntimeFoundationTestManager(t *testing.T) *controlplane.Manager {
	t.Helper()

	tmpDir := t.TempDir()
	truthDir := filepath.Join(tmpDir, "truth")
	writeRuntimeFoundationTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "SOUL.md"), `---
id: core_soul
name: Core Soul
summary: 墨思的全局人格基线
---

## Role
墨思是企业级安全咨询智能体。
`)
	writeRuntimeFoundationTruthMarkdown(t, filepath.Join(truthDir, "sources", "core", "AGENTS.md"), `---
id: core_agents
name: Core Agents
summary: 墨思运行纪律
---

## Operational Discipline
- 不虚构
`)
	manager := controlplane.NewManagerWithTruthAndStateDirs(
		controlplane.NewFileStore(filepath.Join(tmpDir, "overrides.json")),
		truthDir,
		filepath.Join(tmpDir, "state"),
	)
	if err := manager.SyncSystemSources(context.Background()); err != nil {
		t.Fatalf("SyncSystemSources() error = %v", err)
	}
	return manager
}

func writeRuntimeFoundationTruthMarkdown(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%s) error = %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile(%s) error = %v", path, err)
	}
}

type runtimeFoundationMemoryStore struct {
	contracts           map[string]runtime.RuntimeContract
	taskTypesByID       map[string]runtime.TaskTypeRegistration
	taskTypesByKey      map[string]runtime.TaskTypeRegistration
	hooks               map[string]runtime.HookBinding
	activeTruthsByAsset map[string]runtime.SystemTruthActiveVersion
}

func newRuntimeFoundationMemoryStore() *runtimeFoundationMemoryStore {
	return &runtimeFoundationMemoryStore{
		contracts:           map[string]runtime.RuntimeContract{},
		taskTypesByID:       map[string]runtime.TaskTypeRegistration{},
		taskTypesByKey:      map[string]runtime.TaskTypeRegistration{},
		hooks:               map[string]runtime.HookBinding{},
		activeTruthsByAsset: map[string]runtime.SystemTruthActiveVersion{},
	}
}

func (s *runtimeFoundationMemoryStore) AutoMigrate(context.Context) error { return nil }

func (s *runtimeFoundationMemoryStore) CreateRuntimeContract(_ context.Context, item runtime.RuntimeContract) (runtime.RuntimeContract, error) {
	if item.ID == "" {
		item.ID = "contract-" + strings.ReplaceAll(item.TaskType, ".", "-")
	}
	s.contracts[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) PutRuntimeContract(_ context.Context, item runtime.RuntimeContract) (runtime.RuntimeContract, error) {
	if item.ID == "" {
		item.ID = "contract-" + strings.ReplaceAll(item.TaskType, ".", "-")
	}
	s.contracts[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetRuntimeContract(_ context.Context, id string) (runtime.RuntimeContract, bool, error) {
	item, ok := s.contracts[id]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListRuntimeContracts(_ context.Context, _ runtime.RuntimeContractListFilter) ([]runtime.RuntimeContract, error) {
	out := make([]runtime.RuntimeContract, 0, len(s.contracts))
	for _, item := range s.contracts {
		out = append(out, item)
	}
	return out, nil
}

func (s *runtimeFoundationMemoryStore) CreateTaskTypeRegistration(_ context.Context, item runtime.TaskTypeRegistration) (runtime.TaskTypeRegistration, error) {
	if item.ID == "" {
		item.ID = "task-type-" + item.TypeKey
	}
	s.taskTypesByID[item.ID] = item
	s.taskTypesByKey[item.TypeKey] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) PutTaskTypeRegistration(_ context.Context, item runtime.TaskTypeRegistration) (runtime.TaskTypeRegistration, error) {
	if item.ID == "" {
		item.ID = "task-type-" + item.TypeKey
	}
	s.taskTypesByID[item.ID] = item
	s.taskTypesByKey[item.TypeKey] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetTaskTypeRegistration(_ context.Context, id string) (runtime.TaskTypeRegistration, bool, error) {
	item, ok := s.taskTypesByID[id]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) GetTaskTypeRegistrationByKey(_ context.Context, key string) (runtime.TaskTypeRegistration, bool, error) {
	item, ok := s.taskTypesByKey[key]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListTaskTypeRegistrations(_ context.Context, _ runtime.TaskTypeRegistrationListFilter) ([]runtime.TaskTypeRegistration, error) {
	out := make([]runtime.TaskTypeRegistration, 0, len(s.taskTypesByKey))
	for _, item := range s.taskTypesByKey {
		out = append(out, item)
	}
	return out, nil
}

func (s *runtimeFoundationMemoryStore) CreateHookBinding(_ context.Context, item runtime.HookBinding) (runtime.HookBinding, error) {
	if item.ID == "" {
		item.ID = "hook-" + item.BindingRef
	}
	s.hooks[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) PutHookBinding(_ context.Context, item runtime.HookBinding) (runtime.HookBinding, error) {
	if item.ID == "" {
		item.ID = "hook-" + item.BindingRef
	}
	s.hooks[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetHookBinding(_ context.Context, id string) (runtime.HookBinding, bool, error) {
	item, ok := s.hooks[id]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListHookBindings(_ context.Context, _ runtime.HookBindingListFilter) ([]runtime.HookBinding, error) {
	out := make([]runtime.HookBinding, 0, len(s.hooks))
	for _, item := range s.hooks {
		out = append(out, item)
	}
	return out, nil
}

func (s *runtimeFoundationMemoryStore) CreateSystemTruthSource(_ context.Context, item runtime.SystemTruthSource) (runtime.SystemTruthSource, error) {
	return item, nil
}

func (s *runtimeFoundationMemoryStore) CreateSystemTruthDraft(_ context.Context, item runtime.SystemTruthDraft) (runtime.SystemTruthDraft, error) {
	return item, nil
}

func (s *runtimeFoundationMemoryStore) CreateSystemTruthCompileResult(_ context.Context, item runtime.SystemTruthCompileResult) (runtime.SystemTruthCompileResult, error) {
	return item, nil
}

func (s *runtimeFoundationMemoryStore) ActivateSystemTruthVersion(_ context.Context, item runtime.SystemTruthActiveVersion) (runtime.SystemTruthActiveVersion, error) {
	s.activeTruthsByAsset[item.AssetID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetActiveSystemTruthVersion(_ context.Context, assetID string) (runtime.SystemTruthActiveVersion, bool, error) {
	item, ok := s.activeTruthsByAsset[assetID]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListSystemTruthActiveVersions(_ context.Context, assetID string) ([]runtime.SystemTruthActiveVersion, error) {
	if strings.TrimSpace(assetID) != "" {
		item, ok := s.activeTruthsByAsset[assetID]
		if !ok {
			return nil, nil
		}
		return []runtime.SystemTruthActiveVersion{item}, nil
	}
	out := make([]runtime.SystemTruthActiveVersion, 0, len(s.activeTruthsByAsset))
	for _, item := range s.activeTruthsByAsset {
		out = append(out, item)
	}
	return out, nil
}
