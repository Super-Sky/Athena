// runtime_read.go exposes Control Plane read endpoints for persisted runtime records.
// runtime_read.go 暴露控制面读取持久化 runtime 记录的 HTTP 入口。
package server

import (
	"context"
	"errors"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/runtime"
	"moss/internal/sandbox"
)

type runtimeRunDTO struct {
	ID               string         `json:"id"`
	TaskID           string         `json:"task_id"`
	TaskType         string         `json:"task_type,omitempty"`
	TaskSubtype      string         `json:"task_subtype,omitempty"`
	InputKind        string         `json:"input_kind,omitempty"`
	Scene            string         `json:"scene,omitempty"`
	WorkspaceID      string         `json:"workspace_id,omitempty"`
	AppInstanceID    string         `json:"app_instance_id,omitempty"`
	Status           string         `json:"status"`
	IdempotencyScope string         `json:"idempotency_scope,omitempty"`
	IdempotencyKey   string         `json:"idempotency_key,omitempty"`
	RetentionPolicy  string         `json:"retention_policy,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	StartedAt        *time.Time     `json:"started_at,omitempty"`
	CompletedAt      *time.Time     `json:"completed_at,omitempty"`
}

type runtimeStepDTO struct {
	ID          string         `json:"id"`
	RunID       string         `json:"run_id"`
	Sequence    int            `json:"sequence"`
	StepType    string         `json:"step_type,omitempty"`
	Name        string         `json:"name,omitempty"`
	Status      string         `json:"status"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

type runtimeLifecycleEventDTO struct {
	ID          string         `json:"id"`
	RunID       string         `json:"run_id"`
	StepID      string         `json:"step_id,omitempty"`
	EventType   string         `json:"event_type"`
	SubjectType string         `json:"subject_type"`
	SubjectID   string         `json:"subject_id"`
	FromStatus  string         `json:"from_status,omitempty"`
	ToStatus    string         `json:"to_status,omitempty"`
	Reason      string         `json:"reason,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
	OccurredAt  time.Time      `json:"occurred_at"`
}

type runtimeTraceDTO struct {
	ID              string            `json:"id"`
	RunID           string            `json:"run_id"`
	StepID          string            `json:"step_id,omitempty"`
	TraceType       string            `json:"trace_type,omitempty"`
	Summary         string            `json:"summary"`
	SafeLabels      map[string]string `json:"safe_labels,omitempty"`
	RedactedPayload map[string]any    `json:"redacted_payload,omitempty"`
	Metadata        map[string]any    `json:"metadata,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
}

type runtimeUsageDTO struct {
	ID           string         `json:"id"`
	RunID        string         `json:"run_id"`
	StepID       string         `json:"step_id,omitempty"`
	ResourceType string         `json:"resource_type"`
	Provider     string         `json:"provider,omitempty"`
	ResourceName string         `json:"resource_name,omitempty"`
	Unit         string         `json:"unit"`
	Amount       float64        `json:"amount"`
	Cost         *float64       `json:"cost,omitempty"`
	Currency     string         `json:"currency,omitempty"`
	Metadata     map[string]any `json:"metadata,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
}

type runtimeProjectionCandidateDTO struct {
	ID                    string         `json:"id"`
	RunID                 string         `json:"run_id"`
	StepID                string         `json:"step_id,omitempty"`
	CandidateKind         string         `json:"candidate_kind"`
	Status                string         `json:"status,omitempty"`
	Summary               string         `json:"summary,omitempty"`
	SchemaVersion         string         `json:"schema_version,omitempty"`
	RedactedPayload       map[string]any `json:"redacted_payload,omitempty"`
	SemanticPayload       map[string]any `json:"semantic_payload,omitempty"`
	ArtifactRefs          map[string]any `json:"artifact_refs,omitempty"`
	UIHints               map[string]any `json:"ui_hints,omitempty"`
	MaterializationTarget map[string]any `json:"materialization_target,omitempty"`
	Metadata              map[string]any `json:"metadata,omitempty"`
	CreatedAt             time.Time      `json:"created_at"`
}

type runtimeCheckpointReadoutDTO struct {
	CheckpointID       string     `json:"checkpoint_id"`
	RunID              string     `json:"run_id"`
	Stage              string     `json:"stage,omitempty"`
	ResumeTokenPresent bool       `json:"resume_token_present"`
	PayloadSize        int        `json:"payload_size,omitempty"`
	PayloadSHA256      string     `json:"payload_sha256,omitempty"`
	CreatedAt          *time.Time `json:"created_at,omitempty"`
	UpdatedAt          *time.Time `json:"updated_at,omitempty"`
	SnapshotAvailable  bool       `json:"snapshot_available"`
	Source             string     `json:"source,omitempty"`
}

type runtimeContractDTO struct {
	ID                   string         `json:"id"`
	Name                 string         `json:"name"`
	Version              string         `json:"version"`
	Status               string         `json:"status"`
	TaskType             string         `json:"task_type"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	ExecutionProfile     map[string]any `json:"execution_profile,omitempty"`
	ExitPolicy           map[string]any `json:"exit_policy,omitempty"`
	CapabilityProfile    map[string]any `json:"capability_profile,omitempty"`
	GovernancePolicyRefs map[string]any `json:"governance_policy_refs,omitempty"`
	HookBindings         map[string]any `json:"hook_bindings,omitempty"`
	ProjectionPolicy     map[string]any `json:"projection_policy,omitempty"`
	SystemTruthRefs      map[string]any `json:"system_truth_refs,omitempty"`
	IdempotencyScope     string         `json:"idempotency_scope,omitempty"`
	IdempotencyKey       string         `json:"idempotency_key,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
}

type runtimeTaskTypeRegistrationDTO struct {
	ID                string         `json:"id"`
	TypeKey           string         `json:"type_key"`
	DisplayName       string         `json:"display_name,omitempty"`
	Description       string         `json:"description,omitempty"`
	Status            string         `json:"status"`
	InputSchema       map[string]any `json:"input_schema,omitempty"`
	ValidatorRefs     map[string]any `json:"validator_refs,omitempty"`
	DefaultContractID string         `json:"default_contract_id,omitempty"`
	Compatibility     map[string]any `json:"compatibility,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
	CreatedAt         time.Time      `json:"created_at"`
	UpdatedAt         time.Time      `json:"updated_at"`
}

type runtimeHookBindingDTO struct {
	ID            string         `json:"id"`
	ContractID    string         `json:"contract_id"`
	HookPoint     string         `json:"hook_point"`
	BindingKind   string         `json:"binding_kind"`
	BindingRef    string         `json:"binding_ref"`
	OrderIndex    int            `json:"order_index"`
	Enabled       bool           `json:"enabled"`
	FailurePolicy string         `json:"failure_policy"`
	Config        map[string]any `json:"config,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
}

type systemTruthActiveVersionDTO struct {
	ID              string         `json:"id"`
	AssetID         string         `json:"asset_id"`
	CompileResultID string         `json:"compile_result_id"`
	DraftID         string         `json:"draft_id"`
	ActivatedBy     string         `json:"activated_by,omitempty"`
	Reason          string         `json:"reason,omitempty"`
	RollbackFromID  string         `json:"rollback_from_id,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
	ActivatedAt     time.Time      `json:"activated_at"`
}

type runtimeContractFoundationDTO struct {
	Contracts           []runtimeContractDTO             `json:"contracts"`
	TaskTypes           []runtimeTaskTypeRegistrationDTO `json:"task_types"`
	HookBindings        []runtimeHookBindingDTO          `json:"hook_bindings"`
	ActiveSystemTruths  []systemTruthActiveVersionDTO    `json:"active_system_truths"`
	StoreCapabilities   []string                         `json:"store_capabilities"`
	UnavailableSurfaces []string                         `json:"unavailable_surfaces,omitempty"`
}

type runtimeValidationRunRequest struct {
	WorkspaceID string         `json:"workspace_id,omitempty"`
	Scene       string         `json:"scene,omitempty"`
	Prompt      string         `json:"prompt,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

type runtimeContractUpsertRequest struct {
	Name                 string         `json:"name"`
	Version              string         `json:"version"`
	Status               string         `json:"status"`
	TaskType             string         `json:"task_type"`
	InputSchema          map[string]any `json:"input_schema,omitempty"`
	ExecutionProfile     map[string]any `json:"execution_profile,omitempty"`
	ExitPolicy           map[string]any `json:"exit_policy,omitempty"`
	CapabilityProfile    map[string]any `json:"capability_profile,omitempty"`
	GovernancePolicyRefs map[string]any `json:"governance_policy_refs,omitempty"`
	HookBindings         map[string]any `json:"hook_bindings,omitempty"`
	ProjectionPolicy     map[string]any `json:"projection_policy,omitempty"`
	SystemTruthRefs      map[string]any `json:"system_truth_refs,omitempty"`
	IdempotencyScope     string         `json:"idempotency_scope,omitempty"`
	IdempotencyKey       string         `json:"idempotency_key,omitempty"`
	Metadata             map[string]any `json:"metadata,omitempty"`
}

type runtimeTaskTypeUpsertRequest struct {
	ID                string         `json:"id,omitempty"`
	DisplayName       string         `json:"display_name,omitempty"`
	Description       string         `json:"description,omitempty"`
	Status            string         `json:"status"`
	InputSchema       map[string]any `json:"input_schema,omitempty"`
	ValidatorRefs     map[string]any `json:"validator_refs,omitempty"`
	DefaultContractID string         `json:"default_contract_id,omitempty"`
	Compatibility     map[string]any `json:"compatibility,omitempty"`
	Metadata          map[string]any `json:"metadata,omitempty"`
}

type runtimeHookBindingUpsertRequest struct {
	ContractID    string         `json:"contract_id"`
	HookPoint     string         `json:"hook_point"`
	BindingKind   string         `json:"binding_kind"`
	BindingRef    string         `json:"binding_ref"`
	OrderIndex    int            `json:"order_index"`
	Enabled       bool           `json:"enabled"`
	FailurePolicy string         `json:"failure_policy"`
	Config        map[string]any `json:"config,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

type runtimeValidationRunResponse struct {
	Run                     runtimeRunDTO                           `json:"run"`
	Step                    runtimeStepDTO                          `json:"step"`
	Events                  []runtimeLifecycleEventDTO              `json:"events"`
	Trace                   runtimeTraceDTO                         `json:"trace"`
	Usage                   runtimeUsageDTO                         `json:"usage"`
	Projection              runtimeProjectionCandidateDTO           `json:"projection"`
	ValidationMCP           appcore.ValidationMCPInvocationResult   `json:"validation_mcp"`
	ValidationMCPTrace      runtimeTraceDTO                         `json:"validation_mcp_trace"`
	ValidationMCPUsage      runtimeUsageDTO                         `json:"validation_mcp_usage"`
	ValidationMCPProjection runtimeProjectionCandidateDTO           `json:"validation_mcp_projection"`
	Sandbox                 sandbox.ExternalSandboxValidationResult `json:"sandbox"`
	SandboxEvent            runtimeLifecycleEventDTO                `json:"sandbox_event"`
	SandboxTrace            runtimeTraceDTO                         `json:"sandbox_trace"`
	SandboxUsage            runtimeUsageDTO                         `json:"sandbox_usage"`
	SandboxProjection       runtimeProjectionCandidateDTO           `json:"sandbox_projection"`
}

func handleCreateControlPlaneRuntimeValidationRun(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	var req runtimeValidationRunRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	result, err := application.CreateRuntimeValidationRun(ctx, appcore.RuntimeValidationRunRequest{
		WorkspaceID: strings.TrimSpace(req.WorkspaceID),
		Scene:       strings.TrimSpace(req.Scene),
		Prompt:      strings.TrimSpace(req.Prompt),
		Source:      "control_plane_runtime_validation_endpoint",
		Metadata:    req.Metadata,
	})
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusCreated, runtimeValidationRunResponseFromRuntime(result))
}

func handleListControlPlaneRuntimeRuns(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeRuns(ctx, appcore.RuntimeRunReadQuery{
		WorkspaceID: strings.TrimSpace(c.Query("workspace_id")),
		Status:      strings.TrimSpace(c.Query("status")),
		Limit:       parseOptionalInt(c.Query("limit")),
	})
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeRunDTOs(items)})
}

