// agent_runs.go exposes the app-facing Agent Run API on top of the generic runtime path.
// agent_runs.go 在通用 runtime 主链之上暴露面向业务应用的 Agent Run API。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/controlplane"
	"moss/internal/customization"
	platformcontext "moss/internal/extensions/platform/context"
	modelparams "moss/internal/model/parameters"
	"moss/internal/runtime"
)

const (
	agentRunContractTaskType       = "agent_run"
	defaultAgentRunRuntimeTaskType = "chat"
	defaultAgentRunTaskSubtype     = "goal_directed"
	defaultAgentRunScene           = "agent_runtime"
	defaultAgentRunOutputMode      = "text"
	agentRunSourceCreate           = "create"
	agentRunSourceResume           = "resume"
	agentRunCancelUnsupported      = "cancel_unsupported"
	agentRunCancelAlreadyTerminal  = "cancel_rejected"
)

type agentRunStartRequest struct {
	Goal                   string                     `json:"goal,omitempty"`
	Query                  string                     `json:"query,omitempty"`
	SuccessCriteria        []string                   `json:"success_criteria,omitempty"`
	Constraints            map[string]any             `json:"constraints,omitempty"`
	Budget                 map[string]any             `json:"budget,omitempty"`
	ContextAssets          []contextassets.Asset      `json:"context_assets,omitempty"`
	Tools                  []agentRunToolDeclaration  `json:"tools,omitempty"`
	ToolChoice             any                        `json:"tool_choice,omitempty"`
	MemoryScope            map[string]any             `json:"memory_scope,omitempty"`
	GovernanceRefs         []string                   `json:"governance_refs,omitempty"`
	TaskType               string                     `json:"task_type,omitempty"`
	TaskSubtype            string                     `json:"task_subtype,omitempty"`
	Scene                  string                     `json:"scene,omitempty"`
	SessionID              string                     `json:"session_id,omitempty"`
	MainSessionID          string                     `json:"main_session_id,omitempty"`
	WorkspaceID            string                     `json:"workspace_id,omitempty"`
	AppInstanceID          string                     `json:"app_instance_id,omitempty"`
	AppSessionID           string                     `json:"app_session_id,omitempty"`
	IntegrationInstanceID  string                     `json:"integration_instance_id,omitempty"`
	WorkflowRunID          string                     `json:"workflow_run_id,omitempty"`
	StepID                 string                     `json:"step_id,omitempty"`
	TriggerType            string                     `json:"trigger_type,omitempty"`
	AutomationTaskID       string                     `json:"automation_task_id,omitempty"`
	UserLanguage           string                     `json:"user_language,omitempty"`
	DesiredOutputMode      string                     `json:"desired_output_mode,omitempty"`
	GlobalContext          map[string]any             `json:"global_context,omitempty"`
	AppContext             map[string]any             `json:"app_context,omitempty"`
	InputPayload           map[string]any             `json:"input_payload,omitempty"`
	ModelID                string                     `json:"model_id,omitempty"`
	ModelRecordID          string                     `json:"model_record_id,omitempty"`
	EnabledSkills          []string                   `json:"enabled_skills,omitempty"`
	EnabledTools           []string                   `json:"enabled_tools,omitempty"`
	PromptTemplate         string                     `json:"prompt_template,omitempty"`
	DisabledAssetTypes     []string                   `json:"disabled_asset_types,omitempty"`
	AssetPriorityOverrides map[string]int             `json:"asset_priority_overrides,omitempty"`
	Supplement             *runtime.SupplementPayload `json:"supplement,omitempty"`
	SupplementOutcome      runtime.SupplementOutcome  `json:"supplement_outcome,omitempty"`
	ResumeToken            string                     `json:"resume_token,omitempty"`
	TimeoutAfterSeconds    int                        `json:"timeout_after_seconds,omitempty"`
	DisableFastPath        bool                       `json:"disable_fast_path,omitempty"`
	ResumedFromRunID       string                     `json:"-"`
	CanonicalTools         []runtime.ToolDefinition   `json:"-"`
	CanonicalToolChoice    modelparams.ToolChoice     `json:"-"`
}

type agentRunResumeRequest struct {
	Goal                   string                     `json:"goal,omitempty"`
	Query                  string                     `json:"query,omitempty"`
	Supplement             *runtime.SupplementPayload `json:"supplement,omitempty"`
	SupplementOutcome      runtime.SupplementOutcome  `json:"supplement_outcome,omitempty"`
	ResumeToken            string                     `json:"resume_token,omitempty"`
	SessionID              string                     `json:"session_id,omitempty"`
	MainSessionID          string                     `json:"main_session_id,omitempty"`
	ModelID                string                     `json:"model_id,omitempty"`
	EnabledSkills          []string                   `json:"enabled_skills,omitempty"`
	EnabledTools           []string                   `json:"enabled_tools,omitempty"`
	ContextAssets          []contextassets.Asset      `json:"context_assets,omitempty"`
	GlobalContext          map[string]any             `json:"global_context,omitempty"`
	AppContext             map[string]any             `json:"app_context,omitempty"`
	InputPayload           map[string]any             `json:"input_payload,omitempty"`
	TimeoutAfterSeconds    int                        `json:"timeout_after_seconds,omitempty"`
	DisableFastPath        bool                       `json:"disable_fast_path,omitempty"`
	PromptTemplate         string                     `json:"prompt_template,omitempty"`
	DisabledAssetTypes     []string                   `json:"disabled_asset_types,omitempty"`
	AssetPriorityOverrides map[string]int             `json:"asset_priority_overrides,omitempty"`
}

type agentRunCancelRequest struct {
	Reason string `json:"reason,omitempty"`
}

type agentRunToolDeclaration struct {
	Type        string                `json:"type,omitempty"`
	Name        string                `json:"name,omitempty"`
	Description string                `json:"description,omitempty"`
	Parameters  map[string]any        `json:"parameters,omitempty"`
	Function    *agentRunToolFunction `json:"function,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty"`
	Raw         map[string]any        `json:"-"`
}

