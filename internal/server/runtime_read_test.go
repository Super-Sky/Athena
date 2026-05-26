package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/runtime"
)

// TestControlPlaneRuntimeReadEndpointsReturnPersistedRecords verifies Batch 2 read APIs expose Phase 1 runtime truth.
// TestControlPlaneRuntimeReadEndpointsReturnPersistedRecords 验证 Batch 2 读取 API 会暴露 Phase 1 runtime 真相。
func TestControlPlaneRuntimeReadEndpointsReturnPersistedRecords(t *testing.T) {
	now := time.Date(2026, 5, 7, 10, 0, 0, 0, time.UTC)
	cost := 0.012
	store := &testRuntimeReadStore{
		runs: []runtime.TaskRun{{
			ID:              "run-read-1",
			TaskID:          "task-read-1",
			TaskType:        "runtime_validation",
			WorkspaceID:     "ws-read",
			Status:          runtime.TaskRunStatusCompleted,
			RetentionPolicy: "default",
			Metadata: map[string]any{
				"graph": "eino",
				"checkpoint_ref": map[string]any{
					"checkpoint_id":        "checkpoint-read-1",
					"stage":                string(runtime.StageTurnExecution),
					"resume_token_present": true,
					"resume_token":         "secret-resume-token",
				},
			},
			CreatedAt: now,
			UpdatedAt: now,
		}},
		snapshots: map[string]runtime.RuntimeGraphCheckpointSnapshot{
			"checkpoint-read-1": {
				CheckpointID:  "checkpoint-read-1",
				PayloadSize:   29,
				PayloadSHA256: "93f86c65b3442f5fd41e8ce8ce42e9ba151c3f3f4e8841879f566f07ded57bf9",
				CreatedAt:     now,
				UpdatedAt:     now,
			},
		},
		steps: []runtime.TaskStep{{
			ID:        "step-read-1",
			RunID:     "run-read-1",
			Sequence:  1,
			StepType:  "graph_node",
			Name:      "model_execution",
			Status:    runtime.TaskStepStatusSuccess,
			Metadata:  map[string]any{"node": "model"},
			CreatedAt: now,
			UpdatedAt: now,
		}},
		events: []runtime.TaskRunLifecycleEvent{{
			ID:          "event-read-1",
			RunID:       "run-read-1",
			StepID:      "step-read-1",
			EventType:   "step_status_changed",
			SubjectType: runtime.LifecycleSubjectStep,
			SubjectID:   "step-read-1",
			FromStatus:  runtime.TaskStepStatusRunning,
			ToStatus:    runtime.TaskStepStatusSuccess,
			Reason:      "node_completed",
			OccurredAt:  now,
		}},
		traces: []runtime.RuntimeTrace{{
			ID:              "trace-read-1",
			RunID:           "run-read-1",
			StepID:          "step-read-1",
			TraceType:       "model_callback",
			Summary:         "model callback summary",
			SafeLabels:      map[string]string{"component": "model"},
			RedactedPayload: map[string]any{"output_summary": "ok"},
			CreatedAt:       now,
		}},
		usages: []runtime.Usage{{
			ID:           "usage-read-1",
			RunID:        "run-read-1",
			StepID:       "step-read-1",
			ResourceType: "model_tokens",
			Provider:     "test-provider",
			ResourceName: "test-model",
			Unit:         "token",
			Amount:       42,
			Cost:         &cost,
			Currency:     "USD",
			CreatedAt:    now,
		}},
		projections: []runtime.ProjectionCandidate{{
			ID:                    "projection-read-1",
			RunID:                 "run-read-1",
			StepID:                "step-read-1",
			CandidateKind:         "assistant_message",
			Status:                "completed",
			Summary:               "candidate summary",
			SchemaVersion:         "runtime_projection.v1",
			RedactedPayload:       map[string]any{"text_summary": "hello"},
			SemanticPayload:       map[string]any{"kind": "assistant_message"},
			ArtifactRefs:          map[string]any{"primary": "artifact://runtime/read"},
			UIHints:               map[string]any{"surface": "system_validation"},
			MaterializationTarget: map[string]any{"target": "control_plane"},
			CreatedAt:             now,
		}},
		contracts: []runtime.RuntimeContract{{
			ID:        "contract-read-1",
			Name:      "Runtime Read Contract",
			Version:   "v1",
			Status:    runtime.RuntimeContractStatusActive,
			TaskType:  "runtime_validation",
			CreatedAt: now,
			UpdatedAt: now,
		}},
		taskTypes: []runtime.TaskTypeRegistration{{
			ID:                "task-type-read-1",
			TypeKey:           "runtime_validation",
			DisplayName:       "Runtime Validation",
			Status:            runtime.TaskTypeStatusActive,
			DefaultContractID: "contract-read-1",
			CreatedAt:         now,
			UpdatedAt:         now,
		}},
		hooks: []runtime.HookBinding{{
			ID:            "hook-read-1",
			ContractID:    "contract-read-1",
			HookPoint:     runtime.HookPointBeforeRun,
			BindingKind:   runtime.HookBindingKindEinoMiddleware,
			BindingRef:    "runtime_contract_guard",
			Enabled:       true,
			FailurePolicy: runtime.HookFailurePolicyFailClosed,
			CreatedAt:     now,
			UpdatedAt:     now,
		}},
		activeTruths: []runtime.SystemTruthActiveVersion{{
			ID:              "active-truth-read-1",
			AssetID:         "tool_governance_policy",
			CompileResultID: "compile-read-1",
			DraftID:         "draft-read-1",
			ActivatedBy:     "test",
			Reason:          "readout",
			ActivatedAt:     now,
		}},
	}
	httpServer := newRuntimeReadHTTPServer(t, store, "")

	cases := []struct {
		path string
		want string
	}{
		{path: "/api/control-plane/runtime/runs?workspace_id=ws-read", want: `"id":"run-read-1"`},
		{path: "/api/control-plane/runtime/runs/run-read-1", want: `"task_id":"task-read-1"`},
		{path: "/api/control-plane/runtime/runs/run-read-1/steps", want: `"name":"model_execution"`},
		{path: "/api/control-plane/runtime/runs/run-read-1/lifecycle", want: `"event_type":"step_status_changed"`},
		{path: "/api/control-plane/runtime/runs/run-read-1/traces?step_id=step-read-1", want: `"summary":"model callback summary"`},
		{path: "/api/control-plane/runtime/runs/run-read-1/usage?step_id=step-read-1", want: `"resource_type":"model_tokens"`},
		{path: "/api/control-plane/runtime/runs/run-read-1/projections?step_id=step-read-1", want: `"schema_version":"runtime_projection.v1"`},
		{path: "/api/control-plane/runtime/runs/run-read-1/checkpoints", want: `"payload_sha256":"93f86c65b3442f5fd41e8ce8ce42e9ba151c3f3f4e8841879f566f07ded57bf9"`},
		{path: "/api/control-plane/runtime/contracts/foundation", want: `"binding_ref":"runtime_contract_guard"`},
	}
	for _, tt := range cases {
		t.Run(tt.path, func(t *testing.T) {
			resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, tt.path, nil)
			if resp.Code != consts.StatusOK {
				t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
			}
			if !strings.Contains(resp.Body.String(), tt.want) {
				t.Fatalf("body = %s, want %s", resp.Body.String(), tt.want)
			}
			if strings.Contains(tt.path, "/checkpoints") {
				body := resp.Body.String()
				if strings.Contains(body, `"payload":`) || strings.Contains(body, "secret-resume-token") {
					t.Fatalf("checkpoint body leaks private payload or resume token: %s", body)
				}
			}
		})
	}
}

