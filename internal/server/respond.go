// respond.go implements direct respond adapters on top of the common app/runtime path.
// respond.go 负责在通用 app/runtime 路径之上实现直接响应适配器。
//
// New delivery, projection, or orchestration modes should be added behind app/runtime
// boundaries and Eino graph nodes, not by growing this transport adapter.
// 新的交付、投影或编排模式应放到 app/runtime 边界和 Eino graph node 后面，不继续堆进这个 transport adapter。
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	einomessage "github.com/cloudwego/eino/schema"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/controlplane"
	"moss/internal/customization"
	platformcontext "moss/internal/extensions/platform/context"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	"moss/internal/runtime"
	"moss/internal/session"
)

const (
	defaultSchemaRetryCount = 2
	maxSchemaRetryCount     = 5
)

var fencedBlockPattern = regexp.MustCompile("(?s)^```(?:json|markdown)?\\s*(.*?)\\s*```$")
var jsonObjectPattern = regexp.MustCompile(`(?s)\{.*\}`)
var formattingRetryFunc = formattingRetry
var resolveStructuredResultFunc = resolveStructuredResultWithControlPlane

type structuredChatResult = appcore.DirectRespondRichResult

type schemaValidationReport struct {
	Strict              bool   `json:"strict"`
	RepairMode          string `json:"repair_mode,omitempty"`
	RetryCount          int    `json:"retry_count,omitempty"`
	RetriesUsed         int    `json:"retries_used,omitempty"`
	RepairAttempted     bool   `json:"repair_attempted,omitempty"`
	RepairSucceeded     bool   `json:"repair_succeeded,omitempty"`
	RegexFallbackUsed   bool   `json:"regex_fallback_used,omitempty"`
	Valid               bool   `json:"valid"`
	FailureStage        string `json:"failure_stage,omitempty"`
	LastValidationError string `json:"last_validation_error,omitempty"`
}

type chatRespondEnvelope struct {
	RequestID        string                            `json:"request_id,omitempty"`
	SessionID        string                            `json:"session_id,omitempty"`
	Status           string                            `json:"status,omitempty"`
	Result           *structuredChatResult             `json:"result,omitempty"`
	ActionType       string                            `json:"action_type,omitempty"`
	Action           *runtime.Action                   `json:"action,omitempty"`
	WaitState        *runtime.WaitState                `json:"wait_state,omitempty"`
	Error            string                            `json:"error,omitempty"`
	ErrorDetail      *runtime.ProtocolError            `json:"error_detail,omitempty"`
	StructuredOutput *runtime.StructuredOutputContract `json:"structured_output,omitempty"`
	SchemaValidation schemaValidationReport            `json:"schema_validation"`
	Detail           map[string]any                    `json:"detail,omitempty"`
}