type agentRunToolFunction struct {
	Name        string         `json:"name,omitempty"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

type agentRunResponse struct {
	RequestID        string                        `json:"request_id,omitempty"`
	RunID            string                        `json:"run_id,omitempty"`
	ResumedFromRunID string                        `json:"resumed_from_run_id,omitempty"`
	SessionID        string                        `json:"session_id,omitempty"`
	Status           string                        `json:"status"`
	StopReason       string                        `json:"stop_reason,omitempty"`
	Output           string                        `json:"output,omitempty"`
	ActionType       string                        `json:"action_type,omitempty"`
	Action           *runtime.Action               `json:"action,omitempty"`
	WaitState        *runtime.WaitState            `json:"wait_state,omitempty"`
	Error            string                        `json:"error,omitempty"`
	ErrorDetail      *runtime.ProtocolError        `json:"error_detail,omitempty"`
	Run              *runtimeRunDTO                `json:"run,omitempty"`
	TraceSummary     *agentRunTraceSummary         `json:"trace_summary,omitempty"`
	Checkpoints      []runtimeCheckpointReadoutDTO `json:"checkpoints,omitempty"`
	Messages         []agentRunMessageDTO          `json:"messages,omitempty"`
	ToolCalls        []agentRunToolCallDTO         `json:"tool_calls,omitempty"`
	ToolResults      []agentRunToolResultDTO       `json:"tool_results,omitempty"`
	TraceAvailable   bool                          `json:"trace_available"`
	Metadata         map[string]any                `json:"metadata,omitempty"`
}

type agentRunToolCallDTO struct {
	Index    int                     `json:"index"`
	ID       string                  `json:"id"`
	Type     string                  `json:"type"`
	Function agentRunFunctionCallDTO `json:"function"`
	Status   runtime.ToolCallStatus  `json:"status,omitempty"`
}

type agentRunFunctionCallDTO struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type agentRunToolResultDTO struct {
	Role       string                 `json:"role"`
	ToolCallID string                 `json:"tool_call_id"`
	Name       string                 `json:"name,omitempty"`
	Content    string                 `json:"content,omitempty"`
	Status     runtime.ToolCallStatus `json:"status,omitempty"`
	IsError    bool                   `json:"is_error,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

type agentRunMessageDTO struct {
	Role       string                 `json:"role"`
	Content    *string                `json:"content,omitempty"`
	ToolCalls  []agentRunToolCallDTO  `json:"tool_calls,omitempty"`
	ToolCallID string                 `json:"tool_call_id,omitempty"`
	Name       string                 `json:"name,omitempty"`
	Status     runtime.ToolCallStatus `json:"status,omitempty"`
	IsError    bool                   `json:"is_error,omitempty"`
	Error      string                 `json:"error,omitempty"`
}

type agentRunTraceReadout struct {
	Run         runtimeRunDTO                   `json:"run"`
	Steps       []runtimeStepDTO                `json:"steps"`
	Events      []runtimeLifecycleEventDTO      `json:"events"`
	Traces      []runtimeTraceDTO               `json:"traces"`
	Usage       []runtimeUsageDTO               `json:"usage"`
	Projections []runtimeProjectionCandidateDTO `json:"projections"`
	Checkpoints []runtimeCheckpointReadoutDTO   `json:"checkpoints"`
	Summary     agentRunTraceSummary            `json:"summary"`
}

type agentRunTraceSummary struct {
	RunID            string `json:"run_id,omitempty"`
	StepCount        int    `json:"step_count"`
	EventCount       int    `json:"event_count"`
	TraceCount       int    `json:"trace_count"`
	UsageCount       int    `json:"usage_count"`
	ProjectionCount  int    `json:"projection_count"`
	CheckpointCount  int    `json:"checkpoint_count"`
	LastEventType    string `json:"last_event_type,omitempty"`
	LastEventReason  string `json:"last_event_reason,omitempty"`
	LastTraceType    string `json:"last_trace_type,omitempty"`
	LastTraceSummary string `json:"last_trace_summary,omitempty"`
}

func handleCreateAgentRun(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	requestID := newRequestID()
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Runtime.RequestTimeoutSeconds)*time.Second)
	defer cancel()
	ctx = withRequestID(timeoutCtx, requestID)
	c.Header("X-Request-ID", requestID)

	req, err := parseAgentRunStartRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := applyAgentRunIdentityScope(ctx, &req); err != nil {
		writeAppAuthError(c, err)
		return
	}
	if strings.TrimSpace(agentRunGoal(req.Goal, req.Query)) == "" && req.Supplement == nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "goal is required unless supplement is provided"})
		return
	}
	response, status := executeAgentRun(ctx, c, cfg, application, requestID, req, agentRunExecutionOptions{Source: agentRunSourceCreate})
	c.JSON(status, response)
}

func handleResumeAgentRun(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	requestID := newRequestID()
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Runtime.RequestTimeoutSeconds)*time.Second)
	defer cancel()
	ctx = withRequestID(timeoutCtx, requestID)
	c.Header("X-Request-ID", requestID)

	originalRunID := strings.TrimSpace(c.Param("runID"))
	if originalRunID == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "run_id is required"})
		return
	}
	resumeReq, err := parseAgentRunResumeRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if strings.TrimSpace(agentRunGoal(resumeReq.Goal, resumeReq.Query)) == "" && resumeReq.Supplement == nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "goal is required unless supplement is provided"})
		return
	}
	originalReadout, err := readAgentRunTrace(ctx, application, originalRunID)
	if err != nil {
		writeAgentRunReadError(c, err)
		return
	}
	startReq := agentRunStartRequest{
		Goal:                   resumeReq.Goal,
		Query:                  resumeReq.Query,
		ContextAssets:          append([]contextassets.Asset(nil), resumeReq.ContextAssets...),
		SessionID:              resumeReq.SessionID,
		MainSessionID:          resumeReq.MainSessionID,
		ModelID:                resumeReq.ModelID,
		EnabledSkills:          append([]string(nil), resumeReq.EnabledSkills...),
		EnabledTools:           append([]string(nil), resumeReq.EnabledTools...),
		GlobalContext:          cloneAnyMap(resumeReq.GlobalContext),
		AppContext:             cloneAnyMap(resumeReq.AppContext),
		InputPayload:           cloneAnyMap(resumeReq.InputPayload),
		Supplement:             resumeReq.Supplement,
		SupplementOutcome:      resumeReq.SupplementOutcome,
		ResumeToken:            resumeReq.ResumeToken,
		TimeoutAfterSeconds:    resumeReq.TimeoutAfterSeconds,
		DisableFastPath:        resumeReq.DisableFastPath,
		PromptTemplate:         resumeReq.PromptTemplate,
		DisabledAssetTypes:     append([]string(nil), resumeReq.DisabledAssetTypes...),
		AssetPriorityOverrides: cloneIntMap(resumeReq.AssetPriorityOverrides),
		ResumedFromRunID:       originalRunID,
		WorkspaceID:            originalReadout.Run.WorkspaceID,
		AppInstanceID:          originalReadout.Run.AppInstanceID,
	}
	response, status := executeAgentRun(ctx, c, cfg, application, requestID, startReq, agentRunExecutionOptions{Source: agentRunSourceResume, ResumedFromRunID: originalRunID})
	c.JSON(status, response)
}