func handleGetControlPlaneRuntimeRun(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	item, ok, err := application.GetRuntimeRun(ctx, strings.TrimSpace(c.Param("runID")))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	if !ok {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "runtime run not found"})
		return
	}
	c.JSON(consts.StatusOK, runtimeRunDTOFromRuntime(item))
}

func handleListControlPlaneRuntimeSteps(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeSteps(ctx, strings.TrimSpace(c.Param("runID")))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeStepDTOs(items)})
}

func handleListControlPlaneRuntimeLifecycleEvents(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeLifecycleEvents(ctx, strings.TrimSpace(c.Param("runID")))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeLifecycleEventDTOs(items)})
}

func handleListControlPlaneRuntimeTraces(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeTraces(ctx, runtimeRecordReadQueryFromRequest(c))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeTraceDTOs(items)})
}

func handleListControlPlaneRuntimeUsage(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeUsage(ctx, runtimeRecordReadQueryFromRequest(c))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeUsageDTOs(items)})
}

func handleListControlPlaneRuntimeProjectionCandidates(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeProjectionCandidates(ctx, runtimeRecordReadQueryFromRequest(c))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeProjectionCandidateDTOs(items)})
}

func handleListControlPlaneRuntimeCheckpoints(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	items, err := application.ListRuntimeCheckpointReadouts(ctx, strings.TrimSpace(c.Param("runID")))
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": runtimeCheckpointReadoutDTOs(items)})
}

