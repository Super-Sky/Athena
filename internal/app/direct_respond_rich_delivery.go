// direct_respond_rich_delivery.go owns app-layer read-model assembly for direct respond rich delivery.
// direct_respond_rich_delivery.go 负责 direct respond 富交付结果在 app 层的 read model 拼装。
package app

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"moss/internal/alerts"
	"moss/internal/automation"
	"moss/internal/contextassets"
	"moss/internal/controlplane"
	platformautomation "moss/internal/extensions/platform/automation"
	platformcontext "moss/internal/extensions/platform/context"
	platformtools "moss/internal/extensions/platform/tools"
	"moss/internal/inspection"
	"moss/internal/knowledge"
	"moss/internal/runtime"
	runtimeintent "moss/internal/runtime/intent"
	runtimescene "moss/internal/runtime/scene"
	"moss/internal/session"
	"moss/internal/workflow"
)

// DirectRespondRichResult is the app-layer read model returned by direct respond adapters.
// DirectRespondRichResult 是 direct respond 适配器返回的 app 层富交付 read model。
type DirectRespondRichResult struct {
	MainAnswer          string                         `json:"main_answer,omitempty"`
	StructuredResult    map[string]any                 `json:"structured_result,omitempty"`
	ResultSummary       *runtime.ResultSummary         `json:"result_summary,omitempty"`
	ContentCards        []runtime.ContentCard          `json:"content_cards,omitempty"`
	RightPanelView      *runtime.RightPanelView        `json:"right_panel_view,omitempty"`
	NextQuestions       []string                       `json:"next_questions,omitempty"`
	ScoreDelta          *runtime.ScoreDelta            `json:"score_delta,omitempty"`
	DeliveryProfile     *runtime.ResultDeliveryProfile `json:"delivery_profile,omitempty"`
	Answer              string                         `json:"answer,omitempty"`
	FollowUpSuggestions []string                       `json:"follow_up_suggestions,omitempty"`
	Verdict             string                         `json:"verdict,omitempty"`
	Decision            string                         `json:"decision,omitempty"`
	Reason              string                         `json:"reason,omitempty"`
}

// DirectRespondRichRequest is the app-layer request projection used to assemble rich delivery fields.
// DirectRespondRichRequest 是 app 层用于拼装富交付字段的请求投影。
type DirectRespondRichRequest struct {
	TaskType              string
	TaskSubtype           string
	Scene                 string
	Query                 string
	WorkspaceID           string
	AppInstanceID         string
	IntegrationInstanceID string
	WorkflowRunID         string
	StepID                string
	TriggerType           string
	AutomationTaskID      string
	UserLanguage          string
	DesiredOutputMode     string
	GlobalContext         map[string]any
	AppContext            map[string]any
	InputPayload          map[string]any
}

// DirectRespondRichInput carries runtime context required by rich delivery assembly.
// DirectRespondRichInput 承载富交付拼装需要的运行时上下文。
type DirectRespondRichInput struct {
	Request        DirectRespondRichRequest
	Prepared       *runtime.PreparedExecution
	RequestID      string
	SessionID      string
	SharedRootDir  string
	CurrentSession *session.Session
}

type directRespondDeliveryProfile struct {
	EmitResultSummary   bool
	EmitCards           bool
	EmitRightPanel      bool
	EmitNextQuestions   bool
	EmitScoreDelta      bool
	EmitKnowledge       bool
	EmitWorkflowPlan    bool
	EmitWorkflowStep    bool
	EmitInteraction     bool
	EmitExecution       bool
	EmitArtifactWrite   bool
	EmitResourceRead    bool
	EmitStructuredParse bool
	EmitLocalTransform  bool
	EmitFactQuality     bool
	EmitRuntimeState    bool
}

// ShouldBuildDirectRespondPlanDraft reports whether a direct respond request should use automation draft fallback handling.
// ShouldBuildDirectRespondPlanDraft 判断 direct respond 请求是否应该使用自动化草案 fallback 处理。
func ShouldBuildDirectRespondPlanDraft(req DirectRespondRichRequest) bool {
	return platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req))
}

// CanonicalizeDirectRespondRichResult fills legacy alias fields and unwraps nested JSON answers.
// CanonicalizeDirectRespondRichResult 补齐 legacy 别名字段，并展开嵌套 JSON answer。
func CanonicalizeDirectRespondRichResult(result *DirectRespondRichResult) {
	if result == nil {
		return
	}
	unwrapNestedDirectRespondRichResult(result)
	if strings.TrimSpace(result.MainAnswer) == "" && strings.TrimSpace(result.Answer) != "" {
		result.MainAnswer = strings.TrimSpace(result.Answer)
	}
	if strings.TrimSpace(result.Answer) == "" && strings.TrimSpace(result.MainAnswer) != "" {
		result.Answer = strings.TrimSpace(result.MainAnswer)
	}
	if len(result.NextQuestions) == 0 && len(result.FollowUpSuggestions) > 0 {
		result.NextQuestions = append([]string(nil), result.FollowUpSuggestions...)
	}
	if len(result.FollowUpSuggestions) == 0 && len(result.NextQuestions) > 0 {
		result.FollowUpSuggestions = append([]string(nil), result.NextQuestions...)
	}
}

func unwrapNestedDirectRespondRichResult(result *DirectRespondRichResult) {
	if result == nil {
		return
	}
	for _, candidate := range []string{strings.TrimSpace(result.MainAnswer), strings.TrimSpace(result.Answer)} {
		if candidate == "" || !(strings.HasPrefix(candidate, "{") && strings.HasSuffix(candidate, "}")) {
			continue
		}
		var nested DirectRespondRichResult
		if err := json.Unmarshal([]byte(candidate), &nested); err != nil {
			continue
		}
		if strings.TrimSpace(nested.MainAnswer) == "" && strings.TrimSpace(nested.Answer) == "" {
			continue
		}
		if strings.TrimSpace(result.MainAnswer) == "" || strings.TrimSpace(result.MainAnswer) == candidate {
			result.MainAnswer = strings.TrimSpace(nested.MainAnswer)
		}
		if strings.TrimSpace(result.Answer) == "" || strings.TrimSpace(result.Answer) == candidate {
			result.Answer = strings.TrimSpace(nested.Answer)
		}
		if result.StructuredResult == nil && nested.StructuredResult != nil {
			result.StructuredResult = nested.StructuredResult
		}
		if result.ResultSummary == nil && nested.ResultSummary != nil {
			result.ResultSummary = nested.ResultSummary
		}
		if len(result.ContentCards) == 0 && len(nested.ContentCards) > 0 {
			result.ContentCards = nested.ContentCards
		}
		if result.RightPanelView == nil && nested.RightPanelView != nil {
			result.RightPanelView = nested.RightPanelView
		}
		if len(result.NextQuestions) == 0 && len(nested.NextQuestions) > 0 {
			result.NextQuestions = append([]string(nil), nested.NextQuestions...)
		}
		if result.ScoreDelta == nil && nested.ScoreDelta != nil {
			result.ScoreDelta = nested.ScoreDelta
		}
		if result.DeliveryProfile == nil && nested.DeliveryProfile != nil {
			result.DeliveryProfile = nested.DeliveryProfile
		}
		if len(result.FollowUpSuggestions) == 0 && len(nested.FollowUpSuggestions) > 0 {
			result.FollowUpSuggestions = append([]string(nil), nested.FollowUpSuggestions...)
		}
		if strings.TrimSpace(result.Verdict) == "" {
			result.Verdict = strings.TrimSpace(nested.Verdict)
		}
		if strings.TrimSpace(result.Decision) == "" {
			result.Decision = strings.TrimSpace(nested.Decision)
		}
		if strings.TrimSpace(result.Reason) == "" {
			result.Reason = strings.TrimSpace(nested.Reason)
		}
		return
	}
}

