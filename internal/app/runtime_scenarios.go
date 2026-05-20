// runtime_scenarios.go is the app-layer orchestration entry for Athena-side runtime judgment flows.
// runtime_scenarios.go 负责 Athena 侧 runtime judgment 路径的 app 层总编排。
//
// It exists because runtime judgment needs one bounded path that can normalize external host input,
// persist waiting state, resolve runtime assets, and return transport-safe structured decisions
// without leaking host-specific execution logic into transport or low-level runtime packages.
// 这个文件存在的原因是 runtime judgment 需要一个受边界约束的总入口，把外部宿主输入归一化、
// 持久化等待态、解析 runtime assets，并返回 transport-safe 的结构化结论，同时避免把宿主侧执行逻辑散落到 transport 或底层 runtime 包。
//
// Main entry points:
// 主要入口：
// - `ListRuntimeSkills`
// - `AnalyzeRuntimeScenario`
// - runtime scenario request / response structures
//
// Change carefully when:
// 修改时重点注意：
// - task_type / task_subtype / requested_output_mode matching changes
// - waiting/resume persistence or evidence supplement semantics change
// - runtimeassets registry, host projection, or transport contract fields change
package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	einomessage "github.com/cloudwego/eino/schema"
	"moss/internal/model"
	"moss/internal/observability"
	"moss/internal/runtimeassets"
	"moss/internal/session"
)

const runtimeStateMessagePrefix = "runtime_state:"

var (
	scriptReferencePattern    = regexp.MustCompile(`(?i)(?:\b(?:bash|sh|python|python3|node)\s+[^\s]+\.(?:sh|py|js)\b|(?:^|[\s"'` + "`" + `])[^/\s]+\.(?:sh|py|js)(?:$|[\s"'` + "`" + `]))`)
	credentialRiskPattern     = regexp.MustCompile(`(?i)(?:\.aws/credentials|\.ssh/id_rsa|authorized_keys|/proc/self/environ|api[_-]?key|access[_-]?key|bearer\s+[a-z0-9._-]{8,}|system\s*prompt|ignore\s+previous\s+instructions|169\.254\.169\.254|curl\s*\|\s*(?:bash|sh)|rm\s+-rf)`)
	bulkDataApprovalPattern   = regexp.MustCompile(`(?i)(?:最近\s*50\s*条|批量|customer|客户名单|导出|bulk|all\s+customers|customers\.csv|customer_records)`)
	externalOutboundIndicator = regexp.MustCompile(`(?i)(?:@|https?://|pastebin|slack|email|邮件|发送)`)
)

// RuntimeScenarioRequest captures the bounded Athena-side runtime judgment input for one external system.
// RuntimeScenarioRequest 描述 Athena 侧面向外部系统的一次受边界约束的 runtime 研判输入。
type RuntimeScenarioRequest struct {
	TaskType             string         `json:"task_type,omitempty"`
	TaskSubtype          string         `json:"task_subtype,omitempty"`
	RequestedOutputModes []string       `json:"requested_output_mode,omitempty"`
	SessionID            string         `json:"session_id,omitempty"`
	CorrelationID        string         `json:"correlation_id,omitempty"`
	TraceID              string         `json:"trace_id,omitempty"`
	ConnectorID          string         `json:"connector_id,omitempty"`
	HostType             string         `json:"host_type,omitempty"`
	HookName             string         `json:"hook_name,omitempty"`
	EventType            string         `json:"event_type,omitempty"`
	OccurredAt           string         `json:"occurred_at,omitempty"`
	ModelID              string         `json:"model_id,omitempty"`
	AllowUserSupplement  bool           `json:"allow_user_supplement,omitempty"`
	AvailableSkillIDs    []string       `json:"available_skill_ids,omitempty"`
	RawPayload           map[string]any `json:"raw_payload,omitempty"`
	NormalizedContext    map[string]any `json:"normalized_context,omitempty"`
	EvidenceSupplement   map[string]any `json:"evidence_supplement,omitempty"`
	JudgmentContext      map[string]any `json:"judgment_context,omitempty"`
	ResumeToken          string         `json:"resume_token,omitempty"`
}