func handleGetAgentRun(ctx context.Context, c *hertzapp.RequestContext, _ config.Config, application *appcore.Service) {
	runID := strings.TrimSpace(c.Param("runID"))
	readout, err := readAgentRunTrace(ctx, application, runID)
	if err != nil {
		writeAgentRunReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, agentRunResponse{
		RunID:          runID,
		Status:         readout.Run.Status,
		StopReason:     agentRunStopReasonFromTrace(readout),
		Run:            &readout.Run,
		TraceSummary:   &readout.Summary,
		Checkpoints:    readout.Checkpoints,
		TraceAvailable: true,
	})
}

func handleCancelAgentRun(ctx context.Context, c *hertzapp.RequestContext, _ config.Config, application *appcore.Service) {
	runID := strings.TrimSpace(c.Param("runID"))
	if len(c.Request.Body()) > 0 {
		var req agentRunCancelRequest
		if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
			c.JSON(consts.StatusBadRequest, map[string]string{"error": fmt.Sprintf("invalid json body: %v", err)})
			return
		}
	}
	readout, err := readAgentRunTrace(ctx, application, runID)
	if err != nil {
		writeAgentRunReadError(c, err)
		return
	}
	status := agentRunCancelUnsupported
	stopReason := ""
	if agentRunIsTerminal(readout.Run.Status) {
		status = agentRunCancelAlreadyTerminal
		stopReason = agentRunStopReasonFromTrace(readout)
	}
	c.JSON(consts.StatusConflict, agentRunResponse{
		RunID:          runID,
		Status:         status,
		StopReason:     stopReason,
		Run:            &readout.Run,
		TraceSummary:   &readout.Summary,
		Checkpoints:    readout.Checkpoints,
		TraceAvailable: true,
		Metadata: map[string]any{
			"cancel_supported": false,
			"execution_mode":   "synchronous_mvp",
		},
	})
}

func handleGetAgentRunTrace(ctx context.Context, c *hertzapp.RequestContext, _ config.Config, application *appcore.Service) {
	readout, err := readAgentRunTrace(ctx, application, strings.TrimSpace(c.Param("runID")))
	if err != nil {
		writeAgentRunReadError(c, err)
		return
	}
	c.JSON(consts.StatusOK, readout)
}

type agentRunExecutionOptions struct {
	Source           string
	ResumedFromRunID string
}

func executeAgentRun(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service, requestID string, req agentRunStartRequest, opts agentRunExecutionOptions) (agentRunResponse, int) {
	if application == nil {
		return agentRunResponse{RequestID: requestID, Status: "failed", StopReason: string(runtime.ExecutionStopUnrecoverableError), Error: "application is not configured"}, consts.StatusInternalServerError
	}

	runtimeTuning, _ := application.GetControlPlaneRuntime(ctx)
	custom := customization.UserCustomization{
		PromptTemplate:         strings.TrimSpace(req.PromptTemplate),
		EnabledSkills:          compactAgentRunStrings(req.EnabledSkills),
		EnabledTools:           compactAgentRunStrings(append(req.EnabledTools, agentRunToolNames(req.Tools)...)),
		ContextAssetOverrides:  append([]contextassets.Asset(nil), req.ContextAssets...),
		DisabledAssetTypes:     compactAgentRunStrings(req.DisabledAssetTypes),
		AssetPriorityOverrides: cloneIntMap(req.AssetPriorityOverrides),
	}
	goal := strings.TrimSpace(agentRunGoal(req.Goal, req.Query))
	platformPrepared := preparePlatformContext(ctx, c, cfg, agentRunGlobalContext(req), platformcontext.UsageInput{
		Query:             goal,
		TaskType:          agentRunRuntimeTaskType(req),
		Scene:             agentRunDefaultString(strings.TrimSpace(req.Scene), defaultAgentRunScene),
		DesiredOutputMode: agentRunDefaultString(strings.TrimSpace(req.DesiredOutputMode), defaultAgentRunOutputMode),
	})
	contextAssetsPrepared := prepareContextAssets(ctx, application, platformPrepared.GlobalContext, custom, contextassets.UsageInput{
		Query:             goal,
		TaskType:          agentRunRuntimeTaskType(req),
		Scene:             agentRunDefaultString(strings.TrimSpace(req.Scene), defaultAgentRunScene),
		DesiredOutputMode: agentRunDefaultString(strings.TrimSpace(req.DesiredOutputMode), defaultAgentRunOutputMode),
	})
	req.GlobalContext = contextAssetsPrepared.GlobalContext
	chatSession, err := application.OpenChatSession(ctx, requestID, agentRunChatRequest(req, custom))
	if err != nil {
		return agentRunOpenErrorResponse(requestID, req, opts, err), agentRunOpenErrorStatus(err)
	}
	defer chatSession.Release()

	if chatSession.Prepared == nil {
		return agentRunResponse{RequestID: requestID, SessionID: chatSession.SessionID, Status: "failed", StopReason: string(runtime.ExecutionStopUnrecoverableError), Error: "prepared execution is missing"}, consts.StatusInternalServerError
	}

	prepared := chatSession.Prepared
	if prepared.Initial != nil && prepared.Initial.Action != nil {
		_ = chatSession.Complete(ctx, "")
		return agentRunResponseFromSession(ctx, application, requestID, chatSession, opts, agentRunPreparedStatus(prepared), "", "", nil, runtimeTuning), agentRunAcceptedStatus(prepared)
	}
	if prepared.Initial != nil && prepared.Initial.Error != "" {
		_ = chatSession.Complete(ctx, "")
		_ = prepared.ProjectTerminalOutcome(ctx, runtime.RuntimeTerminalOutcome{
			Status:     runtime.RuntimeTerminalStatusFailed,
			StopReason: runtime.NormalizeExecutionStopReason(string(prepared.InitialStatus), errors.New(prepared.Initial.Error), ""),
			Error:      errors.New(prepared.Initial.Error),
			Metadata: map[string]any{
				"agent_run_source": agentRunDefaultString(opts.Source, agentRunSourceCreate),
				"respond_stage":    "initial_error",
			},
		})
		return agentRunResponseFromSession(ctx, application, requestID, chatSession, opts, agentRunPreparedStatus(prepared), "", prepared.Initial.Error, prepared.InitialError, runtimeTuning), agentRunAcceptedStatus(prepared)
	}

	var (
		rawOutput       string
		toolSideEffects bool
	)
	if prepared.Initial != nil && strings.TrimSpace(prepared.Initial.Content) != "" {
		rawOutput = strings.TrimSpace(prepared.Initial.Content)
	} else {
		rawOutput, toolSideEffects, err = collectRespondOutput(ctx, prepared.Runner, prepared.Messages)
		if err != nil {
			projectionCtx, projectionCancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
			defer projectionCancel()
			_ = prepared.ProjectTerminalOutcome(projectionCtx, runtime.RuntimeTerminalOutcome{
				Status:          runtime.RuntimeTerminalStatusFailed,
				StopReason:      runtime.NormalizeExecutionStopReason(agentRunErrorStatus(err), err, ""),
				Error:           err,
				ToolSideEffects: toolSideEffects,
				Metadata: map[string]any{
					"agent_run_source": agentRunDefaultString(opts.Source, agentRunSourceCreate),
					"respond_stage":    "collect_output",
				},
			})
			return agentRunResponseFromSession(ctx, application, requestID, chatSession, opts, agentRunErrorStatus(err), "", err.Error(), nil, runtimeTuning), consts.StatusInternalServerError
		}
	}
	if err := chatSession.Complete(ctx, rawOutput); err != nil {
		_ = prepared.ProjectTerminalOutcome(ctx, runtime.RuntimeTerminalOutcome{
			Status:          runtime.RuntimeTerminalStatusFailed,
			Error:           err,
			ToolSideEffects: toolSideEffects,
			Metadata: map[string]any{
				"agent_run_source": agentRunDefaultString(opts.Source, agentRunSourceCreate),
				"respond_stage":    "complete_session",
			},
		})
		return agentRunResponseFromSession(ctx, application, requestID, chatSession, opts, "failed", rawOutput, err.Error(), nil, runtimeTuning), consts.StatusInternalServerError
	}
	if err := prepared.ProjectTerminalOutcome(ctx, runtime.RuntimeTerminalOutcome{
		Status:          runtime.RuntimeTerminalStatusCompleted,
		Content:         rawOutput,
		ToolSideEffects: toolSideEffects,
		Metadata: map[string]any{
			"agent_run_source": agentRunDefaultString(opts.Source, agentRunSourceCreate),
			"respond_stage":    "complete",
		},
	}); err != nil {
		return agentRunResponseFromSession(ctx, application, requestID, chatSession, opts, "failed", rawOutput, err.Error(), nil, runtimeTuning), consts.StatusInternalServerError
	}
	return agentRunResponseFromSession(ctx, application, requestID, chatSession, opts, "completed", rawOutput, "", nil, runtimeTuning), consts.StatusCreated
}