func handleChatRespond(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	timeoutCtx, cancel := context.WithTimeout(ctx, time.Duration(cfg.Runtime.RequestTimeoutSeconds)*time.Second)
	defer cancel()

	requestID := newRequestID()
	var (
		req            ChatRespondRequest
		runtimeTuning  = controlplane.DefaultRuntimeTuning()
		prepared       *runtime.PreparedExecution
		currentSession *session.Session
		sessionID      string
	)
	defer func() {
		if recovered := recover(); recovered != nil {
			writeRecoveredRespondFailure(c, application, runtimeTuning, prepared, req, requestID, sessionID, cfg.Runtime.SharedRootDir, currentSession, recovered)
		}
	}()
	ctx = withRequestID(timeoutCtx, requestID)
	c.Header("X-Request-ID", requestID)

	var err error
	req, err = parseChatRespondRequest(c)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	query := strings.TrimSpace(req.Query)
	if query == "" && req.Supplement == nil && strings.TrimSpace(req.TaskType) == "" {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "query is required unless supplement is provided"})
		return
	}

	custom := customization.UserCustomization{
		PromptTemplate:         buildStructuredRespondPrompt(req.PromptTemplate),
		EnabledSkills:          req.EnabledSkills,
		EnabledTools:           req.EnabledTools,
		ContextAssetOverrides:  req.ContextAssetOverrides,
		DisabledAssetTypes:     req.DisabledAssetTypes,
		AssetPriorityOverrides: req.AssetPriorityOverrides,
	}
	runtimeTuning, _ = application.GetControlPlaneRuntime(ctx)
	platformPrepared := preparePlatformContext(ctx, c, cfg, req.GlobalContext, platformcontext.UsageInput{
		Query:             query,
		TaskType:          strings.TrimSpace(req.TaskType),
		Scene:             strings.TrimSpace(req.Scene),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
	})
	contextAssetsPrepared := prepareContextAssets(ctx, application, platformPrepared.GlobalContext, custom, contextassets.UsageInput{
		Query:             query,
		TaskType:          strings.TrimSpace(req.TaskType),
		Scene:             strings.TrimSpace(req.Scene),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
	})
	req.GlobalContext = contextAssetsPrepared.GlobalContext

	chatSession, err := application.OpenChatSession(ctx, requestID, appcore.ChatRequest{
		TaskType:              strings.TrimSpace(req.TaskType),
		TaskSubtype:           strings.TrimSpace(req.TaskSubtype),
		Scene:                 strings.TrimSpace(req.Scene),
		Query:                 query,
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
		DesiredOutputMode:     strings.TrimSpace(req.DesiredOutputMode),
		GlobalContext:         cloneAnyMap(req.GlobalContext),
		AppContext:            cloneAnyMap(req.AppContext),
		InputPayload:          cloneAnyMap(req.InputPayload),
		ModelID:               strings.TrimSpace(req.ModelID),
		Customization:         custom,
		Supplement:            req.Supplement,
		TimeoutAfter:          time.Duration(req.TimeoutAfterSeconds) * time.Second,
		DisableFastPath:       req.DisableFastPath,
	})
	if err != nil {
		var invalidTaskErr *appcore.InvalidTaskRequestError
		if errors.As(err, &invalidTaskErr) {
			c.JSON(consts.StatusBadRequest, map[string]any{
				"error":          invalidTaskErr.Error(),
				"task_type":      invalidTaskErr.TaskType,
				"reason":         invalidTaskErr.Reason,
				"missing_fields": invalidTaskErr.MissingFields,
			})
			return
		}
		if shouldBuildAutomationPlanDraft(req) && runtimeTuning.AutomationFallbackEnabled {
			c.JSON(consts.StatusOK, buildAutomationDraftFallbackEnvelopeWithControlPlane(application, runtimeTuning, nil, req, requestID, strings.TrimSpace(req.SessionID), cfg.Runtime.SharedRootDir, nil, fmt.Sprintf("open_chat_session_failed: %v", err)))
			return
		}
		respondWithSessionError(c, requestID, strings.TrimSpace(req.SessionID), err)
		return
	}
	defer chatSession.Release()
	prepared = chatSession.Prepared
	currentSession = chatSession.Session
	sessionID = chatSession.SessionID

	c.Header("X-Session-ID", chatSession.SessionID)

	if chatSession.Prepared.Initial != nil && chatSession.Prepared.Initial.Action != nil {
		c.JSON(consts.StatusOK, chatRespondEnvelope{
			RequestID:        requestID,
			SessionID:        chatSession.SessionID,
			Status:           string(chatSession.Prepared.InitialStatus),
			ActionType:       string(chatSession.Prepared.Initial.Action.Type),
			Action:           chatSession.Prepared.Initial.Action,
			WaitState:        chatSession.Prepared.Initial.WaitState,
			StructuredOutput: structuredOutputOrNil(chatSession.Prepared),
			SchemaValidation: schemaValidationReport{Strict: req.StrictSchemaValidation, RepairMode: normalizeSchemaRepairMode(req.SchemaRepairMode), RetryCount: effectiveSchemaRetryCount(req)},
			Detail: map[string]any{
				"delivery_mode": "structured_result",
			},
		})
		return
	}
	if chatSession.Prepared.Initial != nil && chatSession.Prepared.Initial.Error != "" {
		c.JSON(consts.StatusOK, chatRespondEnvelope{
			RequestID:        requestID,
			SessionID:        chatSession.SessionID,
			Status:           string(chatSession.Prepared.InitialStatus),
			Error:            chatSession.Prepared.Initial.Error,
			ErrorDetail:      chatSession.Prepared.InitialError,
			StructuredOutput: structuredOutputOrNil(chatSession.Prepared),
			SchemaValidation: schemaValidationReport{Strict: req.StrictSchemaValidation, RepairMode: normalizeSchemaRepairMode(req.SchemaRepairMode), RetryCount: effectiveSchemaRetryCount(req), Valid: false, FailureStage: "initial_error", LastValidationError: chatSession.Prepared.Initial.Error},
		})
		return
	}

	var (
		rawOutput       string
		toolSideEffects bool
	)
	if chatSession.Prepared.Initial != nil && strings.TrimSpace(chatSession.Prepared.Initial.Content) != "" {
		rawOutput = strings.TrimSpace(chatSession.Prepared.Initial.Content)
	} else {
		rawOutput, toolSideEffects, err = collectRespondOutput(ctx, chatSession.Prepared.Runner, chatSession.Prepared.Messages)
		if err != nil {
			_ = chatSession.Prepared.ProjectTerminalOutcome(ctx, runtime.RuntimeTerminalOutcome{
				Status:          runtime.RuntimeTerminalStatusFailed,
				Error:           err,
				ToolSideEffects: toolSideEffects,
				Metadata:        map[string]any{"respond_stage": "collect_output"},
			})
			if shouldBuildAutomationPlanDraft(req) && runtimeTuning.AutomationFallbackEnabled {
				c.JSON(consts.StatusOK, buildAutomationDraftFallbackEnvelopeWithControlPlane(application, runtimeTuning, chatSession.Prepared, req, requestID, chatSession.SessionID, cfg.Runtime.SharedRootDir, chatSession.Session, fmt.Sprintf("collect_respond_output_failed: %v", err)))
				return
			}
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
	}
	if shouldBuildAutomationPlanDraft(req) && runtimeTuning.AutomationFallbackEnabled && strings.TrimSpace(rawOutput) == "" {
		c.JSON(consts.StatusOK, buildAutomationDraftFallbackEnvelopeWithControlPlane(application, runtimeTuning, chatSession.Prepared, req, requestID, chatSession.SessionID, cfg.Runtime.SharedRootDir, chatSession.Session, "empty_model_output"))
		return
	}
	if err := chatSession.Complete(ctx, rawOutput); err != nil {
		if shouldBuildAutomationPlanDraft(req) && runtimeTuning.AutomationFallbackEnabled {
			c.JSON(consts.StatusOK, buildAutomationDraftFallbackEnvelopeWithControlPlane(application, runtimeTuning, chatSession.Prepared, req, requestID, chatSession.SessionID, cfg.Runtime.SharedRootDir, chatSession.Session, fmt.Sprintf("complete_failed: %v", err)))
			return
		}
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := chatSession.Prepared.ProjectTerminalOutcome(ctx, runtime.RuntimeTerminalOutcome{
		Status:          runtime.RuntimeTerminalStatusCompleted,
		Content:         rawOutput,
		ToolSideEffects: toolSideEffects,
		Metadata:        map[string]any{"respond_stage": "complete"},
	}); err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	result, report, failureErr, failureDetail := resolveStructuredResultFunc(ctx, application, runtimeTuning, chatSession.Prepared, rawOutput, req, requestID, chatSession.SessionID, toolSideEffects, cfg.Runtime.SharedRootDir, chatSession.Session)
	if failureErr != nil && req.StrictSchemaValidation {
		if normalizeSchemaFailureAction(req.SchemaFailureAction) == "partial" && result != nil {
			c.JSON(consts.StatusOK, chatRespondEnvelope{
				RequestID:        requestID,
				SessionID:        chatSession.SessionID,
				Status:           "completed_with_partial_schema",
				Result:           result,
				StructuredOutput: structuredOutputOrNil(chatSession.Prepared),
				SchemaValidation: report,
				Detail:           failureDetail,
			})
			return
		}
		c.JSON(consts.StatusOK, chatRespondEnvelope{
			RequestID:        requestID,
			SessionID:        chatSession.SessionID,
			Status:           "schema_validation_failed",
			Error:            failureErr.Error(),
			StructuredOutput: structuredOutputOrNil(chatSession.Prepared),
			SchemaValidation: report,
			Detail:           failureDetail,
		})
		return
	}

	c.JSON(consts.StatusOK, chatRespondEnvelope{
		RequestID:        requestID,
		SessionID:        chatSession.SessionID,
		Status:           "completed",
		Result:           result,
		StructuredOutput: structuredOutputOrNil(chatSession.Prepared),
		SchemaValidation: report,
		Detail: map[string]any{
			"tool_side_effects": toolSideEffects,
			"delivery_mode":     "structured_result",
		},
	})
}

// writeRecoveredRespondFailure maps one recovered panic into either an automation fallback envelope or a generic 500 payload.
// writeRecoveredRespondFailure 负责把一次 recovered panic 映射成自动化 fallback 或通用 500 响应。
func writeRecoveredRespondFailure(c *hertzapp.RequestContext, application *appcore.Service, runtimeTuning controlplane.RuntimeTuning, prepared *runtime.PreparedExecution, req ChatRespondRequest, requestID, sessionID, sharedRootDir string, currentSession *session.Session, recovered any) {
	if c == nil {
		return
	}
	if shouldBuildAutomationPlanDraft(req) && runtimeTuning.AutomationFallbackEnabled {
		c.Response.Reset()
		c.JSON(consts.StatusOK, buildAutomationDraftFallbackEnvelopeWithControlPlane(application, runtimeTuning, prepared, req, requestID, sessionID, sharedRootDir, currentSession, fmt.Sprintf("panic_recovered: %v", recovered)))
		return
	}
	c.Response.Reset()
	c.JSON(consts.StatusInternalServerError, map[string]string{"error": fmt.Sprintf("panic recovered: %v", recovered)})
}

func buildAutomationDraftFallbackEnvelope(prepared *runtime.PreparedExecution, req ChatRespondRequest, requestID, sessionID, sharedRootDir string, currentSession *session.Session, fallbackReason string) chatRespondEnvelope {
	return buildAutomationDraftFallbackEnvelopeWithControlPlane(nil, controlplane.DefaultRuntimeTuning(), prepared, req, requestID, sessionID, sharedRootDir, currentSession, fallbackReason)
}

func buildAutomationDraftFallbackEnvelopeWithControlPlane(application *appcore.Service, runtimeTuning controlplane.RuntimeTuning, prepared *runtime.PreparedExecution, req ChatRespondRequest, requestID, sessionID, sharedRootDir string, currentSession *session.Session, fallbackReason string) chatRespondEnvelope {
	result, report, _, _ := resolveStructuredResultFunc(context.Background(), application, runtimeTuning, prepared, `{"main_answer":"自动化计划草案已生成。请先确认后再创建。 "}`, req, requestID, sessionID, false, sharedRootDir, currentSession)
	if result == nil {
		result = &structuredChatResult{
			MainAnswer:       "自动化计划草案已生成。请先确认后再创建。",
			Answer:           "自动化计划草案已生成。请先确认后再创建。",
			StructuredResult: map[string]any{},
		}
		enrichStructuredChatResult(context.Background(), application, runtimeTuning, result, prepared, req, requestID, sessionID, sharedRootDir, currentSession)
	}
	if result.StructuredResult == nil {
		result.StructuredResult = map[string]any{}
	}
	result.StructuredResult["fallback_reason"] = fallbackReason
	return chatRespondEnvelope{
		RequestID:        requestID,
		SessionID:        sessionID,
		Status:           "completed_with_fallback",
		Result:           result,
		StructuredOutput: structuredOutputOrNil(prepared),
		SchemaValidation: report,
		Detail: map[string]any{
			"fallback_reason": fallbackReason,
			"delivery_mode":   "structured_result",
		},
	}
}

func respondWithSessionError(c *hertzapp.RequestContext, requestID string, sessionID string, err error) {
	var pendingErr *appcore.PendingWaitError
	if errors.As(err, &pendingErr) {
		response := buildPendingWaitResponse(requestID, sessionID, pendingErr.Pending, pendingErr.Queued, pendingErr.Dropped)
		c.JSON(consts.StatusOK, response)
		return
	}
	var invalidTokenErr *appcore.InvalidResumeTokenError
	if errors.As(err, &invalidTokenErr) {
		c.JSON(consts.StatusOK, chatRespondEnvelope{
			RequestID: requestID,
			SessionID: invalidTokenErr.SessionID,
			Status:    string(runtime.RequestStatusInvalidResumeToken),
			Error:     invalidTokenErr.Error(),
			ErrorDetail: &runtime.ProtocolError{
				Code:         string(runtime.RequestStatusInvalidResumeToken),
				Reason:       invalidTokenErr.Reason,
				Retryable:    false,
				ClientAction: invalidResumeTokenClientAction(invalidTokenErr.Reason),
				Detail: map[string]any{
					"resume_token": invalidTokenErr.ResumeToken,
					"session_id":   invalidTokenErr.SessionID,
				},
			},
			SchemaValidation: schemaValidationReport{Valid: false, FailureStage: "invalid_resume_token", LastValidationError: invalidTokenErr.Error()},
		})
		return
	}
	status := consts.StatusInternalServerError
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		status = consts.StatusTooManyRequests
	}
	c.JSON(status, map[string]string{"error": err.Error()})
}