// RuntimeSuggestedAction describes one post-judgment business action that the client may execute.
// RuntimeSuggestedAction 描述一条客户端可在研判后执行的业务动作建议。
type RuntimeSuggestedAction struct {
	SkillID         string                             `json:"skill_id,omitempty"`
	ExecutionTarget runtimeassets.SkillExecutionTarget `json:"execution_target,omitempty"`
	Operation       string                             `json:"operation,omitempty"`
	Arguments       map[string]any                     `json:"arguments,omitempty"`
}

// RuntimeHostProjection captures the host-facing action mapping for one canonical judgment.
// RuntimeHostProjection 描述某个 canonical judgment 对宿主的动作投影结果。
type RuntimeHostProjection struct {
	HookActionCode  string `json:"hook_action_code,omitempty"`
	FinalDecision   string `json:"final_decision,omitempty"`
	UserVisibleCopy string `json:"user_visible_copy,omitempty"`
}

// RuntimeEvidenceRequest captures the next evidence-supplement step when the current context is still incomplete.
// RuntimeEvidenceRequest 描述当前上下文不完整时需要进入的下一步证据补全过程。
type RuntimeEvidenceRequest struct {
	ResumeToken      string   `json:"resume_token,omitempty"`
	Kind             string   `json:"kind,omitempty"`
	MissingEvidence  []string `json:"missing_evidence,omitempty"`
	AllowedClientOps []string `json:"allowed_client_ops,omitempty"`
}

// RuntimeScenarioResponse captures the bounded Athena-side response for runtime judgment replacement.
// RuntimeScenarioResponse 描述 Athena 面向 runtime 研判替换场景的受边界约束响应。
type RuntimeScenarioResponse struct {
	RequestID            string                   `json:"request_id,omitempty"`
	SessionID            string                   `json:"session_id,omitempty"`
	TraceID              string                   `json:"trace_id,omitempty"`
	CorrelationID        string                   `json:"correlation_id,omitempty"`
	TaskType             string                   `json:"task_type,omitempty"`
	TaskSubtype          string                   `json:"task_subtype,omitempty"`
	RequestedOutputModes []string                 `json:"requested_output_mode,omitempty"`
	Status               string                   `json:"status,omitempty"`
	Decision             string                   `json:"decision,omitempty"`
	DecisionReason       string                   `json:"decision_reason,omitempty"`
	UserVisibleCopy      string                   `json:"user_visible_copy,omitempty"`
	AuditSummary         string                   `json:"audit_summary,omitempty"`
	RecommendedNextStep  string                   `json:"recommended_next_step,omitempty"`
	HostProjection       *RuntimeHostProjection   `json:"host_projection,omitempty"`
	EvidenceRequest      *RuntimeEvidenceRequest  `json:"evidence_request,omitempty"`
	SuggestedActions     []RuntimeSuggestedAction `json:"suggested_actions,omitempty"`
	Detail               map[string]any           `json:"detail,omitempty"`
}

type runtimePersistedState struct {
	Request RuntimeScenarioRequest `json:"request"`
}

type runtimePromptOutput struct {
	ExecutiveSummary    string                   `json:"executive_summary,omitempty"`
	Decision            string                   `json:"decision,omitempty"`
	DecisionReason      string                   `json:"decision_reason,omitempty"`
	UserVisibleCopy     string                   `json:"user_visible_copy,omitempty"`
	AuditSummary        string                   `json:"audit_summary,omitempty"`
	RecommendedNextStep string                   `json:"recommended_next_step,omitempty"`
	SuggestedActions    []RuntimeSuggestedAction `json:"suggested_actions,omitempty"`
}

// ListRuntimeSkills returns visible runtime skill metadata narrowed by source and task context.
// ListRuntimeSkills 会按来源与任务上下文返回可见 runtime skill 元数据。
func (s *Service) ListRuntimeSkills(ctx context.Context, filter runtimeassets.SkillFilter) ([]runtimeassets.SkillMetadata, error) {
	registry, err := runtimeassets.NewRegistry()
	if err != nil {
		return nil, err
	}
	return registry.ListSkills(ctx, filter), nil
}