// EnrichDirectRespondRichResult assembles rich delivery read-model fields after transport parsing.
// EnrichDirectRespondRichResult 在 transport 完成解析后拼装富交付 read model 字段。
func EnrichDirectRespondRichResult(ctx context.Context, application *Service, runtimeTuning controlplane.RuntimeTuning, input DirectRespondRichInput, result *DirectRespondRichResult) {
	if result == nil {
		return
	}
	req := input.Request
	prepared := input.Prepared
	requestID := input.RequestID
	sessionID := input.SessionID
	sharedRootDir := input.SharedRootDir
	currentSession := input.CurrentSession
	CanonicalizeDirectRespondRichResult(result)
	if result.StructuredResult == nil {
		result.StructuredResult = map[string]any{}
	}

	match := runtimescene.Match(runtimescene.MatchInput{
		TaskType:      effectiveTaskType(req),
		Query:         req.Query,
		AppInstanceID: req.AppInstanceID,
	})
	sceneDef, _ := runtimescene.FindDefinition(runtimescene.BuiltinCatalog(), match.Scene)
	if application != nil {
		match = application.MatchScene(ctx, runtimescene.MatchInput{
			TaskType:      effectiveTaskType(req),
			Query:         req.Query,
			AppInstanceID: req.AppInstanceID,
		})
		sceneDef, _ = application.SceneDefinition(ctx, match.Scene)
	}
	delivery := resolveResultDeliveryProfile(req, match.Scene)
	sceneContext := runtimescene.BuildContext(req.AppContext, req.GlobalContext, req.AppInstanceID, req.UserLanguage)
	intentResolution := buildIntentResolutionResult(prepared, req, match.Scene)
	result.StructuredResult["task_type"] = effectiveTaskType(req)
	result.StructuredResult["task_subtype"] = strings.TrimSpace(req.TaskSubtype)
	result.StructuredResult["scene_match"] = map[string]any{
		"scene":    match.Scene,
		"strength": match.Strength,
		"reason":   match.Reason,
	}
	result.StructuredResult["intent_resolution"] = intentResolution
	platformBundle, contextTrace := buildPlatformContextUsage(req, intentResolution, match.Scene)
	contextAssetBundle, contextAssetTrace, contextAssetViews := buildContextAssetsUsage(req)
	contextAssetTrace = contextassets.BuildCandidateTrace(contextAssetBundle, contextAssetTrace, contextassets.CandidateInput{
		Query:      req.Query,
		MainAnswer: result.MainAnswer,
		Answer:     result.Answer,
		TaskType:   effectiveTaskType(req),
		Scene:      match.Scene,
	})
	attachPlatformContextArtifacts(result, platformBundle, contextTrace)
	attachContextAssetArtifacts(result, contextAssetTrace, contextAssetViews)
	attachInteractionArtifacts(result, req, intentResolution, runtimeTuning)
	if suggestion := runtimescene.BuildSwitchSuggestion(match.Score, match.TargetAppInstanceID, match.Reason); suggestion != nil {
		result.StructuredResult["switch_suggestion"] = suggestion
	}
	if sceneContext != nil {
		result.StructuredResult["scene_context"] = sceneContext
	}
	result.DeliveryProfile = &runtime.ResultDeliveryProfile{
		TaskType:          effectiveTaskType(req),
		Scene:             match.Scene,
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
		StableFields:      stableDeliveryFields(delivery),
		OptionalFields:    optionalDeliveryFields(delivery),
	}
	attachExecutionGovernanceArtifacts(result, req, requestID, sessionID, match.Scene)

	if delivery.EmitResultSummary && result.ResultSummary == nil {
		result.ResultSummary = &runtime.ResultSummary{
			Title:    summaryTitle(req, match.Scene),
			Verdict:  strings.TrimSpace(result.Verdict),
			Severity: summarySeverity(req, result),
			Summary:  defaultSummaryText(result),
		}
	}

	if platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req)) {
		draft := buildAutomationPlanDraft(req, result, platformBundle, contextTrace)
		result.StructuredResult["automation_plan_draft"] = draft
		result.StructuredResult["user_visible_explanation"] = draft.UserVisibleExplanation
		result.StructuredResult["automation_create_payload"] = platformautomation.BuildCreatePayload(platformautomation.BuildPayloadInput{
			Query:           strings.TrimSpace(req.Query),
			Timezone:        resolveAutomationTimezone(req),
			Draft:           draft,
			PlatformContext: platformBundle,
		})
		if payload, ok := result.StructuredResult["automation_create_payload"].(*platformautomation.CreatePayload); ok && payload != nil && runtimeTuning.ToolHintEmissionEnabled {
			hints := platformtools.BuildAutomationHints(payload)
			if runtimeTuning.MaxToolHints > 0 && len(hints) > runtimeTuning.MaxToolHints {
				hints = hints[:runtimeTuning.MaxToolHints]
			}
			result.StructuredResult["platform_tool_hints"] = hints
			result.StructuredResult["platform_tool_descriptors"] = platformtools.DefaultDescriptors()
		}
		if strings.TrimSpace(result.MainAnswer) == "" || strings.TrimSpace(result.MainAnswer) == strings.TrimSpace(result.Answer) {
			result.MainAnswer = draft.UserVisibleExplanation
			result.Answer = draft.UserVisibleExplanation
		}
	}
	attachPlatformContextTooling(result, runtimeTuning, platformBundle, contextTrace)

	switch effectiveTaskType(req) {
	case "inspection_task":
		result.StructuredResult["inspection_report"] = inspection.BuildReport(summaryTitle(req, match.Scene), defaultSummaryText(result), summarySeverity(req, result), result.MainAnswer)
	case "integration_event":
		result.StructuredResult["alert"] = alerts.BuildAlert(summaryTitle(req, match.Scene), defaultSummaryText(result), summarySeverity(req, result), defaultString(strings.TrimSpace(req.TriggerType), "runtime_event"))
	case "scheduled_job":
		if !platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req)) {
			result.StructuredResult["automation_task"] = automation.BuildTask(defaultString(strings.TrimSpace(req.AutomationTaskID), "automation-task"), summaryTitle(req, match.Scene), defaultSummaryText(result), map[string]any{"task_type": effectiveTaskType(req)})
			result.StructuredResult["expected_user_message"] = buildExpectedUserMessage(req, result)
			result.StructuredResult["artifact_outline"] = buildArtifactOutline(req, result)
			result.StructuredResult["recommended_follow_up_action"] = buildRecommendedFollowUpAction(req, result)
		}
	}

	if delivery.EmitCards && len(result.ContentCards) == 0 {
		result.ContentCards = []runtime.ContentCard{{
			CardID:   "card-summary",
			CardType: "summary",
			Title:    summaryTitle(req, match.Scene),
			Summary:  defaultSummaryText(result),
			Source:   stableCardSource(req, match.Scene),
		}}
	}

	if delivery.EmitRightPanel && result.RightPanelView == nil {
		result.RightPanelView = &runtime.RightPanelView{
			ViewID:   "right-panel-summary",
			ViewType: "summary",
			View:     "summary",
			Title:    summaryTitle(req, match.Scene),
			Content:  result.MainAnswer,
			Summary:  defaultSummaryText(result),
			Sections: []runtime.RightPanelSection{{
				SectionID: "main-answer",
				Title:     "主结论",
				Body:      result.MainAnswer,
			}},
		}
	}

	if delivery.EmitNextQuestions && len(result.NextQuestions) == 0 {
		fallbackQuestions := buildNextQuestions(req, match.Scene)
		if len(sceneDef.SuggestedQuestions) > 0 {
			fallbackQuestions = append([]string(nil), sceneDef.SuggestedQuestions...)
		}
		result.NextQuestions = runtimescene.ResolveGuideQuestions(sceneContext, fallbackQuestions, req.UserLanguage)
		result.FollowUpSuggestions = append([]string(nil), result.NextQuestions...)
	}

	if delivery.EmitWorkflowPlan {
		if plan := buildWorkflowPlan(req, match); plan != nil {
			result.StructuredResult["workflow_plan"] = plan
		}
	}
	if delivery.EmitWorkflowStep {
		result.StructuredResult["workflow_step_result"] = buildWorkflowStepResult(req, result)
	}
	if delivery.EmitScoreDelta && result.ScoreDelta == nil {
		result.ScoreDelta = buildScoreDelta(req)
	}
	if delivery.EmitKnowledge && runtimeTuning.KnowledgeRetrievalEnabled {
		candidates := []knowledge.Candidate{
			knowledge.BuildCandidate("knowledge", summaryTitle(req, match.Scene), defaultSummaryText(result), map[string]any{"task_type": effectiveTaskType(req)}),
		}
		result.StructuredResult["knowledge_candidates"] = candidates
		hits := []knowledge.RetrievalHit{{
			EntryID:   candidates[0].CandidateID,
			Title:     candidates[0].Title,
			Summary:   candidates[0].Summary,
			SourceRef: effectiveTaskType(req),
		}}
		result.StructuredResult["knowledge_retrieval"] = knowledge.BuildRetrievalPipeline(strings.TrimSpace(req.Query), hits, 1800)
	}
	attachBaseCapabilityArtifacts(result, runtimeTuning, req, requestID, sessionID, sharedRootDir, currentSession)
}

