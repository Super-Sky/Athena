// runtime_contract_bootstrap.go keeps the runtime contract foundation aligned with active control-plane truth.
// runtime_contract_bootstrap.go 负责把 runtime contract foundation 与当前 control-plane active truth 对齐。
package app

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"moss/internal/controlplane"
	"moss/internal/runtime"
	runtimetask "moss/internal/runtime/task"
)

const (
	runtimeValidationContractID    = "athena.runtime_contract.runtime_validation.v1"
	runtimeValidationTaskTypeID    = "athena.task_type.runtime_validation.v1"
	runtimeValidationTaskTypeKey   = "runtime_validation"
	runtimeValidationContractName  = "Athena Runtime Validation Contract"
	runtimeFoundationBootstrapNote = "control_plane_runtime_foundation_bootstrap"
)

// runtimeContractFoundationStore groups the persistence interfaces needed by the bootstrap sync.
// runtimeContractFoundationStore 汇总 foundation bootstrap 需要的持久化接口。
type runtimeContractFoundationStore interface {
	runtime.RuntimeContractStore
	runtime.TaskTypeRegistryStore
	runtime.HookBindingStore
	runtime.SystemTruthLifecycleStore
}

// runtimeContractFoundationControlPlaneReader exposes the active truth state needed by the bootstrap sync.
// runtimeContractFoundationControlPlaneReader 暴露 foundation bootstrap 所需的 active truth 读取能力。
type runtimeContractFoundationControlPlaneReader interface {
	TruthDirInfo(context.Context) (controlplane.TruthDirInfo, error)
	ListSystemResourceDetails(context.Context) ([]controlplane.SystemResourceDetail, error)
}

type runtimeFoundationHookSeed struct {
	id            string
	hookPoint     string
	bindingKind   string
	bindingRef    string
	orderIndex    int
	failurePolicy string
	config        map[string]any
}

type registeredTaskTypeValidatorSeed struct {
	id          string
	typeKey     string
	displayName string
	description string
	scene       string
	aliases     []any
	properties  map[string]any
}

var runtimeValidationHookSeeds = []runtimeFoundationHookSeed{
	{
		id:            "athena.hook.runtime_validation.before_run.runtime_contract_guard.v1",
		hookPoint:     runtime.HookPointBeforeRun,
		bindingKind:   runtime.HookBindingKindEinoMiddleware,
		bindingRef:    "runtime_contract_guard",
		orderIndex:    10,
		failurePolicy: runtime.HookFailurePolicyFailClosed,
		config:        map[string]any{"mode": "record_contract"},
	},
	{
		id:            "athena.hook.runtime_validation.before_run.system_truth_guard.v1",
		hookPoint:     runtime.HookPointBeforeRun,
		bindingKind:   runtime.HookBindingKindPolicyRef,
		bindingRef:    "system_truth_guard",
		orderIndex:    20,
		failurePolicy: runtime.HookFailurePolicyRecordOnly,
		config:        map[string]any{"mode": "record_active_truth"},
	},
	{
		id:            "athena.hook.runtime_validation.before_projection.projection_boundary_guard.v1",
		hookPoint:     runtime.HookPointBeforeProjection,
		bindingKind:   runtime.HookBindingKindGraphNode,
		bindingRef:    "projection_boundary_guard",
		orderIndex:    30,
		failurePolicy: runtime.HookFailurePolicyRecordOnly,
		config:        map[string]any{"mode": "candidate_boundary"},
	},
}