// AnalyzeRuntimeScenario runs Athena-side runtime judgment for the first real mosi/OpenClaw closures.
// AnalyzeRuntimeScenario 会为首批真实 mosi/OpenClaw 场景执行 Athena 侧 runtime 研判。
func (s *Service) AnalyzeRuntimeScenario(ctx context.Context, requestID string, req RuntimeScenarioRequest) (*RuntimeScenarioResponse, error) {
	registry, err := runtimeassets.NewRegistry()
	if err != nil {
		return nil, err
	}

	currentReq := req
	sess, err := s.getOrPrepareRuntimeSession(ctx, &currentReq)
	if err != nil {
		return nil, err
	}

	resolvedTaskType, resolvedSubtype, resolvedOutputModes := resolveRuntimeTask(currentReq)
	if resolvedTaskType == "" || resolvedSubtype == "" {
		return nil, fmt.Errorf("runtime task_type and task_subtype could not be resolved")
	}
	bundle, ok := registry.SelectTaskBundle(resolvedTaskType, resolvedSubtype, resolvedOutputModes)
	if !ok {
		return nil, &InvalidTaskRequestError{
			TaskType: resolvedTaskType,
			Reason:   "runtime_assets_not_found",
		}
	}

	visibleSkills, err := registry.ResolveVisibleSkills(currentReq.AvailableSkillIDs, resolvedTaskType, resolvedSubtype, resolvedOutputModes)
	if err != nil {
		return nil, err
	}
	currentReq.TaskType = resolvedTaskType
	currentReq.TaskSubtype = resolvedSubtype
	currentReq.RequestedOutputModes = append([]string(nil), resolvedOutputModes...)

	if need, evidence := detectEvidenceSupplementNeed(currentReq); need {
		resumeToken := fmt.Sprintf("resume-%d", time.Now().UnixNano())
		pending := &session.PendingState{
			Stage:         "evidence_supplement",
			Status:        "waiting_for_evidence",
			ActionType:    "information_request",
			ResumeToken:   resumeToken,
			MissingFields: append([]string(nil), evidence...),
			Preserved: &session.PreservedContext{
				Goal:           resolvedSubtype,
				MissingFields:  append([]string(nil), evidence...),
				Facts:          map[string]string{"task_type": resolvedTaskType, "task_subtype": resolvedSubtype},
				WaitStage:      "evidence_supplement",
				ResumeToken:    resumeToken,
				TimeoutAt:      time.Now().Add(5 * time.Minute),
				TimeoutAfter:   5 * time.Minute,
				LastUserIntent: "",
			},
			TimeoutAt:    time.Now().Add(5 * time.Minute),
			TimeoutAfter: 5 * time.Minute,
		}
		if err := persistRuntimeState(ctx, s.SessionStore, sess, currentReq, pending); err != nil {
			return nil, err
		}
		return &RuntimeScenarioResponse{
			RequestID:            requestID,
			SessionID:            sess.ID,
			TraceID:              strings.TrimSpace(currentReq.TraceID),
			CorrelationID:        strings.TrimSpace(currentReq.CorrelationID),
			TaskType:             resolvedTaskType,
			TaskSubtype:          resolvedSubtype,
			RequestedOutputModes: append([]string(nil), resolvedOutputModes...),
			Status:               "waiting_for_evidence",
			EvidenceRequest: &RuntimeEvidenceRequest{
				ResumeToken:      resumeToken,
				Kind:             "evidence_supplement",
				MissingEvidence:  append([]string(nil), evidence...),
				AllowedClientOps: []string{"read_file", "list_dir"},
			},
			Detail: map[string]any{
				"task_bundle_id":          bundle.ID,
				"supplement_kind":         "evidence",
				"user_supplement_allowed": currentReq.AllowUserSupplement,
			},
		}, nil
	}

	output, detail := buildRuntimeFallback(currentReq, bundle)
	if modelOutput, modelDetail, err := s.tryModelRuntimeAnalysis(ctx, currentReq, bundle); err == nil && modelOutput != nil {
		output = *modelOutput
		for key, value := range modelDetail {
			detail[key] = value
		}
	} else if err != nil {
		detail["model_fallback_reason"] = err.Error()
	}
	output = reconcileRuntimeOutput(currentReq, output, visibleSkills)

	projection := buildHostProjection(resolvedSubtype, output)
	response := &RuntimeScenarioResponse{
		RequestID:            requestID,
		SessionID:            sess.ID,
		TraceID:              strings.TrimSpace(currentReq.TraceID),
		CorrelationID:        strings.TrimSpace(currentReq.CorrelationID),
		TaskType:             resolvedTaskType,
		TaskSubtype:          resolvedSubtype,
		RequestedOutputModes: append([]string(nil), resolvedOutputModes...),
		Status:               "completed",
		Decision:             output.Decision,
		DecisionReason:       output.DecisionReason,
		UserVisibleCopy:      output.UserVisibleCopy,
		AuditSummary:         output.AuditSummary,
		RecommendedNextStep:  output.RecommendedNextStep,
		HostProjection:       projection,
		SuggestedActions:     append([]RuntimeSuggestedAction(nil), output.SuggestedActions...),
		Detail:               detail,
	}
	if err := clearRuntimePending(ctx, s.SessionStore, sess, currentReq); err != nil {
		return nil, err
	}
	return response, nil
}