func buildPendingWaitResponse(requestID, sessionID string, pending *session.PendingState, queued *session.DeferredMessage, dropped *session.DeferredMessage) chatRespondEnvelope {
	waitState := &runtime.WaitState{
		Stage:        runtime.RuntimeStage(pending.Stage),
		StartedAt:    pending.TimeoutAt.Add(-pending.TimeoutAfter),
		TimeoutAt:    pending.TimeoutAt,
		TimeoutAfter: pending.TimeoutAfter,
		ResumeToken:  pending.ResumeToken,
	}
	action := &runtime.Action{
		Type:    runtime.ActionTypeInformationRequest,
		Code:    string(runtime.ActionTypeInformationRequest),
		Message: "the current analysis is waiting for supplemental information",
		Target:  runtime.SupplementTargetClient,
		Schema: &runtime.ActionSchema{
			Input: map[string]runtime.ActionSchemaField{
				"supplement.data":    {Type: "object"},
				"supplement_outcome": {Type: "string", Required: true, Enum: []string{"provided", "unable_to_provide", "timeout_expired", "abandon_and_continue", "pending_human"}},
				"resume_token":       {Type: "string", Required: true},
			},
		},
		InformationRequest: &runtime.InformationRequestAction{
			AllowDegrade:    true,
			SuggestedAction: "provide the requested supplemental data together with the matching resume_token, or explicitly send supplement_outcome if the data cannot be provided",
			Target:          runtime.SupplementTargetClient,
			WaitPolicy:      runtime.WaitTimeoutPolicy{TimeoutAfter: pending.TimeoutAfter},
		},
		Payload:        map[string]any{"missing_fields": pending.MissingFields},
		TimeoutPolicy:  &runtime.WaitTimeoutPolicy{TimeoutAfter: pending.TimeoutAfter},
		ExpectedResult: &runtime.ActionExpectedResult{ResumeTokenRequired: true},
	}
	for _, field := range pending.MissingFields {
		action.InformationRequest.Missing = append(action.InformationRequest.Missing, runtime.MissingInformationItem{
			Field:    field,
			Reason:   "the current analysis stage is paused until this missing information is resolved",
			Impact:   "the pending gap cannot be closed and the analysis chain cannot continue without an explicit resume outcome",
			Required: true,
		})
	}
	detail := map[string]any{
		"accepted":                 false,
		"blocked_by_pending_wait":  true,
		"resume_required":          true,
		"pending_action_type":      pending.ActionType,
		"pending_status":           pending.Status,
		"received_but_not_applied": true,
		"queued_for_follow_up":     queued != nil,
	}
	if dropped != nil {
		detail["queue_overflow"] = map[string]any{
			"dropped_oldest": true,
			"query":          dropped.Query,
			"received_at":    dropped.ReceivedAt,
		}
	}
	return chatRespondEnvelope{
		RequestID:        requestID,
		SessionID:        sessionID,
		Status:           string(runtime.RequestStatusWaitingForInformation),
		ActionType:       string(action.Type),
		Action:           action,
		WaitState:        waitState,
		SchemaValidation: schemaValidationReport{Valid: false, FailureStage: "waiting"},
		Detail:           detail,
	}
}