func agentRunResponseFromSession(ctx context.Context, application *appcore.Service, requestID string, chatSession *appcore.ChatSession, opts agentRunExecutionOptions, status string, output string, errText string, errDetail *runtime.ProtocolError, runtimeTuning controlplane.RuntimeTuning) agentRunResponse {
	response := agentRunResponse{
		RequestID:        requestID,
		ResumedFromRunID: strings.TrimSpace(opts.ResumedFromRunID),
		SessionID:        chatSession.SessionID,
		Status:           status,
		StopReason:       agentRunStopReason(status, errText),
		Output:           strings.TrimSpace(output),
		Error:            strings.TrimSpace(errText),
		ErrorDetail:      errDetail,
		TraceAvailable:   false,
		Metadata: map[string]any{
			"agent_run_source":       agentRunDefaultString(opts.Source, agentRunSourceCreate),
			"automation_fallback_on": runtimeTuning.AutomationFallbackEnabled,
		},
	}
	if chatSession.Prepared != nil && chatSession.Prepared.Initial != nil && chatSession.Prepared.Initial.Action != nil {
		response.Action = chatSession.Prepared.Initial.Action
		response.ActionType = string(chatSession.Prepared.Initial.Action.Type)
		response.WaitState = chatSession.Prepared.Initial.WaitState
		if response.WaitState == nil {
			response.WaitState = chatSession.Prepared.TimeoutWait
		}
		if response.StopReason == "" {
			response.StopReason = agentRunStopReason(string(chatSession.Prepared.InitialStatus), "")
		}
	}
	if chatSession.Prepared != nil && chatSession.Prepared.ToolTranscript != nil {
		calls := chatSession.Prepared.ToolTranscript.Snapshot()
		response.ToolCalls, response.ToolResults = agentRunToolTranscriptDTOs(calls)
		response.Messages = agentRunToolMessages(calls, response.Output)
	} else if strings.TrimSpace(response.Output) != "" {
		response.Messages = agentRunToolMessages(nil, response.Output)
	}
	runID := agentRunIDFromPrepared(chatSession.Prepared)
	if runID == "" {
		return response
	}
	response.RunID = runID
	readout, err := readAgentRunTrace(ctx, application, runID)
	if err != nil {
		response.Metadata["trace_read_error"] = err.Error()
		return response
	}
	response.Run = &readout.Run
	response.TraceSummary = &readout.Summary
	response.Checkpoints = readout.Checkpoints
	response.TraceAvailable = true
	if stopReason, authoritative := agentRunTerminalStopReasonFromTrace(readout); authoritative {
		response.StopReason = stopReason
	}
	return response
}

func agentRunToolTranscriptDTOs(calls []runtime.ToolCall) ([]agentRunToolCallDTO, []agentRunToolResultDTO) {
	toolCalls := make([]agentRunToolCallDTO, 0, len(calls))
	toolResults := make([]agentRunToolResultDTO, 0, len(calls))
	for _, call := range calls {
		toolCalls = append(toolCalls, agentRunToolCallDTO{
			Index:  call.Index,
			ID:     call.ID,
			Type:   agentRunDefaultString(strings.TrimSpace(call.Type), runtime.ToolTypeFunction),
			Status: call.Status,
			Function: agentRunFunctionCallDTO{
				Name:      call.Name,
				Arguments: call.Arguments,
			},
		})
		if call.Result == nil {
			continue
		}
		toolResults = append(toolResults, agentRunToolResultDTO{
			Role:       "tool",
			ToolCallID: call.Result.ToolCallID,
			Name:       call.Result.Name,
			Content:    call.Result.Content,
			Status:     call.Status,
			IsError:    call.Result.IsError,
			Error:      call.Result.Error,
		})
	}
	return toolCalls, toolResults
}

func agentRunToolMessages(calls []runtime.ToolCall, finalOutput string) []agentRunMessageDTO {
	messages := make([]agentRunMessageDTO, 0, len(calls)*2+1)
	for start := 0; start < len(calls); {
		round := calls[start].Round
		end := start + 1
		for end < len(calls) && calls[end].Round == round {
			end++
		}
		roundCalls, roundResults := agentRunToolTranscriptDTOs(calls[start:end])
		messages = append(messages, agentRunMessageDTO{
			Role:      "assistant",
			ToolCalls: roundCalls,
		})
		for _, result := range roundResults {
			content := result.Content
			messages = append(messages, agentRunMessageDTO{
				Role:       result.Role,
				Content:    &content,
				ToolCallID: result.ToolCallID,
				Name:       result.Name,
				Status:     result.Status,
				IsError:    result.IsError,
				Error:      result.Error,
			})
		}
		start = end
	}
	if strings.TrimSpace(finalOutput) != "" {
		content := finalOutput
		messages = append(messages, agentRunMessageDTO{Role: "assistant", Content: &content})
	}
	return messages
}