func resolveRuntimeTask(req RuntimeScenarioRequest) (string, string, []string) {
	taskType := strings.TrimSpace(req.TaskType)
	taskSubtype := strings.TrimSpace(req.TaskSubtype)
	if taskType == "" {
		if strings.TrimSpace(req.HookName) != "" || strings.TrimSpace(req.HostType) != "" {
			taskType = "runtime_event_analysis"
		}
	}
	if taskSubtype == "" {
		switch strings.TrimSpace(req.HookName) {
		case "before_tool_call":
			taskSubtype = "openclaw_before_tool_call"
		case "message_sending":
			taskSubtype = "openclaw_message_sending"
		default:
			if strings.TrimSpace(stringValue(req.JudgmentContext["final_decision"])) != "" || len(req.JudgmentContext) > 0 {
				taskSubtype = "openclaw_runtime_explanation"
			}
		}
	}
	outputModes := normalizeOutputModes(req.RequestedOutputModes)
	if len(outputModes) == 0 {
		switch taskSubtype {
		case "openclaw_runtime_explanation":
			outputModes = []string{"summary", "remediation"}
		default:
			outputModes = []string{"judgment", "decision"}
		}
	}
	return taskType, taskSubtype, outputModes
}

func normalizeOutputModes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	sort.Strings(result)
	return result
}

func (s *Service) getOrPrepareRuntimeSession(ctx context.Context, req *RuntimeScenarioRequest) (*session.Session, error) {
	if strings.TrimSpace(req.SessionID) != "" {
		current, ok := s.SessionStore.Get(ctx, strings.TrimSpace(req.SessionID))
		if !ok {
			return nil, &InvalidSessionError{
				SessionID: strings.TrimSpace(req.SessionID),
				Reason:    "not_found",
			}
		}
		if current.Pending == nil {
			return current, nil
		}
		if strings.TrimSpace(req.ResumeToken) == "" || strings.TrimSpace(req.ResumeToken) != current.Pending.ResumeToken {
			return nil, &InvalidResumeTokenError{
				SessionID:   current.ID,
				ResumeToken: strings.TrimSpace(req.ResumeToken),
				Reason:      "not_found",
			}
		}
		if state, ok := restoreRuntimeState(current); ok {
			req.TaskType = state.Request.TaskType
			req.TaskSubtype = state.Request.TaskSubtype
			if len(req.RequestedOutputModes) == 0 {
				req.RequestedOutputModes = append([]string(nil), state.Request.RequestedOutputModes...)
			}
			req.TraceID = firstNonEmpty(strings.TrimSpace(req.TraceID), strings.TrimSpace(state.Request.TraceID))
			req.CorrelationID = firstNonEmpty(strings.TrimSpace(req.CorrelationID), strings.TrimSpace(state.Request.CorrelationID))
			req.ConnectorID = firstNonEmpty(strings.TrimSpace(req.ConnectorID), strings.TrimSpace(state.Request.ConnectorID))
			req.HostType = firstNonEmpty(strings.TrimSpace(req.HostType), strings.TrimSpace(state.Request.HostType))
			req.HookName = firstNonEmpty(strings.TrimSpace(req.HookName), strings.TrimSpace(state.Request.HookName))
			req.EventType = firstNonEmpty(strings.TrimSpace(req.EventType), strings.TrimSpace(state.Request.EventType))
			req.ModelID = firstNonEmpty(strings.TrimSpace(req.ModelID), strings.TrimSpace(state.Request.ModelID))
			req.RawPayload = mergeMaps(state.Request.RawPayload, req.RawPayload)
			req.NormalizedContext = mergeMaps(state.Request.NormalizedContext, req.NormalizedContext)
			req.JudgmentContext = mergeMaps(state.Request.JudgmentContext, req.JudgmentContext)
			req.AvailableSkillIDs = mergeStringLists(state.Request.AvailableSkillIDs, req.AvailableSkillIDs)
		}
		return current, nil
	}
	return &session.Session{ID: session.NewID()}, nil
}