func effectiveTaskType(req DirectRespondRichRequest) string {
	taskType := strings.TrimSpace(req.TaskType)
	if taskType == "" {
		return "chat"
	}
	return taskType
}

func resolveResultDeliveryProfile(req DirectRespondRichRequest, currentScene string) directRespondDeliveryProfile {
	profile := directRespondDeliveryProfile{
		EmitNextQuestions: true,
		EmitInteraction:   true,
	}
	switch {
	case effectiveTaskType(req) == "workflow_step_request":
		profile.EmitResultSummary = true
		profile.EmitCards = true
		profile.EmitRightPanel = true
		profile.EmitWorkflowPlan = true
		profile.EmitWorkflowStep = true
	case effectiveTaskType(req) == "inspection_task":
		profile.EmitResultSummary = true
		profile.EmitCards = true
		profile.EmitRightPanel = true
		profile.EmitScoreDelta = true
		profile.EmitKnowledge = true
	case effectiveTaskType(req) == "integration_event":
		profile.EmitResultSummary = true
		profile.EmitCards = true
		profile.EmitRightPanel = true
		profile.EmitScoreDelta = strings.Contains(strings.ToLower(strings.TrimSpace(req.TriggerType)), "completed")
	case effectiveTaskType(req) == "scheduled_job":
		profile.EmitResultSummary = true
		profile.EmitCards = true
	case currentScene == "security_review":
		profile.EmitResultSummary = true
		profile.EmitCards = true
		profile.EmitRightPanel = true
		profile.EmitKnowledge = true
	case currentScene == "application_dialogue":
		profile.EmitResultSummary = true
	}
	if platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req)) {
		profile.EmitResultSummary = true
		profile.EmitCards = true
		profile.EmitRightPanel = true
		profile.EmitWorkflowPlan = true
	}
	switch strings.TrimSpace(req.DesiredOutputMode) {
	case "execution_governance", "execution_intent", "execution_result":
		profile.EmitExecution = true
	case runtime.DesiredOutputModeArtifactWrite:
		profile.EmitArtifactWrite = true
	case runtime.DesiredOutputModeReadOnlyResourceRead:
		profile.EmitResourceRead = true
	case runtime.DesiredOutputModeStructuredDataParse:
		profile.EmitStructuredParse = true
	case runtime.DesiredOutputModeQueryRuntimeState:
		profile.EmitRuntimeState = true
	case "result_summary":
		profile.EmitResultSummary = true
	case "content_cards", "card_view":
		profile.EmitCards = true
	case "right_panel_view":
		profile.EmitRightPanel = true
	case "score_delta":
		profile.EmitScoreDelta = true
	case "knowledge_candidate":
		profile.EmitKnowledge = true
	case "workflow_plan":
		profile.EmitResultSummary = true
		profile.EmitWorkflowPlan = true
		if effectiveTaskType(req) == "workflow_step_request" {
			profile.EmitWorkflowStep = true
		}
	}
	if hasExplicitExecutionPayload(req) {
		profile.EmitExecution = true
	}
	if runtime.ResolveArtifactWriteRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)) != nil {
		profile.EmitArtifactWrite = true
	}
	if runtime.ResolveReadOnlyResourceRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)) != nil {
		profile.EmitResourceRead = true
	}
	if runtime.ResolveStructuredDataParseRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)) != nil {
		profile.EmitStructuredParse = true
	}
	if runtime.ResolveLocalDataTransformRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)) != nil {
		profile.EmitLocalTransform = true
	}
	if runtime.ResolveFactQualityGateRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode), strings.TrimSpace(req.Query)) != nil {
		profile.EmitFactQuality = true
	}
	if runtime.ResolveRuntimeStateQueryRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)) != nil {
		profile.EmitRuntimeState = true
	}
	return profile
}

func buildWorkflowPlan(req DirectRespondRichRequest, match runtimescene.MatchResult) *workflow.Plan {
	if strings.EqualFold(strings.TrimSpace(req.DesiredOutputMode), "workflow_plan") || strings.EqualFold(strings.TrimSpace(req.DesiredOutputMode), "automation_plan_draft") || effectiveTaskType(req) == "workflow_step_request" || platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req)) || match.Scene == "workflow" {
		plan := workflow.GenerateDefaultPlan(workflow.GenerateInput{
			TaskID:              defaultString(strings.TrimSpace(req.WorkflowRunID), "task-chat"),
			TaskType:            effectiveTaskType(req),
			Scene:               match.Scene,
			SuggestedEntryAppID: match.TargetAppInstanceID,
		})
		if plan != nil && platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req)) {
			draft := buildAutomationPlanDraft(req, nil, platformcontext.BuildBundle(req.GlobalContext), platformcontext.UsageTrace{})
			plan.Goal = draft.Goal
			plan.Summary = draft.Summary
			plan.RiskLevel = draft.RiskLevel
			plan.RequiresConfirmation = draft.RequiresConfirmation
		}
		return plan
	}
	return nil
}

func buildScoreDelta(req DirectRespondRichRequest) *runtime.ScoreDelta {
	delta := 0
	switch effectiveTaskType(req) {
	case "inspection_task":
		delta = 3
	case "integration_event":
		delta = 5
	default:
		delta = 2
	}
	return &runtime.ScoreDelta{
		Dimension:     "growth_signal",
		Delta:         delta,
		ReasonSummary: "event-settled score delta",
		EvidenceRefs:  compactDirectRespondStrings([]string{strings.TrimSpace(req.TriggerType), strings.TrimSpace(req.TaskSubtype)}),
	}
}

