package app

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"moss/internal/controlplane"
	"moss/internal/runtime"
	runtimetask "moss/internal/runtime/task"
)

func TestSyncRuntimeContractFoundationSnapshotSeedsActiveTruth(t *testing.T) {
	t.Parallel()

	manager := newRuntimeFoundationTestManager(t)
	store := newRuntimeFoundationWriteTestStore()

	if err := syncRuntimeContractFoundationSnapshot(context.Background(), manager, store); err != nil {
		t.Fatalf("syncRuntimeContractFoundationSnapshot() error = %v", err)
	}
	if len(store.contracts) != 1+len(registeredTaskTypeValidatorSeeds) {
		t.Fatalf("contracts len = %d, want %d", len(store.contracts), 1+len(registeredTaskTypeValidatorSeeds))
	}
	contract, ok := store.contracts[runtimeValidationContractID]
	if !ok || contract.TaskType != runtimeValidationTaskTypeKey {
		t.Fatalf("contract = %#v, want %q task type", contract, runtimeValidationTaskTypeKey)
	}
	taskType, ok := store.taskTypesByKey[runtimeValidationTaskTypeKey]
	if !ok || taskType.DefaultContractID != runtimeValidationContractID {
		t.Fatalf("task type = %#v, want default contract %q", taskType, runtimeValidationContractID)
	}
	chatTaskType, ok := store.taskTypesByKey["chat"]
	if !ok {
		t.Fatal("task type registry missing chat, the default Agent Run task type must be executable after bootstrap")
	}
	chatContractID := registeredTaskTypeValidatorContractID("chat")
	if chatTaskType.DefaultContractID != chatContractID {
		t.Fatalf("chat default contract = %q, want %q", chatTaskType.DefaultContractID, chatContractID)
	}
	if chatContract, ok := store.contracts[chatContractID]; !ok || chatContract.TaskType != "chat" {
		t.Fatalf("chat contract = %#v, want registered chat validator contract", chatContract)
	}
	if len(store.hooks) != len(runtimeValidationHookSeeds) {
		t.Fatalf("hooks len = %d, want %d", len(store.hooks), len(runtimeValidationHookSeeds))
	}
	for _, hook := range store.hooks {
		if hook.ContractID != runtimeValidationContractID {
			t.Fatalf("hook %s contract = %q, want runtime validation contract", hook.ID, hook.ContractID)
		}
	}
	for _, seed := range registeredTaskTypeValidatorSeeds {
		taskType, ok := store.taskTypesByKey[seed.typeKey]
		if !ok {
			t.Fatalf("task type registry missing %s", seed.typeKey)
		}
		if taskType.DefaultContractID != registeredTaskTypeValidatorContractID(seed.typeKey) {
			t.Fatalf("%s default contract = %q, want %q", seed.typeKey, taskType.DefaultContractID, registeredTaskTypeValidatorContractID(seed.typeKey))
		}
		if status, _ := taskType.ValidatorRefs["status"].(string); status != "ready" {
			t.Fatalf("%s validator status = %q, want ready", seed.typeKey, status)
		}
		if taskType.Compatibility["core_materialization_scope"] != "projection_candidate_only" {
			t.Fatalf("%s compatibility = %#v, want projection-only scope", seed.typeKey, taskType.Compatibility)
		}
	}
	service := &Service{RuntimeStore: store}
	resolved, err := service.resolveRuntimeContractResolution(context.Background(), runtimetask.InputKindChat)
	if err != nil {
		t.Fatalf("resolve default chat contract error = %v", err)
	}
	if resolved == nil || resolved.TaskType.TypeKey != runtimetask.InputKindChat {
		t.Fatalf("resolved chat contract = %#v, want active chat task type", resolved)
	}
	if len(store.activeTruthsByAsset) == 0 {
		t.Fatalf("activeTruthsByAsset is empty, want at least one active truth")
	}
	if _, ok := store.activeTruthsByAsset["persona.default"]; !ok {
		t.Fatalf("activeTruthsByAsset missing persona.default: %#v", store.activeTruthsByAsset)
	}
}