func persistRuntimeState(ctx context.Context, store session.Store, current *session.Session, req RuntimeScenarioRequest, pending *session.PendingState) error {
	statePayload, err := json.Marshal(runtimePersistedState{Request: req})
	if err != nil {
		return err
	}
	next := current.Clone()
	next.Messages = append(next.Messages, session.Message{Role: "system", Content: runtimeStateMessagePrefix + string(statePayload)})
	next.Pending = pending
	return store.Put(ctx, next)
}

func clearRuntimePending(ctx context.Context, store session.Store, current *session.Session, req RuntimeScenarioRequest) error {
	next := current.Clone()
	statePayload, err := json.Marshal(runtimePersistedState{Request: req})
	if err != nil {
		return err
	}
	next.Messages = append(next.Messages, session.Message{Role: "system", Content: runtimeStateMessagePrefix + string(statePayload)})
	next.Pending = nil
	return store.Put(ctx, next)
}

func restoreRuntimeState(current *session.Session) (runtimePersistedState, bool) {
	for idx := len(current.Messages) - 1; idx >= 0; idx-- {
		item := current.Messages[idx]
		if !strings.HasPrefix(item.Content, runtimeStateMessagePrefix) {
			continue
		}
		var state runtimePersistedState
		if err := json.Unmarshal([]byte(strings.TrimPrefix(item.Content, runtimeStateMessagePrefix)), &state); err != nil {
			return runtimePersistedState{}, false
		}
		return state, true
	}
	return runtimePersistedState{}, false
}

func detectEvidenceSupplementNeed(req RuntimeScenarioRequest) (bool, []string) {
	if strings.TrimSpace(req.TaskSubtype) != "openclaw_before_tool_call" {
		return false, nil
	}
	if hasEvidenceContent(req) {
		return false, nil
	}
	command := strings.ToLower(strings.TrimSpace(stringValue(req.RawPayload["command"])))
	if command == "" {
		if params, ok := req.RawPayload["params"].(map[string]any); ok {
			command = strings.ToLower(strings.TrimSpace(stringValue(params["command"])))
		}
	}
	if command == "" {
		command = strings.ToLower(strings.TrimSpace(stringValue(req.NormalizedContext["command_normalized"])))
	}
	targetPath := strings.ToLower(strings.TrimSpace(firstNonEmpty(
		stringValue(req.RawPayload["path"]),
		stringValue(req.RawPayload["file_path"]),
		stringValue(req.NormalizedContext["target_path"]),
	)))
	if scriptReferencePattern.MatchString(command) || scriptReferencePattern.MatchString(targetPath) {
		return true, []string{"script_content"}
	}
	return false, nil
}

func hasEvidenceContent(req RuntimeScenarioRequest) bool {
	if len(req.EvidenceSupplement) > 0 {
		return true
	}
	if len(req.NormalizedContext) == 0 {
		return false
	}
	for _, key := range []string{"script_content", "file_content", "directory_listing"} {
		if strings.TrimSpace(stringValue(req.NormalizedContext[key])) != "" {
			return true
		}
	}
	return false
}