func collectRespondOutput(ctx context.Context, runner *adk.Runner, messages []adk.Message) (string, bool, error) {
	if runner == nil {
		return "", false, fmt.Errorf("prepared execution is missing runner")
	}
	var assistantOutput strings.Builder
	toolSideEffects := false
	iter := runner.Run(ctx, messages)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", toolSideEffects, event.Err
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			return "", toolSideEffects, fmt.Errorf("agent execution interrupted with %d checkpoint contexts", len(event.Action.Interrupted.InterruptContexts))
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		msgOutput := event.Output.MessageOutput
		if msg := msgOutput.Message; msg != nil {
			if msg.Extra != nil {
				if eventType, _ := msg.Extra["event_type"].(string); strings.HasPrefix(eventType, "tool_call_") {
					toolSideEffects = true
				}
			}
			if msg.Role == schema.Tool || msg.ToolCallID != "" || len(msg.ToolCalls) > 0 {
				toolSideEffects = true
			}
			if msg.Content != "" && msg.Role != schema.Tool {
				assistantOutput.WriteString(msg.Content)
			}
		}
		if msgStream := msgOutput.MessageStream; msgStream != nil {
			for {
				chunk, err := msgStream.Recv()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					return "", toolSideEffects, fmt.Errorf("stream error: %w", err)
				}
				if len(chunk.ToolCalls) > 0 || chunk.Role == schema.Tool || chunk.ToolCallID != "" || chunk.ToolName != "" {
					toolSideEffects = true
				}
				if chunk.Content != "" && chunk.Role != schema.Tool {
					assistantOutput.WriteString(chunk.Content)
				}
			}
		}
	}
	return assistantOutput.String(), toolSideEffects, nil
}