func stableDeliveryFields(profile directRespondDeliveryProfile) []string {
	fields := []string{"main_answer", "answer", "structured_result", "follow_up_suggestions", "used_contexts", "context_usage"}
	if profile.EmitInteraction {
		fields = append(fields, "interaction_mode")
	}
	if profile.EmitResultSummary {
		fields = append(fields, "result_summary")
	}
	if profile.EmitCards {
		fields = append(fields, "content_cards")
	}
	if profile.EmitRightPanel {
		fields = append(fields, "right_panel_view")
	}
	if profile.EmitNextQuestions {
		fields = append(fields, "next_questions")
	}
	if profile.EmitScoreDelta {
		fields = append(fields, "score_delta")
	}
	if profile.EmitWorkflowPlan {
		fields = append(fields, "workflow_plan")
	}
	if profile.EmitWorkflowStep {
		fields = append(fields, "workflow_step_result")
	}
	if profile.EmitKnowledge {
		fields = append(fields, "knowledge_candidates")
	}
	if profile.EmitExecution {
		fields = append(fields, "execution_intent", "execution_result")
	}
	if profile.EmitArtifactWrite {
		fields = append(fields, "artifact_write")
	}
	if profile.EmitResourceRead {
		fields = append(fields, "read_only_resource_read")
	}
	if profile.EmitStructuredParse {
		fields = append(fields, "structured_data_parse")
	}
	if profile.EmitLocalTransform {
		fields = append(fields, "local_data_transform")
	}
	if profile.EmitFactQuality {
		fields = append(fields, "fact_quality")
	}
	if profile.EmitRuntimeState {
		fields = append(fields, "runtime_state")
	}
	return fields
}

func optionalDeliveryFields(profile directRespondDeliveryProfile) []string {
	optional := []string{"context_details_requested", "context_details_loaded", "candidate_asset_targets", "candidate_asset_diffs", "candidate_asset_updates"}
	if profile.EmitInteraction {
		optional = append(optional, "interaction_options", "interaction_progress")
	}
	if !profile.EmitResultSummary {
		optional = append(optional, "result_summary")
	}
	if !profile.EmitCards {
		optional = append(optional, "content_cards")
	}
	if !profile.EmitRightPanel {
		optional = append(optional, "right_panel_view")
	}
	if !profile.EmitScoreDelta {
		optional = append(optional, "score_delta")
	}
	if !profile.EmitWorkflowPlan {
		optional = append(optional, "workflow_plan")
	}
	if !profile.EmitWorkflowStep {
		optional = append(optional, "workflow_step_result")
	}
	if !profile.EmitKnowledge {
		optional = append(optional, "knowledge_candidates")
	}
	if !profile.EmitExecution {
		optional = append(optional, "execution_intent", "execution_result")
	}
	if !profile.EmitArtifactWrite {
		optional = append(optional, "artifact_write")
	}
	if !profile.EmitResourceRead {
		optional = append(optional, "read_only_resource_read")
	}
	if !profile.EmitStructuredParse {
		optional = append(optional, "structured_data_parse")
	}
	if !profile.EmitLocalTransform {
		optional = append(optional, "local_data_transform")
	}
	if !profile.EmitFactQuality {
		optional = append(optional, "fact_quality")
	}
	if !profile.EmitRuntimeState {
		optional = append(optional, "runtime_state")
	}
	return optional
}

func attachBaseCapabilityArtifacts(result *DirectRespondRichResult, runtimeTuning controlplane.RuntimeTuning, req DirectRespondRichRequest, requestID, sessionID, sharedRootDir string, currentSession *session.Session) {
	if result == nil || result.StructuredResult == nil {
		return
	}

	if parseRequest := runtime.ResolveStructuredDataParseRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)); parseRequest != nil {
		if parseResult, err := runtime.ParseStructuredData(*parseRequest); err == nil && parseResult != nil {
			result.StructuredResult["structured_data_parse"] = parseResult
		}
	}

	if transformRequest := runtime.ResolveLocalDataTransformRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)); transformRequest != nil {
		if transformResult, err := runtime.TransformLocalData(*transformRequest, req.InputPayload, req.GlobalContext, req.AppContext); err == nil && transformResult != nil {
			result.StructuredResult["local_data_transform"] = transformResult
		}
	}

	if runtimeTuning.FactQualityGateEnabled {
		if factQualityRequest := runtime.ResolveFactQualityGateRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode), strings.TrimSpace(req.Query)); factQualityRequest != nil {
			if factQualityResult := runtime.EvaluateFactQualityGate(*factQualityRequest); factQualityResult != nil {
				result.StructuredResult["fact_quality"] = factQualityResult
			}
		}
	}

	if resourceRequest := runtime.ResolveReadOnlyResourceRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)); resourceRequest != nil {
		if resourceResult := runtime.ReadOnlyResourceRead(*resourceRequest, req.InputPayload, req.GlobalContext, req.AppContext); resourceResult != nil {
			result.StructuredResult["read_only_resource_read"] = resourceResult
		}
	}

	if artifactRequest := runtime.ResolveArtifactWriteRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)); artifactRequest != nil {
		populateArtifactRequestContext(artifactRequest, req, requestID, sessionID)
		if artifactResult, err := runtime.WriteArtifact(runtime.ArtifactWriteInput{
			SharedRootDir:         sharedRootDir,
			Request:               *artifactRequest,
			Content:               result.MainAnswer,
			GenerationReason:      defaultArtifactGenerationReason(req),
			SourceRefs:            defaultArtifactSourceRefs(req, result),
			SourceSummary:         defaultSummaryText(result),
			InputSources:          defaultArtifactInputSources(req),
			EvidenceRefs:          defaultArtifactEvidenceRefs(req),
			Assumptions:           defaultArtifactAssumptions(req),
			RelatedTaskType:       effectiveTaskType(req),
			RelatedScene:          strings.TrimSpace(req.Scene),
			RelatedWorkflowStepID: strings.TrimSpace(req.StepID),
			Summary:               defaultSummaryText(result),
			Description:           defaultArtifactDescriptionText(req, result),
			Completeness:          "complete",
		}); err == nil && artifactResult != nil {
			result.StructuredResult["artifact_write"] = artifactResult
		} else if err != nil {
			result.StructuredResult["artifact_write"] = &runtime.ArtifactWriteResult{
				ArtifactID:            artifactRequest.ArtifactID,
				Filename:              artifactRequest.Filename,
				Kind:                  artifactRequest.Kind,
				Title:                 artifactRequest.Title,
				GenerationReason:      defaultArtifactGenerationReason(req),
				SourceRefs:            defaultArtifactSourceRefs(req, result),
				SourceSummary:         defaultSummaryText(result),
				InputSources:          defaultArtifactInputSources(req),
				EvidenceRefs:          defaultArtifactEvidenceRefs(req),
				Assumptions:           defaultArtifactAssumptions(req),
				RelatedTaskType:       effectiveTaskType(req),
				RelatedScene:          strings.TrimSpace(req.Scene),
				RelatedWorkflowStepID: strings.TrimSpace(req.StepID),
				Completeness:          "partial",
				RequiresConfirmation:  artifactRequest.RequiresConfirmation,
				SafeToAutoWrite:       artifactRequest.SafeToAutoWrite,
				Description:           err.Error(),
				WriteStatus:           "rejected",
			}
		}
	}

	if stateRequest := runtime.ResolveRuntimeStateQueryRequest(req.InputPayload, strings.TrimSpace(req.DesiredOutputMode)); stateRequest != nil {
		if strings.TrimSpace(stateRequest.SessionID) == "" {
			stateRequest.SessionID = sessionID
		}
		if strings.TrimSpace(stateRequest.RequestID) == "" {
			stateRequest.RequestID = requestID
		}
		if strings.TrimSpace(stateRequest.WorkflowRunID) == "" {
			stateRequest.WorkflowRunID = strings.TrimSpace(req.WorkflowRunID)
		}
		var workflowPlan *workflow.Plan
		if plan, ok := result.StructuredResult["workflow_plan"].(*workflow.Plan); ok {
			workflowPlan = plan
		}
		var executionIntent *runtime.ExecutionIntent
		if intent, ok := result.StructuredResult["execution_intent"].(*runtime.ExecutionIntent); ok {
			executionIntent = intent
		}
		var executionResult *runtime.ExecutionResult
		if resolved, ok := result.StructuredResult["execution_result"].(*runtime.ExecutionResult); ok {
			executionResult = resolved
		}
		if stateResult := runtime.QueryRuntimeState(*stateRequest, currentSession, result.StructuredResult, workflowPlan, executionIntent, executionResult, defaultSummaryText(result)); stateResult != nil {
			result.StructuredResult["runtime_state"] = stateResult
		}
	}
}