func handleGetControlPlaneRuntimeContractFoundation(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	readout, err := application.GetRuntimeContractFoundation(ctx)
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, runtimeContractFoundationDTOFromApp(readout))
}

func handlePutControlPlaneRuntimeContract(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	var req runtimeContractUpsertRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateRuntimeContract(ctx, strings.TrimSpace(c.Param("contractID")), runtime.RuntimeContract{
		Name:                 strings.TrimSpace(req.Name),
		Version:              strings.TrimSpace(req.Version),
		Status:               strings.TrimSpace(req.Status),
		TaskType:             strings.TrimSpace(req.TaskType),
		InputSchema:          req.InputSchema,
		ExecutionProfile:     req.ExecutionProfile,
		ExitPolicy:           req.ExitPolicy,
		CapabilityProfile:    req.CapabilityProfile,
		GovernancePolicyRefs: req.GovernancePolicyRefs,
		HookBindings:         req.HookBindings,
		ProjectionPolicy:     req.ProjectionPolicy,
		SystemTruthRefs:      req.SystemTruthRefs,
		IdempotencyScope:     strings.TrimSpace(req.IdempotencyScope),
		IdempotencyKey:       strings.TrimSpace(req.IdempotencyKey),
		Metadata:             req.Metadata,
	})
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, runtimeContractDTOFromRuntime(item))
}