func resolveStructuredResult(ctx context.Context, prepared *runtime.PreparedExecution, rawOutput string, req ChatRespondRequest, requestID, sessionID string, toolSideEffects bool, sharedRootDir string, currentSession *session.Session) (*structuredChatResult, schemaValidationReport, error, map[string]any) {
	return resolveStructuredResultFunc(ctx, nil, controlplane.DefaultRuntimeTuning(), prepared, rawOutput, req, requestID, sessionID, toolSideEffects, sharedRootDir, currentSession)
}

func resolveStructuredResultWithControlPlane(ctx context.Context, application *appcore.Service, runtimeTuning controlplane.RuntimeTuning, prepared *runtime.PreparedExecution, rawOutput string, req ChatRespondRequest, requestID, sessionID string, toolSideEffects bool, sharedRootDir string, currentSession *session.Session) (*structuredChatResult, schemaValidationReport, error, map[string]any) {
	report := schemaValidationReport{
		Strict:     req.StrictSchemaValidation,
		RepairMode: normalizeSchemaRepairMode(req.SchemaRepairMode),
		RetryCount: effectiveSchemaRetryCount(req),
	}

	result, repaired, err := parseStructuredChatResult(rawOutput, report.RepairMode)
	report.RepairAttempted = report.RepairMode != "off"
	report.RepairSucceeded = repaired
	if err == nil {
		report.Valid = true
		enrichStructuredChatResult(ctx, application, runtimeTuning, result, prepared, req, requestID, sessionID, sharedRootDir, currentSession)
		return result, report, nil, nil
	}
	report.LastValidationError = err.Error()
	report.FailureStage = "initial_validation"

	if !req.StrictSchemaValidation {
		report.Valid = false
		result := &structuredChatResult{MainAnswer: strings.TrimSpace(rawOutput)}
		canonicalizeStructuredChatResult(result)
		enrichStructuredChatResult(ctx, application, runtimeTuning, result, prepared, req, requestID, sessionID, sharedRootDir, currentSession)
		return result, report, nil, map[string]any{"fallback_reason": "non_strict_text_wrap"}
	}

	if toolSideEffects {
		report.FailureStage = "tool_side_effect_guard"
	} else {
		for attempt := 0; attempt < report.RetryCount; attempt++ {
			formatted, formatErr := formattingRetryFunc(ctx, prepared, rawOutput, report.LastValidationError)
			report.RetriesUsed = attempt + 1
			if formatErr != nil {
				report.LastValidationError = formatErr.Error()
				report.FailureStage = "format_retry"
				break
			}
			result, repaired, err = parseStructuredChatResult(formatted, report.RepairMode)
			if err == nil {
				report.Valid = true
				report.RepairSucceeded = report.RepairSucceeded || repaired
				enrichStructuredChatResult(ctx, application, runtimeTuning, result, prepared, req, requestID, sessionID, sharedRootDir, currentSession)
				return result, report, nil, nil
			}
			report.LastValidationError = err.Error()
			report.FailureStage = "retry_validation"
		}
	}

	if regexResult, ok := regexFallbackStructuredChatResult(rawOutput); ok {
		report.RegexFallbackUsed = true
		if normalizeSchemaFailureAction(req.SchemaFailureAction) == "partial" {
			report.Valid = false
			if report.FailureStage == "" {
				report.FailureStage = "regex_partial_fallback"
			}
			enrichStructuredChatResult(ctx, application, runtimeTuning, regexResult, prepared, req, requestID, sessionID, sharedRootDir, currentSession)
			return regexResult, report, fmt.Errorf("strict schema validation failed: %s", report.LastValidationError), map[string]any{
				"tool_side_effects": toolSideEffects,
				"partial":           true,
				"fallback_source":   "regex",
			}
		}
	}

	if normalizeSchemaFailureAction(req.SchemaFailureAction) == "partial" {
		report.Valid = false
		if report.FailureStage == "" {
			report.FailureStage = "raw_text_partial_fallback"
		}
		result := &structuredChatResult{MainAnswer: strings.TrimSpace(rawOutput)}
		canonicalizeStructuredChatResult(result)
		enrichStructuredChatResult(ctx, application, runtimeTuning, result, prepared, req, requestID, sessionID, sharedRootDir, currentSession)
		return result, report, fmt.Errorf("strict schema validation failed: %s", report.LastValidationError), map[string]any{
			"tool_side_effects": toolSideEffects,
			"partial":           true,
			"fallback_source":   "raw_text",
		}
	}

	detail := map[string]any{
		"tool_side_effects": toolSideEffects,
		"retry_count":       report.RetryCount,
		"retries_used":      report.RetriesUsed,
	}
	if report.RegexFallbackUsed {
		detail["fallback_source"] = "regex"
	}
	return nil, report, fmt.Errorf("strict schema validation failed: %s", report.LastValidationError), detail
}