func populateArtifactRequestContext(request *runtime.ArtifactWriteRequest, req DirectRespondRichRequest, requestID, sessionID string) {
	if request == nil {
		return
	}
	if strings.TrimSpace(request.RequestID) == "" {
		request.RequestID = requestID
	}
	if strings.TrimSpace(request.SessionID) == "" {
		request.SessionID = sessionID
	}
	if strings.TrimSpace(request.ArtifactOwnerKey) == "" || request.ArtifactOwnerType == "" {
		request.ArtifactOwnerType, request.ArtifactOwnerKey = resolveArtifactOwner(req)
	}
	if strings.TrimSpace(request.Title) == "" {
		request.Title = summaryTitle(req, strings.TrimSpace(req.Scene))
	}
}

func resolveArtifactOwner(req DirectRespondRichRequest) (runtime.ArtifactOwnerType, string) {
	candidates := []struct {
		OwnerType runtime.ArtifactOwnerType
		Value     string
	}{
		{OwnerType: runtime.ArtifactOwnerTypeUser, Value: nestedContextValue(req.GlobalContext, req.InputPayload, "user_id")},
		{OwnerType: runtime.ArtifactOwnerTypeWorkspace, Value: strings.TrimSpace(req.WorkspaceID)},
		{OwnerType: runtime.ArtifactOwnerTypeApp, Value: strings.TrimSpace(req.AppInstanceID)},
		{OwnerType: runtime.ArtifactOwnerTypeIntegration, Value: strings.TrimSpace(req.IntegrationInstanceID)},
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.Value) != "" {
			return candidate.OwnerType, strings.TrimSpace(candidate.Value)
		}
	}
	return runtime.ArtifactOwnerTypeWorkspace, "unknown"
}

func nestedContextValue(globalContext, inputPayload map[string]any, key string) string {
	if value := strings.TrimSpace(directRespondStringValue(inputPayload[key])); value != "" {
		return value
	}
	return strings.TrimSpace(directRespondStringValue(globalContext[key]))
}

func defaultArtifactGenerationReason(req DirectRespondRichRequest) string {
	if strings.TrimSpace(req.Query) != "" {
		return "generated from the current task request and written as a delivery artifact"
	}
	return "generated from the current runtime result and written as a delivery artifact"
}

func defaultArtifactSourceRefs(req DirectRespondRichRequest, result *DirectRespondRichResult) []string {
	return compactDirectRespondStrings([]string{
		strings.TrimSpace(req.WorkflowRunID),
		strings.TrimSpace(req.StepID),
		strings.TrimSpace(req.TaskSubtype),
		strings.TrimSpace(result.Decision),
	})
}

func defaultArtifactInputSources(req DirectRespondRichRequest) []string {
	sources := []string{"user_prompt", "structured_result"}
	if len(req.InputPayload) > 0 {
		sources = append(sources, "input_payload")
	}
	if len(req.GlobalContext) > 0 {
		sources = append(sources, "global_context")
	}
	if len(req.AppContext) > 0 {
		sources = append(sources, "app_context")
	}
	return compactDirectRespondStrings(sources)
}

func defaultArtifactEvidenceRefs(req DirectRespondRichRequest) []string {
	return compactDirectRespondStrings([]string{
		strings.TrimSpace(req.WorkflowRunID),
		strings.TrimSpace(req.StepID),
		strings.TrimSpace(req.TriggerType),
	})
}

func defaultArtifactAssumptions(req DirectRespondRichRequest) []string {
	assumptions := []string{}
	if strings.TrimSpace(req.UserLanguage) == "" {
		assumptions = append(assumptions, "default language fallback may apply")
	}
	return assumptions
}

func defaultArtifactDescriptionText(req DirectRespondRichRequest, result *DirectRespondRichResult) string {
	return fmt.Sprintf("delivery artifact for %s generated from the current Athena runtime result", effectiveTaskType(req))
}

func directRespondStringValue(value any) string {
	text, _ := value.(string)
	return text
}

func buildWorkflowStepResult(req DirectRespondRichRequest, result *DirectRespondRichResult) workflow.StepResult {
	executionResult := runtime.ParseExecutionResult(req.InputPayload)
	summary := defaultSummaryText(result)
	decision := defaultString(strings.TrimSpace(result.Decision), "return_full_plan")
	recommendedAction := "consume_latest_plan_and_apply_platform_execution_policy"
	if executionResult != nil {
		summary = defaultString(strings.TrimSpace(executionResult.Status), summary)
		if executionResult.TimedOut {
			decision = "review_execution_timeout"
			recommendedAction = "review_timeout_and_decide_retry_or_plan_adjustment"
		} else if executionResult.SandboxViolation != nil {
			decision = "review_sandbox_violation"
			recommendedAction = "review_sandbox_violation_and_adjust_execution_policy"
		} else if strings.EqualFold(strings.TrimSpace(executionResult.Status), "failed") || executionResult.ExitCode != 0 {
			decision = "review_execution_failure"
			recommendedAction = "review_execution_failure_and_adjust_workflow_plan"
		}
	}
	stepResult := workflow.StepResult{
		WorkflowRunID:      strings.TrimSpace(req.WorkflowRunID),
		StepID:             strings.TrimSpace(req.StepID),
		Summary:            summary,
		Decision:           decision,
		RecommendedAction:  recommendedAction,
		PlanRefreshAllowed: true,
	}
	if plan, ok := result.StructuredResult["workflow_plan"].(*workflow.Plan); ok && plan != nil {
		for _, step := range plan.Steps {
			if step.StepID == stepResult.StepID {
				stepResult.ParallelGroup = step.ParallelGroup
				stepResult.DependsOn = append([]string(nil), step.DependsOn...)
				break
			}
		}
	}
	return stepResult
}

func attachExecutionGovernanceArtifacts(result *DirectRespondRichResult, req DirectRespondRichRequest, requestID, sessionID, currentScene string) {
	if result == nil {
		return
	}
	intent := runtime.ResolveExecutionIntent(runtime.ExecutionGovernanceInput{
		RequestID:         requestID,
		SessionID:         sessionID,
		TaskType:          effectiveTaskType(req),
		Scene:             currentScene,
		WorkflowRunID:     strings.TrimSpace(req.WorkflowRunID),
		StepID:            strings.TrimSpace(req.StepID),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
		InputPayload:      req.InputPayload,
	})
	if intent != nil {
		result.StructuredResult["execution_intent"] = intent
	}
	if executionResult := runtime.ParseExecutionResult(req.InputPayload); executionResult != nil {
		result.StructuredResult["execution_result"] = executionResult
	}
}

func hasExplicitExecutionPayload(req DirectRespondRichRequest) bool {
	if runtime.ParseExecutionResult(req.InputPayload) != nil {
		return true
	}
	return runtime.ResolveExecutionIntent(runtime.ExecutionGovernanceInput{
		RequestID:         "intent-check",
		SessionID:         "session-check",
		TaskType:          effectiveTaskType(req),
		Scene:             strings.TrimSpace(req.Scene),
		WorkflowRunID:     strings.TrimSpace(req.WorkflowRunID),
		StepID:            strings.TrimSpace(req.StepID),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
		InputPayload:      req.InputPayload,
	}) != nil
}