func buildRuntimeFallback(req RuntimeScenarioRequest, bundle runtimeassets.TaskAssetBundle) (runtimePromptOutput, map[string]any) {
	text := strings.ToLower(marshalRuntimeIntentInput(req))
	detail := map[string]any{
		"task_bundle_id":  bundle.ID,
		"decision_source": "deterministic_fallback",
	}

	switch strings.TrimSpace(req.TaskSubtype) {
	case "openclaw_runtime_explanation":
		decision := strings.TrimSpace(stringValue(req.JudgmentContext["final_decision"]))
		if decision == "" {
			decision = strings.TrimSpace(stringValue(req.JudgmentContext["decision"]))
		}
		reason := firstNonEmpty(strings.TrimSpace(stringValue(req.JudgmentContext["reason"])), "the runtime judgment was already projected by the host")
		return runtimePromptOutput{
			ExecutiveSummary:    "当前 explanation 仅解释既有 judgment，不重写结果。",
			Decision:            normalizeDecisionValue(decision),
			DecisionReason:      reason,
			UserVisibleCopy:     strings.TrimSpace(stringValue(req.JudgmentContext["user_visible_copy"])),
			AuditSummary:        firstNonEmpty(strings.TrimSpace(stringValue(req.JudgmentContext["audit_summary"])), bundle.AuditSummaryHint),
			RecommendedNextStep: "reuse the existing host projection and review the linked audit record before further business continuation",
		}, detail
	default:
		if credentialRiskPattern.MatchString(text) || (externalOutboundIndicator.MatchString(text) && strings.Contains(text, "secret")) {
			return runtimePromptOutput{
				ExecutiveSummary:    "检测到高风险运行时行为，建议拒绝继续执行。",
				Decision:            "deny",
				DecisionReason:      "the current event includes a high-risk command, secret exposure pattern, or prompt-injection-like signal",
				UserVisibleCopy:     "Athena detected a high-risk runtime action and recommends blocking this step.",
				AuditSummary:        "Runtime event denied because the current evidence points to a high-risk action or sensitive disclosure attempt.",
				RecommendedNextStep: "stop the current step, preserve audit evidence, and escalate for immediate review",
			}, detail
		}
		if bulkDataApprovalPattern.MatchString(text) {
			return runtimePromptOutput{
				ExecutiveSummary:    "检测到灰区高风险运行时行为，建议进入审批确认。",
				Decision:            "ask",
				DecisionReason:      "the current event appears legitimate but requests bulk or high-sensitivity data handling that still needs approval",
				UserVisibleCopy:     "Athena detected a runtime action that requires explicit approval before continuing.",
				AuditSummary:        "Runtime event requires approval because the current request is plausible but still high-impact or bulk-scoped.",
				RecommendedNextStep: "create or reuse an approval ticket and wait for the platform-side business continuation decision",
			}, detail
		}
		return runtimePromptOutput{
			ExecutiveSummary:    "当前运行时事件未命中明确高风险信号，可继续执行。",
			Decision:            "allow",
			DecisionReason:      "the current runtime event does not show a confirmed high-risk action under the current bounded evidence",
			UserVisibleCopy:     "Athena found no confirmed high-risk signal for this runtime step.",
			AuditSummary:        "Runtime event allowed because the current evidence is closer to benign maintenance or low-risk execution.",
			RecommendedNextStep: "continue execution and keep the audit trail for downstream review if needed",
		}, detail
	}
}

func buildHostProjection(taskSubtype string, output runtimePromptOutput) *RuntimeHostProjection {
	decision := normalizeDecisionValue(output.Decision)
	projection := &RuntimeHostProjection{
		FinalDecision:   decision,
		UserVisibleCopy: strings.TrimSpace(output.UserVisibleCopy),
	}
	switch taskSubtype {
	case "openclaw_message_sending":
		if decision == "allow" {
			projection.HookActionCode = "allow"
		} else {
			projection.HookActionCode = "modify"
		}
	case "openclaw_runtime_explanation":
		projection.HookActionCode = "none"
	default:
		switch decision {
		case "deny":
			projection.HookActionCode = "block"
		case "ask":
			projection.HookActionCode = "require_approval"
		default:
			projection.HookActionCode = "allow"
		}
	}
	return projection
}