func TestControlPlaneRuntimeFoundationWriteEndpointsUpdateRecords(t *testing.T) {
	now := time.Date(2026, 5, 19, 10, 0, 0, 0, time.UTC)
	store := &testRuntimeReadStore{
		contracts: []runtime.RuntimeContract{{
			ID:        "contract-edit-1",
			Name:      "Contract Before",
			Version:   "v1",
			Status:    runtime.RuntimeContractStatusDraft,
			TaskType:  "runtime_validation",
			CreatedAt: now,
			UpdatedAt: now,
		}},
		taskTypes: []runtime.TaskTypeRegistration{{
			ID:                "task-type-edit-1",
			TypeKey:           "runtime_validation",
			DisplayName:       "Runtime Validation Before",
			Status:            runtime.TaskTypeStatusDraft,
			DefaultContractID: "contract-edit-1",
			CreatedAt:         now,
			UpdatedAt:         now,
		}},
		hooks: []runtime.HookBinding{{
			ID:            "hook-edit-1",
			ContractID:    "contract-edit-1",
			HookPoint:     runtime.HookPointBeforeRun,
			BindingKind:   runtime.HookBindingKindEinoMiddleware,
			BindingRef:    "runtime_contract_guard",
			Enabled:       true,
			FailurePolicy: runtime.HookFailurePolicyFailClosed,
			CreatedAt:     now,
			UpdatedAt:     now,
		}},
	}
	httpServer := newRuntimeReadHTTPServer(t, store, "")

	contractBody := bytes.NewBufferString(`{"name":"Contract After","version":"v2","status":"active","task_type":"runtime_validation","metadata":{"editor":"system_validation"}}`)
	contractResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/contracts/contract-edit-1", &ut.Body{Body: contractBody, Len: contractBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if contractResp.Code != consts.StatusOK {
		t.Fatalf("contract status = %d, want %d; body=%s", contractResp.Code, consts.StatusOK, contractResp.Body.String())
	}
	if !strings.Contains(contractResp.Body.String(), `"name":"Contract After"`) {
		t.Fatalf("contract body = %s", contractResp.Body.String())
	}

	taskTypeBody := bytes.NewBufferString(`{"display_name":"Runtime Validation After","status":"active","default_contract_id":"contract-edit-1","metadata":{"editor":"system_validation"}}`)
	taskTypeResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/task-types/runtime_validation", &ut.Body{Body: taskTypeBody, Len: taskTypeBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if taskTypeResp.Code != consts.StatusOK {
		t.Fatalf("task type status = %d, want %d; body=%s", taskTypeResp.Code, consts.StatusOK, taskTypeResp.Body.String())
	}
	if !strings.Contains(taskTypeResp.Body.String(), `"display_name":"Runtime Validation After"`) {
		t.Fatalf("task type body = %s", taskTypeResp.Body.String())
	}

	hookBody := bytes.NewBufferString(`{"contract_id":"contract-edit-1","hook_point":"before_run","binding_kind":"eino_middleware","binding_ref":"system_truth_guard","enabled":true,"failure_policy":"record_only","metadata":{"editor":"system_validation"}}`)
	hookResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/hook-bindings/hook-edit-1", &ut.Body{Body: hookBody, Len: hookBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if hookResp.Code != consts.StatusOK {
		t.Fatalf("hook status = %d, want %d; body=%s", hookResp.Code, consts.StatusOK, hookResp.Body.String())
	}
	if !strings.Contains(hookResp.Body.String(), `"binding_ref":"system_truth_guard"`) {
		t.Fatalf("hook body = %s", hookResp.Body.String())
	}

	foundationResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/contracts/foundation", nil)
	if foundationResp.Code != consts.StatusOK {
		t.Fatalf("foundation status = %d, want %d; body=%s", foundationResp.Code, consts.StatusOK, foundationResp.Body.String())
	}
	body := foundationResp.Body.String()
	for _, want := range []string{`"Contract After"`, `"Runtime Validation After"`, `"system_truth_guard"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("foundation body = %s, want %s", body, want)
		}
	}
}

func TestControlPlaneRuntimeHookBindingWriteRejectsNonAllowlistedBinding(t *testing.T) {
	httpServer := newRuntimeReadHTTPServer(t, &testRuntimeReadStore{}, "")
	body := bytes.NewBufferString(`{"contract_id":"contract-edit-1","hook_point":"before_run","binding_kind":"eino_middleware","binding_ref":"not_allowlisted","enabled":true,"failure_policy":"fail_closed"}`)

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/hook-bindings/hook-edit-1", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusBadRequest, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "not allowlisted") {
		t.Fatalf("body = %s, want allowlist error", resp.Body.String())
	}
}

// TestControlPlaneRuntimeReadEndpointMissingRunReturnsNotFound verifies detail reads fail clearly.
// TestControlPlaneRuntimeReadEndpointMissingRunReturnsNotFound 验证明细读取缺失 run 时会明确返回 not found。
func TestControlPlaneRuntimeReadEndpointMissingRunReturnsNotFound(t *testing.T) {
	httpServer := newRuntimeReadHTTPServer(t, &testRuntimeReadStore{}, "")

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/runs/missing-run", nil)
	if resp.Code != consts.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusNotFound, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "runtime run not found") {
		t.Fatalf("body = %s, want not found error", resp.Body.String())
	}
}

// TestControlPlaneRuntimeReadEndpointRequiresAuth verifies read APIs use the existing Control Plane auth wrapper.
// TestControlPlaneRuntimeReadEndpointRequiresAuth 验证 runtime read API 使用现有 Control Plane 认证包装。
func TestControlPlaneRuntimeReadEndpointRequiresAuth(t *testing.T) {
	httpServer := newRuntimeReadHTTPServer(t, &testRuntimeReadStore{}, "runtime-read-token")

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/runs", nil)
	if resp.Code != consts.StatusUnauthorized {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusUnauthorized, resp.Body.String())
	}
}

// TestControlPlaneRuntimeValidationRunEndpointPersistsGraphRecordSet verifies the trigger writes through Eino graph persistence.
// TestControlPlaneRuntimeValidationRunEndpointPersistsGraphRecordSet 验证触发入口会通过 Eino graph persistence 写入记录集。
func TestControlPlaneRuntimeValidationRunEndpointPersistsGraphRecordSet(t *testing.T) {
	store := &testRuntimeReadStore{}
	httpServer := newRuntimeReadHTTPServer(t, store, "")
	body := bytes.NewBufferString(`{"workspace_id":"ws-validation","scene":"system_validation","prompt":"run runtime validation"}`)

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/control-plane/runtime/validation-runs", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusCreated, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"task_type":"runtime_validation"`) {
		t.Fatalf("body = %s, want runtime_validation task type", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"graph_writer":"eino_runtime_graph"`) {
		t.Fatalf("body = %s, want Eino graph writer metadata", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"validation_mcp"`) || !strings.Contains(resp.Body.String(), `"external_sandbox_ref"`) {
		t.Fatalf("body = %s, want deterministic MCP and sandbox validation output", resp.Body.String())
	}
	if len(store.runs) != 1 || len(store.steps) != 1 || len(store.events) < 4 || len(store.traces) < 3 || len(store.usages) < 3 || len(store.projections) < 3 {
		t.Fatalf("persisted counts runs=%d steps=%d events=%d traces=%d usage=%d projections=%d", len(store.runs), len(store.steps), len(store.events), len(store.traces), len(store.usages), len(store.projections))
	}

	listResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/runs?workspace_id=ws-validation", nil)
	if listResp.Code != consts.StatusOK {
		t.Fatalf("list status = %d, want %d; body=%s", listResp.Code, consts.StatusOK, listResp.Body.String())
	}
	if !strings.Contains(listResp.Body.String(), store.runs[0].ID) {
		t.Fatalf("list body = %s, want run id %s", listResp.Body.String(), store.runs[0].ID)
	}
}