var registeredTaskTypeValidatorSeeds = []registeredTaskTypeValidatorSeed{
	{
		id:          "athena.task_type.inspection_task.v1",
		typeKey:     runtimetask.InputKindInspectionTask,
		displayName: "Inspection Task",
		description: "Register the generic inspection task boundary without owning business evidence.",
		scene:       "inspection",
		aliases:     []any{"legacy_inspection_task"},
		properties: map[string]any{
			"workspace_id":            map[string]any{"type": "string"},
			"main_session_id":         map[string]any{"type": "string"},
			"integration_instance_id": map[string]any{"type": "string"},
			"trigger_type":            map[string]any{"type": "string"},
			"query":                   map[string]any{"type": "string"},
			"input_payload":           map[string]any{"type": "object"},
			"global_context":          map[string]any{"type": "object"},
			"app_context":             map[string]any{"type": "object"},
		},
	},
	{
		id:          "athena.task_type.integration_event.v1",
		typeKey:     runtimetask.InputKindIntegrationEvent,
		displayName: "Integration Event",
		description: "Register the generic integration event boundary while keeping connector semantics outside core.",
		scene:       "default",
		aliases:     []any{"legacy_integration_event"},
		properties: map[string]any{
			"workspace_id":            map[string]any{"type": "string"},
			"integration_instance_id": map[string]any{"type": "string"},
			"trigger_type":            map[string]any{"type": "string"},
			"input_payload":           map[string]any{"type": "object"},
			"global_context":          map[string]any{"type": "object"},
			"app_context":             map[string]any{"type": "object"},
		},
	},
	{
		id:          "athena.task_type.scheduled_job.v1",
		typeKey:     runtimetask.InputKindScheduledJob,
		displayName: "Scheduled Job",
		description: "Register the generic scheduled job boundary without taking ownership of scheduler policy.",
		scene:       "workflow",
		aliases:     []any{"legacy_scheduled_job"},
		properties: map[string]any{
			"workspace_id":       map[string]any{"type": "string"},
			"automation_task_id": map[string]any{"type": "string"},
			"trigger_type":       map[string]any{"type": "string"},
			"input_payload":      map[string]any{"type": "object"},
			"global_context":     map[string]any{"type": "object"},
		},
	},
	{
		id:          "athena.task_type.workflow_step_request.v1",
		typeKey:     runtimetask.InputKindWorkflowStepRequest,
		displayName: "Workflow Step Request",
		description: "Register the generic workflow step request boundary without materializing business state.",
		scene:       "workflow",
		aliases:     []any{"legacy_workflow_step_request"},
		properties: map[string]any{
			"workspace_id":    map[string]any{"type": "string"},
			"workflow_run_id": map[string]any{"type": "string"},
			"step_id":         map[string]any{"type": "string"},
			"input_payload":   map[string]any{"type": "object"},
			"global_context":  map[string]any{"type": "object"},
			"app_context":     map[string]any{"type": "object"},
		},
	},
}

// syncRuntimeContractFoundation keeps Athena-owned runtime foundation records present for the current control-plane truth state.
// syncRuntimeContractFoundation 保证当前 control-plane truth 状态对应的 Athena-owned runtime foundation 记录已存在。
func (s *Service) syncRuntimeContractFoundation(ctx context.Context) error {
	if s == nil || s.ControlPlane == nil || s.RuntimeStore == nil {
		return nil
	}
	store, ok := s.RuntimeStore.(runtimeContractFoundationStore)
	if !ok {
		return nil
	}
	return syncRuntimeContractFoundationSnapshot(ctx, s.ControlPlane, store)
}

func syncRuntimeContractFoundationSnapshot(ctx context.Context, controlPlane runtimeContractFoundationControlPlaneReader, store runtimeContractFoundationStore) error {
	if controlPlane == nil || store == nil {
		return nil
	}
	truthInfo, err := controlPlane.TruthDirInfo(ctx)
	if err != nil {
		return err
	}
	validationContract, err := ensureRuntimeValidationContract(ctx, store, truthInfo)
	if err != nil {
		return err
	}
	if err := ensureRuntimeValidationTaskType(ctx, store, validationContract.ID); err != nil {
		return err
	}
	for _, seed := range registeredTaskTypeValidatorSeeds {
		validatorContract, err := ensureRegisteredTaskTypeValidatorContract(ctx, store, seed, truthInfo)
		if err != nil {
			return err
		}
		if err := ensureRegisteredTaskTypeValidator(ctx, store, seed, validatorContract.ID); err != nil {
			return err
		}
	}
	if err := ensureRuntimeValidationHooks(ctx, store, validationContract.ID); err != nil {
		return err
	}
	items, err := controlPlane.ListSystemResourceDetails(ctx)
	if err != nil {
		return err
	}
	for _, item := range items {
		if err := ensureActiveSystemTruthLifecycle(ctx, store, item); err != nil {
			return err
		}
	}
	return nil
}