func handlePutControlPlaneRuntimeTaskType(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	var req runtimeTaskTypeUpsertRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateRuntimeTaskTypeRegistration(ctx, strings.TrimSpace(c.Param("typeKey")), runtime.TaskTypeRegistration{
		ID:                strings.TrimSpace(req.ID),
		DisplayName:       strings.TrimSpace(req.DisplayName),
		Description:       strings.TrimSpace(req.Description),
		Status:            strings.TrimSpace(req.Status),
		InputSchema:       req.InputSchema,
		ValidatorRefs:     req.ValidatorRefs,
		DefaultContractID: strings.TrimSpace(req.DefaultContractID),
		Compatibility:     req.Compatibility,
		Metadata:          req.Metadata,
	})
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, runtimeTaskTypeRegistrationDTOFromRuntime(item))
}

func handlePutControlPlaneRuntimeHookBinding(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	var req runtimeHookBindingUpsertRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateRuntimeHookBinding(ctx, strings.TrimSpace(c.Param("bindingID")), runtime.HookBinding{
		ContractID:    strings.TrimSpace(req.ContractID),
		HookPoint:     strings.TrimSpace(req.HookPoint),
		BindingKind:   strings.TrimSpace(req.BindingKind),
		BindingRef:    strings.TrimSpace(req.BindingRef),
		OrderIndex:    req.OrderIndex,
		Enabled:       req.Enabled,
		FailurePolicy: strings.TrimSpace(req.FailurePolicy),
		Config:        req.Config,
		Metadata:      req.Metadata,
	})
	if err != nil {
		writeRuntimeReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, runtimeHookBindingDTOFromRuntime(item))
}

func runtimeRecordReadQueryFromRequest(c *hertzapp.RequestContext) appcore.RuntimeRecordReadQuery {
	return appcore.RuntimeRecordReadQuery{
		RunID:  strings.TrimSpace(c.Param("runID")),
		StepID: strings.TrimSpace(c.Query("step_id")),
		Limit:  parseOptionalInt(c.Query("limit")),
	}
}

