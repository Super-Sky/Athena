// runtime_validation.go exposes a control-plane validation trigger for persisted runtime records.
// runtime_validation.go 暴露控制面触发持久化 runtime 记录的验证入口。
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	einomessage "github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"moss/internal/runtime"
	runtimetask "moss/internal/runtime/task"
	"moss/internal/sandbox"
	"moss/internal/validationmcp"
)

const (
	defaultRuntimeValidationWorkspace = "system-validation"
	defaultRuntimeValidationScene     = "system_validation"
	defaultRuntimeValidationPrompt    = "validate runtime persistence read surface"
)

// RuntimeValidationRunRequest contains the safe control-plane fields for one validation write.
// RuntimeValidationRunRequest 保存一次控制面验证写入所需的安全字段。
type RuntimeValidationRunRequest struct {
	WorkspaceID string
	Scene       string
	Prompt      string
	Source      string
	Metadata    map[string]any
}

// RuntimeValidationRunResult returns the deterministic validation records produced by the graph, MCP, and sandbox path.
// RuntimeValidationRunResult 返回 graph、MCP 与 sandbox 路径生成的确定性验证记录。
type RuntimeValidationRunResult struct {
	RecordSet               runtime.MinimalPersistenceRecordSet
	ValidationMCP           ValidationMCPInvocationResult
	ValidationMCPTrace      runtime.RuntimeTrace
	ValidationMCPUsage      runtime.Usage
	ValidationMCPProjection runtime.ProjectionCandidate
	Sandbox                 sandbox.ExternalSandboxValidationResult
	SandboxEvent            runtime.TaskRunLifecycleEvent
	SandboxTrace            runtime.RuntimeTrace
	SandboxUsage            runtime.Usage
	SandboxProjection       runtime.ProjectionCandidate
}

