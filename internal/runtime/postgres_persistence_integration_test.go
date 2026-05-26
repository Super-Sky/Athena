package runtime

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	runtimetask "moss/internal/runtime/task"
)

// TestPostgresRuntimeContractStoreIntegrationRoundTrip verifies v2 runtime contracts can be created, read, and listed through PostgreSQL.
// TestPostgresRuntimeContractStoreIntegrationRoundTrip 用于验证 v2 runtime contract 可以通过 PostgreSQL 创建、读取和列出。
func TestPostgresRuntimeContractStoreIntegrationRoundTrip(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	contractID := "itest-runtime-contract-" + time.Now().UTC().Format("20060102150405.000000000")
	contract, err := store.CreateRuntimeContract(ctx, RuntimeContract{
		ID:       contractID,
		Name:     "Runtime Validation Contract",
		Version:  "v1",
		Status:   RuntimeContractStatusActive,
		TaskType: "runtime.validation",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"prompt": map[string]any{"type": "string"},
			},
		},
		ExecutionProfile: map[string]any{
			"framework": "eino",
			"surface":   "graph",
		},
		ExitPolicy: map[string]any{
			"max_turns": float64(4),
		},
		CapabilityProfile: map[string]any{
			"model_profile_ref": "default_chat_model",
			"sandbox_required":  true,
		},
		GovernancePolicyRefs: map[string]any{
			"tool_governance": "tool_governance_policy.core.default",
		},
		HookBindings: map[string]any{
			"before_run": []any{"runtime_contract_guard"},
		},
		ProjectionPolicy: map[string]any{
			"candidate_kinds": []any{"terminal_output", "validation_summary"},
		},
		SystemTruthRefs: map[string]any{
			"system_prompt": "system_prompt.core.active",
		},
		IdempotencyScope: "runtime_contract:runtime.validation",
		IdempotencyKey:   "runtime.validation:v1",
		Metadata: map[string]any{
			"source": "integration_test",
		},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateRuntimeContract() error = %v", err)
	}
	defer cleanupPostgresRuntimeContractArtifacts(t, db, contract.ID)

	got, ok, err := store.GetRuntimeContract(ctx, contract.ID)
	if err != nil {
		t.Fatalf("GetRuntimeContract() error = %v", err)
	}
	if !ok || got.ID != contract.ID || got.TaskType != "runtime.validation" || got.Status != RuntimeContractStatusActive {
		t.Fatalf("GetRuntimeContract() = %#v, %v; want contract %q", got, ok, contract.ID)
	}
	if got.ExecutionProfile["framework"] != "eino" || got.ProjectionPolicy["candidate_kinds"] == nil {
		t.Fatalf("GetRuntimeContract() lost JSONB fields: %#v", got)
	}

	listed, err := store.ListRuntimeContracts(ctx, RuntimeContractListFilter{TaskType: "runtime.validation", Status: RuntimeContractStatusActive, Limit: 10})
	if err != nil {
		t.Fatalf("ListRuntimeContracts() error = %v", err)
	}
	if !containsRuntimeContract(listed, contract.ID) {
		t.Fatalf("ListRuntimeContracts() missing %q: %#v", contract.ID, listed)
	}

	_, err = store.CreateRuntimeContract(ctx, RuntimeContract{
		Name:        "Credential Contract",
		Version:     "v1",
		Status:      RuntimeContractStatusDraft,
		TaskType:    "runtime.validation",
		InputSchema: map[string]any{"api_key": "sk-raw"},
	})
	if !errors.Is(err, ErrInvalidRuntimePersistenceInput) {
		t.Fatalf("credential-like CreateRuntimeContract() error = %v, want ErrInvalidRuntimePersistenceInput", err)
	}
}