func writeRuntimeReadError(c *hertzapp.RequestContext, err error) {
	if errors.Is(err, appcore.ErrRuntimeStoreNotConfigured) || errors.Is(err, appcore.ErrRuntimeFoundationWriteUnsupported) {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, runtime.ErrInvalidRuntimePersistenceInput) {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func runtimeRunDTOs(items []runtime.TaskRun) []runtimeRunDTO {
	out := make([]runtimeRunDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeRunDTOFromRuntime(item))
	}
	return out
}

func runtimeValidationRunResponseFromRuntime(item appcore.RuntimeValidationRunResult) runtimeValidationRunResponse {
	return runtimeValidationRunResponse{
		Run:                     runtimeRunDTOFromRuntime(item.RecordSet.Run),
		Step:                    runtimeStepDTOFromRuntime(item.RecordSet.Step),
		Events:                  runtimeLifecycleEventDTOs(item.RecordSet.Events),
		Trace:                   runtimeTraceDTOFromRuntime(item.RecordSet.Trace),
		Usage:                   runtimeUsageDTOFromRuntime(item.RecordSet.Usage),
		Projection:              runtimeProjectionCandidateDTOFromRuntime(item.RecordSet.Projection),
		ValidationMCP:           item.ValidationMCP,
		ValidationMCPTrace:      runtimeTraceDTOFromRuntime(item.ValidationMCPTrace),
		ValidationMCPUsage:      runtimeUsageDTOFromRuntime(item.ValidationMCPUsage),
		ValidationMCPProjection: runtimeProjectionCandidateDTOFromRuntime(item.ValidationMCPProjection),
		Sandbox:                 item.Sandbox,
		SandboxEvent:            runtimeLifecycleEventDTOFromRuntime(item.SandboxEvent),
		SandboxTrace:            runtimeTraceDTOFromRuntime(item.SandboxTrace),
		SandboxUsage:            runtimeUsageDTOFromRuntime(item.SandboxUsage),
		SandboxProjection:       runtimeProjectionCandidateDTOFromRuntime(item.SandboxProjection),
	}
}

func runtimeRunDTOFromRuntime(item runtime.TaskRun) runtimeRunDTO {
	return runtimeRunDTO{
		ID:               item.ID,
		TaskID:           item.TaskID,
		TaskType:         item.TaskType,
		TaskSubtype:      item.TaskSubtype,
		InputKind:        item.InputKind,
		Scene:            item.Scene,
		WorkspaceID:      item.WorkspaceID,
		AppInstanceID:    item.AppInstanceID,
		Status:           item.Status,
		IdempotencyScope: item.IdempotencyScope,
		IdempotencyKey:   item.IdempotencyKey,
		RetentionPolicy:  item.RetentionPolicy,
		Metadata:         item.Metadata,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		StartedAt:        item.StartedAt,
		CompletedAt:      item.CompletedAt,
	}
}

func runtimeStepDTOs(items []runtime.TaskStep) []runtimeStepDTO {
	out := make([]runtimeStepDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeStepDTOFromRuntime(item))
	}
	return out
}

func runtimeStepDTOFromRuntime(item runtime.TaskStep) runtimeStepDTO {
	return runtimeStepDTO{
		ID:          item.ID,
		RunID:       item.RunID,
		Sequence:    item.Sequence,
		StepType:    item.StepType,
		Name:        item.Name,
		Status:      item.Status,
		Metadata:    item.Metadata,
		CreatedAt:   item.CreatedAt,
		UpdatedAt:   item.UpdatedAt,
		StartedAt:   item.StartedAt,
		CompletedAt: item.CompletedAt,
	}
}

func runtimeLifecycleEventDTOs(items []runtime.TaskRunLifecycleEvent) []runtimeLifecycleEventDTO {
	out := make([]runtimeLifecycleEventDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeLifecycleEventDTOFromRuntime(item))
	}
	return out
}