func formattingRetry(ctx context.Context, prepared *runtime.PreparedExecution, rawOutput, validationError string) (string, error) {
	if prepared == nil || prepared.Spec == nil || prepared.Spec.Model.ExecutedConfig == nil {
		return "", fmt.Errorf("executed model config is unavailable for formatting retry")
	}
	provider := model.NewProvider()
	retryConfig, err := resolveFormattingRetryModelConfig(prepared)
	if err != nil {
		return "", err
	}
	chatModel, err := provider.NewChatModel(ctx, retryConfig)
	if err != nil {
		return "", err
	}
	reply, err := chatModel.Generate(ctx, []*schema.Message{
		einomessage.SystemMessage("You are a strict JSON formatter. Return only one JSON object. Do not use markdown fences. Do not include commentary."),
		einomessage.UserMessage(fmt.Sprintf("Target schema: %s\nValidation error: %s\nOriginal output:\n%s", structuredRespondSchemaText(), validationError, rawOutput)),
	})
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(reply.Content), nil
}

func resolveFormattingRetryModelConfig(prepared *runtime.PreparedExecution) (model.ChatConfig, error) {
	if prepared == nil || prepared.Spec == nil || prepared.Spec.Model.ExecutedConfig == nil {
		return model.ChatConfig{}, fmt.Errorf("executed model config is unavailable for formatting retry")
	}
	configCopy := *prepared.Spec.Model.ExecutedConfig
	context := modelparams.ModelPolicyContext{
		TaskType:                 runtimeStringValue(prepared.Spec.Metadata.Constraints, "task_type"),
		Scene:                    runtimeStringValue(prepared.Spec.Metadata.Constraints, "scene"),
		DesiredOutputMode:        runtimeStringValue(prepared.Spec.Metadata.Constraints, "desired_output_mode"),
		LoopStage:                modelparams.LoopStageRetryFormatting,
		StepType:                 runtimeStringValue(prepared.Spec.Metadata.Constraints, "step_type"),
		StepRiskLevel:            modelparams.StepRiskLevel(runtimeStringValue(prepared.Spec.Metadata.Constraints, "step_risk_level")),
		HasToolCall:              runtimeBoolValue(prepared.Spec.Metadata.Constraints, "has_tool_call"),
		IsRetry:                  true,
		StructuredOutputRequired: true,
		AllowedTools:             append([]string(nil), prepared.Spec.Tools.AllowedTools...),
	}
	if raw, ok := prepared.Spec.Metadata.Constraints["model_policy_override"].(map[string]any); ok {
		override, err := modelparams.ParseControlledOverride(raw)
		if err != nil {
			return model.ChatConfig{}, err
		}
		context.ControlledOverride = override
	}
	resolved, err := modelparams.ResolveModelParameters(context)
	if err != nil {
		return model.ChatConfig{}, err
	}
	configCopy.ResolvedParameters = &resolved
	return configCopy, nil
}