func readAgentRunTrace(ctx context.Context, application *appcore.Service, runID string) (agentRunTraceReadout, error) {
	if application == nil {
		return agentRunTraceReadout{}, fmt.Errorf("application is not configured")
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		return agentRunTraceReadout{}, fmt.Errorf("runtime run id is required")
	}
	run, ok, err := application.GetRuntimeRun(ctx, runID)
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	if !ok {
		if _, authenticated := appRequestIdentity(ctx); authenticated {
			return agentRunTraceReadout{}, errAgentRunNotFound
		}
		return agentRunTraceReadout{}, fmt.Errorf("runtime run %q not found", runID)
	}
	runDTO := runtimeRunDTOFromRuntime(run)
	if err := authorizeAgentRunForRequest(ctx, runDTO); err != nil {
		return agentRunTraceReadout{}, err
	}
	steps, err := application.ListRuntimeSteps(ctx, runID)
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	events, err := application.ListRuntimeLifecycleEvents(ctx, runID)
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	runDTO.Status = agentRunEffectiveStatus(runDTO.Status, runtimeLifecycleEventDTOs(events))
	traces, err := application.ListRuntimeTraces(ctx, appcore.RuntimeRecordReadQuery{RunID: runID, Limit: 200})
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	usage, err := application.ListRuntimeUsage(ctx, appcore.RuntimeRecordReadQuery{RunID: runID, Limit: 200})
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	projections, err := application.ListRuntimeProjectionCandidates(ctx, appcore.RuntimeRecordReadQuery{RunID: runID, Limit: 200})
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	checkpoints, err := application.ListRuntimeCheckpointReadouts(ctx, runID)
	if err != nil {
		return agentRunTraceReadout{}, err
	}
	readout := agentRunTraceReadout{
		Run:         runDTO,
		Steps:       runtimeStepDTOs(steps),
		Events:      runtimeLifecycleEventDTOs(events),
		Traces:      runtimeTraceDTOs(traces),
		Usage:       runtimeUsageDTOs(usage),
		Projections: runtimeProjectionCandidateDTOs(projections),
		Checkpoints: runtimeCheckpointReadoutDTOs(checkpoints),
	}
	if _, appFacing := appRequestIdentity(ctx); appFacing {
		for index := range readout.Projections {
			readout.Projections[index].SemanticPayload = nil
		}
	}
	readout.Summary = agentRunTraceSummaryFromReadout(readout)
	return readout, nil
}

func parseAgentRunStartRequest(c *hertzapp.RequestContext) (agentRunStartRequest, error) {
	var req agentRunStartRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) == 0 {
		return agentRunStartRequest{}, fmt.Errorf("request body is required")
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return agentRunStartRequest{}, fmt.Errorf("invalid json body: %w", err)
	}
	if strings.TrimSpace(req.ModelRecordID) != "" {
		return agentRunStartRequest{}, fmt.Errorf("model_record_id is no longer supported; use model_id")
	}
	canonicalTools, err := canonicalAgentRunTools(req.Tools)
	if err != nil {
		return agentRunStartRequest{}, err
	}
	canonicalToolChoice, err := canonicalAgentRunToolChoice(req.ToolChoice, canonicalTools)
	if err != nil {
		return agentRunStartRequest{}, err
	}
	req.CanonicalTools = canonicalTools
	req.CanonicalToolChoice = canonicalToolChoice
	normalizeAgentRunSupplement(&req)
	return req, nil
}

func parseAgentRunResumeRequest(c *hertzapp.RequestContext) (agentRunResumeRequest, error) {
	var req agentRunResumeRequest
	body := bytesTrimSpace(c.Request.Body())
	if len(body) == 0 {
		return agentRunResumeRequest{}, fmt.Errorf("request body is required")
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return agentRunResumeRequest{}, fmt.Errorf("invalid json body: %w", err)
	}
	start := agentRunStartRequest{
		Supplement:        req.Supplement,
		SupplementOutcome: req.SupplementOutcome,
		ResumeToken:       req.ResumeToken,
	}
	normalizeAgentRunSupplement(&start)
	req.Supplement = start.Supplement
	return req, nil
}

func normalizeAgentRunSupplement(req *agentRunStartRequest) {
	if req == nil {
		return
	}
	if req.Supplement == nil && req.SupplementOutcome != "" {
		req.Supplement = &runtime.SupplementPayload{}
	}
	if req.Supplement == nil && strings.TrimSpace(req.ResumeToken) != "" {
		req.Supplement = &runtime.SupplementPayload{}
	}
	if req.Supplement == nil {
		return
	}
	if req.Supplement.Outcome == "" && req.SupplementOutcome != "" {
		req.Supplement.Outcome = req.SupplementOutcome
	}
	if req.Supplement.Resume == nil && strings.TrimSpace(req.ResumeToken) != "" {
		req.Supplement.Resume = &runtime.ResumeContext{ResumeToken: strings.TrimSpace(req.ResumeToken)}
	}
	if req.Supplement.Resume != nil && strings.TrimSpace(req.Supplement.Resume.ResumeToken) == "" {
		req.Supplement.Resume.ResumeToken = strings.TrimSpace(req.ResumeToken)
	}
	if len(req.Supplement.Data) > 0 && req.Supplement.Outcome == "" {
		req.Supplement.Outcome = runtime.SupplementOutcomeProvided
	}
	if len(req.Supplement.Data) == 0 && req.Supplement.Outcome == "" && req.Supplement.Resume == nil {
		req.Supplement = nil
	}
}