func runtimeLifecycleEventDTOFromRuntime(item runtime.TaskRunLifecycleEvent) runtimeLifecycleEventDTO {
	return runtimeLifecycleEventDTO{
		ID:          item.ID,
		RunID:       item.RunID,
		StepID:      item.StepID,
		EventType:   item.EventType,
		SubjectType: item.SubjectType,
		SubjectID:   item.SubjectID,
		FromStatus:  item.FromStatus,
		ToStatus:    item.ToStatus,
		Reason:      item.Reason,
		Metadata:    item.Metadata,
		OccurredAt:  item.OccurredAt,
	}
}

func runtimeTraceDTOs(items []runtime.RuntimeTrace) []runtimeTraceDTO {
	out := make([]runtimeTraceDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeTraceDTOFromRuntime(item))
	}
	return out
}

func runtimeTraceDTOFromRuntime(item runtime.RuntimeTrace) runtimeTraceDTO {
	return runtimeTraceDTO{
		ID:              item.ID,
		RunID:           item.RunID,
		StepID:          item.StepID,
		TraceType:       item.TraceType,
		Summary:         item.Summary,
		SafeLabels:      item.SafeLabels,
		RedactedPayload: item.RedactedPayload,
		Metadata:        item.Metadata,
		CreatedAt:       item.CreatedAt,
	}
}

func runtimeUsageDTOs(items []runtime.Usage) []runtimeUsageDTO {
	out := make([]runtimeUsageDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeUsageDTOFromRuntime(item))
	}
	return out
}

func runtimeUsageDTOFromRuntime(item runtime.Usage) runtimeUsageDTO {
	return runtimeUsageDTO{
		ID:           item.ID,
		RunID:        item.RunID,
		StepID:       item.StepID,
		ResourceType: item.ResourceType,
		Provider:     item.Provider,
		ResourceName: item.ResourceName,
		Unit:         item.Unit,
		Amount:       item.Amount,
		Cost:         item.Cost,
		Currency:     item.Currency,
		Metadata:     item.Metadata,
		CreatedAt:    item.CreatedAt,
	}
}

func runtimeProjectionCandidateDTOs(items []runtime.ProjectionCandidate) []runtimeProjectionCandidateDTO {
	out := make([]runtimeProjectionCandidateDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeProjectionCandidateDTOFromRuntime(item))
	}
	return out
}

func runtimeProjectionCandidateDTOFromRuntime(item runtime.ProjectionCandidate) runtimeProjectionCandidateDTO {
	return runtimeProjectionCandidateDTO{
		ID:                    item.ID,
		RunID:                 item.RunID,
		StepID:                item.StepID,
		CandidateKind:         item.CandidateKind,
		Status:                item.Status,
		Summary:               item.Summary,
		SchemaVersion:         item.SchemaVersion,
		RedactedPayload:       item.RedactedPayload,
		SemanticPayload:       item.SemanticPayload,
		ArtifactRefs:          item.ArtifactRefs,
		UIHints:               item.UIHints,
		MaterializationTarget: item.MaterializationTarget,
		Metadata:              item.Metadata,
		CreatedAt:             item.CreatedAt,
	}
}

func runtimeCheckpointReadoutDTOs(items []appcore.RuntimeCheckpointReadout) []runtimeCheckpointReadoutDTO {
	out := make([]runtimeCheckpointReadoutDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeCheckpointReadoutDTOFromApp(item))
	}
	return out
}

func runtimeCheckpointReadoutDTOFromApp(item appcore.RuntimeCheckpointReadout) runtimeCheckpointReadoutDTO {
	return runtimeCheckpointReadoutDTO{
		CheckpointID:       item.CheckpointID,
		RunID:              item.RunID,
		Stage:              item.Stage,
		ResumeTokenPresent: item.ResumeTokenPresent,
		PayloadSize:        item.PayloadSize,
		PayloadSHA256:      item.PayloadSHA256,
		CreatedAt:          runtimeTimePtr(item.CreatedAt),
		UpdatedAt:          runtimeTimePtr(item.UpdatedAt),
		SnapshotAvailable:  item.SnapshotAvailable,
		Source:             item.Source,
	}
}

func runtimeTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func runtimeContractFoundationDTOFromApp(item appcore.RuntimeContractFoundationReadout) runtimeContractFoundationDTO {
	return runtimeContractFoundationDTO{
		Contracts:           runtimeContractDTOs(item.Contracts),
		TaskTypes:           runtimeTaskTypeRegistrationDTOs(item.TaskTypes),
		HookBindings:        runtimeHookBindingDTOs(item.HookBindings),
		ActiveSystemTruths:  systemTruthActiveVersionDTOs(item.ActiveSystemTruths),
		StoreCapabilities:   item.StoreCapabilities,
		UnavailableSurfaces: item.UnavailableSurfaces,
	}
}