func ensureRuntimeValidationContract(ctx context.Context, store runtime.RuntimeContractStore, truthInfo controlplane.TruthDirInfo) (runtime.RuntimeContract, error) {
	if existing, ok, err := store.GetRuntimeContract(ctx, runtimeValidationContractID); err != nil {
		return runtime.RuntimeContract{}, err
	} else if ok {
		return existing, nil
	}
	now := time.Now().UTC()
	return store.CreateRuntimeContract(ctx, runtime.RuntimeContract{
		ID:       runtimeValidationContractID,
		Name:     runtimeValidationContractName,
		Version:  "v1",
		Status:   runtime.RuntimeContractStatusActive,
		TaskType: runtimeValidationTaskTypeKey,
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"workspace_id": map[string]any{"type": "string"},
				"scene":        map[string]any{"type": "string"},
				"prompt":       map[string]any{"type": "string"},
			},
		},
		ExecutionProfile: map[string]any{
			"framework": "eino",
			"surface":   "graph",
			"mode":      "control_plane_validation",
		},
		ExitPolicy: map[string]any{
			"terminal_projector": "runtime_terminal_projector",
			"max_turns":          float64(1),
		},
		CapabilityProfile: map[string]any{
			"primary_skill":         "runtime_validation",
			"prefer_direct_answer":  true,
			"validation_mcp_server": "athena-validation-mcp",
		},
		GovernancePolicyRefs: map[string]any{
			"tool_governance": "tool_governance_policy.effective",
		},
		HookBindings: map[string]any{
			"before_run":        []any{"runtime_contract_guard", "system_truth_guard"},
			"before_projection": []any{"projection_boundary_guard"},
		},
		ProjectionPolicy: map[string]any{
			"default_candidate_kind": "prepared_execution",
			"semantic_boundary":      "runtime_projection.v1",
		},
		SystemTruthRefs: map[string]any{
			"truth_dir":         strings.TrimSpace(truthInfo.Path),
			"truth_dir_version": strings.TrimSpace(truthInfo.Version),
			"source":            runtimeFoundationBootstrapNote,
		},
		IdempotencyScope: "runtime_contract:" + runtimeValidationTaskTypeKey,
		IdempotencyKey:   runtimeValidationTaskTypeKey + ":v1",
		Metadata: map[string]any{
			"source": runtimeFoundationBootstrapNote,
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func ensureRuntimeValidationTaskType(ctx context.Context, store runtime.TaskTypeRegistryStore, contractID string) error {
	if existing, ok, err := store.GetTaskTypeRegistrationByKey(ctx, runtimeValidationTaskTypeKey); err != nil {
		return err
	} else if ok && strings.TrimSpace(existing.DefaultContractID) != "" {
		return nil
	}
	now := time.Now().UTC()
	_, err := store.PutTaskTypeRegistration(ctx, runtime.TaskTypeRegistration{
		ID:                runtimeValidationTaskTypeID,
		TypeKey:           runtimeValidationTaskTypeKey,
		DisplayName:       "Runtime Validation",
		Description:       "Validate Athena runtime persistence through the Eino graph foundation.",
		Status:            runtime.TaskTypeStatusActive,
		InputSchema:       map[string]any{"type": "object"},
		ValidatorRefs:     map[string]any{"validators": []any{"runtime_contract_input"}},
		DefaultContractID: contractID,
		Compatibility:     map[string]any{"legacy_aliases": []any{"validation_run"}},
		Metadata:          map[string]any{"source": runtimeFoundationBootstrapNote},
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	return err
}

func ensureRegisteredTaskTypeValidatorContract(ctx context.Context, store runtime.RuntimeContractStore, seed registeredTaskTypeValidatorSeed, truthInfo controlplane.TruthDirInfo) (runtime.RuntimeContract, error) {
	contractID := registeredTaskTypeValidatorContractID(seed.typeKey)
	if existing, ok, err := store.GetRuntimeContract(ctx, contractID); err != nil {
		return runtime.RuntimeContract{}, err
	} else if ok && strings.TrimSpace(existing.ID) != "" {
		return existing, nil
	}
	now := time.Now().UTC()
	return store.PutRuntimeContract(ctx, runtime.RuntimeContract{
		ID:       contractID,
		Name:     seed.displayName + " Validator Contract",
		Version:  "v1",
		Status:   runtime.RuntimeContractStatusActive,
		TaskType: seed.typeKey,
		InputSchema: map[string]any{
			"type":                 "object",
			"additionalProperties": true,
			"properties":           cloneAnyMap(seed.properties),
			"required":             []any{},
		},
		ExecutionProfile: map[string]any{
			"framework": "athena_runtime",
			"surface":   "registered_task_type_validator",
			"mode":      "advisory_contract",
		},
		ExitPolicy: map[string]any{
			"failure_policy":       "record_only",
			"business_truth_scope": "out_of_core",
		},
		CapabilityProfile: map[string]any{
			"validator_contract": true,
			"scene":              seed.scene,
			"evidence_ownership": "application_or_connector",
		},
		ProjectionPolicy: map[string]any{
			"semantic_boundary":         "runtime_task_input.v1",
			"forbid_business_record":    true,
			"projection_candidate_only": true,
		},
		SystemTruthRefs: map[string]any{
			"truth_dir":         strings.TrimSpace(truthInfo.Path),
			"truth_dir_version": strings.TrimSpace(truthInfo.Version),
			"source":            runtimeFoundationBootstrapNote,
		},
		IdempotencyScope: "runtime_task_type_validator:" + seed.typeKey,
		IdempotencyKey:   seed.typeKey + ":validator:v1",
		Metadata: map[string]any{
			"source":                  runtimeFoundationBootstrapNote,
			"validator_contract_type": "registered_task_type",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
}

func ensureRegisteredTaskTypeValidator(ctx context.Context, store runtime.TaskTypeRegistryStore, seed registeredTaskTypeValidatorSeed, contractID string) error {
	existing, ok, err := store.GetTaskTypeRegistrationByKey(ctx, seed.typeKey)
	if err != nil {
		return err
	}
	if ok && registeredTaskTypeValidatorReady(existing, contractID) {
		return nil
	}
	now := time.Now().UTC()
	item := runtime.TaskTypeRegistration{
		ID:                seed.id,
		TypeKey:           seed.typeKey,
		DisplayName:       seed.displayName,
		Description:       seed.description,
		Status:            runtime.TaskTypeStatusActive,
		InputSchema:       registeredTaskTypeInputSchema(seed),
		ValidatorRefs:     registeredTaskTypeValidatorRefs(seed),
		DefaultContractID: contractID,
		Compatibility:     registeredTaskTypeCompatibility(seed),
		Metadata:          map[string]any{"source": runtimeFoundationBootstrapNote},
		CreatedAt:         now,
		UpdatedAt:         now,
	}
	if ok {
		item.ID = defaultString(strings.TrimSpace(existing.ID), item.ID)
		item.CreatedAt = existing.CreatedAt
	}
	_, err = store.PutTaskTypeRegistration(ctx, item)
	return err
}

func registeredTaskTypeValidatorReady(item runtime.TaskTypeRegistration, contractID string) bool {
	if strings.TrimSpace(item.DefaultContractID) != strings.TrimSpace(contractID) {
		return false
	}
	if len(item.InputSchema) == 0 || len(item.ValidatorRefs) == 0 {
		return false
	}
	if status, _ := item.ValidatorRefs["status"].(string); strings.TrimSpace(status) != "" && !strings.EqualFold(strings.TrimSpace(status), "ready") {
		return false
	}
	raw, ok := item.ValidatorRefs["validators"]
	if !ok {
		return false
	}
	switch typed := raw.(type) {
	case []any:
		return len(typed) > 0
	case []string:
		return len(typed) > 0
	default:
		return false
	}
}

func registeredTaskTypeInputSchema(seed registeredTaskTypeValidatorSeed) map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": true,
		"properties":           cloneAnyMap(seed.properties),
		"required":             []any{},
	}
}

func registeredTaskTypeValidatorRefs(seed registeredTaskTypeValidatorSeed) map[string]any {
	return map[string]any{
		"contract_version": "task_type_validator.v1",
		"schema_ref":       "runtime.task_type." + seed.typeKey + ".input_schema.v1",
		"validators":       []any{"registered_task_type_input_schema", "runtime_context_boundary"},
		"failure_policy":   "record_only",
		"status":           "ready",
	}
}

func registeredTaskTypeCompatibility(seed registeredTaskTypeValidatorSeed) map[string]any {
	return map[string]any{
		"legacy_aliases":             seed.aliases,
		"normalization_entry":        "internal/runtime/task.NormalizeRequest",
		"required_fields_mode":       "advisory",
		"business_evidence_owner":    "application_or_connector",
		"core_materialization_scope": "projection_candidate_only",
	}
}

func registeredTaskTypeValidatorContractID(typeKey string) string {
	return "athena.runtime_contract." + strings.ReplaceAll(typeKey, "_", ".") + ".validator.v1"
}

func ensureRuntimeValidationHooks(ctx context.Context, store runtime.HookBindingStore, contractID string) error {
	for _, seed := range runtimeValidationHookSeeds {
		if existing, ok, err := store.GetHookBinding(ctx, seed.id); err != nil {
			return err
		} else if ok && strings.TrimSpace(existing.ContractID) != "" {
			continue
		}
		now := time.Now().UTC()
		if _, err := store.CreateHookBinding(ctx, runtime.HookBinding{
			ID:            seed.id,
			ContractID:    contractID,
			HookPoint:     seed.hookPoint,
			BindingKind:   seed.bindingKind,
			BindingRef:    seed.bindingRef,
			OrderIndex:    seed.orderIndex,
			Enabled:       true,
			FailurePolicy: seed.failurePolicy,
			Config:        cloneAnyMap(seed.config),
			Metadata:      map[string]any{"source": runtimeFoundationBootstrapNote},
			CreatedAt:     now,
			UpdatedAt:     now,
		}); err != nil {
			return err
		}
	}
	return nil
}

func ensureActiveSystemTruthLifecycle(ctx context.Context, store runtime.SystemTruthLifecycleStore, item controlplane.SystemResourceDetail) error {
	if strings.TrimSpace(item.AssetID) == "" || !strings.EqualFold(strings.TrimSpace(item.Status), "active") || item.CompileResult == nil {
		return nil
	}
	compileResult := item.CompileResult
	if !strings.EqualFold(strings.TrimSpace(compileResult.Status), "compiled") || strings.TrimSpace(compileResult.CompiledVersion) == "" {
		return nil
	}
	if existing, ok, err := store.GetActiveSystemTruthVersion(ctx, item.AssetID); err != nil {
		return err
	} else if ok && strings.TrimSpace(existing.CompileResultID) != "" {
		return nil
	}
	now := parseRuntimeFoundationTime(compileResult.UpdatedAt)
	source, err := store.CreateSystemTruthSource(ctx, runtime.SystemTruthSource{
		ID:          "athena.system_truth_source." + item.AssetID + "." + uuid.NewString(),
		AssetID:     item.AssetID,
		SourceKind:  defaultString(strings.TrimSpace(item.SourceKind), "truth_dir_source"),
		SourceRef:   strings.TrimSpace(item.SourcePath),
		Status:      runtime.SystemTruthSourceStatusImported,
		Content:     runtimeFoundationSystemTruthPayload(item, compileResult),
		ContentHash: defaultString(strings.TrimSpace(compileResult.SourceChecksum), strings.TrimSpace(compileResult.CompiledChecksum)),
		Metadata: map[string]any{
			"source":            runtimeFoundationBootstrapNote,
			"truth_dir_version": strings.TrimSpace(compileResult.TruthDirVersion),
			"compiled_version":  strings.TrimSpace(compileResult.CompiledVersion),
		},
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	draft, err := store.CreateSystemTruthDraft(ctx, runtime.SystemTruthDraft{
		ID:           "athena.system_truth_draft." + item.AssetID + "." + uuid.NewString(),
		SourceID:     source.ID,
		AssetID:      item.AssetID,
		Status:       runtime.SystemTruthDraftStatusCompiled,
		Author:       "athena-control-plane",
		Reason:       "bootstrap current active system truth",
		BaseActiveID: "",
		Content:      runtimeFoundationSystemTruthPayload(item, compileResult),
		DiffSummary:  "bootstrap active control-plane truth snapshot",
		Metadata: map[string]any{
			"source":            runtimeFoundationBootstrapNote,
			"truth_dir_version": strings.TrimSpace(compileResult.TruthDirVersion),
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		return err
	}
	compiled, err := store.CreateSystemTruthCompileResult(ctx, runtime.SystemTruthCompileResult{
		ID:      "athena.system_truth_compile." + item.AssetID + "." + uuid.NewString(),
		DraftID: draft.ID,
		AssetID: item.AssetID,
		Status:  runtime.SystemTruthCompileStatusSucceeded,
		Summary: defaultString(strings.TrimSpace(compileResult.Summary), "compiled active system truth"),
		Diagnostics: map[string]any{
			"status":            strings.TrimSpace(compileResult.Status),
			"compiled_version":  strings.TrimSpace(compileResult.CompiledVersion),
			"truth_dir_version": strings.TrimSpace(compileResult.TruthDirVersion),
		},
		CompiledPayload: map[string]any{
			"asset_id":          item.AssetID,
			"compiled_version":  strings.TrimSpace(compileResult.CompiledVersion),
			"truth_dir_version": strings.TrimSpace(compileResult.TruthDirVersion),
			"compiled_checksum": strings.TrimSpace(compileResult.CompiledChecksum),
			"asset_type":        strings.TrimSpace(item.AssetType),
		},
		ContentHash: defaultString(strings.TrimSpace(compileResult.CompiledChecksum), strings.TrimSpace(compileResult.SourceChecksum)),
		Metadata: map[string]any{
			"source": runtimeFoundationBootstrapNote,
		},
		CreatedAt: now,
	})
	if err != nil {
		return err
	}
	_, err = store.ActivateSystemTruthVersion(ctx, runtime.SystemTruthActiveVersion{
		ID:              "athena.system_truth_active." + item.AssetID + "." + strings.TrimSpace(compileResult.CompiledVersion),
		AssetID:         item.AssetID,
		CompileResultID: compiled.ID,
		DraftID:         draft.ID,
		ActivatedBy:     "athena-control-plane",
		Reason:          "bootstrap active control-plane truth",
		Metadata: map[string]any{
			"source":            runtimeFoundationBootstrapNote,
			"truth_dir_version": strings.TrimSpace(compileResult.TruthDirVersion),
			"compiled_version":  strings.TrimSpace(compileResult.CompiledVersion),
		},
		ActivatedAt: now,
	})
	return err
}

func runtimeFoundationSystemTruthPayload(item controlplane.SystemResourceDetail, compileResult *controlplane.SystemResourceCompileResult) map[string]any {
	return map[string]any{
		"asset_id":          strings.TrimSpace(item.AssetID),
		"asset_name":        strings.TrimSpace(item.AssetName),
		"asset_type":        strings.TrimSpace(item.AssetType),
		"source_path":       strings.TrimSpace(item.SourcePath),
		"compiled_version":  strings.TrimSpace(compileResult.CompiledVersion),
		"truth_dir_version": strings.TrimSpace(compileResult.TruthDirVersion),
	}
}

func parseRuntimeFoundationTime(raw string) time.Time {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return time.Now().UTC()
	}
	for _, layout := range []string{time.RFC3339Nano, time.RFC3339} {
		if ts, err := time.Parse(layout, raw); err == nil {
			return ts.UTC()
		}
	}
	return time.Now().UTC()
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		if strings.TrimSpace(key) == "" {
			continue
		}
		out[key] = value
	}
	return out
}