func buildNextQuestions(req DirectRespondRichRequest, currentScene string) []string {
	switch currentScene {
	case "security_review":
		return []string{"是否需要我按 OWASP Top 10 展开风险？", "是否需要我补充 STRIDE 威胁建模？"}
	case "inspection":
		return []string{"是否需要我总结高风险发现？", "是否需要我生成体检后续动作？"}
	case "workflow":
		return []string{"是否需要我展开关键步骤说明？", "是否需要我标出需要确认的高风险步骤？"}
	case "alerts":
		return []string{"是否需要我解释告警原因？", "是否需要我生成处置建议？"}
	default:
		switch effectiveTaskType(req) {
		case "scheduled_job":
			return []string{"是否需要我给出自动化后续建议？", "是否需要我补充执行风险说明？"}
		case "integration_event":
			return []string{"是否需要我解释这个集成事件的影响？", "是否需要我补充下一步动作？"}
		default:
			return []string{"是否需要我继续展开关键结论？", "是否需要我给出下一步建议？"}
		}
	}
}

func summaryTitle(req DirectRespondRichRequest, currentScene string) string {
	switch effectiveTaskType(req) {
	case "inspection_task":
		return "体检结果摘要"
	case "integration_event":
		return "告警结果摘要"
	case "workflow_step_request":
		return "工作流结果摘要"
	case "scheduled_job":
		return "自动化结果摘要"
	default:
		if currentScene == "application_dialogue" {
			return "应用场景结果摘要"
		}
		if currentScene == "security_review" {
			return "安全审计结果摘要"
		}
		return "对话结果摘要"
	}
}

func summarySeverity(req DirectRespondRichRequest, result *DirectRespondRichResult) string {
	if strings.EqualFold(strings.TrimSpace(result.Verdict), "deny") {
		return "high"
	}
	switch effectiveTaskType(req) {
	case "inspection_task", "integration_event":
		return "medium"
	default:
		return ""
	}
}

func defaultSummaryText(result *DirectRespondRichResult) string {
	if result == nil {
		return ""
	}
	if strings.TrimSpace(result.Reason) != "" {
		return strings.TrimSpace(result.Reason)
	}
	return strings.TrimSpace(result.MainAnswer)
}

func buildAutomationPlanDraft(req DirectRespondRichRequest, result *DirectRespondRichResult, platformBundle *platformcontext.Bundle, contextTrace platformcontext.UsageTrace) *automation.PlanDraft {
	planType := inferAutomationPlanType(req.Query)
	if platformBundle != nil && strings.TrimSpace(planType) == "" {
		planType = platformBundle.DomainHint()
	}
	summary := defaultString(strings.TrimSpace(defaultSummaryText(result)), "每天整理上下文并生成摘要。")
	if snippet := firstContextSnippet(platformBundle, contextTrace); snippet != "" {
		summary = defaultString(strings.TrimSpace(defaultSummaryText(result)), fmt.Sprintf("结合平台上下文摘要：%s。", snippet))
	}
	goal := resolveAutomationGoal(req.Query, planType, platformBundle)
	if strings.TrimSpace(planType) == "" {
		planType = "knowledge_refresh"
	}
	requiredCapabilities := []string{"memory", "knowledge_summary", "profile_analysis"}
	requiredIntegrations := []string{}
	if strings.Contains(strings.ToLower(strings.TrimSpace(req.Query)), "news") || strings.Contains(req.Query, "动态") {
		requiredCapabilities = []string{"knowledge_summary", "structured_output"}
	}
	requiredCapabilities = mergeCapabilities(requiredCapabilities, contextCapabilities(platformBundle, contextTrace))
	expectedOutputs := inferExpectedOutputs(planType)
	explanation := buildAutomationExplanation(goal, requiredCapabilities, expectedOutputs, contextTrace.UsedContexts)
	return automation.BuildPlanDraft(
		planType,
		goal,
		summary,
		defaultString(strings.TrimSpace(req.Scene), "main"),
		requiredCapabilities,
		requiredIntegrations,
		expectedOutputs,
		"low",
		true,
		"",
		explanation,
	)
}

func inferAutomationPlanType(query string) string {
	lower := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.Contains(lower, "habit") || strings.Contains(query, "习惯"):
		return "habit_analysis"
	case strings.Contains(lower, "runtime") || strings.Contains(query, "运行情况"):
		return "runtime_daily_summary"
	case strings.Contains(lower, "news") || strings.Contains(query, "动态"):
		return "security_news_digest"
	case strings.Contains(lower, "profile") || strings.Contains(query, "画像"):
		return "profile_refresh"
	default:
		return ""
	}
}

func inferExpectedOutputs(planType string) []string {
	switch planType {
	case "habit_analysis":
		return []string{"habit_summary", "trend_suggestion"}
	case "runtime_daily_summary":
		return []string{"runtime_summary", "risk_highlights"}
	case "security_news_digest":
		return []string{"news_digest", "follow_up_topics"}
	case "profile_refresh":
		return []string{"profile_summary", "change_highlights"}
	default:
		return []string{"knowledge_summary", "refresh_suggestion"}
	}
}

func buildAutomationExplanation(goal string, capabilities, outputs, usedContexts []string) string {
	contextPart := ""
	if len(usedContexts) > 0 {
		contextPart = fmt.Sprintf("我会优先结合这些平台上下文：%s。", strings.Join(usedContexts, "、"))
	}
	return fmt.Sprintf("我理解你的目标是：%s。%s我会按计划收集当前上下文、分析关键信息，并使用这些能力：%s。最终我会产生这些结果：%s。创建正式自动化前需要你确认。", goal, contextPart, strings.Join(capabilities, "、"), strings.Join(outputs, "、"))
}

func buildPlatformContextUsage(req DirectRespondRichRequest, resolution *runtime.IntentResolutionResult, scene string) (*platformcontext.Bundle, platformcontext.UsageTrace) {
	bundle := platformcontext.BuildBundle(req.GlobalContext)
	if bundle == nil {
		return nil, platformcontext.UsageTrace{ContextUsage: map[string]bool{}}
	}
	usage := bundle.ResolveUsage(platformcontext.UsageInput{
		Query:             strings.TrimSpace(req.Query),
		TaskType:          effectiveTaskType(req),
		Scene:             strings.TrimSpace(defaultString(scene, req.Scene)),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
		InteractionMode:   interactionModeValue(resolution),
	})
	return bundle, usage
}

func buildContextAssetsUsage(req DirectRespondRichRequest) (*contextassets.Bundle, contextassets.UsageTrace, contextassets.EffectiveViews) {
	bundle := contextassets.BuildBundle(req.GlobalContext)
	if bundle == nil {
		return nil, contextassets.UsageTrace{}, contextassets.EffectiveViews{}
	}
	usage := bundle.ResolveUsage(contextassets.UsageInput{
		Query:             strings.TrimSpace(req.Query),
		TaskType:          effectiveTaskType(req),
		Scene:             strings.TrimSpace(req.Scene),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
	})
	return bundle, usage, contextassets.BuildEffectiveViews(bundle, usage)
}

func attachPlatformContextArtifacts(result *DirectRespondRichResult, bundle *platformcontext.Bundle, trace platformcontext.UsageTrace) {
	if result == nil || result.StructuredResult == nil {
		return
	}
	if len(trace.UsedContexts) > 0 {
		result.StructuredResult["used_contexts"] = append([]string(nil), trace.UsedContexts...)
	}
	if len(trace.ContextUsage) > 0 {
		result.StructuredResult["context_usage"] = cloneDirectRespondBoolMap(trace.ContextUsage)
	}
	if len(trace.ContextDetailsRequested) > 0 {
		result.StructuredResult["context_details_requested"] = append([]string(nil), trace.ContextDetailsRequested...)
	}
	if loaded := loadedContextTypes(bundle, trace.ContextDetailsRequested); len(loaded) > 0 {
		result.StructuredResult["context_details_loaded"] = loaded
	}
}