func runtimeContractDTOs(items []runtime.RuntimeContract) []runtimeContractDTO {
	out := make([]runtimeContractDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeContractDTOFromRuntime(item))
	}
	return out
}

func runtimeContractDTOFromRuntime(item runtime.RuntimeContract) runtimeContractDTO {
	return runtimeContractDTO{
		ID:                   item.ID,
		Name:                 item.Name,
		Version:              item.Version,
		Status:               item.Status,
		TaskType:             item.TaskType,
		InputSchema:          item.InputSchema,
		ExecutionProfile:     item.ExecutionProfile,
		ExitPolicy:           item.ExitPolicy,
		CapabilityProfile:    item.CapabilityProfile,
		GovernancePolicyRefs: item.GovernancePolicyRefs,
		HookBindings:         item.HookBindings,
		ProjectionPolicy:     item.ProjectionPolicy,
		SystemTruthRefs:      item.SystemTruthRefs,
		IdempotencyScope:     item.IdempotencyScope,
		IdempotencyKey:       item.IdempotencyKey,
		Metadata:             item.Metadata,
		CreatedAt:            item.CreatedAt,
		UpdatedAt:            item.UpdatedAt,
	}
}

func runtimeTaskTypeRegistrationDTOs(items []runtime.TaskTypeRegistration) []runtimeTaskTypeRegistrationDTO {
	out := make([]runtimeTaskTypeRegistrationDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeTaskTypeRegistrationDTOFromRuntime(item))
	}
	return out
}

func runtimeTaskTypeRegistrationDTOFromRuntime(item runtime.TaskTypeRegistration) runtimeTaskTypeRegistrationDTO {
	return runtimeTaskTypeRegistrationDTO{
		ID:                item.ID,
		TypeKey:           item.TypeKey,
		DisplayName:       item.DisplayName,
		Description:       item.Description,
		Status:            item.Status,
		InputSchema:       item.InputSchema,
		ValidatorRefs:     item.ValidatorRefs,
		DefaultContractID: item.DefaultContractID,
		Compatibility:     item.Compatibility,
		Metadata:          item.Metadata,
		CreatedAt:         item.CreatedAt,
		UpdatedAt:         item.UpdatedAt,
	}
}

func runtimeHookBindingDTOs(items []runtime.HookBinding) []runtimeHookBindingDTO {
	out := make([]runtimeHookBindingDTO, 0, len(items))
	for _, item := range items {
		out = append(out, runtimeHookBindingDTOFromRuntime(item))
	}
	return out
}

func runtimeHookBindingDTOFromRuntime(item runtime.HookBinding) runtimeHookBindingDTO {
	return runtimeHookBindingDTO{
		ID:            item.ID,
		ContractID:    item.ContractID,
		HookPoint:     item.HookPoint,
		BindingKind:   item.BindingKind,
		BindingRef:    item.BindingRef,
		OrderIndex:    item.OrderIndex,
		Enabled:       item.Enabled,
		FailurePolicy: item.FailurePolicy,
		Config:        item.Config,
		Metadata:      item.Metadata,
		CreatedAt:     item.CreatedAt,
		UpdatedAt:     item.UpdatedAt,
	}
}

func systemTruthActiveVersionDTOs(items []runtime.SystemTruthActiveVersion) []systemTruthActiveVersionDTO {
	out := make([]systemTruthActiveVersionDTO, 0, len(items))
	for _, item := range items {
		out = append(out, systemTruthActiveVersionDTOFromRuntime(item))
	}
	return out
}

func systemTruthActiveVersionDTOFromRuntime(item runtime.SystemTruthActiveVersion) systemTruthActiveVersionDTO {
	return systemTruthActiveVersionDTO{
		ID:              item.ID,
		AssetID:         item.AssetID,
		CompileResultID: item.CompileResultID,
		DraftID:         item.DraftID,
		ActivatedBy:     item.ActivatedBy,
		Reason:          item.Reason,
		RollbackFromID:  item.RollbackFromID,
		Metadata:        item.Metadata,
		ActivatedAt:     item.ActivatedAt,
	}
}