func TestRegisteredTaskTypeValidatorSeedsUseUniqueTypeKeys(t *testing.T) {
	t.Parallel()

	seen := make(map[string]struct{}, len(registeredTaskTypeValidatorSeeds))
	for _, seed := range registeredTaskTypeValidatorSeeds {
		if _, exists := seen[seed.typeKey]; exists {
			t.Fatalf("duplicate runtime task type seed %q", seed.typeKey)
		}
		seen[seed.typeKey] = struct{}{}
	}
	if _, ok := seen[runtimetask.InputKindChat]; !ok {
		t.Fatal("chat must remain a registered generic runtime task type")
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
	if len(store.contracts) != 1+len(registeredTaskTypeValidatorSeeds) || len(store.taskTypesByKey) != 1+len(registeredTaskTypeValidatorSeeds) {
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
	truthSources        map[string]runtime.SystemTruthSource
	truthDrafts         map[string]runtime.SystemTruthDraft
	truthCompiles       map[string]runtime.SystemTruthCompileResult
	activeTruthsByAsset map[string]runtime.SystemTruthActiveVersion
	activeTruthsByID    map[string]runtime.SystemTruthActiveVersion
	activeTruthHistory  []runtime.SystemTruthActiveVersion
}

func newRuntimeFoundationMemoryStore() *runtimeFoundationMemoryStore {
	return &runtimeFoundationMemoryStore{
		contracts:           map[string]runtime.RuntimeContract{},
		taskTypesByID:       map[string]runtime.TaskTypeRegistration{},
		taskTypesByKey:      map[string]runtime.TaskTypeRegistration{},
		hooks:               map[string]runtime.HookBinding{},
		truthSources:        map[string]runtime.SystemTruthSource{},
		truthDrafts:         map[string]runtime.SystemTruthDraft{},
		truthCompiles:       map[string]runtime.SystemTruthCompileResult{},
		activeTruthsByAsset: map[string]runtime.SystemTruthActiveVersion{},
		activeTruthsByID:    map[string]runtime.SystemTruthActiveVersion{},
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
	if item.ID == "" {
		item.ID = "source-" + strings.ReplaceAll(item.AssetID, ".", "-")
	}
	s.truthSources[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetSystemTruthSource(_ context.Context, id string) (runtime.SystemTruthSource, bool, error) {
	item, ok := s.truthSources[strings.TrimSpace(id)]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListSystemTruthSources(_ context.Context, filter runtime.SystemTruthSourceListFilter) ([]runtime.SystemTruthSource, error) {
	out := make([]runtime.SystemTruthSource, 0, len(s.truthSources))
	for _, item := range s.truthSources {
		if strings.TrimSpace(filter.AssetID) != "" && item.AssetID != strings.TrimSpace(filter.AssetID) {
			continue
		}
		if strings.TrimSpace(filter.Status) != "" && item.Status != strings.TrimSpace(filter.Status) {
			continue
		}
		out = append(out, item)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *runtimeFoundationMemoryStore) CreateSystemTruthDraft(_ context.Context, item runtime.SystemTruthDraft) (runtime.SystemTruthDraft, error) {
	if item.ID == "" {
		item.ID = "draft-" + strings.ReplaceAll(item.AssetID, ".", "-") + "-" + strconv.Itoa(len(s.truthDrafts)+1)
	}
	s.truthDrafts[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetSystemTruthDraft(_ context.Context, id string) (runtime.SystemTruthDraft, bool, error) {
	item, ok := s.truthDrafts[strings.TrimSpace(id)]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListSystemTruthDrafts(_ context.Context, filter runtime.SystemTruthDraftListFilter) ([]runtime.SystemTruthDraft, error) {
	out := make([]runtime.SystemTruthDraft, 0, len(s.truthDrafts))
	for _, item := range s.truthDrafts {
		if strings.TrimSpace(filter.SourceID) != "" && item.SourceID != strings.TrimSpace(filter.SourceID) {
			continue
		}
		if strings.TrimSpace(filter.AssetID) != "" && item.AssetID != strings.TrimSpace(filter.AssetID) {
			continue
		}
		if strings.TrimSpace(filter.Status) != "" && item.Status != strings.TrimSpace(filter.Status) {
			continue
		}
		out = append(out, item)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *runtimeFoundationMemoryStore) CreateSystemTruthCompileResult(_ context.Context, item runtime.SystemTruthCompileResult) (runtime.SystemTruthCompileResult, error) {
	if item.ID == "" {
		item.ID = "compile-" + strings.ReplaceAll(item.AssetID, ".", "-") + "-" + strconv.Itoa(len(s.truthCompiles)+1)
	}
	s.truthCompiles[item.ID] = item
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetSystemTruthCompileResult(_ context.Context, id string) (runtime.SystemTruthCompileResult, bool, error) {
	item, ok := s.truthCompiles[strings.TrimSpace(id)]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListSystemTruthCompileResults(_ context.Context, filter runtime.SystemTruthCompileResultListFilter) ([]runtime.SystemTruthCompileResult, error) {
	out := make([]runtime.SystemTruthCompileResult, 0, len(s.truthCompiles))
	for _, item := range s.truthCompiles {
		if strings.TrimSpace(filter.DraftID) != "" && item.DraftID != strings.TrimSpace(filter.DraftID) {
			continue
		}
		if strings.TrimSpace(filter.AssetID) != "" && item.AssetID != strings.TrimSpace(filter.AssetID) {
			continue
		}
		if strings.TrimSpace(filter.Status) != "" && item.Status != strings.TrimSpace(filter.Status) {
			continue
		}
		out = append(out, item)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *runtimeFoundationMemoryStore) ActivateSystemTruthVersion(_ context.Context, item runtime.SystemTruthActiveVersion) (runtime.SystemTruthActiveVersion, error) {
	if item.ID == "" {
		item.ID = "active-" + strings.ReplaceAll(item.AssetID, ".", "-") + "-" + strconv.Itoa(len(s.activeTruthHistory)+1)
	}
	s.activeTruthsByAsset[item.AssetID] = item
	s.activeTruthsByID[item.ID] = item
	s.activeTruthHistory = append(s.activeTruthHistory, item)
	return item, nil
}

func (s *runtimeFoundationMemoryStore) GetSystemTruthActiveVersion(_ context.Context, id string) (runtime.SystemTruthActiveVersion, bool, error) {
	item, ok := s.activeTruthsByID[strings.TrimSpace(id)]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) GetActiveSystemTruthVersion(_ context.Context, assetID string) (runtime.SystemTruthActiveVersion, bool, error) {
	item, ok := s.activeTruthsByAsset[assetID]
	return item, ok, nil
}

func (s *runtimeFoundationMemoryStore) ListSystemTruthActiveVersions(_ context.Context, assetID string) ([]runtime.SystemTruthActiveVersion, error) {
	if strings.TrimSpace(assetID) != "" {
		var filtered []runtime.SystemTruthActiveVersion
		for _, item := range s.activeTruthHistory {
			if item.AssetID == strings.TrimSpace(assetID) {
				filtered = append(filtered, item)
			}
		}
		return filtered, nil
	}
	return append([]runtime.SystemTruthActiveVersion(nil), s.activeTruthHistory...), nil
}