func (s *Service) tryModelRuntimeAnalysis(ctx context.Context, req RuntimeScenarioRequest, bundle runtimeassets.TaskAssetBundle) (*runtimePromptOutput, map[string]any, error) {
	if s.ModelStore == nil {
		return nil, nil, errors.New("model store is not configured")
	}
	selection, err := s.ModelStore.Resolve(ctx, strings.TrimSpace(req.ModelID))
	if err != nil {
		return nil, nil, err
	}
	provider := s.ModelProvider
	if provider == nil {
		provider = model.NewProvider()
	}
	chatConfig, err := resolveAppModelConfig(selection.Primary, runtimeScenarioPolicyContext(req))
	if err != nil {
		return nil, nil, err
	}
	chatModel, err := provider.NewChatModel(ctx, chatConfig)
	if err != nil {
		return nil, nil, err
	}
	payload, err := json.MarshalIndent(buildRuntimePromptEnvelope(req), "", "  ")
	if err != nil {
		return nil, nil, err
	}
	reply, err := chatModel.Generate(ctx, []*einomessage.Message{
		einomessage.SystemMessage(bundle.SystemPrompt),
		einomessage.UserMessage(string(payload)),
	})
	if err != nil {
		return nil, nil, err
	}
	output, err := parseRuntimePromptOutput(reply.Content)
	if err != nil {
		return nil, nil, err
	}
	if s.Observability != nil {
		s.Observability.LogAction(ctx, observability.LogLevelInfo, observability.ActionLog{
			Module:    "app",
			Action:    "runtime_model_analysis",
			Step:      "completed",
			Status:    "ok",
			RequestID: firstNonEmpty(strings.TrimSpace(req.TraceID), strings.TrimSpace(req.CorrelationID)),
			SessionID: strings.TrimSpace(req.SessionID),
			Reason:    "runtime_prompt_output_ready",
			Detail: map[string]any{
				"task_type":         strings.TrimSpace(req.TaskType),
				"task_subtype":      strings.TrimSpace(req.TaskSubtype),
				"provider_id":       selection.Primary.ProviderID,
				"model_record_id":   selection.Primary.ModelRecordID,
				"provider_model_id": selection.Primary.ProviderModelID,
			},
		})
	}
	return &output, map[string]any{
		"decision_source":    "model",
		"provider_id":        selection.Primary.ProviderID,
		"model_record_id":    selection.Primary.ModelRecordID,
		"provider_model_id":  selection.Primary.ProviderModelID,
		"provider_name":      selection.Primary.ProviderName,
		"provider_protocol":  selection.Primary.ProviderProtocol,
		"explicit_selection": selection.Explicit,
	}, nil
}

func buildRuntimePromptEnvelope(req RuntimeScenarioRequest) map[string]any {
	return map[string]any{
		"task": map[string]any{
			"task_id":               firstNonEmpty(strings.TrimSpace(req.TraceID), strings.TrimSpace(req.CorrelationID), "runtime-task"),
			"task_type":             req.TaskType,
			"task_subtype":          req.TaskSubtype,
			"requested_output_mode": req.RequestedOutputModes,
			"objective":             fmt.Sprintf("produce a bounded runtime judgment for %s", req.TaskSubtype),
		},
		"scenario": map[string]any{
			"source_channel":   req.HostType,
			"source_stage":     req.HookName,
			"scenario_summary": summarizeRuntimeRequest(req),
			"environment": map[string]any{
				"host_type":  req.HostType,
				"hook_name":  req.HookName,
				"event_type": req.EventType,
			},
		},
		"evidence_context": map[string]any{
			"normalized_context":  req.NormalizedContext,
			"evidence_supplement": req.EvidenceSupplement,
			"judgment_context":    req.JudgmentContext,
		},
		"business_policy_context": map[string]any{
			"allow_user_supplement": req.AllowUserSupplement,
			"available_skill_ids":   req.AvailableSkillIDs,
		},
		"technical_context": map[string]any{
			"trace_id":       req.TraceID,
			"session_id":     req.SessionID,
			"connector_id":   req.ConnectorID,
			"occurred_at":    req.OccurredAt,
			"host_type":      req.HostType,
			"hook_name":      req.HookName,
			"event_type":     req.EventType,
			"correlation_id": req.CorrelationID,
		},
		"analysis_constraints": map[string]any{
			"deterministic_first":     true,
			"user_supplement_allowed": req.AllowUserSupplement,
			"bounded_reasoning":       true,
		},
		"output_contract": map[string]any{
			"decision_enum":         []string{"allow", "ask", "deny"},
			"must_return_json":      true,
			"must_not_return_block": true,
		},
		"raw_inputs": map[string]any{
			"raw_payload": req.RawPayload,
		},
	}
}

func summarizeRuntimeRequest(req RuntimeScenarioRequest) string {
	return firstNonEmpty(
		strings.TrimSpace(stringValue(req.NormalizedContext["intent_summary"])),
		strings.TrimSpace(stringValue(req.NormalizedContext["actual_behavior"])),
		fmt.Sprintf("%s/%s runtime event", strings.TrimSpace(req.HostType), strings.TrimSpace(req.HookName)),
	)
}