// AttachDirectRespondContextAssetArtifacts appends resolved context asset usage into a rich result.
// AttachDirectRespondContextAssetArtifacts 会把已解析的上下文资产使用信息附加到富交付结果。
func AttachDirectRespondContextAssetArtifacts(result *DirectRespondRichResult, trace contextassets.UsageTrace, views contextassets.EffectiveViews) {
	attachContextAssetArtifacts(result, trace, views)
}

func attachContextAssetArtifacts(result *DirectRespondRichResult, trace contextassets.UsageTrace, views contextassets.EffectiveViews) {
	if result == nil || result.StructuredResult == nil {
		return
	}
	if len(trace.UsedContextAssets) > 0 {
		result.StructuredResult["used_context_assets"] = append([]string(nil), trace.UsedContextAssets...)
	}
	if len(trace.ResidentAssets) > 0 {
		result.StructuredResult["resident_assets"] = append([]string(nil), trace.ResidentAssets...)
	}
	if len(trace.OnDemandAssets) > 0 {
		result.StructuredResult["on_demand_assets"] = append([]string(nil), trace.OnDemandAssets...)
	}
	if len(trace.SuppressedAssets) > 0 {
		result.StructuredResult["suppressed_assets"] = append([]string(nil), trace.SuppressedAssets...)
	}
	if len(trace.AssetConflictsResolved) > 0 {
		result.StructuredResult["asset_conflicts_resolved"] = cloneDirectRespondAnySlice(trace.AssetConflictsResolved)
	}
	if len(trace.RequestedAssetDetails) > 0 {
		result.StructuredResult["requested_asset_details"] = append([]string(nil), trace.RequestedAssetDetails...)
	}
	if len(trace.LoadedAssetDetails) > 0 {
		result.StructuredResult["loaded_asset_details"] = append([]string(nil), trace.LoadedAssetDetails...)
	}
	if len(trace.CandidateAssetTargets) > 0 {
		result.StructuredResult["candidate_asset_targets"] = cloneDirectRespondAnySlice(trace.CandidateAssetTargets)
	}
	if len(trace.CandidateAssetDiffs) > 0 {
		result.StructuredResult["candidate_asset_diffs"] = cloneDirectRespondAnySlice(trace.CandidateAssetDiffs)
	}
	if len(trace.AssetUsageTrace) > 0 {
		result.StructuredResult["asset_usage_trace"] = cloneDirectRespondAnySlice(trace.AssetUsageTrace)
	}
	if len(trace.CandidateAssetUpdates) > 0 {
		result.StructuredResult["candidate_asset_updates"] = cloneDirectRespondAnySlice(trace.CandidateAssetUpdates)
	}
	for key, value := range views.AsGlobalContext() {
		result.StructuredResult[key] = value
	}
}

func attachPlatformContextTooling(result *DirectRespondRichResult, runtimeTuning controlplane.RuntimeTuning, bundle *platformcontext.Bundle, trace platformcontext.UsageTrace) {
	if result == nil || result.StructuredResult == nil || !runtimeTuning.ToolHintEmissionEnabled || len(trace.ContextDetailsRequested) == 0 {
		return
	}
	pending := pendingContextTypes(bundle, trace.ContextDetailsRequested)
	if len(pending) == 0 {
		return
	}
	hints := mergeToolHints(extractToolHints(result.StructuredResult["platform_tool_hints"]), platformtools.BuildContextDetailHints(pending))
	if runtimeTuning.MaxToolHints > 0 && len(hints) > runtimeTuning.MaxToolHints {
		hints = hints[:runtimeTuning.MaxToolHints]
	}
	result.StructuredResult["platform_tool_hints"] = hints
	result.StructuredResult["platform_tool_descriptors"] = mergeToolDescriptors(extractToolDescriptors(result.StructuredResult["platform_tool_descriptors"]), platformtools.DefaultDescriptors())
}

func loadedContextTypes(bundle *platformcontext.Bundle, requested []string) []string {
	if bundle == nil || len(requested) == 0 {
		return nil
	}
	loaded := make([]string, 0, len(requested))
	for _, typ := range compactDirectRespondStrings(requested) {
		if bundle.HasDetail(platformcontext.Type(typ)) {
			loaded = append(loaded, typ)
		}
	}
	return loaded
}

func pendingContextTypes(bundle *platformcontext.Bundle, requested []string) []string {
	if len(requested) == 0 {
		return nil
	}
	pending := make([]string, 0, len(requested))
	for _, typ := range compactDirectRespondStrings(requested) {
		if bundle == nil || !bundle.HasDetail(platformcontext.Type(typ)) {
			pending = append(pending, typ)
		}
	}
	return pending
}

func interactionModeValue(resolution *runtime.IntentResolutionResult) string {
	if resolution == nil {
		return ""
	}
	return strings.TrimSpace(resolution.InteractionMode)
}

func resolveAutomationGoal(query, planType string, bundle *platformcontext.Bundle) string {
	query = strings.TrimSpace(query)
	if query != "" && !looksGenericAutomationGoal(query) {
		return query
	}
	switch strings.TrimSpace(planType) {
	case "habit_analysis":
		return "定期分析用户的个人习惯并生成摘要"
	case "runtime_daily_summary":
		return "定期分析当前运行情况并生成摘要"
	case "security_news_digest":
		return "定期汇总安全动态并生成摘要"
	case "profile_refresh":
		return "定期更新用户画像并生成变化摘要"
	case "supply_chain_security":
		return "定期分析近期供应链安全关注点并生成摘要"
	}
	if bundle != nil {
		switch bundle.DomainHint() {
		case "supply_chain_security":
			return "定期分析近期供应链安全关注点并生成摘要"
		case "habit_analysis":
			return "定期分析用户的个人习惯并生成摘要"
		}
	}
	return defaultString(query, "为当前需求生成一份可确认的自动化计划草案")
}

func looksGenericAutomationGoal(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	return directRespondContainsAnyString(normalized,
		"自动化", "automation", "周期", "schedule", "scheduled", "定期", "任务", "task", "计划", "草案", "draft",
	) && !directRespondContainsAnyString(normalized, "习惯", "habit", "供应链", "supply chain", "画像", "profile", "运行", "runtime", "动态", "news")
}

func contextCapabilities(bundle *platformcontext.Bundle, trace platformcontext.UsageTrace) []string {
	values := []string{}
	for _, item := range trace.UsedContexts {
		switch item {
		case "identity":
			values = append(values, "identity_summary")
		case "memory":
			values = append(values, "memory_summary")
		case "knowledge":
			values = append(values, "knowledge_summary")
		case "skills":
			values = append(values, "skills_summary")
		case "persona":
			values = append(values, "persona_summary")
		}
	}
	if bundle != nil {
		values = append(values, bundle.CapabilityHints()...)
	}
	return compactDirectRespondStrings(values)
}

func firstContextSnippet(bundle *platformcontext.Bundle, trace platformcontext.UsageTrace) string {
	if bundle == nil {
		return ""
	}
	for _, typ := range trace.UsedContexts {
		switch typ {
		case "knowledge":
			if text := platformcontext.RenderSummary(bundle.KnowledgeSummary); text != "" {
				return text
			}
		case "memory":
			if text := platformcontext.RenderSummary(bundle.MemorySummary); text != "" {
				return text
			}
		case "identity":
			if text := platformcontext.RenderSummary(bundle.IdentitySummary); text != "" {
				return text
			}
		}
	}
	return ""
}

func mergeCapabilities(base, extra []string) []string {
	seen := map[string]struct{}{}
	result := make([]string, 0, len(base)+len(extra))
	for _, item := range append(base, extra...) {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	return result
}

func mergeToolHints(base, extra []platformtools.ToolInvocationHint) []platformtools.ToolInvocationHint {
	result := append([]platformtools.ToolInvocationHint(nil), base...)
	seen := map[string]struct{}{}
	for _, item := range base {
		seen[toolHintKey(item)] = struct{}{}
	}
	for _, item := range extra {
		key := toolHintKey(item)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, item)
	}
	return result
}