// TestControlPlaneRuntimeValidationRunEndpointRequiresStore verifies missing persistence is explicit.
// TestControlPlaneRuntimeValidationRunEndpointRequiresStore 验证缺少持久化 store 时返回明确错误。
func TestControlPlaneRuntimeValidationRunEndpointRequiresStore(t *testing.T) {
	httpServer := newRuntimeReadHTTPServer(t, nil, "")
	body := bytes.NewBufferString(`{}`)

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/control-plane/runtime/validation-runs", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusServiceUnavailable, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), "runtime persistence store is not configured") {
		t.Fatalf("body = %s, want store not configured error", resp.Body.String())
	}
}

// TestControlPlaneRuntimeReadEndpointPostgresIntegration verifies read APIs against the real PostgreSQL runtime store.
// TestControlPlaneRuntimeReadEndpointPostgresIntegration 验证 runtime read API 可以读取真实 PostgreSQL runtime store。
func TestControlPlaneRuntimeReadEndpointPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres runtime read API integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := runtime.NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	now := time.Date(2026, 5, 8, 10, 0, 0, 0, time.UTC)
	runID := "itest-runtime-read-api-" + time.Now().UTC().Format("20060102150405.000000000")
	run, err := store.CreateTaskRun(ctx, runtime.TaskRun{
		ID:          runID,
		TaskID:      "task-runtime-read-api",
		TaskType:    "runtime_validation",
		InputKind:   "chat",
		Status:      runtime.TaskRunStatusRunning,
		WorkspaceID: "itest-runtime-read",
		Metadata:    map[string]any{"source": "server_runtime_read_test"},
		CreatedAt:   now,
		UpdatedAt:   now,
		StartedAt:   &now,
	})
	if err != nil {
		t.Fatalf("CreateTaskRun() error = %v", err)
	}
	defer cleanupPostgresRuntimeReadAPIArtifacts(t, db, run.ID)
	step, err := store.CreateTaskStep(ctx, runtime.TaskStep{
		RunID:     run.ID,
		Sequence:  1,
		StepType:  "graph_node",
		Name:      "runtime_read_api_node",
		Status:    runtime.TaskStepStatusSuccess,
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("CreateTaskStep() error = %v", err)
	}
	if _, err := store.CreateRuntimeTrace(ctx, runtime.RuntimeTrace{
		RunID:           run.ID,
		StepID:          step.ID,
		TraceType:       "server_read_api",
		Summary:         "postgres runtime read api trace",
		SafeLabels:      map[string]string{"component": "server"},
		RedactedPayload: map[string]any{"summary": "ok"},
		CreatedAt:       now,
	}); err != nil {
		t.Fatalf("CreateRuntimeTrace() error = %v", err)
	}

	httpServer := newRuntimeReadHTTPServer(t, store, "")
	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/runs/"+run.ID+"/traces", nil)
	if resp.Code != consts.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"summary":"postgres runtime read api trace"`) {
		t.Fatalf("body = %s, want postgres trace summary", resp.Body.String())
	}
}

// TestControlPlaneRuntimeFoundationWriteEndpointPostgresIntegration verifies foundation write endpoints against PostgreSQL.
// TestControlPlaneRuntimeFoundationWriteEndpointPostgresIntegration 验证 foundation 写接口可在 PostgreSQL 上完成写入并可读回。
func TestControlPlaneRuntimeFoundationWriteEndpointPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres foundation write API integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := runtime.NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	suffix := time.Now().UTC().Format("20060102150405.000000000")
	contractID := "itest-runtime-contract-write-" + suffix
	taskTypeKey := "runtime_validation_write_" + strings.ReplaceAll(suffix, ".", "")
	hookID := "itest-runtime-hook-write-" + suffix
	defer cleanupPostgresRuntimeFoundationWriteArtifacts(t, db, contractID, taskTypeKey, hookID)

	httpServer := newRuntimeReadHTTPServer(t, store, "")

	contractBody := bytes.NewBufferString(`{"name":"Integration Runtime Contract","version":"v1","status":"active","task_type":"` + taskTypeKey + `","metadata":{"integration":"postgres_write"}}`)
	contractResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/contracts/"+contractID, &ut.Body{Body: contractBody, Len: contractBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if contractResp.Code != consts.StatusOK {
		t.Fatalf("contract status = %d, want %d; body=%s", contractResp.Code, consts.StatusOK, contractResp.Body.String())
	}

	taskTypeBody := bytes.NewBufferString(`{"display_name":"Integration Runtime Task Type","status":"active","default_contract_id":"` + contractID + `","metadata":{"integration":"postgres_write"}}`)
	taskTypeResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/task-types/"+taskTypeKey, &ut.Body{Body: taskTypeBody, Len: taskTypeBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if taskTypeResp.Code != consts.StatusOK {
		t.Fatalf("task type status = %d, want %d; body=%s", taskTypeResp.Code, consts.StatusOK, taskTypeResp.Body.String())
	}

	hookBody := bytes.NewBufferString(`{"contract_id":"` + contractID + `","hook_point":"before_run","binding_kind":"policy_ref","binding_ref":"system_truth_guard","enabled":true,"failure_policy":"record_only","metadata":{"integration":"postgres_write"}}`)
	hookResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPut, "/api/control-plane/runtime/hook-bindings/"+hookID, &ut.Body{Body: hookBody, Len: hookBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if hookResp.Code != consts.StatusOK {
		t.Fatalf("hook status = %d, want %d; body=%s", hookResp.Code, consts.StatusOK, hookResp.Body.String())
	}

	foundationResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/contracts/foundation", nil)
	if foundationResp.Code != consts.StatusOK {
		t.Fatalf("foundation status = %d, want %d; body=%s", foundationResp.Code, consts.StatusOK, foundationResp.Body.String())
	}
	body := foundationResp.Body.String()
	for _, want := range []string{contractID, taskTypeKey, hookID} {
		if !strings.Contains(body, want) {
			t.Fatalf("foundation body missing %s: %s", want, body)
		}
	}
}

// TestControlPlaneRuntimeValidationRunEndpointPostgresIntegration verifies the trigger writes to PostgreSQL.
// TestControlPlaneRuntimeValidationRunEndpointPostgresIntegration 验证触发入口会写入 PostgreSQL。
func TestControlPlaneRuntimeValidationRunEndpointPostgresIntegration(t *testing.T) {
	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres runtime validation trigger integration test: ATHENA_PG_TEST_DSN is not set")
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("gorm.Open() error = %v", err)
	}
	store := runtime.NewPostgresRuntimeStore(db)
	ctx := context.Background()
	if err := store.AutoMigrate(ctx); err != nil {
		t.Fatalf("AutoMigrate() error = %v", err)
	}
	httpServer := newRuntimeReadHTTPServer(t, store, "")
	body := bytes.NewBufferString(`{"workspace_id":"itest-runtime-validation","scene":"system_validation","prompt":"postgres validation"}`)

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/control-plane/runtime/validation-runs", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusCreated {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusCreated, resp.Body.String())
	}
	var payload runtimeValidationRunResponse
	if err := json.Unmarshal(resp.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response error = %v; body=%s", err, resp.Body.String())
	}
	defer cleanupPostgresRuntimeReadAPIArtifacts(t, db, payload.Run.ID)

	tracesResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/runs/"+payload.Run.ID+"/traces", nil)
	if tracesResp.Code != consts.StatusOK {
		t.Fatalf("traces status = %d, want %d; body=%s", tracesResp.Code, consts.StatusOK, tracesResp.Body.String())
	}
	if !strings.Contains(tracesResp.Body.String(), `"writer_summary"`) {
		t.Fatalf("traces body = %s, want writer_summary trace", tracesResp.Body.String())
	}
	if !strings.Contains(tracesResp.Body.String(), `"runtime_hook_binding"`) {
		t.Fatalf("traces body = %s, want runtime_hook_binding trace", tracesResp.Body.String())
	}
	foundationResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/contracts/foundation", nil)
	if foundationResp.Code != consts.StatusOK {
		t.Fatalf("foundation status = %d, want %d; body=%s", foundationResp.Code, consts.StatusOK, foundationResp.Body.String())
	}
	for _, want := range []string{`"task_type":"runtime_validation"`, `"binding_ref":"runtime_contract_guard"`, `"asset_id":"persona.default"`} {
		if !strings.Contains(foundationResp.Body.String(), want) {
			t.Fatalf("foundation body = %s, want %s", foundationResp.Body.String(), want)
		}
	}
}

func newRuntimeReadHTTPServer(t *testing.T, store runtime.RuntimePersistenceStore, authToken string) *HTTPServer {
	t.Helper()
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
			AuthToken: authToken,
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}
	application := appcore.NewServiceWithRuntimeStore(cfg, nil, nil, nil, store)
	return NewHTTPServer(cfg, application)
}

func cleanupPostgresRuntimeReadAPIArtifacts(t *testing.T, db *gorm.DB, runID string) {
	t.Helper()
	ctx := context.Background()
	tables := []string{
		"runtime_projections",
		"runtime_usage",
		"runtime_traces",
		"task_run_lifecycle_events",
		"task_steps",
	}
	for _, table := range tables {
		if err := db.WithContext(ctx).Exec("DELETE FROM "+table+" WHERE run_id = ?", runID).Error; err != nil {
			t.Fatalf("cleanup %s failed: %v", table, err)
		}
	}
	if err := db.WithContext(ctx).Exec("DELETE FROM task_runs WHERE id = ?", runID).Error; err != nil {
		t.Fatalf("cleanup task_runs failed: %v", err)
	}
}

func cleanupPostgresRuntimeFoundationWriteArtifacts(t *testing.T, db *gorm.DB, contractID string, taskTypeKey string, hookID string) {
	t.Helper()
	ctx := context.Background()
	if err := db.WithContext(ctx).Exec("DELETE FROM runtime_hook_bindings WHERE id = ?", hookID).Error; err != nil {
		t.Fatalf("cleanup runtime_hook_bindings failed: %v", err)
	}
	if err := db.WithContext(ctx).Exec("DELETE FROM runtime_task_types WHERE type_key = ?", taskTypeKey).Error; err != nil {
		t.Fatalf("cleanup runtime_task_types failed: %v", err)
	}
	if err := db.WithContext(ctx).Exec("DELETE FROM runtime_contracts WHERE id = ?", contractID).Error; err != nil {
		t.Fatalf("cleanup runtime_contracts failed: %v", err)
	}
}

type testRuntimeReadStore struct {
	runs         []runtime.TaskRun
	steps        []runtime.TaskStep
	events       []runtime.TaskRunLifecycleEvent
	traces       []runtime.RuntimeTrace
	usages       []runtime.Usage
	projections  []runtime.ProjectionCandidate
	snapshots    map[string]runtime.RuntimeGraphCheckpointSnapshot
	contracts    []runtime.RuntimeContract
	taskTypes    []runtime.TaskTypeRegistration
	hooks        []runtime.HookBinding
	activeTruths []runtime.SystemTruthActiveVersion
}

func (s *testRuntimeReadStore) AutoMigrate(context.Context) error { return nil }

func (s *testRuntimeReadStore) CreateTaskRun(_ context.Context, run runtime.TaskRun) (runtime.TaskRun, error) {
	s.runs = append(s.runs, run)
	return run, nil
}

func (s *testRuntimeReadStore) GetTaskRun(_ context.Context, id string) (runtime.TaskRun, bool, error) {
	for _, run := range s.runs {
		if run.ID == strings.TrimSpace(id) {
			return run, true, nil
		}
	}
	return runtime.TaskRun{}, false, nil
}

func (s *testRuntimeReadStore) ListTaskRuns(_ context.Context, filter runtime.TaskRunListFilter) ([]runtime.TaskRun, error) {
	var out []runtime.TaskRun
	for _, run := range s.runs {
		if strings.TrimSpace(filter.WorkspaceID) != "" && run.WorkspaceID != strings.TrimSpace(filter.WorkspaceID) {
			continue
		}
		if strings.TrimSpace(filter.Status) != "" && run.Status != strings.TrimSpace(filter.Status) {
			continue
		}
		out = append(out, run)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateTaskStep(_ context.Context, step runtime.TaskStep) (runtime.TaskStep, error) {
	s.steps = append(s.steps, step)
	return step, nil
}

func (s *testRuntimeReadStore) GetTaskStep(_ context.Context, id string) (runtime.TaskStep, bool, error) {
	for _, step := range s.steps {
		if step.ID == strings.TrimSpace(id) {
			return step, true, nil
		}
	}
	return runtime.TaskStep{}, false, nil
}

func (s *testRuntimeReadStore) ListTaskSteps(_ context.Context, runID string) ([]runtime.TaskStep, error) {
	var out []runtime.TaskStep
	for _, step := range s.steps {
		if step.RunID == strings.TrimSpace(runID) {
			out = append(out, step)
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateLifecycleEvent(_ context.Context, event runtime.TaskRunLifecycleEvent) (runtime.TaskRunLifecycleEvent, error) {
	s.events = append(s.events, event)
	return event, nil
}

func (s *testRuntimeReadStore) GetLifecycleEvent(_ context.Context, id string) (runtime.TaskRunLifecycleEvent, bool, error) {
	for _, event := range s.events {
		if event.ID == strings.TrimSpace(id) {
			return event, true, nil
		}
	}
	return runtime.TaskRunLifecycleEvent{}, false, nil
}

func (s *testRuntimeReadStore) ListLifecycleEventsByRun(_ context.Context, runID string) ([]runtime.TaskRunLifecycleEvent, error) {
	var out []runtime.TaskRunLifecycleEvent
	for _, event := range s.events {
		if event.RunID == strings.TrimSpace(runID) {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) ListLifecycleEventsBySubject(_ context.Context, subjectType string, subjectID string) ([]runtime.TaskRunLifecycleEvent, error) {
	var out []runtime.TaskRunLifecycleEvent
	for _, event := range s.events {
		if event.SubjectType == strings.TrimSpace(subjectType) && event.SubjectID == strings.TrimSpace(subjectID) {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateRuntimeTrace(_ context.Context, trace runtime.RuntimeTrace) (runtime.RuntimeTrace, error) {
	s.traces = append(s.traces, trace)
	return trace, nil
}

func (s *testRuntimeReadStore) GetRuntimeTrace(_ context.Context, id string) (runtime.RuntimeTrace, bool, error) {
	for _, trace := range s.traces {
		if trace.ID == strings.TrimSpace(id) {
			return trace, true, nil
		}
	}
	return runtime.RuntimeTrace{}, false, nil
}

func (s *testRuntimeReadStore) ListRuntimeTraces(_ context.Context, filter runtime.RuntimeTraceListFilter) ([]runtime.RuntimeTrace, error) {
	var out []runtime.RuntimeTrace
	for _, trace := range s.traces {
		if trace.RunID != strings.TrimSpace(filter.RunID) {
			continue
		}
		if strings.TrimSpace(filter.StepID) != "" && trace.StepID != strings.TrimSpace(filter.StepID) {
			continue
		}
		out = append(out, trace)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateUsage(_ context.Context, usage runtime.Usage) (runtime.Usage, error) {
	s.usages = append(s.usages, usage)
	return usage, nil
}

func (s *testRuntimeReadStore) GetUsage(_ context.Context, id string) (runtime.Usage, bool, error) {
	for _, usage := range s.usages {
		if usage.ID == strings.TrimSpace(id) {
			return usage, true, nil
		}
	}
	return runtime.Usage{}, false, nil
}

func (s *testRuntimeReadStore) ListUsage(_ context.Context, filter runtime.UsageListFilter) ([]runtime.Usage, error) {
	var out []runtime.Usage
	for _, usage := range s.usages {
		if usage.RunID != strings.TrimSpace(filter.RunID) {
			continue
		}
		if strings.TrimSpace(filter.StepID) != "" && usage.StepID != strings.TrimSpace(filter.StepID) {
			continue
		}
		out = append(out, usage)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateProjectionCandidate(_ context.Context, projection runtime.ProjectionCandidate) (runtime.ProjectionCandidate, error) {
	s.projections = append(s.projections, projection)
	return projection, nil
}

func (s *testRuntimeReadStore) GetProjectionCandidate(_ context.Context, id string) (runtime.ProjectionCandidate, bool, error) {
	for _, projection := range s.projections {
		if projection.ID == strings.TrimSpace(id) {
			return projection, true, nil
		}
	}
	return runtime.ProjectionCandidate{}, false, nil
}

func (s *testRuntimeReadStore) ListProjectionCandidates(_ context.Context, filter runtime.ProjectionCandidateListFilter) ([]runtime.ProjectionCandidate, error) {
	var out []runtime.ProjectionCandidate
	for _, projection := range s.projections {
		if projection.RunID != strings.TrimSpace(filter.RunID) {
			continue
		}
		if strings.TrimSpace(filter.StepID) != "" && projection.StepID != strings.TrimSpace(filter.StepID) {
			continue
		}
		out = append(out, projection)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) GetCheckpointSnapshot(_ context.Context, checkpointID string) (runtime.RuntimeGraphCheckpointSnapshot, bool, error) {
	if s.snapshots == nil {
		return runtime.RuntimeGraphCheckpointSnapshot{}, false, nil
	}
	snapshot, ok := s.snapshots[strings.TrimSpace(checkpointID)]
	return snapshot, ok, nil
}

func (s *testRuntimeReadStore) CreateRuntimeContract(_ context.Context, contract runtime.RuntimeContract) (runtime.RuntimeContract, error) {
	s.contracts = append(s.contracts, contract)
	return contract, nil
}

func (s *testRuntimeReadStore) PutRuntimeContract(_ context.Context, contract runtime.RuntimeContract) (runtime.RuntimeContract, error) {
	for idx := range s.contracts {
		if s.contracts[idx].ID == strings.TrimSpace(contract.ID) {
			s.contracts[idx] = contract
			return contract, nil
		}
	}
	s.contracts = append(s.contracts, contract)
	return contract, nil
}

func (s *testRuntimeReadStore) GetRuntimeContract(_ context.Context, id string) (runtime.RuntimeContract, bool, error) {
	for _, contract := range s.contracts {
		if contract.ID == strings.TrimSpace(id) {
			return contract, true, nil
		}
	}
	return runtime.RuntimeContract{}, false, nil
}

func (s *testRuntimeReadStore) ListRuntimeContracts(_ context.Context, filter runtime.RuntimeContractListFilter) ([]runtime.RuntimeContract, error) {
	var out []runtime.RuntimeContract
	for _, contract := range s.contracts {
		if strings.TrimSpace(filter.Status) != "" && contract.Status != strings.TrimSpace(filter.Status) {
			continue
		}
		if strings.TrimSpace(filter.TaskType) != "" && contract.TaskType != strings.TrimSpace(filter.TaskType) {
			continue
		}
		out = append(out, contract)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateTaskTypeRegistration(_ context.Context, item runtime.TaskTypeRegistration) (runtime.TaskTypeRegistration, error) {
	s.taskTypes = append(s.taskTypes, item)
	return item, nil
}

func (s *testRuntimeReadStore) PutTaskTypeRegistration(_ context.Context, item runtime.TaskTypeRegistration) (runtime.TaskTypeRegistration, error) {
	for idx := range s.taskTypes {
		if s.taskTypes[idx].ID == strings.TrimSpace(item.ID) || s.taskTypes[idx].TypeKey == strings.TrimSpace(item.TypeKey) {
			s.taskTypes[idx] = item
			return item, nil
		}
	}
	s.taskTypes = append(s.taskTypes, item)
	return item, nil
}

func (s *testRuntimeReadStore) GetTaskTypeRegistration(_ context.Context, id string) (runtime.TaskTypeRegistration, bool, error) {
	for _, item := range s.taskTypes {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return runtime.TaskTypeRegistration{}, false, nil
}

func (s *testRuntimeReadStore) GetTaskTypeRegistrationByKey(_ context.Context, key string) (runtime.TaskTypeRegistration, bool, error) {
	for _, item := range s.taskTypes {
		if item.TypeKey == strings.TrimSpace(key) {
			return item, true, nil
		}
	}
	return runtime.TaskTypeRegistration{}, false, nil
}

func (s *testRuntimeReadStore) ListTaskTypeRegistrations(_ context.Context, filter runtime.TaskTypeRegistrationListFilter) ([]runtime.TaskTypeRegistration, error) {
	var out []runtime.TaskTypeRegistration
	for _, item := range s.taskTypes {
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

func (s *testRuntimeReadStore) CreateHookBinding(_ context.Context, item runtime.HookBinding) (runtime.HookBinding, error) {
	s.hooks = append(s.hooks, item)
	return item, nil
}

func (s *testRuntimeReadStore) PutHookBinding(_ context.Context, item runtime.HookBinding) (runtime.HookBinding, error) {
	for idx := range s.hooks {
		if s.hooks[idx].ID == strings.TrimSpace(item.ID) {
			s.hooks[idx] = item
			return item, nil
		}
	}
	s.hooks = append(s.hooks, item)
	return item, nil
}

func (s *testRuntimeReadStore) GetHookBinding(_ context.Context, id string) (runtime.HookBinding, bool, error) {
	for _, item := range s.hooks {
		if item.ID == strings.TrimSpace(id) {
			return item, true, nil
		}
	}
	return runtime.HookBinding{}, false, nil
}

func (s *testRuntimeReadStore) ListHookBindings(_ context.Context, filter runtime.HookBindingListFilter) ([]runtime.HookBinding, error) {
	var out []runtime.HookBinding
	for _, item := range s.hooks {
		if strings.TrimSpace(filter.ContractID) != "" && item.ContractID != strings.TrimSpace(filter.ContractID) {
			continue
		}
		if strings.TrimSpace(filter.HookPoint) != "" && item.HookPoint != strings.TrimSpace(filter.HookPoint) {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		out = append(out, item)
		if filter.Limit > 0 && len(out) >= filter.Limit {
			break
		}
	}
	return out, nil
}

func (s *testRuntimeReadStore) CreateSystemTruthSource(_ context.Context, item runtime.SystemTruthSource) (runtime.SystemTruthSource, error) {
	return item, nil
}

func (s *testRuntimeReadStore) CreateSystemTruthDraft(_ context.Context, item runtime.SystemTruthDraft) (runtime.SystemTruthDraft, error) {
	return item, nil
}

func (s *testRuntimeReadStore) CreateSystemTruthCompileResult(_ context.Context, item runtime.SystemTruthCompileResult) (runtime.SystemTruthCompileResult, error) {
	return item, nil
}

func (s *testRuntimeReadStore) ActivateSystemTruthVersion(_ context.Context, item runtime.SystemTruthActiveVersion) (runtime.SystemTruthActiveVersion, error) {
	s.activeTruths = append(s.activeTruths, item)
	return item, nil
}

func (s *testRuntimeReadStore) GetActiveSystemTruthVersion(_ context.Context, assetID string) (runtime.SystemTruthActiveVersion, bool, error) {
	for _, item := range s.activeTruths {
		if item.AssetID == strings.TrimSpace(assetID) {
			return item, true, nil
		}
	}
	return runtime.SystemTruthActiveVersion{}, false, nil
}

func (s *testRuntimeReadStore) ListSystemTruthActiveVersions(_ context.Context, assetID string) ([]runtime.SystemTruthActiveVersion, error) {
	var out []runtime.SystemTruthActiveVersion
	for _, item := range s.activeTruths {
		if strings.TrimSpace(assetID) != "" && item.AssetID != strings.TrimSpace(assetID) {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}