func parseRuntimePromptOutput(raw string) (runtimePromptOutput, error) {
	cleaned := strings.TrimSpace(raw)
	cleaned = strings.TrimPrefix(cleaned, "```json")
	cleaned = strings.TrimPrefix(cleaned, "```")
	cleaned = strings.TrimSuffix(cleaned, "```")
	cleaned = strings.TrimSpace(cleaned)
	if start := strings.Index(cleaned, "{"); start >= 0 {
		if end := strings.LastIndex(cleaned, "}"); end >= start {
			cleaned = cleaned[start : end+1]
		}
	}
	var output runtimePromptOutput
	if err := json.Unmarshal([]byte(cleaned), &output); err != nil {
		return runtimePromptOutput{}, err
	}
	output.Decision = normalizeDecisionValue(output.Decision)
	if output.Decision == "" {
		return runtimePromptOutput{}, fmt.Errorf("runtime prompt output is missing canonical decision")
	}
	return output, nil
}

func reconcileRuntimeOutput(req RuntimeScenarioRequest, output runtimePromptOutput, visibleSkills []runtimeassets.SkillMetadata) runtimePromptOutput {
	output.Decision = normalizeDecisionValue(output.Decision)
	if strings.TrimSpace(req.TaskSubtype) == "openclaw_runtime_explanation" {
		judgmentDecision := normalizeDecisionValue(firstNonEmpty(
			strings.TrimSpace(stringValue(req.JudgmentContext["final_decision"])),
			strings.TrimSpace(stringValue(req.JudgmentContext["decision"])),
		))
		if judgmentDecision != "" {
			output.Decision = judgmentDecision
		}
		output.DecisionReason = firstNonEmpty(output.DecisionReason, strings.TrimSpace(stringValue(req.JudgmentContext["reason"])))
		output.UserVisibleCopy = firstNonEmpty(output.UserVisibleCopy, strings.TrimSpace(stringValue(req.JudgmentContext["user_visible_copy"])))
		output.AuditSummary = firstNonEmpty(output.AuditSummary, strings.TrimSpace(stringValue(req.JudgmentContext["audit_summary"])))
	}
	output.SuggestedActions = filterRuntimeSuggestedActions(output.SuggestedActions, visibleSkills)
	return output
}

func filterRuntimeSuggestedActions(actions []RuntimeSuggestedAction, visibleSkills []runtimeassets.SkillMetadata) []RuntimeSuggestedAction {
	if len(actions) == 0 || len(visibleSkills) == 0 {
		return nil
	}
	allowed := make(map[string]runtimeassets.SkillMetadata, len(visibleSkills))
	for _, item := range visibleSkills {
		allowed[strings.TrimSpace(item.ID)] = item
	}
	filtered := make([]RuntimeSuggestedAction, 0, len(actions))
	for _, item := range actions {
		id := strings.TrimSpace(item.SkillID)
		if id == "" {
			continue
		}
		meta, ok := allowed[id]
		if !ok {
			continue
		}
		item.SkillID = id
		item.ExecutionTarget = meta.ExecutionTarget
		filtered = append(filtered, item)
	}
	return filtered
}

func normalizeDecisionValue(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	switch value {
	case "allow", "ask", "deny":
		return value
	default:
		return ""
	}
}

func marshalRuntimeIntentInput(req RuntimeScenarioRequest) string {
	payload, _ := json.Marshal(map[string]any{
		"raw_payload":         req.RawPayload,
		"normalized_context":  req.NormalizedContext,
		"judgment_context":    req.JudgmentContext,
		"evidence_supplement": req.EvidenceSupplement,
		"hook_name":           req.HookName,
		"event_type":          req.EventType,
	})
	return strings.ToLower(string(payload))
}

func mergeMaps(base map[string]any, overlay map[string]any) map[string]any {
	if len(base) == 0 && len(overlay) == 0 {
		return nil
	}
	result := make(map[string]any, len(base)+len(overlay))
	for key, value := range base {
		result[key] = value
	}
	for key, value := range overlay {
		result[key] = value
	}
	return result
}

func mergeStringLists(base []string, overlay []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(base)+len(overlay))
	for _, group := range [][]string{base, overlay} {
		for _, item := range group {
			item = strings.TrimSpace(item)
			if item == "" {
				continue
			}
			if _, exists := seen[item]; exists {
				continue
			}
			seen[item] = struct{}{}
			result = append(result, item)
		}
	}
	sort.Strings(result)
	return result
}

func stringValue(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