// TestPostgresRuntimeContractFoundationRoundTrip verifies task type, hook, and system truth lifecycle foundation records.
// TestPostgresRuntimeContractFoundationRoundTrip 用于验证 task type、hook 和 system truth lifecycle foundation 记录。
func TestPostgresRuntimeContractFoundationRoundTrip(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	suffix := time.Now().UTC().Format("20060102150405.000000000")
	now := time.Date(2026, 5, 19, 11, 0, 0, 0, time.UTC)
	contract, err := store.CreateRuntimeContract(ctx, RuntimeContract{
		ID:        "itest-contract-foundation-" + suffix,
		Name:      "Runtime Validation Contract",
		Version:   "v1",
		Status:    RuntimeContractStatusActive,
		TaskType:  "runtime.validation." + suffix,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateRuntimeContract() error = %v", err)
	}
	defer cleanupPostgresRuntimeContractFoundationArtifacts(t, db, contract.ID, "runtime.validation."+suffix, "system_prompt.core."+suffix)

	taskType, err := store.CreateTaskTypeRegistration(ctx, TaskTypeRegistration{
		TypeKey:           "runtime.validation." + suffix,
		DisplayName:       "Runtime Validation",
		Description:       "Internal validation task type",
		Status:            TaskTypeStatusActive,
		InputSchema:       map[string]any{"type": "object"},
		ValidatorRefs:     map[string]any{"validators": []any{"runtime_contract_input"}},
		DefaultContractID: contract.ID,
		Compatibility:     map[string]any{"legacy_aliases": []any{"validation_run"}},
		Metadata:          map[string]any{"source": "integration_test"},
		CreatedAt:         now,
		UpdatedAt:         now,
	})
	if err != nil {
		t.Fatalf("CreateTaskTypeRegistration() error = %v", err)
	}
	byKey, ok, err := store.GetTaskTypeRegistrationByKey(ctx, taskType.TypeKey)
	if err != nil {
		t.Fatalf("GetTaskTypeRegistrationByKey() error = %v", err)
	}
	if !ok || byKey.DefaultContractID != contract.ID {
		t.Fatalf("GetTaskTypeRegistrationByKey() = %#v, %v; want default contract %q", byKey, ok, contract.ID)
	}
	taskTypes, err := store.ListTaskTypeRegistrations(ctx, TaskTypeRegistrationListFilter{Status: TaskTypeStatusActive, Limit: 20})
	if err != nil {
		t.Fatalf("ListTaskTypeRegistrations() error = %v", err)
	}
	if !containsTaskType(taskTypes, taskType.TypeKey) {
		t.Fatalf("ListTaskTypeRegistrations() missing %q: %#v", taskType.TypeKey, taskTypes)
	}

	hook, err := store.CreateHookBinding(ctx, HookBinding{
		ContractID:    contract.ID,
		HookPoint:     HookPointBeforeRun,
		BindingKind:   HookBindingKindEinoMiddleware,
		BindingRef:    "runtime_contract_guard",
		OrderIndex:    10,
		Enabled:       true,
		FailurePolicy: HookFailurePolicyFailClosed,
		Config:        map[string]any{"mode": "record_contract"},
		Metadata:      map[string]any{"source": "integration_test"},
		CreatedAt:     now,
		UpdatedAt:     now,
	})
	if err != nil {
		t.Fatalf("CreateHookBinding() error = %v", err)
	}
	enabled := true
	hooks, err := store.ListHookBindings(ctx, HookBindingListFilter{ContractID: contract.ID, HookPoint: HookPointBeforeRun, Enabled: &enabled})
	if err != nil {
		t.Fatalf("ListHookBindings() error = %v", err)
	}
	if len(hooks) != 1 || hooks[0].ID != hook.ID {
		t.Fatalf("ListHookBindings() = %#v, want hook %q", hooks, hook.ID)
	}
	if _, err := store.CreateHookBinding(ctx, HookBinding{ContractID: contract.ID, HookPoint: HookPointBeforeRun, BindingKind: HookBindingKindEinoMiddleware, BindingRef: "arbitrary_user_code"}); !errors.Is(err, ErrInvalidRuntimePersistenceInput) {
		t.Fatalf("unallowlisted CreateHookBinding() error = %v, want ErrInvalidRuntimePersistenceInput", err)
	}

	source, err := store.CreateSystemTruthSource(ctx, SystemTruthSource{
		AssetID:     "system_prompt.core." + suffix,
		SourceKind:  "repo_file",
		SourceRef:   "config/system/truth/sources/core/system_prompt/default.md",
		Status:      SystemTruthSourceStatusImported,
		Content:     map[string]any{"summary": "safe source"},
		ContentHash: "sha256:source",
		Metadata:    map[string]any{"source": "integration_test"},
		CreatedAt:   now,
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthSource() error = %v", err)
	}
	draft, err := store.CreateSystemTruthDraft(ctx, SystemTruthDraft{
		SourceID:    source.ID,
		AssetID:     source.AssetID,
		Status:      SystemTruthDraftStatusDraft,
		Author:      "operator",
		Reason:      "integration test",
		Content:     map[string]any{"summary": "safe draft"},
		DiffSummary: "initial draft",
		Metadata:    map[string]any{"source": "integration_test"},
		CreatedAt:   now,
		UpdatedAt:   now,
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthDraft() error = %v", err)
	}
	failedCompile, err := store.CreateSystemTruthCompileResult(ctx, SystemTruthCompileResult{
		DraftID:         draft.ID,
		AssetID:         draft.AssetID,
		Status:          SystemTruthCompileStatusFailed,
		Summary:         "compile failed",
		Diagnostics:     map[string]any{"errors": []any{"invalid frontmatter"}},
		CompiledPayload: map[string]any{},
		ContentHash:     "sha256:failed",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthCompileResult(failed) error = %v", err)
	}
	_, err = store.ActivateSystemTruthVersion(ctx, SystemTruthActiveVersion{
		AssetID:         draft.AssetID,
		CompileResultID: failedCompile.ID,
		DraftID:         draft.ID,
		ActivatedBy:     "operator",
		Reason:          "should fail",
		ActivatedAt:     now,
	})
	if !errors.Is(err, ErrInvalidRuntimePersistenceInput) {
		t.Fatalf("ActivateSystemTruthVersion(failed compile) error = %v, want ErrInvalidRuntimePersistenceInput", err)
	}
	successCompile, err := store.CreateSystemTruthCompileResult(ctx, SystemTruthCompileResult{
		DraftID:         draft.ID,
		AssetID:         draft.AssetID,
		Status:          SystemTruthCompileStatusSucceeded,
		Summary:         "compile succeeded",
		Diagnostics:     map[string]any{"warnings": []any{}},
		CompiledPayload: map[string]any{"guidance_text": "safe guidance"},
		ContentHash:     "sha256:success",
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateSystemTruthCompileResult(success) error = %v", err)
	}
	active, err := store.ActivateSystemTruthVersion(ctx, SystemTruthActiveVersion{
		AssetID:         draft.AssetID,
		CompileResultID: successCompile.ID,
		DraftID:         draft.ID,
		ActivatedBy:     "operator",
		Reason:          "integration activation",
		ActivatedAt:     now,
	})
	if err != nil {
		t.Fatalf("ActivateSystemTruthVersion() error = %v", err)
	}
	gotActive, ok, err := store.GetActiveSystemTruthVersion(ctx, draft.AssetID)
	if err != nil {
		t.Fatalf("GetActiveSystemTruthVersion() error = %v", err)
	}
	if !ok || gotActive.ID != active.ID {
		t.Fatalf("GetActiveSystemTruthVersion() = %#v, %v; want active %q", gotActive, ok, active.ID)
	}
}

// TestPostgresRuntimeStoreIntegrationRoundTrip verifies every core runtime persistence object can be created and listed through PostgreSQL.
// TestPostgresRuntimeStoreIntegrationRoundTrip 用于验证每个核心 runtime 持久化对象都能通过 PostgreSQL 创建和列出。
func TestPostgresRuntimeStoreIntegrationRoundTrip(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Date(2026, 4, 29, 10, 0, 0, 0, time.UTC)
	run, err := store.CreateTaskRun(ctx, TaskRun{
		ID:               "itest-runtime-run-" + time.Now().UTC().Format("20060102150405.000000000"),
		TaskID:           "task-itest-runtime",
		TaskType:         runtimetask.InputKindChat,
		TaskSubtype:      "default",
		InputKind:        runtimetask.InputKindChat,
		Scene:            "chat",
		WorkspaceID:      "itest-workspace",
		AppInstanceID:    "itest-app",
		Status:           TaskRunStatusRunning,
		IdempotencyScope: "itest-workspace:itest-app:chat:chat",
		IdempotencyKey:   "itest-idempotency-key",
		RetentionPolicy:  "runtime_default",
		Metadata:         map[string]any{"source": "integration_test"},
		StartedAt:        &now,
		CreatedAt:        now,
		UpdatedAt:        now,
	})
	if err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	defer cleanupPostgresRuntimeArtifacts(t, db, run.ID)

	gotRun, ok, err := store.GetTaskRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("GetTaskRun() error = %v", err)
	}
	if !ok || gotRun.ID != run.ID {
		t.Fatalf("GetTaskRun() = %#v, %v; want run %q", gotRun, ok, run.ID)
	}
	listedRuns, err := store.ListTaskRuns(ctx, TaskRunListFilter{WorkspaceID: "itest-workspace", Status: TaskRunStatusRunning, Limit: 10})
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if !containsRun(listedRuns, run.ID) {
		t.Fatalf("ListTaskRuns() missing run %q: %#v", run.ID, listedRuns)
	}

	step, err := store.CreateTaskStep(ctx, TaskStep{
		RunID:     run.ID,
		Sequence:  1,
		StepType:  "capability_resolution",
		Name:      "resolve_capabilities",
		Status:    TaskStepStatusSuccess,
		Metadata:  map[string]any{"safe": true},
		StartedAt: &now,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateTaskStep() error = %v", err)
	}
	steps, err := store.ListTaskSteps(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListTaskSteps() error = %v", err)
	}
	if len(steps) != 1 || steps[0].ID != step.ID {
		t.Fatalf("ListTaskSteps() = %#v, want step %q", steps, step.ID)
	}
	gotStep, ok, err := store.GetTaskStep(ctx, step.ID)
	if err != nil {
		t.Fatalf("GetTaskStep() error = %v", err)
	}
	if !ok || gotStep.ID != step.ID {
		t.Fatalf("GetTaskStep() = %#v, %v; want step %q", gotStep, ok, step.ID)
	}

	runEvent, err := store.CreateLifecycleEvent(ctx, TaskRunLifecycleEvent{
		RunID:       run.ID,
		EventType:   "run_running",
		SubjectType: LifecycleSubjectRun,
		SubjectID:   run.ID,
		FromStatus:  TaskRunStatusCreated,
		ToStatus:    TaskRunStatusRunning,
		Reason:      "integration_test",
		Metadata:    map[string]any{"safe_label": "run"},
		OccurredAt:  now,
	})
	if err != nil {
		t.Fatalf("CreateLifecycleEvent(run) error = %v", err)
	}
	stepEvent, err := store.CreateLifecycleEvent(ctx, TaskRunLifecycleEvent{
		RunID:       run.ID,
		StepID:      step.ID,
		EventType:   "step_completed",
		SubjectType: LifecycleSubjectStep,
		SubjectID:   step.ID,
		FromStatus:  TaskStepStatusRunning,
		ToStatus:    TaskStepStatusSuccess,
		Reason:      "integration_test",
		Metadata:    map[string]any{"safe_label": "step"},
		OccurredAt:  now.Add(time.Millisecond),
	})
	if err != nil {
		t.Fatalf("CreateLifecycleEvent(step) error = %v", err)
	}
	eventsByRun, err := store.ListLifecycleEventsByRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ListLifecycleEventsByRun() error = %v", err)
	}
	if len(eventsByRun) != 2 || eventsByRun[0].ID != runEvent.ID || eventsByRun[1].ID != stepEvent.ID {
		t.Fatalf("ListLifecycleEventsByRun() = %#v", eventsByRun)
	}
	eventsByStep, err := store.ListLifecycleEventsBySubject(ctx, LifecycleSubjectStep, step.ID)
	if err != nil {
		t.Fatalf("ListLifecycleEventsBySubject() error = %v", err)
	}
	if len(eventsByStep) != 1 || eventsByStep[0].ID != stepEvent.ID {
		t.Fatalf("ListLifecycleEventsBySubject() = %#v, want %q", eventsByStep, stepEvent.ID)
	}
	gotEvent, ok, err := store.GetLifecycleEvent(ctx, stepEvent.ID)
	if err != nil {
		t.Fatalf("GetLifecycleEvent() error = %v", err)
	}
	if !ok || gotEvent.ID != stepEvent.ID {
		t.Fatalf("GetLifecycleEvent() = %#v, %v; want %q", gotEvent, ok, stepEvent.ID)
	}

	trace, err := store.CreateRuntimeTrace(ctx, RuntimeTrace{
		RunID:     run.ID,
		StepID:    step.ID,
		TraceType: "safe_summary",
		Summary:   "integration trace summary",
		SafeLabels: map[string]string{
			"component": "runtime",
		},
		RedactedPayload: map[string]any{"credential_raw": "[redacted]", "stable_field": "ok"},
		Metadata:        map[string]any{"redaction_policy": "whitelist_summary"},
		CreatedAt:       now,
	})
	if err != nil {
		t.Fatalf("CreateRuntimeTrace() error = %v", err)
	}
	traces, err := store.ListRuntimeTraces(ctx, RuntimeTraceListFilter{RunID: run.ID, StepID: step.ID})
	if err != nil {
		t.Fatalf("ListRuntimeTraces() error = %v", err)
	}
	if len(traces) != 1 || traces[0].ID != trace.ID || traces[0].RedactedPayload["credential_raw"] != "[redacted]" {
		t.Fatalf("ListRuntimeTraces() = %#v, want trace %q", traces, trace.ID)
	}
	gotTrace, ok, err := store.GetRuntimeTrace(ctx, trace.ID)
	if err != nil {
		t.Fatalf("GetRuntimeTrace() error = %v", err)
	}
	if !ok || gotTrace.ID != trace.ID {
		t.Fatalf("GetRuntimeTrace() = %#v, %v; want trace %q", gotTrace, ok, trace.ID)
	}

	cost := 0.01
	usage, err := store.CreateUsage(ctx, Usage{
		RunID:        run.ID,
		StepID:       step.ID,
		ResourceType: "tool_call",
		Provider:     "athena",
		ResourceName: "test_tool",
		Unit:         "operation",
		Amount:       1,
		Cost:         &cost,
		Currency:     "USD",
		Metadata:     map[string]any{"generic": true},
		CreatedAt:    now,
	})
	if err != nil {
		t.Fatalf("CreateUsage() error = %v", err)
	}
	usages, err := store.ListUsage(ctx, UsageListFilter{RunID: run.ID, StepID: step.ID})
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	if len(usages) != 1 || usages[0].ID != usage.ID || usages[0].ResourceType != "tool_call" {
		t.Fatalf("ListUsage() = %#v, want usage %q", usages, usage.ID)
	}
	gotUsage, ok, err := store.GetUsage(ctx, usage.ID)
	if err != nil {
		t.Fatalf("GetUsage() error = %v", err)
	}
	if !ok || gotUsage.ID != usage.ID {
		t.Fatalf("GetUsage() = %#v, %v; want usage %q", gotUsage, ok, usage.ID)
	}

	projection, err := store.CreateProjectionCandidate(ctx, ProjectionCandidate{
		RunID:           run.ID,
		StepID:          step.ID,
		CandidateKind:   "minimal_output",
		Status:          "ready",
		Summary:         "candidate summary",
		SchemaVersion:   "projection.v1",
		RedactedPayload: map[string]any{"summary": "safe output"},
		SemanticPayload: map[string]any{"answer": "safe output"},
		ArtifactRefs:    map[string]any{"refs": []any{"artifact://runtime/test"}},
		UIHints:         map[string]any{"severity": "info"},
		MaterializationTarget: map[string]any{
			"target_type": "none",
		},
		Metadata:  map[string]any{"projection_scope": "candidate_output"},
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateProjectionCandidate() error = %v", err)
	}
	projections, err := store.ListProjectionCandidates(ctx, ProjectionCandidateListFilter{RunID: run.ID, StepID: step.ID})
	if err != nil {
		t.Fatalf("ListProjectionCandidates() error = %v", err)
	}
	if len(projections) != 1 || projections[0].ID != projection.ID {
		t.Fatalf("ListProjectionCandidates() = %#v, want projection %q", projections, projection.ID)
	}
	gotProjection, ok, err := store.GetProjectionCandidate(ctx, projection.ID)
	if err != nil {
		t.Fatalf("GetProjectionCandidate() error = %v", err)
	}
	if !ok || gotProjection.ID != projection.ID {
		t.Fatalf("GetProjectionCandidate() = %#v, %v; want projection %q", gotProjection, ok, projection.ID)
	}
	if gotProjection.SchemaVersion != "projection.v1" || gotProjection.SemanticPayload["answer"] != "safe output" || gotProjection.UIHints["severity"] != "info" {
		t.Fatalf("GetProjectionCandidate() lost semantic projection fields: %#v", gotProjection)
	}

	autoSchemaProjection, err := store.CreateProjectionCandidate(ctx, ProjectionCandidate{
		RunID:         run.ID,
		StepID:        step.ID,
		CandidateKind: "prepared_execution",
		Status:        "ready",
		Summary:       "projection with implicit schema version",
		RedactedPayload: map[string]any{
			"message_count": 1,
		},
		CreatedAt: now.Add(time.Millisecond),
	})
	if err != nil {
		t.Fatalf("CreateProjectionCandidate(implicit schema version) error = %v", err)
	}
	if autoSchemaProjection.SchemaVersion != ProjectionSchemaVersionPreparedExecution {
		t.Fatalf("implicit schema version = %q, want %q", autoSchemaProjection.SchemaVersion, ProjectionSchemaVersionPreparedExecution)
	}
}

// TestPersistenceWriterPostgresIntegration verifies the deterministic writer creates the minimum accepted record set in PostgreSQL.
// TestPersistenceWriterPostgresIntegration 用于验证确定性 writer 会在 PostgreSQL 中创建验收要求的最小记录集。
func TestPersistenceWriterPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Date(2026, 4, 29, 11, 0, 0, 0, time.UTC)
	writer := PersistenceWriter{Store: store, Now: func() time.Time { return now }}
	recordSet, err := writer.WriteMinimalRun(ctx, MinimalPersistenceInput{
		Task: &runtimetask.RuntimeTask{
			TaskID:        "writer-task-itest",
			TaskType:      runtimetask.InputKindChat,
			TaskSubtype:   "default",
			InputKind:     runtimetask.InputKindChat,
			Scene:         "chat",
			WorkspaceID:   "writer-workspace",
			AppInstanceID: "writer-app",
			OutputMode:    runtimetask.DefaultOutputModeText,
		},
		IdempotencyKey:  "writer-idempotency-key",
		RetentionPolicy: "runtime_default",
	})
	if err != nil {
		t.Fatalf("WriteMinimalRun() error = %v", err)
	}
	defer cleanupPostgresRuntimeArtifacts(t, db, recordSet.Run.ID)

	if recordSet.Run.ID == "" || recordSet.Step.ID == "" || len(recordSet.Events) < 5 {
		t.Fatalf("incomplete record set = %#v", recordSet)
	}
	if recordSet.Trace.RedactedPayload["credential_raw"] != "[redacted]" {
		t.Fatalf("trace did not store redacted credential marker: %#v", recordSet.Trace.RedactedPayload)
	}
	if recordSet.Usage.ResourceType != "runtime_writer" || recordSet.Usage.Unit != "operation" {
		t.Fatalf("usage is not generic writer usage: %#v", recordSet.Usage)
	}
	if recordSet.Projection.CandidateKind != "minimal_output" {
		t.Fatalf("projection candidate kind = %q", recordSet.Projection.CandidateKind)
	}
}

// TestRuntimeTerminalProjectorPostgresIntegration verifies terminal graph outcomes are persisted as safe runtime records.
// TestRuntimeTerminalProjectorPostgresIntegration 用于验证 graph 终态结果会作为安全 runtime 记录写入 PostgreSQL。
func TestRuntimeTerminalProjectorPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	now := time.Date(2026, 5, 6, 9, 30, 0, 0, time.UTC)
	recordSet, err := (PersistenceWriter{Store: store, Now: func() time.Time { return now }}).WriteMinimalRun(ctx, MinimalPersistenceInput{
		Task: &runtimetask.RuntimeTask{
			TaskID:        "terminal-projector-itest",
			TaskType:      runtimetask.InputKindChat,
			InputKind:     runtimetask.InputKindChat,
			Scene:         "chat",
			WorkspaceID:   "terminal-workspace",
			AppInstanceID: "terminal-app",
			OutputMode:    runtimetask.DefaultOutputModeText,
		},
		IdempotencyKey: "terminal-idempotency-key",
	})
	if err != nil {
		t.Fatalf("WriteMinimalRun() error = %v", err)
	}
	defer cleanupPostgresRuntimeArtifacts(t, db, recordSet.Run.ID)

	projector := RuntimeTerminalProjector{
		Store:     store,
		Now:       func() time.Time { return now.Add(time.Second) },
		RecordSet: &recordSet,
		Metadata:  map[string]any{"graph_mode": "wrapped_turn_executor"},
	}
	err = projector.Project(ctx, RuntimeTerminalOutcome{
		Status:          RuntimeTerminalStatusCompleted,
		Content:         "safe answer with api_key=secret and Authorization: Bearer sk-itest",
		ToolSideEffects: true,
		Metadata:        map[string]any{"respond_stage": "itest"},
	})
	if err != nil {
		t.Fatalf("Project() error = %v", err)
	}

	traces, err := store.ListRuntimeTraces(ctx, RuntimeTraceListFilter{RunID: recordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListRuntimeTraces() error = %v", err)
	}
	if !containsTraceType(traces, "terminal_output_summary") {
		t.Fatalf("traces = %#v, want terminal output trace", traces)
	}
	for _, trace := range traces {
		if strings.Contains(trace.Summary, "sk-itest") || strings.Contains(trace.Summary, "secret") || strings.Contains(fmt.Sprint(trace.RedactedPayload), "sk-itest") {
			t.Fatalf("trace leaked raw credential-like content: %#v", trace)
		}
	}
	usages, err := store.ListUsage(ctx, UsageListFilter{RunID: recordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	if !containsUsageResource(usages, "runtime_output") {
		t.Fatalf("usages = %#v, want terminal runtime_output usage", usages)
	}
	projections, err := store.ListProjectionCandidates(ctx, ProjectionCandidateListFilter{RunID: recordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListProjectionCandidates() error = %v", err)
	}
	if !containsProjectionKind(projections, "terminal_output") {
		t.Fatalf("projections = %#v, want terminal output projection", projections)
	}
	events, err := store.ListLifecycleEventsByRun(ctx, recordSet.Run.ID)
	if err != nil {
		t.Fatalf("ListLifecycleEventsByRun() error = %v", err)
	}
	if !containsLifecycleEvent(events, "run_terminal_observed") || !containsLifecycleEvent(events, "step_terminal_observed") {
		t.Fatalf("events = %#v, want terminal lifecycle events", events)
	}
}

// TestPostgresRuntimeGraphCheckpointStoreIntegration verifies the private Eino checkpoint store round-trips opaque payloads.
// TestPostgresRuntimeGraphCheckpointStoreIntegration 用于验证私有 Eino checkpoint store 可以往返保存 opaque payload。
func TestPostgresRuntimeGraphCheckpointStoreIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	checkpointID := "itest-runtime-checkpoint-" + time.Now().UTC().Format("20060102150405.000000000")
	defer func() {
		if err := db.WithContext(ctx).Where("checkpoint_id = ?", checkpointID).Delete(&postgresRuntimeGraphCheckpointModel{}).Error; err != nil {
			t.Fatalf("cleanup runtime_graph_checkpoints error = %v", err)
		}
	}()
	payload := []byte("eino checkpoint opaque bytes")
	if err := store.Set(ctx, checkpointID, payload); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	got, ok, err := store.Get(ctx, checkpointID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok || string(got) != string(payload) {
		t.Fatalf("Get() = %q, %v; want %q", string(got), ok, string(payload))
	}
	snapshot, ok, err := store.GetCheckpointSnapshot(ctx, checkpointID)
	if err != nil {
		t.Fatalf("GetCheckpointSnapshot() error = %v", err)
	}
	if !ok || snapshot.CheckpointID != checkpointID || snapshot.PayloadSize != len(payload) || snapshot.PayloadSHA256 == "" {
		t.Fatalf("GetCheckpointSnapshot() = %#v, %v; want safe metadata", snapshot, ok)
	}
	err = store.Set(ctx, checkpointID, []byte("Authorization: Bearer sk-itest"))
	if !errors.Is(err, ErrRuntimeCheckpointRejected) {
		t.Fatalf("credential-like Set() error = %v, want ErrRuntimeCheckpointRejected", err)
	}
}

// TestPostgresRuntimeStoreTransactionRollsBackInvalidWrite verifies grouped runtime writes do not leave partial rows.
// TestPostgresRuntimeStoreTransactionRollsBackInvalidWrite 用于验证成组 runtime 写入失败时不会留下半截记录。
func TestPostgresRuntimeStoreTransactionRollsBackInvalidWrite(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	runID := "itest-runtime-rollback-" + time.Now().UTC().Format("20060102150405.000000000")
	err = store.WithTransaction(ctx, func(txStore RuntimePersistenceStore) error {
		_, err := txStore.CreateTaskRun(ctx, TaskRun{
			ID:        runID,
			TaskID:    "task-rollback",
			Status:    TaskRunStatusRunning,
			CreatedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
			UpdatedAt: time.Date(2026, 4, 29, 12, 0, 0, 0, time.UTC),
		})
		if err != nil {
			return err
		}
		_, err = txStore.CreateTaskStep(ctx, TaskStep{
			RunID:    runID,
			Sequence: 1,
			Status:   "not-a-valid-step-status",
		})
		return err
	})
	if !errors.Is(err, ErrInvalidRuntimePersistenceInput) {
		t.Fatalf("WithTransaction() error = %v, want ErrInvalidRuntimePersistenceInput", err)
	}
	_, found, err := store.GetTaskRun(ctx, runID)
	if err != nil {
		t.Fatalf("GetTaskRun(after rollback) error = %v", err)
	}
	if found {
		t.Fatalf("transaction left partial task_run %q", runID)
	}
}

// TestPostgresRuntimeStoreRejectsInvalidInputs verifies validation failures are explicit before DB writes.
// TestPostgresRuntimeStoreRejectsInvalidInputs 用于验证非法输入会在 DB 写入前得到明确错误。
func TestPostgresRuntimeStoreRejectsInvalidInputs(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}

	invalidWrites := []struct {
		name string
		run  func() error
	}{
		{name: "task run missing task id", run: func() error {
			_, err := store.CreateTaskRun(ctx, TaskRun{Status: TaskRunStatusCreated})
			return err
		}},
		{name: "task step missing run id", run: func() error {
			_, err := store.CreateTaskStep(ctx, TaskStep{Sequence: 1, Status: TaskStepStatusPending})
			return err
		}},
		{name: "lifecycle invalid subject", run: func() error {
			_, err := store.CreateLifecycleEvent(ctx, TaskRunLifecycleEvent{RunID: "run", EventType: "x", SubjectType: "bad", SubjectID: "id", ToStatus: TaskRunStatusRunning})
			return err
		}},
		{name: "trace missing summary", run: func() error {
			_, err := store.CreateRuntimeTrace(ctx, RuntimeTrace{RunID: "run"})
			return err
		}},
		{name: "usage negative amount", run: func() error {
			_, err := store.CreateUsage(ctx, Usage{RunID: "run", ResourceType: "tool", Unit: "operation", Amount: -1})
			return err
		}},
		{name: "projection missing kind", run: func() error {
			_, err := store.CreateProjectionCandidate(ctx, ProjectionCandidate{RunID: "run"})
			return err
		}},
		{name: "projection credential-like semantic payload", run: func() error {
			_, err := store.CreateProjectionCandidate(ctx, ProjectionCandidate{RunID: "run", CandidateKind: "candidate", SemanticPayload: map[string]any{"api_key": "sk-raw"}})
			return err
		}},
		{name: "runtime contract missing task type", run: func() error {
			_, err := store.CreateRuntimeContract(ctx, RuntimeContract{Name: "contract", Version: "v1"})
			return err
		}},
		{name: "runtime contract invalid status", run: func() error {
			_, err := store.CreateRuntimeContract(ctx, RuntimeContract{Name: "contract", Version: "v1", TaskType: "runtime.validation", Status: "invalid"})
			return err
		}},
		{name: "runtime contract credential-like metadata", run: func() error {
			_, err := store.CreateRuntimeContract(ctx, RuntimeContract{Name: "contract", Version: "v1", TaskType: "runtime.validation", Metadata: map[string]any{"authorization": "Bearer raw"}})
			return err
		}},
		{name: "task type credential-like schema", run: func() error {
			_, err := store.CreateTaskTypeRegistration(ctx, TaskTypeRegistration{TypeKey: "runtime.invalid", InputSchema: map[string]any{"password": "raw"}})
			return err
		}},
		{name: "hook binding invalid hook point", run: func() error {
			_, err := store.CreateHookBinding(ctx, HookBinding{ContractID: "contract", HookPoint: "bad", BindingKind: HookBindingKindEinoMiddleware, BindingRef: "runtime_contract_guard"})
			return err
		}},
		{name: "system truth source credential-like payload", run: func() error {
			_, err := store.CreateSystemTruthSource(ctx, SystemTruthSource{AssetID: "asset", SourceKind: "repo", Content: map[string]any{"secret": "raw"}})
			return err
		}},
	}

	for _, tt := range invalidWrites {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(); !errors.Is(err, ErrInvalidRuntimePersistenceInput) {
				t.Fatalf("error = %v, want ErrInvalidRuntimePersistenceInput", err)
			}
		})
	}
}

func cleanupPostgresRuntimeContractArtifacts(t *testing.T, db *gorm.DB, contractID string) {
	t.Helper()
	if err := db.WithContext(context.Background()).Where("id = ?", contractID).Delete(&postgresRuntimeContractModel{}).Error; err != nil {
		t.Fatalf("cleanup runtime_contracts error = %v", err)
	}
}

func cleanupPostgresRuntimeContractFoundationArtifacts(t *testing.T, db *gorm.DB, contractID string, taskTypeKey string, assetID string) {
	t.Helper()
	ctx := context.Background()
	for _, cleanup := range []struct {
		name  string
		model any
		where string
		value string
	}{
		{name: "runtime_hook_bindings", model: &postgresHookBindingModel{}, where: "contract_id = ?", value: contractID},
		{name: "runtime_task_types", model: &postgresTaskTypeRegistrationModel{}, where: "type_key = ?", value: taskTypeKey},
		{name: "runtime_contracts", model: &postgresRuntimeContractModel{}, where: "id = ?", value: contractID},
		{name: "system_truth_active_versions", model: &postgresSystemTruthActiveVersionModel{}, where: "asset_id = ?", value: assetID},
		{name: "system_truth_compile_results", model: &postgresSystemTruthCompileResultModel{}, where: "asset_id = ?", value: assetID},
		{name: "system_truth_drafts", model: &postgresSystemTruthDraftModel{}, where: "asset_id = ?", value: assetID},
		{name: "system_truth_sources", model: &postgresSystemTruthSourceModel{}, where: "asset_id = ?", value: assetID},
	} {
		if err := db.WithContext(ctx).Where(cleanup.where, cleanup.value).Delete(cleanup.model).Error; err != nil {
			t.Fatalf("cleanup %s error = %v", cleanup.name, err)
		}
	}
}

func cleanupPostgresRuntimeArtifacts(t *testing.T, db *gorm.DB, runID string) {
	t.Helper()
	ctx := context.Background()
	for _, cleanup := range []struct {
		name  string
		model any
		where string
	}{
		{name: "runtime_projections", model: &postgresProjectionCandidateModel{}, where: "run_id = ?"},
		{name: "runtime_usage", model: &postgresUsageModel{}, where: "run_id = ?"},
		{name: "runtime_traces", model: &postgresRuntimeTraceModel{}, where: "run_id = ?"},
		{name: "task_run_lifecycle_events", model: &postgresLifecycleEventModel{}, where: "run_id = ?"},
		{name: "task_steps", model: &postgresTaskStepModel{}, where: "run_id = ?"},
		{name: "task_runs", model: &postgresTaskRunModel{}, where: "id = ?"},
	} {
		if err := db.WithContext(ctx).Where(cleanup.where, runID).Delete(cleanup.model).Error; err != nil {
			t.Fatalf("cleanup %s error = %v", cleanup.name, err)
		}
	}
}

func containsRun(runs []TaskRun, id string) bool {
	for _, run := range runs {
		if run.ID == id {
			return true
		}
	}
	return false
}

func containsRuntimeContract(contracts []RuntimeContract, id string) bool {
	for _, contract := range contracts {
		if contract.ID == id {
			return true
		}
	}
	return false
}

func containsTaskType(taskTypes []TaskTypeRegistration, typeKey string) bool {
	for _, item := range taskTypes {
		if item.TypeKey == typeKey {
			return true
		}
	}
	return false
}

func containsTraceType(traces []RuntimeTrace, traceType string) bool {
	for _, trace := range traces {
		if trace.TraceType == traceType {
			return true
		}
	}
	return false
}