func mergeToolDescriptors(base, extra []platformtools.ToolDescriptor) []platformtools.ToolDescriptor {
	result := append([]platformtools.ToolDescriptor(nil), base...)
	seen := map[string]struct{}{}
	for _, item := range base {
		seen[strings.TrimSpace(item.Name)] = struct{}{}
	}
	for _, item := range extra {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		result = append(result, item)
	}
	return result
}

func extractToolHints(value any) []platformtools.ToolInvocationHint {
	if items, ok := value.([]platformtools.ToolInvocationHint); ok {
		return append([]platformtools.ToolInvocationHint(nil), items...)
	}
	return nil
}

func extractToolDescriptors(value any) []platformtools.ToolDescriptor {
	if items, ok := value.([]platformtools.ToolDescriptor); ok {
		return append([]platformtools.ToolDescriptor(nil), items...)
	}
	return nil
}

func toolHintKey(hint platformtools.ToolInvocationHint) string {
	return strings.TrimSpace(hint.ToolName) + ":" + strings.TrimSpace(hint.WhenToInvoke) + ":" + fmt.Sprintf("%v", hint.Arguments)
}

func cloneDirectRespondBoolMap(input map[string]bool) map[string]bool {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]bool, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func directRespondContainsAnyString(input string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(input, strings.ToLower(strings.TrimSpace(term))) {
			return true
		}
	}
	return false
}

func attachInteractionArtifacts(result *DirectRespondRichResult, req DirectRespondRichRequest, resolution *runtime.IntentResolutionResult, runtimeTuning controlplane.RuntimeTuning) {
	if result == nil || result.StructuredResult == nil {
		return
	}
	mode := interactionModeFromResolution(resolution)
	result.StructuredResult["interaction_mode"] = mode
	if runtimeTuning.ChoiceRequiredEnabled && mode != nil && mode.Mode == "choice_required" {
		result.StructuredResult["interaction_options"] = platformautomation.BuildInteractionOptions(runtimeintent.InteractionMode(mode.Mode))
	}
	if progress := platformautomation.BuildInteractionProgress(runtimeintent.InteractionMode(mode.Mode), runtimeTuning.PlanningProgressEnabled, runtimeTuning.MaxPlanningSteps); progress != nil {
		result.StructuredResult["interaction_progress"] = progress
	}
}

func buildIntentResolutionResult(prepared *runtime.PreparedExecution, req DirectRespondRichRequest, scene string) *runtime.IntentResolutionResult {
	primarySkill := ""
	var allowedTools []string
	if prepared != nil && prepared.Spec != nil {
		primarySkill = strings.TrimSpace(prepared.Spec.Skill.PrimarySkill)
		allowedTools = append([]string(nil), prepared.Spec.Tools.AllowedTools...)
	}
	resolution := runtimeintent.Resolve(buildIntentContext(req, scene, primarySkill, allowedTools))
	resolution = platformautomation.EnhanceResolution(directRespondRouteInput(req), resolution)
	return &runtime.IntentResolutionResult{
		InteractionMode:       string(resolution.InteractionMode),
		Scene:                 resolution.Scene,
		PrimarySkill:          resolution.PrimarySkill,
		AllowedTools:          append([]string(nil), resolution.AllowedTools...),
		SelectedRoute:         resolution.SelectedRoute,
		RequiresClarification: resolution.RequiresClarification,
		Reason:                resolution.Reason,
	}
}

func buildIntentContext(req DirectRespondRichRequest, scene, primarySkill string, allowedTools []string) runtimeintent.Context {
	return runtimeintent.Context{
		Query:             strings.TrimSpace(req.Query),
		TaskType:          effectiveTaskType(req),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
		AutomationTaskID:  strings.TrimSpace(req.AutomationTaskID),
		EntryMode:         runtimeintent.EntryMode(entryMode(req)),
		UserSelectedMode:  runtimeintent.UserSelectedMode(userSelectedMode(req)),
		Scene:             strings.TrimSpace(defaultString(scene, "default")),
		PrimarySkill:      strings.TrimSpace(primarySkill),
		AllowedTools:      append([]string(nil), allowedTools...),
	}
}

func interactionModeFromResolution(resolution *runtime.IntentResolutionResult) *runtime.InteractionModeResult {
	if resolution == nil {
		return nil
	}
	return &runtime.InteractionModeResult{
		Mode:   resolution.InteractionMode,
		Reason: resolution.Reason,
	}
}

func entryMode(req DirectRespondRichRequest) string {
	context := interactionContextMap(req.InputPayload)
	return strings.TrimSpace(interactionContextString(context["entry_mode"]))
}

func directRespondRouteInput(req DirectRespondRichRequest) platformautomation.RouteInput {
	return platformautomation.RouteInput{
		Query:             strings.TrimSpace(req.Query),
		TaskType:          effectiveTaskType(req),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
		AutomationTaskID:  strings.TrimSpace(req.AutomationTaskID),
		EntryMode:         entryMode(req),
		UserSelectedMode:  userSelectedMode(req),
	}
}

func userSelectedMode(req DirectRespondRichRequest) string {
	context := interactionContextMap(req.InputPayload)
	return strings.TrimSpace(interactionContextString(context["user_selected_mode"]))
}

func interactionContextMap(inputPayload map[string]any) map[string]any {
	if len(inputPayload) == 0 {
		return nil
	}
	mapped, _ := inputPayload["interaction_context"].(map[string]any)
	return mapped
}

func interactionContextString(value any) string {
	text, _ := value.(string)
	return strings.TrimSpace(text)
}

func resolveAutomationTimezone(req DirectRespondRichRequest) string {
	for _, values := range []map[string]any{req.InputPayload, req.GlobalContext, req.AppContext} {
		if text := strings.TrimSpace(interactionContextString(values["timezone"])); text != "" {
			return text
		}
		if text := strings.TrimSpace(interactionContextString(values["user_timezone"])); text != "" {
			return text
		}
		if text := strings.TrimSpace(interactionContextString(values["tz"])); text != "" {
			return text
		}
	}
	return "Asia/Shanghai"
}

func buildExpectedUserMessage(req DirectRespondRichRequest, result *DirectRespondRichResult) string {
	return defaultString(strings.TrimSpace(result.MainAnswer), "自动化执行已完成，我已整理出一份可读摘要。")
}

func buildArtifactOutline(req DirectRespondRichRequest, result *DirectRespondRichResult) map[string]any {
	return map[string]any{
		"kind":    "markdown",
		"title":   summaryTitle(req, strings.TrimSpace(req.Scene)),
		"summary": defaultSummaryText(result),
	}
}

func buildRecommendedFollowUpAction(req DirectRespondRichRequest, result *DirectRespondRichResult) string {
	return "write_result_back_to_main_session"
}

func stableCardSource(req DirectRespondRichRequest, scene string) string {
	if platformautomation.ShouldBuildPlanDraft(directRespondRouteInput(req)) {
		return "automation_plan_draft"
	}
	switch effectiveTaskType(req) {
	case "workflow_step_request":
		return "workflow_plan"
	case "inspection_task":
		return "inspection_report"
	case "integration_event":
		return "alert"
	case "scheduled_job":
		return "automation_result"
	default:
		if scene == "application_dialogue" {
			return "application_dialogue"
		}
		return "chat_result"
	}
}

func compactDirectRespondStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func cloneDirectRespondAnySlice(input []map[string]any) []map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(input))
	for _, item := range input {
		result = append(result, cloneAnyMap(item))
	}
	return result
}