func agentRunChatRequest(req agentRunStartRequest, custom customization.UserCustomization) appcore.ChatRequest {
	return appcore.ChatRequest{
		TaskType:              agentRunRuntimeTaskType(req),
		TaskSubtype:           agentRunDefaultString(strings.TrimSpace(req.TaskSubtype), defaultAgentRunTaskSubtype),
		Scene:                 agentRunDefaultString(strings.TrimSpace(req.Scene), defaultAgentRunScene),
		Query:                 strings.TrimSpace(agentRunGoal(req.Goal, req.Query)),
		SessionID:             strings.TrimSpace(req.SessionID),
		MainSessionID:         strings.TrimSpace(req.MainSessionID),
		WorkspaceID:           strings.TrimSpace(req.WorkspaceID),
		AppInstanceID:         strings.TrimSpace(req.AppInstanceID),
		AppSessionID:          strings.TrimSpace(req.AppSessionID),
		IntegrationInstanceID: strings.TrimSpace(req.IntegrationInstanceID),
		WorkflowRunID:         strings.TrimSpace(req.WorkflowRunID),
		StepID:                strings.TrimSpace(req.StepID),
		TriggerType:           strings.TrimSpace(req.TriggerType),
		AutomationTaskID:      strings.TrimSpace(req.AutomationTaskID),
		UserLanguage:          strings.TrimSpace(req.UserLanguage),
		DesiredOutputMode:     agentRunDefaultString(strings.TrimSpace(req.DesiredOutputMode), defaultAgentRunOutputMode),
		GlobalContext:         agentRunGlobalContext(req),
		AppContext:            agentRunAppContext(req),
		InputPayload:          agentRunInputPayload(req),
		ModelID:               strings.TrimSpace(req.ModelID),
		ToolDeclarations:      append([]runtime.ToolDefinition(nil), req.CanonicalTools...),
		Customization:         custom,
		Supplement:            req.Supplement,
		TimeoutAfter:          time.Duration(req.TimeoutAfterSeconds) * time.Second,
		DisableFastPath:       req.DisableFastPath || len(req.CanonicalTools) > 0,
	}
}

func agentRunGlobalContext(req agentRunStartRequest) map[string]any {
	ctx := cloneAnyMap(req.GlobalContext)
	if ctx == nil {
		ctx = map[string]any{}
	}
	if len(req.ContextAssets) > 0 {
		ctx["context_assets"] = toAnySlice(contextassets.AssetMaps(req.ContextAssets))
	}
	return ctx
}

func agentRunAppContext(req agentRunStartRequest) map[string]any {
	ctx := cloneAnyMap(req.AppContext)
	if ctx == nil {
		ctx = map[string]any{}
	}
	ctx["agent_run"] = agentRunContractMap(req)
	return ctx
}

func agentRunInputPayload(req agentRunStartRequest) map[string]any {
	payload := cloneAnyMap(req.InputPayload)
	if payload == nil {
		payload = map[string]any{}
	}
	payload["agent_run"] = agentRunContractMap(req)
	if override := agentRunToolChoiceOverride(req.CanonicalToolChoice); len(override) > 0 {
		payload["model_policy_override"] = override
	}
	return payload
}

func agentRunContractMap(req agentRunStartRequest) map[string]any {
	contract := map[string]any{
		"goal":                strings.TrimSpace(agentRunGoal(req.Goal, req.Query)),
		"success_criteria":    compactAgentRunStrings(req.SuccessCriteria),
		"constraints":         cloneAnyMap(req.Constraints),
		"budget":              cloneAnyMap(req.Budget),
		"tools":               agentRunToolMaps(req.Tools),
		"tool_choice":         req.ToolChoice,
		"memory_scope":        cloneAnyMap(req.MemoryScope),
		"governance_refs":     compactAgentRunStrings(req.GovernanceRefs),
		"requested_task_type": strings.TrimSpace(req.TaskType),
		"runtime_task_type":   agentRunRuntimeTaskType(req),
		"contract_version":    "agent_run.v1",
	}
	if strings.TrimSpace(req.ResumedFromRunID) != "" {
		contract["resumed_from_run_id"] = strings.TrimSpace(req.ResumedFromRunID)
	}
	return contract
}

func (d *agentRunToolDeclaration) UnmarshalJSON(payload []byte) error {
	var name string
	if err := json.Unmarshal(payload, &name); err == nil {
		d.Type = "function"
		d.Name = strings.TrimSpace(name)
		d.Function = &agentRunToolFunction{Name: strings.TrimSpace(name)}
		return nil
	}
	type alias agentRunToolDeclaration
	var item alias
	if err := json.Unmarshal(payload, &item); err != nil {
		return err
	}
	*d = agentRunToolDeclaration(item)
	var raw map[string]any
	if err := json.Unmarshal(payload, &raw); err == nil {
		d.Raw = raw
	}
	if d.Name == "" && d.Function != nil {
		d.Name = strings.TrimSpace(d.Function.Name)
	}
	if d.Type == "" && d.Function != nil {
		d.Type = "function"
	}
	return nil
}

func canonicalAgentRunTools(declarations []agentRunToolDeclaration) ([]runtime.ToolDefinition, error) {
	canonical := make([]runtime.ToolDefinition, 0, len(declarations))
	seen := make(map[string]struct{}, len(declarations))
	for _, declaration := range declarations {
		callType := strings.TrimSpace(declaration.Type)
		if callType == "" {
			callType = runtime.ToolTypeFunction
		}
		if callType != runtime.ToolTypeFunction {
			return nil, fmt.Errorf("tools only support type %q", runtime.ToolTypeFunction)
		}
		function := declaration.Function
		if function == nil {
			function = &agentRunToolFunction{
				Name:        declaration.Name,
				Description: declaration.Description,
				Parameters:  declaration.Parameters,
			}
		}
		name := strings.TrimSpace(function.Name)
		if err := validateAgentRunToolName(name); err != nil {
			return nil, err
		}
		if _, exists := seen[name]; exists {
			return nil, fmt.Errorf("tool function name %q is duplicated", name)
		}
		if err := validateAgentRunToolParameters(name, function.Parameters); err != nil {
			return nil, err
		}
		seen[name] = struct{}{}
		canonical = append(canonical, runtime.ToolDefinition{
			Type: runtime.ToolTypeFunction,
			Function: runtime.ToolFunctionDefinition{
				Name:        name,
				Description: strings.TrimSpace(function.Description),
				Parameters:  cloneAnyMap(function.Parameters),
			},
			Metadata: cloneAnyMap(declaration.Metadata),
		})
	}
	return canonical, nil
}