func runtimeStringValue(values map[string]any, key string) string {
	if len(values) == 0 {
		return ""
	}
	text, _ := values[key].(string)
	return strings.TrimSpace(text)
}

func runtimeBoolValue(values map[string]any, key string) bool {
	if len(values) == 0 {
		return false
	}
	typed, _ := values[key].(bool)
	return typed
}

func parseStructuredChatResult(rawOutput, repairMode string) (*structuredChatResult, bool, error) {
	candidates := []string{strings.TrimSpace(rawOutput)}
	repaired := false
	if normalizeSchemaRepairMode(repairMode) == "basic" {
		if cleaned := stripMarkdownFence(rawOutput); cleaned != strings.TrimSpace(rawOutput) {
			candidates = append(candidates, cleaned)
			repaired = true
		}
		if extracted := extractJSONObject(rawOutput); extracted != "" {
			candidates = append(candidates, extracted)
			repaired = true
		}
		if extracted := extractJSONObject(stripMarkdownFence(rawOutput)); extracted != "" {
			candidates = append(candidates, extracted)
			repaired = true
		}
	}

	seen := map[string]struct{}{}
	var lastErr error
	for _, candidate := range candidates {
		candidate = strings.TrimSpace(candidate)
		if candidate == "" {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}

		var result structuredChatResult
		if err := json.Unmarshal([]byte(candidate), &result); err == nil {
			canonicalizeStructuredChatResult(&result)
			if err := validateStructuredChatResult(result); err == nil {
				return &result, repaired, nil
			} else {
				lastErr = err
			}
		} else {
			lastErr = err
		}

		var wrapped struct {
			Result structuredChatResult `json:"result"`
		}
		if err := json.Unmarshal([]byte(candidate), &wrapped); err == nil {
			canonicalizeStructuredChatResult(&wrapped.Result)
			if err := validateStructuredChatResult(wrapped.Result); err == nil {
				return &wrapped.Result, repaired, nil
			}
			lastErr = validateStructuredChatResult(wrapped.Result)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no structured JSON object could be parsed")
	}
	return nil, repaired, lastErr
}

func validateStructuredChatResult(result structuredChatResult) error {
	answer := strings.TrimSpace(result.MainAnswer)
	if answer == "" {
		answer = strings.TrimSpace(result.Answer)
	}
	if answer == "" {
		return fmt.Errorf("main_answer is required")
	}
	for _, item := range result.NextQuestions {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("next_questions must not contain empty items")
		}
	}
	if len(result.NextQuestions) > 3 {
		return fmt.Errorf("next_questions must contain at most 3 items")
	}
	for _, item := range result.FollowUpSuggestions {
		if strings.TrimSpace(item) == "" {
			return fmt.Errorf("follow_up_suggestions must not contain empty items")
		}
	}
	return nil
}

func stripMarkdownFence(raw string) string {
	trimmed := strings.TrimSpace(raw)
	matches := fencedBlockPattern.FindStringSubmatch(trimmed)
	if len(matches) == 2 {
		return strings.TrimSpace(matches[1])
	}
	return trimmed
}

func extractJSONObject(raw string) string {
	match := jsonObjectPattern.FindString(strings.TrimSpace(raw))
	return strings.TrimSpace(match)
}