// CreateRuntimeValidationRun runs the Eino graph foundation and persists one safe validation record set.
// CreateRuntimeValidationRun 执行 Eino graph 基础链路并写入一组安全验证记录。
func (s *Service) CreateRuntimeValidationRun(ctx context.Context, req RuntimeValidationRunRequest) (RuntimeValidationRunResult, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	if err := s.syncRuntimeContractFoundation(ctx); err != nil {
		return RuntimeValidationRunResult{}, err
	}

	requestID := "runtime-validation-" + uuid.NewString()
	workspaceID := defaultString(strings.TrimSpace(req.WorkspaceID), defaultRuntimeValidationWorkspace)
	scene := defaultString(strings.TrimSpace(req.Scene), defaultRuntimeValidationScene)
	prompt := defaultString(strings.TrimSpace(req.Prompt), defaultRuntimeValidationPrompt)
	source := defaultString(strings.TrimSpace(req.Source), "control_plane")

	spec := runtimeValidationExecutionSpec(requestID, workspaceID, scene, source, prompt, req.Metadata)
	graph := runtime.EinoGraphFoundation{
		Executor: runtimeValidationTurnExecutor{},
		Store:    store,
		Now:      func() time.Time { return time.Now().UTC() },
	}
	frame, err := graph.Run(ctx, &runtime.RuntimeGraphFrame{
		State: runtime.RuntimeState{
			RequestID: requestID,
			SessionID: "system-validation",
			Turn:      1,
		},
		Spec:     spec,
		Messages: []adk.Message{einomessage.UserMessage(prompt)},
		Metadata: map[string]any{},
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	if frame == nil || frame.RecordSet == nil {
		return RuntimeValidationRunResult{}, fmt.Errorf("runtime validation graph completed without persistence records")
	}
	extensions, err := s.persistRuntimeValidationExtensions(ctx, store, *frame.RecordSet, requestID, req.Metadata)
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	return extensions, nil
}

func (s *Service) persistRuntimeValidationExtensions(ctx context.Context, store runtime.RuntimePersistenceStore, recordSet runtime.MinimalPersistenceRecordSet, requestID string, metadata map[string]any) (RuntimeValidationRunResult, error) {
	mcp, err := s.InvokeValidationMCPTool(ctx, validationmcp.InvocationRequest{
		ToolName: "risk_signal_lookup",
		Input: map[string]any{
			"risk_key": "sandbox_write_validation",
			"credentials": map[string]any{
				"authorization": "sample-token-redacted",
			},
		},
		Metadata: mergeRuntimeValidationMetadata(metadata, map[string]any{
			"ui_surface":          "system_validation",
			"validation_request":  requestID,
			"authorization_token": "sample-token-redacted",
		}),
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}

	now := time.Now().UTC()
	mcpEvent, err := store.CreateLifecycleEvent(ctx, runtime.TaskRunLifecycleEvent{
		RunID:       recordSet.Run.ID,
		StepID:      recordSet.Step.ID,
		EventType:   "validation_mcp_invoked",
		SubjectType: runtime.LifecycleSubjectStep,
		SubjectID:   recordSet.Step.ID,
		FromStatus:  runtime.TaskStepStatusRunning,
		ToStatus:    runtime.TaskStepStatusSuccess,
		Reason:      "deterministic_validation_mcp_invocation",
		Metadata: map[string]any{
			"server_id":   mcp.Result.ServerID,
			"tool_name":   mcp.Result.ToolName,
			"decision_id": mcp.GovernanceDecision.DecisionID,
			"decision":    mcp.GovernanceDecision.Decision,
		},
		OccurredAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	mcpTrace, err := store.CreateRuntimeTrace(ctx, runtime.RuntimeTrace{
		RunID:           recordSet.Run.ID,
		StepID:          recordSet.Step.ID,
		TraceType:       mcp.Result.Trace.TraceType,
		Summary:         mcp.Result.Trace.Summary,
		SafeLabels:      mcp.Result.Trace.SafeLabels,
		RedactedPayload: mcp.Result.Trace.RedactedPayload,
		Metadata: map[string]any{
			"source":        "runtime_validation_flow",
			"invocation_id": mcp.Result.InvocationID,
		},
		CreatedAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	mcpUsage, err := store.CreateUsage(ctx, runtime.Usage{
		RunID:        recordSet.Run.ID,
		StepID:       recordSet.Step.ID,
		ResourceType: "validation_mcp_tool",
		Provider:     mcp.Result.ServerID,
		ResourceName: mcp.Result.ToolName,
		Unit:         "invocation",
		Amount:       1,
		Metadata: map[string]any{
			"decision":          mcp.GovernanceDecision.Decision,
			"applied_redaction": mcp.Result.AppliedRedaction,
		},
		CreatedAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	mcpProjection, err := store.CreateProjectionCandidate(ctx, runtime.ProjectionCandidate{
		RunID:         recordSet.Run.ID,
		StepID:        recordSet.Step.ID,
		CandidateKind: "validation_mcp_result",
		Status:        mcp.Result.Status,
		Summary:       mcp.Result.ResultSummary,
		RedactedPayload: map[string]any{
			"server_id":  mcp.Result.ServerID,
			"tool_name":  mcp.Result.ToolName,
			"output":     mcp.Result.Output,
			"trace_type": mcp.Result.Trace.TraceType,
		},
		Metadata:  map[string]any{"source": "runtime_validation_flow"},
		CreatedAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}

	sandboxResult := sandbox.BuildExternalSandboxValidationResult(sandbox.ValidationRequest{
		RequestID:          requestID,
		RunID:              recordSet.Run.ID,
		StepID:             recordSet.Step.ID,
		ToolName:           mcp.Result.ToolName,
		GovernanceDecision: mcp.GovernanceDecision.Decision,
		RiskLevel:          stringFromMap(mcp.Result.Output, "risk_level"),
		Signals:            stringSliceFromMap(mcp.Result.Output, "signals"),
		Metadata:           metadata,
	})
	sandboxEvent, err := store.CreateLifecycleEvent(ctx, runtime.TaskRunLifecycleEvent{
		RunID:       recordSet.Run.ID,
		StepID:      recordSet.Step.ID,
		EventType:   "external_sandbox_ref_recorded",
		SubjectType: runtime.LifecycleSubjectStep,
		SubjectID:   recordSet.Step.ID,
		FromStatus:  runtime.TaskStepStatusRunning,
		ToStatus:    runtime.TaskStepStatusSuccess,
		Reason:      "deterministic_external_sandbox_ref",
		Metadata: map[string]any{
			"sandbox_ref_id": sandboxResult.SandboxRef.RefID,
			"sandbox_mode":   sandboxResult.SandboxRef.Mode,
			"audit_summary":  sandboxResult.AuditSummary.Summary,
		},
		OccurredAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	sandboxTrace, err := store.CreateRuntimeTrace(ctx, runtime.RuntimeTrace{
		RunID:           recordSet.Run.ID,
		StepID:          recordSet.Step.ID,
		TraceType:       "external_sandbox_ref",
		Summary:         sandboxResult.StructuredResult.Summary,
		SafeLabels:      sandboxResult.AuditSummary.SafeLabels,
		RedactedPayload: sandboxResult.RedactedPayload(),
		Metadata: map[string]any{
			"source":         "runtime_validation_flow",
			"sandbox_ref_id": sandboxResult.SandboxRef.RefID,
		},
		CreatedAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	sandboxUsage, err := store.CreateUsage(ctx, runtime.Usage{
		RunID:        recordSet.Run.ID,
		StepID:       recordSet.Step.ID,
		ResourceType: "sandbox_boundary",
		Provider:     sandboxResult.SandboxRef.Provider,
		ResourceName: sandboxResult.SandboxRef.Mode,
		Unit:         "boundary",
		Amount:       1,
		Metadata: map[string]any{
			"sandbox_ref_id": sandboxResult.SandboxRef.RefID,
			"boundary":       sandboxResult.SandboxRef.Boundary,
		},
		CreatedAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}
	sandboxProjection, err := store.CreateProjectionCandidate(ctx, runtime.ProjectionCandidate{
		RunID:           recordSet.Run.ID,
		StepID:          recordSet.Step.ID,
		CandidateKind:   "external_sandbox_ref",
		Status:          sandboxResult.StructuredResult.Status,
		Summary:         sandboxResult.StructuredResult.Summary,
		RedactedPayload: sandboxResult.RedactedPayload(),
		Metadata: map[string]any{
			"source":         "runtime_validation_flow",
			"sandbox_ref_id": sandboxResult.SandboxRef.RefID,
		},
		CreatedAt: now,
	})
	if err != nil {
		return RuntimeValidationRunResult{}, err
	}

	result := RuntimeValidationRunResult{
		RecordSet:               recordSet,
		ValidationMCP:           mcp,
		ValidationMCPTrace:      mcpTrace,
		ValidationMCPUsage:      mcpUsage,
		ValidationMCPProjection: mcpProjection,
		Sandbox:                 sandboxResult,
		SandboxEvent:            sandboxEvent,
		SandboxTrace:            sandboxTrace,
		SandboxUsage:            sandboxUsage,
		SandboxProjection:       sandboxProjection,
	}
	result.RecordSet.Events = append(result.RecordSet.Events, mcpEvent, sandboxEvent)
	return result, nil
}

type runtimeValidationTurnExecutor struct{}

func (runtimeValidationTurnExecutor) Prepare(_ context.Context, _ runtime.RuntimeState, spec *runtime.ExecutionSpec, messages []adk.Message) (*runtime.PreparedExecution, error) {
	return &runtime.PreparedExecution{
		Spec:          spec,
		Messages:      messages,
		Initial:       &runtime.TurnResult{Kind: runtime.TurnResultFinal, Content: "runtime validation graph completed"},
		InitialStatus: runtime.RequestStatusCompleted,
		Orchestration: runtime.OrchestrationStateCompleted,
	}, nil
}

func runtimeValidationExecutionSpec(requestID string, workspaceID string, scene string, source string, prompt string, metadata map[string]any) *runtime.ExecutionSpec {
	constraints := map[string]any{
		"task_id":             requestID,
		"task_type":           "runtime_validation",
		"task_subtype":        "control_plane_trigger",
		"runtime_contract_id": runtimeValidationContractID,
		"task_kind":           runtimetask.InputKindChat,
		"workspace_id":        workspaceID,
		"app_instance_id":     "athena-control-plane",
		"scene":               scene,
		"desired_output_mode": runtimetask.DefaultOutputModeText,
		"source":              source,
		"prompt_length":       len(prompt),
	}
	for key, value := range metadata {
		trimmedKey := strings.TrimSpace(key)
		if trimmedKey != "" && value != nil {
			constraints[trimmedKey] = value
		}
	}
	return &runtime.ExecutionSpec{
		Skill: runtime.SkillSpec{
			PrimarySkill: "runtime_validation",
			Guidance:     "validate runtime persistence through the Eino graph foundation",
		},
		Tools: runtime.ToolSpec{AllowedTools: nil},
		Inference: runtime.InferenceSpec{
			Goal:       "validate runtime persistence read surface",
			OutputMode: runtimetask.DefaultOutputModeText,
		},
		Processing: runtime.ProcessingSpec{PreferDirectAnswer: true},
		Metadata: runtime.ExecutionMetadata{
			ResolverReason: "control_plane_runtime_validation_trigger",
			Governance:     &runtime.GovernanceDecision{Decision: runtime.GovernanceDecisionAllow, Reason: "internal_validation"},
			Orchestration: &runtime.OrchestrationStatus{
				EntryState:   runtime.OrchestrationStateNormalized,
				CurrentState: runtime.OrchestrationStateExecuting,
				Reason:       "control_plane_validation",
			},
			Constraints: constraints,
		},
	}
}

func defaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return fallback
}

func mergeRuntimeValidationMetadata(base map[string]any, extra map[string]any) map[string]any {
	result := map[string]any{}
	for key, value := range base {
		if strings.TrimSpace(key) != "" && value != nil {
			result[key] = value
		}
	}
	for key, value := range extra {
		if strings.TrimSpace(key) != "" && value != nil {
			result[key] = value
		}
	}
	return result
}

func stringFromMap(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	value, ok := values[key]
	if !ok || value == nil {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(value))
}

func stringSliceFromMap(values map[string]any, key string) []string {
	raw, ok := values[key]
	if !ok || raw == nil {
		return nil
	}
	switch typed := raw.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := strings.TrimSpace(fmt.Sprint(item)); value != "" {
				out = append(out, value)
			}
		}
		return out
	default:
		value := strings.TrimSpace(fmt.Sprint(typed))
		if value == "" {
			return nil
		}
		return []string{value}
	}
}