func canonicalAgentRunToolChoice(raw any, tools []runtime.ToolDefinition) (modelparams.ToolChoice, error) {
	choice := modelparams.ToolChoice{Kind: modelparams.ToolChoiceAuto}
	if raw == nil {
		if len(tools) == 0 {
			choice.Kind = modelparams.ToolChoiceNone
		}
		return choice, nil
	}
	if value, ok := raw.(string); ok {
		switch strings.TrimSpace(value) {
		case "none":
			choice.Kind = modelparams.ToolChoiceNone
		case "auto":
			choice.Kind = modelparams.ToolChoiceAuto
		case "required":
			choice.Kind = modelparams.ToolChoiceRequired
		default:
			return modelparams.ToolChoice{}, fmt.Errorf("tool_choice must be none, auto, required, or a function object")
		}
	} else {
		payload, ok := raw.(map[string]any)
		if !ok {
			return modelparams.ToolChoice{}, fmt.Errorf("tool_choice must be a string or object")
		}
		if callType := strings.TrimSpace(agentRunStringValue(payload["type"])); callType != runtime.ToolTypeFunction {
			return modelparams.ToolChoice{}, fmt.Errorf("tool_choice object type must be %q", runtime.ToolTypeFunction)
		}
		function, ok := payload["function"].(map[string]any)
		if !ok {
			return modelparams.ToolChoice{}, fmt.Errorf("tool_choice function object is required")
		}
		name := strings.TrimSpace(agentRunStringValue(function["name"]))
		if err := validateAgentRunToolName(name); err != nil {
			return modelparams.ToolChoice{}, fmt.Errorf("tool_choice: %w", err)
		}
		choice = modelparams.ToolChoice{Kind: modelparams.ToolChoiceSpecificTool, ToolName: name}
	}
	if choice.Kind == modelparams.ToolChoiceRequired && len(tools) == 0 {
		return modelparams.ToolChoice{}, fmt.Errorf("tool_choice=required requires at least one tool")
	}
	if choice.Kind == modelparams.ToolChoiceSpecificTool && !agentRunHasCanonicalTool(tools, choice.ToolName) {
		return modelparams.ToolChoice{}, fmt.Errorf("tool_choice function %q is not declared in tools", choice.ToolName)
	}
	return choice, nil
}

func agentRunToolChoiceOverride(choice modelparams.ToolChoice) map[string]any {
	switch choice.Kind {
	case modelparams.ToolChoiceNone:
		return map[string]any{"tool_policy": string(modelparams.ToolPolicyIntentNone)}
	case modelparams.ToolChoiceRequired:
		return map[string]any{"tool_policy": string(modelparams.ToolPolicyIntentRequired)}
	case modelparams.ToolChoiceSpecificTool:
		return map[string]any{
			"tool_policy": string(modelparams.ToolPolicyIntentSpecificTool),
			"tool_name":   strings.TrimSpace(choice.ToolName),
		}
	case modelparams.ToolChoiceAuto:
		return map[string]any{"tool_policy": string(modelparams.ToolPolicyIntentAuto)}
	default:
		return nil
	}
}

func validateAgentRunToolName(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("tool function name is required")
	}
	if len(name) > 64 {
		return fmt.Errorf("tool function name %q exceeds 64 characters", name)
	}
	for _, char := range name {
		if (char >= 'a' && char <= 'z') || (char >= 'A' && char <= 'Z') || (char >= '0' && char <= '9') || char == '_' || char == '-' {
			continue
		}
		return fmt.Errorf("tool function name %q contains unsupported characters", name)
	}
	return nil
}

func validateAgentRunToolParameters(name string, parameters map[string]any) error {
	if len(parameters) == 0 {
		return nil
	}
	schemaType := strings.TrimSpace(agentRunStringValue(parameters["type"]))
	if schemaType != "object" {
		return fmt.Errorf("tool function %q parameters must use an object JSON schema", name)
	}
	if properties, exists := parameters["properties"]; exists {
		if _, ok := properties.(map[string]any); !ok {
			return fmt.Errorf("tool function %q parameters.properties must be an object", name)
		}
	}
	return nil
}

func agentRunHasCanonicalTool(tools []runtime.ToolDefinition, name string) bool {
	for _, tool := range tools {
		if strings.TrimSpace(tool.Function.Name) == strings.TrimSpace(name) {
			return true
		}
	}
	return false
}

func agentRunStringValue(value any) string {
	typed, _ := value.(string)
	return typed
}