func regexFallbackStructuredChatResult(raw string) (*structuredChatResult, bool) {
	trimmed := strings.TrimSpace(stripMarkdownFence(raw))
	if trimmed == "" {
		return nil, false
	}
	result := &structuredChatResult{MainAnswer: trimmed}
	if match := regexp.MustCompile(`(?im)^\s*(verdict|结论)\s*[:：]\s*(.+)$`).FindStringSubmatch(trimmed); len(match) == 3 {
		result.Verdict = strings.TrimSpace(match[2])
	}
	if match := regexp.MustCompile(`(?im)^\s*(decision|建议|处置建议)\s*[:：]\s*(.+)$`).FindStringSubmatch(trimmed); len(match) == 3 {
		result.Decision = strings.TrimSpace(match[2])
	}
	if match := regexp.MustCompile(`(?im)^\s*(reason|原因)\s*[:：]\s*(.+)$`).FindStringSubmatch(trimmed); len(match) == 3 {
		result.Reason = strings.TrimSpace(match[2])
	}
	canonicalizeStructuredChatResult(result)
	return result, strings.TrimSpace(result.MainAnswer) != ""
}

func buildStructuredRespondPrompt(prompt string) string {
	base := strings.TrimSpace(prompt)
	extra := fmt.Sprintf("Return the final answer as one JSON object matching this schema exactly: %s. Do not use markdown fences. Do not include explanatory text outside JSON.", structuredRespondSchemaText())
	if base == "" {
		return extra
	}
	return base + "\n" + extra
}

func structuredRespondSchemaText() string {
	return `{"type":"object","required":["main_answer"],"properties":{"main_answer":{"type":"string"},"structured_result":{"type":"object"},"result_summary":{"type":"object"},"content_cards":{"type":"array","items":{"type":"object"}},"right_panel_view":{"type":"object"},"next_questions":{"type":"array","items":{"type":"string"},"maxItems":3},"score_delta":{"type":"object"},"delivery_profile":{"type":"object"},"answer":{"type":"string"},"follow_up_suggestions":{"type":"array","items":{"type":"string"}},"verdict":{"type":"string"},"decision":{"type":"string"},"reason":{"type":"string"}}}`
}

func canonicalizeStructuredChatResult(result *structuredChatResult) {
	appcore.CanonicalizeDirectRespondRichResult(result)
}

func enrichStructuredChatResult(ctx context.Context, application *appcore.Service, runtimeTuning controlplane.RuntimeTuning, result *structuredChatResult, prepared *runtime.PreparedExecution, req ChatRespondRequest, requestID, sessionID, sharedRootDir string, currentSession *session.Session) {
	appcore.EnrichDirectRespondRichResult(ctx, application, runtimeTuning, directRespondRichInput(prepared, req, requestID, sessionID, sharedRootDir, currentSession), result)
}

func shouldBuildAutomationPlanDraft(req ChatRespondRequest) bool {
	return appcore.ShouldBuildDirectRespondPlanDraft(directRespondRichRequest(req))
}

func directRespondRichInput(prepared *runtime.PreparedExecution, req ChatRespondRequest, requestID, sessionID, sharedRootDir string, currentSession *session.Session) appcore.DirectRespondRichInput {
	return appcore.DirectRespondRichInput{
		Request:        directRespondRichRequest(req),
		Prepared:       prepared,
		RequestID:      requestID,
		SessionID:      sessionID,
		SharedRootDir:  sharedRootDir,
		CurrentSession: currentSession,
	}
}

func directRespondRichRequest(req ChatRespondRequest) appcore.DirectRespondRichRequest {
	return appcore.DirectRespondRichRequest{
		TaskType:              strings.TrimSpace(req.TaskType),
		TaskSubtype:           strings.TrimSpace(req.TaskSubtype),
		Scene:                 strings.TrimSpace(req.Scene),
		Query:                 strings.TrimSpace(req.Query),
		WorkspaceID:           strings.TrimSpace(req.WorkspaceID),
		AppInstanceID:         strings.TrimSpace(req.AppInstanceID),
		IntegrationInstanceID: strings.TrimSpace(req.IntegrationInstanceID),
		WorkflowRunID:         strings.TrimSpace(req.WorkflowRunID),
		StepID:                strings.TrimSpace(req.StepID),
		TriggerType:           strings.TrimSpace(req.TriggerType),
		AutomationTaskID:      strings.TrimSpace(req.AutomationTaskID),
		UserLanguage:          strings.TrimSpace(req.UserLanguage),
		DesiredOutputMode:     strings.TrimSpace(req.DesiredOutputMode),
		GlobalContext:         cloneAnyMap(req.GlobalContext),
		AppContext:            cloneAnyMap(req.AppContext),
		InputPayload:          cloneAnyMap(req.InputPayload),
	}
}

func normalizeSchemaRepairMode(mode string) string {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", "basic":
		return "basic"
	case "off":
		return "off"
	default:
		return "basic"
	}
}

func normalizeSchemaFailureAction(action string) string {
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "partial":
		return "partial"
	default:
		return "error"
	}
}

func effectiveSchemaRetryCount(req ChatRespondRequest) int {
	retries := req.SchemaRetryCount
	if req.StrictSchemaValidation && retries <= 0 {
		retries = defaultSchemaRetryCount
	}
	if retries < 0 {
		retries = 0
	}
	if retries > maxSchemaRetryCount {
		retries = maxSchemaRetryCount
	}
	return retries
}