func agentRunToolNames(tools []agentRunToolDeclaration) []string {
	names := make([]string, 0, len(tools))
	for _, tool := range tools {
		name := strings.TrimSpace(tool.Name)
		if name == "" && tool.Function != nil {
			name = strings.TrimSpace(tool.Function.Name)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return compactAgentRunStrings(names)
}

func agentRunToolMaps(tools []agentRunToolDeclaration) []map[string]any {
	out := make([]map[string]any, 0, len(tools))
	for _, tool := range tools {
		if len(tool.Raw) > 0 {
			out = append(out, cloneAnyMap(tool.Raw))
			continue
		}
		item := map[string]any{}
		if strings.TrimSpace(tool.Type) != "" {
			item["type"] = strings.TrimSpace(tool.Type)
		}
		if strings.TrimSpace(tool.Name) != "" {
			item["name"] = strings.TrimSpace(tool.Name)
		}
		if strings.TrimSpace(tool.Description) != "" {
			item["description"] = strings.TrimSpace(tool.Description)
		}
		if len(tool.Parameters) > 0 {
			item["parameters"] = cloneAnyMap(tool.Parameters)
		}
		if tool.Function != nil {
			item["function"] = map[string]any{
				"name":        strings.TrimSpace(tool.Function.Name),
				"description": strings.TrimSpace(tool.Function.Description),
				"parameters":  cloneAnyMap(tool.Function.Parameters),
			}
		}
		if len(tool.Metadata) > 0 {
			item["metadata"] = cloneAnyMap(tool.Metadata)
		}
		if len(item) > 0 {
			out = append(out, item)
		}
	}
	return out
}

func agentRunGoal(goal string, query string) string {
	if strings.TrimSpace(goal) != "" {
		return strings.TrimSpace(goal)
	}
	return strings.TrimSpace(query)
}

func agentRunRuntimeTaskType(req agentRunStartRequest) string {
	taskType := strings.TrimSpace(req.TaskType)
	if taskType == "" || taskType == agentRunContractTaskType {
		return defaultAgentRunRuntimeTaskType
	}
	return taskType
}

func agentRunIDFromPrepared(prepared *runtime.PreparedExecution) string {
	if prepared == nil || prepared.RuntimeRecords == nil {
		return ""
	}
	return strings.TrimSpace(prepared.RuntimeRecords.Run.ID)
}

func agentRunPreparedStatus(prepared *runtime.PreparedExecution) string {
	if prepared == nil {
		return "unknown"
	}
	if prepared.InitialStatus != "" {
		return string(prepared.InitialStatus)
	}
	return "running"
}

func agentRunAcceptedStatus(prepared *runtime.PreparedExecution) int {
	if agentRunIDFromPrepared(prepared) == "" {
		return consts.StatusOK
	}
	return consts.StatusCreated
}

func agentRunTraceSummaryFromReadout(readout agentRunTraceReadout) agentRunTraceSummary {
	summary := agentRunTraceSummary{
		RunID:           readout.Run.ID,
		StepCount:       len(readout.Steps),
		EventCount:      len(readout.Events),
		TraceCount:      len(readout.Traces),
		UsageCount:      len(readout.Usage),
		ProjectionCount: len(readout.Projections),
		CheckpointCount: len(readout.Checkpoints),
	}
	if len(readout.Events) > 0 {
		last := readout.Events[len(readout.Events)-1]
		summary.LastEventType = last.EventType
		summary.LastEventReason = last.Reason
	}
	if len(readout.Traces) > 0 {
		last := readout.Traces[len(readout.Traces)-1]
		summary.LastTraceType = last.TraceType
		summary.LastTraceSummary = last.Summary
	}
	return summary
}

func agentRunStopReasonFromTrace(readout agentRunTraceReadout) string {
	if reason, ok := agentRunTerminalStopReasonFromTrace(readout); ok {
		return reason
	}
	return agentRunStopReason(readout.Run.Status, "")
}

func agentRunTerminalStopReasonFromTrace(readout agentRunTraceReadout) (string, bool) {
	for idx := len(readout.Events) - 1; idx >= 0; idx-- {
		event := readout.Events[idx]
		if event.SubjectType != runtime.LifecycleSubjectRun || event.EventType != "run_terminal_observed" {
			continue
		}
		if reason, ok := runtime.ParseExecutionStopReason(event.Reason); ok {
			return string(reason), true
		}
	}
	return "", false
}

func agentRunEffectiveStatus(persisted string, events []runtimeLifecycleEventDTO) string {
	for idx := len(events) - 1; idx >= 0; idx-- {
		event := events[idx]
		if event.SubjectType == runtime.LifecycleSubjectRun && event.EventType == "run_terminal_observed" && agentRunIsTerminal(event.ToStatus) {
			return event.ToStatus
		}
	}
	for idx := len(events) - 1; idx >= 0; idx-- {
		event := events[idx]
		if event.SubjectType != runtime.LifecycleSubjectRun || strings.TrimSpace(event.ToStatus) == "" {
			continue
		}
		if event.EventType == "run_completed" && event.Reason == "minimal_persistence_complete" {
			continue
		}
		switch event.ToStatus {
		case runtime.TaskRunStatusCreated,
			runtime.TaskRunStatusRunning,
			runtime.TaskRunStatusWaiting,
			runtime.TaskRunStatusResumed,
			runtime.TaskRunStatusCompleted,
			runtime.TaskRunStatusFailed,
			runtime.TaskRunStatusCancelled:
			return event.ToStatus
		}
	}
	return persisted
}

func agentRunStopReason(status string, errText string) string {
	var err error
	if strings.TrimSpace(errText) != "" {
		err = errors.New("execution failed")
	}
	return string(runtime.NormalizeExecutionStopReason(status, err, ""))
}

func agentRunErrorStatus(err error) string {
	if errors.Is(err, context.DeadlineExceeded) {
		return string(runtime.RequestStatusTimedOut)
	}
	if errors.Is(err, context.Canceled) {
		return runtime.TaskRunStatusCancelled
	}
	return runtime.RuntimeTerminalStatusFailed
}

func agentRunIsTerminal(status string) bool {
	switch strings.TrimSpace(status) {
	case runtime.TaskRunStatusCompleted, runtime.TaskRunStatusFailed, runtime.TaskRunStatusCancelled:
		return true
	default:
		return false
	}
}

func agentRunOpenErrorResponse(requestID string, req agentRunStartRequest, opts agentRunExecutionOptions, err error) agentRunResponse {
	response := agentRunResponse{
		RequestID:        requestID,
		ResumedFromRunID: strings.TrimSpace(opts.ResumedFromRunID),
		SessionID:        strings.TrimSpace(req.SessionID),
		Status:           "failed",
		StopReason:       agentRunStopReason("failed", err.Error()),
		Error:            err.Error(),
		TraceAvailable:   false,
	}
	var invalidTokenErr *appcore.InvalidResumeTokenError
	if errors.As(err, &invalidTokenErr) {
		response.Status = string(runtime.RequestStatusInvalidResumeToken)
		response.StopReason = string(runtime.ExecutionStopUnrecoverableError)
		response.SessionID = invalidTokenErr.SessionID
		response.ErrorDetail = &runtime.ProtocolError{
			Code:      string(runtime.RequestStatusInvalidResumeToken),
			Reason:    invalidTokenErr.Reason,
			Retryable: false,
			Detail: map[string]any{
				"resume_token": invalidTokenErr.ResumeToken,
			},
		}
	}
	var invalidTaskErr *appcore.InvalidTaskRequestError
	if errors.As(err, &invalidTaskErr) {
		response.StopReason = string(runtime.ExecutionStopUnrecoverableError)
		response.Metadata = map[string]any{
			"task_type":      invalidTaskErr.TaskType,
			"missing_fields": invalidTaskErr.MissingFields,
		}
	}
	var pendingErr *appcore.PendingWaitError
	if errors.As(err, &pendingErr) {
		response.Status = string(runtime.RequestStatusWaitingForInformation)
		response.StopReason = string(runtime.ExecutionStopAwaitingInput)
		response.Metadata = map[string]any{
			"pending": pendingErr.Pending != nil,
			"queued":  pendingErr.Queued != nil,
			"dropped": pendingErr.Dropped != nil,
		}
	}
	if errors.Is(err, context.DeadlineExceeded) {
		response.StopReason = string(runtime.ExecutionStopDeadlineExceeded)
	}
	if errors.Is(err, context.Canceled) {
		response.StopReason = string(runtime.ExecutionStopCancelled)
	}
	return response
}

func agentRunOpenErrorStatus(err error) int {
	var invalidTaskErr *appcore.InvalidTaskRequestError
	if errors.As(err, &invalidTaskErr) {
		return consts.StatusBadRequest
	}
	var invalidTokenErr *appcore.InvalidResumeTokenError
	if errors.As(err, &invalidTokenErr) {
		return consts.StatusOK
	}
	var pendingErr *appcore.PendingWaitError
	if errors.As(err, &pendingErr) {
		return consts.StatusOK
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return consts.StatusTooManyRequests
	}
	return consts.StatusInternalServerError
}

func writeAgentRunReadError(c *hertzapp.RequestContext, err error) {
	if errors.Is(err, appcore.ErrRuntimeStoreNotConfigured) {
		c.JSON(consts.StatusServiceUnavailable, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, errAgentRunNotFound) {
		c.JSON(consts.StatusNotFound, map[string]string{"error": "resource_not_found"})
		return
	}
	if strings.Contains(err.Error(), "not found") {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	if strings.Contains(err.Error(), "required") {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
}

func compactAgentRunStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		result = append(result, trimmed)
	}
	return result
}

func agentRunDefaultString(value string, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}
