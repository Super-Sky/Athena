// service.go is the central runtime orchestration file that wires context assembly, capability resolution,
// turn execution, turn processing, and loop control into one executable chain.
// service.go 负责 runtime 中央编排，把上下文组装、能力解析、单轮执行、结果处理和 loop 控制接成一条可执行主链。
//
// It exists because Athena needs one stable runtime backbone where policy, waiting, fallback,
// and structured execution behavior can evolve without leaking orchestration concerns into transport or app packages.
// 这个文件存在的原因是 Athena 需要一条稳定的 runtime 骨架，让 policy、waiting、fallback
// 和结构化执行行为可以集中演进，而不把编排细节泄漏到 transport 或 app 包。
//
// Main entry points:
// 主要入口：
// - `Service`
// - `NewService`
// - `DefaultContextAssembler`
// - `DefaultCapabilityResolver`
// - `DefaultLoopController`
//
// Change carefully when:
// 修改时重点注意：
// - `ExecutionSpec` shape changes usually ripple into app, server, and tests
// - waiting/resume semantics affect session persistence and client-visible behavior
// - this file is already large, so new orchestration slices should prefer extraction over further in-place growth
package runtime

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	einomessage "github.com/cloudwego/eino/schema"
	"moss/internal/config"
	"moss/internal/contextassets"
	platformcontext "moss/internal/extensions/platform/context"
	"moss/internal/memory"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	"moss/internal/observability"
	"moss/internal/policy"
	runtimepersona "moss/internal/runtime/persona"
	"moss/internal/runtime/scene"
	runtimetask "moss/internal/runtime/task"
	"moss/internal/session"
	"moss/internal/skills"
	"moss/internal/tools"
)

var userIDPattern = regexp.MustCompile(`\bu\d+\b`)

// PolicyRejectError reports that the current request cannot continue because policy forbids a required step.
// PolicyRejectError 表示当前请求因为策略禁止必要步骤而不能继续。
type PolicyRejectError struct {
	Reason       string
	Message      string
	ClientAction string
	Detail       map[string]any
}

// Error returns the human-readable summary for one policy rejection.
// Error 返回一次策略拒绝的人类可读摘要。
func (e *PolicyRejectError) Error() string {
	if e == nil {
		return "policy rejected the request"
	}
	if strings.TrimSpace(e.Message) != "" {
		return e.Message
	}
	return "policy rejected the request"
}

// Service coordinates the replaceable runtime steps around the default Eino-backed executor.
// Service 负责围绕默认基于 Eino 的执行器编排各个可替换 runtime 步骤。
type Service struct {
	Config             config.Config
	Policy             policy.CapabilityPolicy
	ContextAssembler   ContextAssembler
	CapabilityResolver CapabilityResolver
	TurnExecutor       TurnExecutor
	TurnProcessor      TurnProcessor
	LoopController     LoopController
	Observability      *observability.Manager
}

// SubjectContextRequirement captures whether the current runtime path requires subject identity or subject context.
// SubjectContextRequirement 描述当前 runtime 路径是否要求主体标识或主体上下文。
type SubjectContextRequirement struct {
	Required      bool
	Reason        string
	MissingFields []string
}

// DefaultContextAssembler builds one turn of model input from session history and supplements.
// DefaultContextAssembler 负责根据 session history 和补充信息组装单轮输入上下文。
type DefaultContextAssembler struct {
	Policy memory.ContextPolicy
}

// Assemble normalizes history and any supplemental payload into the final model messages.
// Assemble 会把历史消息与补充数据整理成最终送入模型的消息列表。
func (a DefaultContextAssembler) Assemble(_ context.Context, s *session.Session, in Input) ([]adk.Message, error) {
	prepared := memory.PrepareHistoryWithPreservedContext(s.Messages, preservedContextFromPending(in.Pending), a.Policy)
	result := make([]adk.Message, 0, len(prepared)+1)
	for _, msg := range prepared {
		switch strings.ToLower(msg.Role) {
		case "assistant":
			result = append(result, einomessage.AssistantMessage(msg.Content, nil))
		default:
			result = append(result, einomessage.UserMessage(msg.Content))
		}
	}

	currentTask := ensureRuntimeTask(in)
	query := strings.TrimSpace(in.Query)
	if currentTask != nil && strings.TrimSpace(currentTask.UserGoal) != "" {
		query = strings.TrimSpace(currentTask.UserGoal)
	}
	if in.Supplement != nil && len(in.Supplement.Data) > 0 {
		var supplementParts []string
		keys := make([]string, 0, len(in.Supplement.Data))
		for key := range in.Supplement.Data {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			supplementParts = append(supplementParts, fmt.Sprintf("%s=%s", key, in.Supplement.Data[key]))
		}
		if query != "" {
			query += "\n"
		}
		query += "Supplemental information:\n" + strings.Join(supplementParts, "\n")
	}

	result = append(result, einomessage.UserMessage(query))
	return result, nil
}

// DefaultCapabilityResolver converts skills, tools, policy, and request state into an ExecutionSpec.
// DefaultCapabilityResolver 负责把 skill、tool、policy 与请求状态收敛成 ExecutionSpec。
type DefaultCapabilityResolver struct {
	Registry             skills.Registry
	RegistryProvider     func(context.Context) *skills.Registry
	SceneCatalogProvider func(context.Context) []scene.Definition
	Adapter              skills.Adapter
	Tools                map[string]tools.Definition
	Policy               policy.CapabilityPolicy
}

// Resolve decides the current ExecutionSpec and may emit an information_request action before execution starts.
// Resolve 会决定当前 ExecutionSpec，并可在执行前提前产出 information_request 动作。
func (r DefaultCapabilityResolver) Resolve(ctx context.Context, state RuntimeState, in Input) (*ExecutionSpec, *Action, error) {
	task := ensureRuntimeTask(in)
	effectiveQuery := resolveEffectiveQuery(in)
	orchestration := normalizeOrchestrationInput(in)
	explicitSkills := compactStrings(in.Customization.EnabledSkills)
	explicitTools := compactStrings(in.Customization.EnabledTools)
	registry := &r.Registry
	if r.RegistryProvider != nil {
		if current := r.RegistryProvider(ctx); current != nil {
			registry = current
		}
	}
	sceneCatalog := scene.BuiltinCatalog()
	if r.SceneCatalogProvider != nil {
		if defs := r.SceneCatalogProvider(ctx); len(defs) > 0 {
			sceneCatalog = defs
		}
	}

	selectedSkills := append([]string(nil), explicitSkills...)
	usedAppSkillBundle := false
	if len(selectedSkills) == 0 {
		selectedSkills = taskSkillBundle(task)
		usedAppSkillBundle = len(selectedSkills) > 0
	}
	if len(selectedSkills) == 0 {
		selectedSkills = defaultSceneSkillBundle(task, effectiveQuery, sceneCatalog)
	}
	requestedSkills := append([]string(nil), selectedSkills...)
	if len(selectedSkills) == 0 {
		selectedSkills = defaultBuiltinSkillBundle()
	}
	filteredSkills := r.Policy.FilterSkills(selectedSkills)
	governedSkills := make([]string, 0, len(filteredSkills))
	if len(filteredSkills) == 0 {
		return nil, nil, fmt.Errorf("no skills available after policy filtering")
	}

	toolSet := make(map[string]string)
	var primary skills.AdaptedSkill
	var auxiliary []skills.AdaptedSkill
	for idx, name := range filteredSkills {
		def, ok := registry.Get(name)
		if !ok {
			continue
		}
		governedSkills = append(governedSkills, name)
		adapted := r.Adapter.Adapt(def)
		if idx == 0 {
			primary = adapted
		} else {
			auxiliary = append(auxiliary, adapted)
		}
		for _, toolName := range adapted.ToolNames {
			toolSet[toolName] = adapted.Name
		}
	}
	for _, toolName := range explicitTools {
		if _, ok := r.Tools[toolName]; !ok {
			continue
		}
		if _, exists := toolSet[toolName]; exists {
			continue
		}
		toolSet[toolName] = "explicit"
	}
	if primary.Name == "" {
		return nil, nil, fmt.Errorf("no primary skill resolved")
	}

	allowedTools := make([]string, 0, len(toolSet))
	sources := make(map[string]string, len(toolSet))
	for toolName, source := range toolSet {
		if _, ok := r.Tools[toolName]; !ok {
			continue
		}
		allowedTools = append(allowedTools, toolName)
		sources[toolName] = source
	}
	sort.Strings(allowedTools)
	if len(explicitTools) > 0 {
		allowedTools, sources = restrictAllowedTools(allowedTools, sources, explicitTools)
	}
	requestedTools := append([]string(nil), allowedTools...)
	allowedTools = r.Policy.FilterTools(allowedTools)
	subjectRequirement := resolveSubjectContextRequirement(task, explicitSkills, explicitTools, usedAppSkillBundle, allowedTools)

	guidanceParts := []string{strings.TrimSpace(primary.Guidance)}
	for _, skill := range auxiliary {
		if strings.TrimSpace(skill.Guidance) != "" {
			guidanceParts = append(guidanceParts, skill.Guidance)
		}
	}
	if strings.TrimSpace(in.Customization.PromptTemplate) != "" {
		guidanceParts = append(guidanceParts, fmt.Sprintf("Request-specific prompt override: %s", strings.TrimSpace(in.Customization.PromptTemplate)))
	}
	if persona := taskPersona(task); strings.TrimSpace(persona) != "" {
		guidanceParts = append(guidanceParts, fmt.Sprintf("Current app persona: %s", strings.TrimSpace(persona)))
	}
	if personaContext := taskPersonaContext(task); personaContext != nil {
		guidanceParts = append(guidanceParts, personaContext.GuidanceLines()...)
	}
	if platformUsage := taskPlatformContextUsage(task); platformUsage != nil {
		guidanceParts = append(guidanceParts, platformUsage.GuidanceLines...)
	}
	if assetUsage := taskContextAssetsUsage(task); assetUsage != nil {
		guidanceParts = append(guidanceParts, assetUsage.GuidanceLines...)
	}
	if assetViews := taskContextAssetEffectiveViews(task); assetViews != nil {
		guidanceParts = append(guidanceParts, assetViews.GuidanceLines()...)
	}
	if userLanguage := taskUserLanguage(task); strings.TrimSpace(userLanguage) != "" {
		guidanceParts = append(guidanceParts, fmt.Sprintf("Current user language: %s", strings.TrimSpace(userLanguage)))
	}
	guidanceParts = append(guidanceParts, fmt.Sprintf("Current user request: %s", effectiveQuery))
	if task != nil && strings.TrimSpace(task.UserGoal) != "" {
		guidanceParts = append(guidanceParts, fmt.Sprintf("Current task goal: %s", strings.TrimSpace(task.UserGoal)))
		if strings.TrimSpace(task.TaskType) != "" {
			guidanceParts = append(guidanceParts, fmt.Sprintf("Current task type: %s", strings.TrimSpace(task.TaskType)))
		}
		if strings.TrimSpace(task.TaskSubtype) != "" {
			guidanceParts = append(guidanceParts, fmt.Sprintf("Current task subtype: %s", strings.TrimSpace(task.TaskSubtype)))
		}
		if strings.TrimSpace(task.Scene) != "" {
			guidanceParts = append(guidanceParts, fmt.Sprintf("Current scene: %s", strings.TrimSpace(task.Scene)))
		}
	}

	spec := &ExecutionSpec{
		Skill: SkillSpec{
			PrimarySkill: primary.Name,
			Guidance:     strings.Join(compactStrings(guidanceParts), "\n"),
		},
		Tools: ToolSpec{
			AllowedTools: allowedTools,
			Sources:      sources,
		},
		Model: buildModelSpec(in.ModelSelection),
		Inference: InferenceSpec{
			Goal:       effectiveQuery,
			OutputMode: defaultInferenceOutputMode(task),
			StructuredOutput: &StructuredOutputContract{
				ContractID:    "structured-output.v1",
				Mode:          "structured",
				SchemaName:    "structured_output",
				SchemaVersion: "v1",
				Requested:     true,
			},
		},
		Processing: ProcessingSpec{
			PreferDirectAnswer: false,
			PreferActionFirst:  true,
			GatherInfo:         true,
		},
		Metadata: ExecutionMetadata{
			ResolverReason: "resolved from official skills plus request, policy, and state",
			Governance: &GovernanceDecision{
				Decision: GovernanceDecisionAllow,
				Reason:   "capability_resolution_passed",
			},
			Capability: &CapabilityContract{
				Declarations: CapabilityDeclarations{
					RequestedSkills: requestedSkills,
					RequestedTools:  requestedTools,
				},
				GovernedState: GovernedCapabilities{
					Skills: append([]string(nil), governedSkills...),
					Tools:  append([]string(nil), allowedTools...),
				},
				RuntimeConsumption: RuntimeConsumption{
					ContractName: "ExecutionSpec",
				},
			},
			Orchestration: &OrchestrationStatus{
				EntryState:   orchestration,
				CurrentState: runnableOrchestrationState(orchestration),
				Reason:       runnableOrchestrationReason(orchestration),
			},
			Constraints: map[string]any{
				"turn":                     state.Turn,
				"request_id":               state.RequestID,
				"task_id":                  task.TaskID,
				"task_type":                task.TaskType,
				"task_subtype":             task.TaskSubtype,
				"task_kind":                task.InputKind,
				"scene":                    task.Scene,
				"workspace_id":             task.WorkspaceID,
				"main_session_id":          task.MainSessionID,
				"app_instance_id":          task.AppInstanceID,
				"integration_instance_id":  task.IntegrationInstanceID,
				"workflow_run_id":          task.WorkflowRunID,
				"step_id":                  task.StepID,
				"trigger_type":             task.TriggerType,
				"automation_task_id":       task.AutomationTaskID,
				"desired_output_mode":      task.OutputMode,
				"user_language":            task.UserLanguage,
				"guide_questions":          taskGuideQuestions(task),
				"missing_facts":            append([]string(nil), task.MissingFacts...),
				"subject_context_required": subjectRequirement.Required,
				"subject_context_reason":   subjectRequirement.Reason,
				"orchestration_in":         orchestration,
			},
		},
	}
	spec.Metadata.PreservedContext = buildPreservedContext(in, nil)
	if personaContext := taskPersonaContext(task); personaContext != nil {
		if personaContext.ID != "" {
			spec.Metadata.Constraints["persona_id"] = personaContext.ID
		}
		if personaContext.Name != "" {
			spec.Metadata.Constraints["persona_name"] = personaContext.Name
		}
	}
	if platformUsage := taskPlatformContextUsage(task); platformUsage != nil {
		spec.Metadata.Constraints["used_contexts"] = append([]string(nil), platformUsage.UsedContexts...)
		spec.Metadata.Constraints["context_details_requested"] = append([]string(nil), platformUsage.ContextDetailsRequested...)
	}
	if assetUsage := taskContextAssetsUsage(task); assetUsage != nil {
		spec.Metadata.Constraints["used_context_assets"] = append([]string(nil), assetUsage.UsedContextAssets...)
		spec.Metadata.Constraints["resident_assets"] = append([]string(nil), assetUsage.ResidentAssets...)
		spec.Metadata.Constraints["requested_asset_details"] = append([]string(nil), assetUsage.RequestedAssetDetails...)
	}
	if assetViews := taskContextAssetEffectiveViews(task); assetViews != nil && !assetViews.Empty() {
		for key, value := range assetViews.AsGlobalContext() {
			spec.Metadata.Constraints[key] = value
		}
	}

	policyContext, policyErr := buildModelPolicyContext(task, spec)
	if policyErr != nil {
		return nil, nil, fmt.Errorf("resolve model policy context: %w", policyErr)
	}
	resolvedParameters, policyErr := modelparams.ResolveModelParameters(policyContext)
	if policyErr != nil {
		return nil, nil, fmt.Errorf("resolve model parameters: %w", policyErr)
	}
	spec.Model.ResolvedParameters = &resolvedParameters
	if spec.Model.PrimaryConfig != nil {
		spec.Model.PrimaryConfig.ResolvedParameters = &resolvedParameters
	}
	if spec.Model.FallbackConfig != nil {
		spec.Model.FallbackConfig.ResolvedParameters = &resolvedParameters
	}
	applyResolvedToolChoice(spec, resolvedParameters)
	spec.Metadata.Constraints["model_policy"] = resolvedParameters.PolicyName
	spec.Metadata.Constraints["model_policy_version"] = resolvedParameters.PolicyVersion
	spec.Metadata.Constraints["loop_stage"] = string(policyContext.LoopStage)

	if len(auxiliary) > 0 {
		spec.Skill.AuxiliarySkills = make([]string, 0, len(auxiliary))
		for _, skill := range auxiliary {
			spec.Skill.AuxiliarySkills = append(spec.Skill.AuxiliarySkills, skill.Name)
		}
	}

	timeoutAfter := r.Policy.DefaultWaitTimeout
	if in.TimeoutOverride > 0 && in.TimeoutOverride < timeoutAfter {
		timeoutAfter = in.TimeoutOverride
	}
	if timeoutAfter <= 0 || timeoutAfter > r.Policy.MaxWaitTimeout {
		timeoutAfter = r.Policy.MaxWaitTimeout
	}
	spec.Processing.WaitPolicy.TimeoutAfter = timeoutAfter

	if subjectRequirement.Required && !hasSubjectContext(task, in.Supplement) {
		if shouldContinueWithoutSupplement(in, r.Policy) {
			spec.Metadata.Governance = &GovernanceDecision{
				Decision: GovernanceDecisionDegrade,
				Reason:   string(resolveSupplementOutcome(in)),
				Detail: map[string]any{
					"policy_allow_degrade": true,
				},
			}
			spec.Metadata.Orchestration = &OrchestrationStatus{
				EntryState:   orchestration,
				CurrentState: OrchestrationStateDegraded,
				Reason:       string(resolveSupplementOutcome(in)),
			}
			spec.Metadata.Constraints["degraded"] = true
			spec.Metadata.Constraints["degrade_reason"] = string(resolveSupplementOutcome(in))
			spec.Processing.PreferDirectAnswer = true
			spec.Processing.PreferActionFirst = false
			spec.Processing.GatherInfo = false
			spec.Skill.Guidance = strings.TrimSpace(spec.Skill.Guidance + "\nProceed with a best-effort answer and clearly state that required identifying information was not available.")
			return spec, nil, nil
		}
		if !r.Policy.AllowSupplementRequests {
			spec.Metadata.Governance = &GovernanceDecision{
				Decision: GovernanceDecisionDeny,
				Reason:   "supplement_not_allowed",
				Detail: map[string]any{
					"missing_fields": subjectRequirement.MissingFields,
				},
			}
			spec.Metadata.Orchestration = &OrchestrationStatus{
				EntryState:   orchestration,
				CurrentState: OrchestrationStateAborted,
				Reason:       "supplement_not_allowed",
			}
			return spec, nil, &PolicyRejectError{
				Reason:       "supplement_not_allowed",
				Message:      "missing required information and supplement requests are disabled",
				ClientAction: "stop_and_surface_error",
				Detail: map[string]any{
					"missing_fields": subjectRequirement.MissingFields,
					"stage":          string(StageCapabilityResolution),
				},
			}
		}
		spec.Metadata.Governance = &GovernanceDecision{
			Decision: GovernanceDecisionAsk,
			Reason:   string(ActionTypeInformationRequest),
			Detail: map[string]any{
				"target": string(SupplementTargetClient),
			},
		}
		spec.Metadata.Orchestration = &OrchestrationStatus{
			EntryState:   orchestration,
			CurrentState: OrchestrationStateWaiting,
			Reason:       string(ActionTypeInformationRequest),
		}
		action := &Action{
			Type:    ActionTypeInformationRequest,
			Code:    string(ActionTypeInformationRequest),
			Message: "the selected runtime stage needs supplemental information before it can continue",
			Target:  SupplementTargetClient,
			Schema: &ActionSchema{
				Input: map[string]ActionSchemaField{
					"supplement.data": {
						Type: "object",
					},
					"supplement_outcome": {
						Type:     "string",
						Required: true,
						Enum: []string{
							string(SupplementOutcomeProvided),
							string(SupplementOutcomeUnableToProvide),
							string(SupplementOutcomeTimeoutExpired),
							string(SupplementOutcomeAbandonAndContinue),
							string(SupplementOutcomePendingHuman),
						},
					},
					"resume_token": {
						Type:     "string",
						Required: true,
					},
				},
			},
			InformationRequest: &InformationRequestAction{
				Missing: []MissingInformationItem{{
					Field:    "user_id",
					Reason:   "the selected runtime path requires subject identity or equivalent subject context before it can continue",
					Impact:   "subject-dependent retrieval, profile, and downstream execution decisions cannot be completed without a stable subject anchor",
					Required: true,
				}},
				AllowDegrade:    r.Policy.AllowDegradeWithoutResponse,
				SuggestedAction: "provide the target user_id such as u1001, or attach equivalent subject context before retrying this stage",
				Target:          SupplementTargetClient,
				WaitPolicy: WaitTimeoutPolicy{
					TimeoutAfter: timeoutAfter,
				},
			},
			Payload: map[string]any{
				"missing_fields": subjectRequirement.MissingFields,
			},
			TimeoutPolicy: &WaitTimeoutPolicy{
				TimeoutAfter: timeoutAfter,
			},
			ExpectedResult: &ActionExpectedResult{
				ResumeTokenRequired: true,
				AllowedOutcomes: []SupplementOutcome{
					SupplementOutcomeProvided,
					SupplementOutcomeUnableToProvide,
					SupplementOutcomeTimeoutExpired,
					SupplementOutcomeAbandonAndContinue,
					SupplementOutcomePendingHuman,
				},
			},
		}
		spec.Metadata.PreservedContext = buildPreservedContext(in, action)
		return spec, action, nil
	}

	return spec, nil, nil
}

func defaultInferenceOutputMode(task *runtimetask.RuntimeTask) string {
	if task != nil && strings.TrimSpace(task.OutputMode) != "" {
		return strings.TrimSpace(task.OutputMode)
	}
	return "text"
}

func buildModelSpec(selection *model.Selection) ModelSpec {
	if selection == nil {
		return ModelSpec{}
	}
	requested := ModelEndpoint{
		ProviderID:       selection.Primary.ProviderID,
		ProviderName:     selection.Primary.ProviderName,
		ProviderProtocol: selection.Primary.ProviderProtocol,
		ModelRecordID:    selection.Primary.ModelRecordID,
		ProviderModelID:  selection.Primary.ProviderModelID,
		ModelDisplayName: selection.Primary.ModelDisplayName,
		Headers:          redactHeaders(selection.Primary.Headers),
	}
	spec := ModelSpec{
		Requested:         requested,
		Executed:          requested,
		ExplicitSelection: selection.Explicit,
		PrimaryConfig:     &selection.Primary,
	}
	if selection.Fallback != nil {
		copyFallback := *selection.Fallback
		spec.FallbackConfig = &copyFallback
		spec.FallbackAvailable = true
	}
	return spec
}

func buildModelPolicyContext(task *runtimetask.RuntimeTask, spec *ExecutionSpec) (modelparams.ModelPolicyContext, error) {
	if spec == nil {
		return modelparams.ModelPolicyContext{}, nil
	}
	context := modelparams.ModelPolicyContext{
		AllowedTools: append([]string(nil), spec.Tools.AllowedTools...),
	}
	if task != nil {
		context.TaskType = strings.TrimSpace(task.TaskType)
		context.Scene = strings.TrimSpace(task.Scene)
		context.DesiredOutputMode = strings.TrimSpace(task.OutputMode)
	}
	context.LoopStage = resolveLoopStage(context.TaskType, context.DesiredOutputMode)
	context.StructuredOutputRequired = spec.Inference.StructuredOutput != nil && spec.Inference.StructuredOutput.Requested
	if spec.Metadata.Constraints != nil {
		context.StepType = strings.TrimSpace(stringValue(spec.Metadata.Constraints["step_type"]))
		context.StepRiskLevel = modelparams.StepRiskLevel(strings.TrimSpace(stringValue(spec.Metadata.Constraints["step_risk_level"])))
		context.HasToolCall = modelPolicyBoolValue(spec.Metadata.Constraints["has_tool_call"])
		context.IsRetry = modelPolicyBoolValue(spec.Metadata.Constraints["is_retry"])
		if raw, ok := spec.Metadata.Constraints["model_policy_override"].(map[string]any); ok {
			override, err := modelparams.ParseControlledOverride(raw)
			if err != nil {
				return modelparams.ModelPolicyContext{}, err
			}
			context.ControlledOverride = override
		}
	}
	if task != nil && len(task.InputPayload) > 0 {
		if raw, ok := task.InputPayload["model_policy_override"].(map[string]any); ok {
			override, err := modelparams.ParseControlledOverride(raw)
			if err != nil {
				return modelparams.ModelPolicyContext{}, err
			}
			context.ControlledOverride = override
		}
	}
	return context, nil
}

func resolveLoopStage(taskType, desiredOutputMode string) modelparams.LoopStage {
	switch strings.TrimSpace(desiredOutputMode) {
	case "artifact_write":
		return modelparams.LoopStageArtifactWrite
	case "workflow_plan":
		return modelparams.LoopStageWorkflowPlan
	case "execution_governance", "execution_intent", "execution_result":
		return modelparams.LoopStageExecutionGovernance
	case "next_questions":
		return modelparams.LoopStageNextQuestions
	}
	switch strings.TrimSpace(taskType) {
	case "workflow_step_request":
		return modelparams.LoopStageWorkflowStep
	default:
		return modelparams.LoopStageInitialAnswer
	}
}

func applyResolvedToolChoice(spec *ExecutionSpec, resolved modelparams.ResolvedModelParameters) {
	if spec == nil {
		return
	}
	switch resolved.ToolChoice.Kind {
	case modelparams.ToolChoiceNone:
		spec.Tools.AllowedTools = nil
		spec.Tools.Sources = nil
	case modelparams.ToolChoiceSpecificTool:
		if strings.TrimSpace(resolved.ToolChoice.ToolName) == "" {
			return
		}
		filteredTools := make([]string, 0, 1)
		filteredSources := make(map[string]string, 1)
		for _, toolName := range spec.Tools.AllowedTools {
			if toolName == resolved.ToolChoice.ToolName {
				filteredTools = append(filteredTools, toolName)
				if source, ok := spec.Tools.Sources[toolName]; ok {
					filteredSources[toolName] = source
				}
				break
			}
		}
		spec.Tools.AllowedTools = filteredTools
		spec.Tools.Sources = filteredSources
	}
}

func resolveSubjectContextRequirement(task *runtimetask.RuntimeTask, explicitSkills []string, explicitTools []string, usedAppSkillBundle bool, allowedTools []string) SubjectContextRequirement {
	explicitSubjectCapability := false
	for _, skillName := range explicitSkills {
		if isSubjectDependentSkill(skillName) {
			explicitSubjectCapability = true
			break
		}
	}
	if !explicitSubjectCapability {
		for _, toolName := range explicitTools {
			if isSubjectDependentTool(toolName) {
				explicitSubjectCapability = true
				break
			}
		}
	}
	resolvedSubjectCapability := explicitSubjectCapability
	if !resolvedSubjectCapability && taskAllowsImplicitSubjectRequirement(task) {
		for _, toolName := range allowedTools {
			if isSubjectDependentTool(toolName) {
				resolvedSubjectCapability = true
				break
			}
		}
	}
	if !resolvedSubjectCapability && !taskRequestsSubjectContext(task) {
		return SubjectContextRequirement{}
	}
	return SubjectContextRequirement{
		Required:      true,
		Reason:        defaultSubjectContextReason(subjectContextReason(task, explicitSubjectCapability, resolvedSubjectCapability, usedAppSkillBundle)),
		MissingFields: []string{"user_id"},
	}
}

func resolveProvidedUserID(supplement *SupplementPayload) string {
	if supplement == nil {
		return ""
	}
	return strings.TrimSpace(supplement.Data["user_id"])
}

func redactHeaders(headers map[string]string) map[string]string {
	if len(headers) == 0 {
		return nil
	}
	result := make(map[string]string, len(headers))
	for key := range headers {
		result[key] = "***redacted***"
	}
	return result
}

// DefaultTurnProcessor handles the current first-phase action-to-result processing path.
// DefaultTurnProcessor 负责当前第一阶段动作到结果的处理路径。
type DefaultTurnProcessor struct{}

// ProcessAction wraps a transport-facing control action into the normalized turn result envelope.
// ProcessAction 会把对外控制动作包装成统一的 TurnResult。
func (DefaultTurnProcessor) ProcessAction(_ context.Context, _ RuntimeState, action *Action) (*TurnResult, error) {
	if action == nil {
		return &TurnResult{Kind: TurnResultFinal}, nil
	}
	return &TurnResult{
		Kind:   TurnResultAction,
		Action: action,
	}, nil
}

// DefaultLoopController derives waiting behavior from the current information request and policy.
// DefaultLoopController 负责根据当前信息补充动作和 policy 计算等待行为。
type DefaultLoopController struct {
	Policy policy.CapabilityPolicy
}

// Evaluate turns an information request into a waiting state with an effective timeout.
// Evaluate 会把 information_request 转换成带有效超时的等待状态。
func (c DefaultLoopController) Evaluate(_ context.Context, _ RuntimeState, result *TurnResult) (*WaitState, TimeoutOutcome) {
	if result == nil || result.Action == nil || result.Action.InformationRequest == nil {
		return nil, TimeoutOutcomeFinish
	}
	timeoutAfter := result.Action.InformationRequest.WaitPolicy.TimeoutAfter
	if timeoutAfter <= 0 {
		timeoutAfter = c.Policy.DefaultWaitTimeout
	}
	if timeoutAfter <= 0 || timeoutAfter > c.Policy.MaxWaitTimeout {
		timeoutAfter = c.Policy.MaxWaitTimeout
	}
	waitState := &WaitState{
		Stage:        StageCapabilityResolution,
		StartedAt:    time.Now(),
		TimeoutAfter: timeoutAfter,
		TimeoutAt:    time.Now().Add(timeoutAfter),
		ResumeToken:  fmt.Sprintf("resume-%d", time.Now().UnixNano()),
	}
	return waitState, TimeoutOutcomePending
}

// NewService wires the default runtime skeleton used by the scaffold.
// NewService 负责装配当前脚手架默认使用的 runtime 骨架。
func NewService(
	cfg config.Config,
	p policy.CapabilityPolicy,
	contextPolicy memory.ContextPolicy,
	registry *skills.Registry,
	adapter skills.Adapter,
	toolDefs map[string]tools.Definition,
	turnExecutor TurnExecutor,
	obs *observability.Manager,
) *Service {
	reg := skills.Registry{}
	if registry != nil {
		reg = *registry
	}
	return &Service{
		Config:             cfg,
		Policy:             p,
		ContextAssembler:   DefaultContextAssembler{Policy: contextPolicy},
		CapabilityResolver: DefaultCapabilityResolver{Registry: reg, Adapter: adapter, Tools: toolDefs, Policy: p},
		TurnExecutor:       turnExecutor,
		TurnProcessor:      DefaultTurnProcessor{},
		LoopController:     DefaultLoopController{Policy: p},
		Observability:      obs,
	}
}

// Prepare executes the fixed runtime stages until the request becomes runnable, waiting, or terminal.
// Prepare 会依次执行固定 runtime 阶段，直到请求进入可运行、等待或终止状态。
func (s *Service) Prepare(ctx context.Context, sess *session.Session, in Input) (*PreparedExecution, error) {
	state := RuntimeState{
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Turn:      len(sess.Messages) + 1,
	}
	orchestration := normalizeOrchestrationInput(in)

	s.Observability.Emit(ctx, observability.Event{
		Name:      "runtime.prepare.started",
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Turn:      state.Turn,
		Stage:     string(StageContextAssembly),
	})
	s.Observability.Trace(ctx, "runtime.prepare.started", map[string]string{
		"request_id": state.RequestID,
		"session_id": state.SessionID,
		"turn":       fmt.Sprintf("%d", state.Turn),
	})
	s.Observability.Inc("runtime_prepare_started_total", map[string]string{
		"stage": string(StageContextAssembly),
	})

	messages, err := s.ContextAssembler.Assemble(ctx, sess, in)
	if err != nil {
		return nil, err
	}
	s.Observability.Emit(ctx, observability.Event{
		Name:      "runtime.context_assembled",
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Turn:      state.Turn,
		Stage:     string(StageContextAssembly),
		Detail: map[string]any{
			"message_count": len(messages),
		},
	})

	spec, action, err := s.CapabilityResolver.Resolve(ctx, state, in)
	if err != nil {
		var policyReject *PolicyRejectError
		if errors.As(err, &policyReject) {
			s.Observability.Emit(ctx, observability.Event{
				Name:      "runtime.policy_rejected",
				RequestID: in.RequestID,
				SessionID: in.SessionID,
				Turn:      state.Turn,
				Stage:     string(StageCapabilityResolution),
				Level:     string(observability.LogLevelWarn),
				Detail: map[string]any{
					"reason":        policyReject.Reason,
					"client_action": policyReject.ClientAction,
				},
			})
			return &PreparedExecution{
				Spec: spec,
				Initial: &TurnResult{
					Kind:  TurnResultError,
					Error: policyReject.Error(),
				},
				InitialError: &ProtocolError{
					Code:         string(RequestStatusPolicyRejected),
					Reason:       policyReject.Reason,
					Retryable:    false,
					ClientAction: policyReject.ClientAction,
					Detail:       policyReject.Detail,
				},
				InitialStatus:    RequestStatusPolicyRejected,
				StructuredOutput: structuredOutputOutcome(spec, false, "policy_rejected", nil),
				Governance: &GovernanceDecision{
					Decision: GovernanceDecisionDeny,
					Reason:   policyReject.Reason,
					Detail:   cloneGovernanceDetail(policyReject.Detail),
				},
			}, nil
		}
		return nil, err
	}
	s.Observability.Emit(ctx, observability.Event{
		Name:      "runtime.capability_resolved",
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Turn:      state.Turn,
		Stage:     string(StageCapabilityResolution),
		Detail: map[string]any{
			"primary_skill":    spec.Skill.PrimarySkill,
			"auxiliary_skills": spec.Skill.AuxiliarySkills,
			"allowed_tools":    spec.Tools.AllowedTools,
			"has_action":       action != nil,
		},
	})
	if outcome := resolveSupplementOutcome(in); outcome == SupplementOutcomeTimeoutExpired && !s.Policy.AllowDegradeWithoutResponse {
		spec.Metadata.Orchestration = &OrchestrationStatus{
			EntryState:   orchestration,
			CurrentState: OrchestrationStateAborted,
			Reason:       "timeout_closed",
		}
		s.Observability.Emit(ctx, observability.Event{
			Name:      "runtime.timeout_closed",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Turn:      state.Turn,
			Stage:     string(StageCapabilityResolution),
			Level:     string(observability.LogLevelWarn),
			Detail: map[string]any{
				"outcome":              string(SupplementOutcomeTimeoutExpired),
				"policy_allow_degrade": false,
			},
		})
		s.Observability.Inc("runtime_timeout_total", map[string]string{
			"decision": "finish",
		})
		s.Observability.RecordAudit(ctx, observability.AuditRecord{
			Action:    "waiting_timeout_finished",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Detail: map[string]any{
				"policy_allow_degrade": false,
				"governance": map[string]any{
					"decision": GovernanceDecisionDeny,
					"reason":   "timeout_closed",
				},
			},
		})
		return &PreparedExecution{
			Spec: spec,
			Initial: &TurnResult{
				Kind:  TurnResultError,
				Error: "waiting for supplemental information timed out before required data was provided",
			},
			InitialError: &ProtocolError{
				Code:         string(RequestStatusTimedOut),
				Reason:       "timeout_closed",
				Retryable:    false,
				ClientAction: "inspect_gap_closed_notification",
				Detail: map[string]any{
					"close_reason":         string(SupplementOutcomeTimeoutExpired),
					"policy_allow_degrade": false,
					"next_step":            "timeout_policy",
				},
			},
			InitialStatus:    RequestStatusTimedOut,
			StructuredOutput: structuredOutputOutcome(spec, false, "timeout_closed", nil),
			Governance: &GovernanceDecision{
				Decision: GovernanceDecisionDeny,
				Reason:   "timeout_closed",
				Detail: map[string]any{
					"policy_allow_degrade": false,
				},
			},
		}, nil
	}
	if outcome := resolveSupplementOutcome(in); outcome == SupplementOutcomeUnableToProvide && !s.Policy.AllowDegradeWithoutResponse {
		spec.Metadata.Orchestration = &OrchestrationStatus{
			EntryState:   orchestration,
			CurrentState: OrchestrationStateAborted,
			Reason:       "policy_reject",
		}
		s.Observability.Emit(ctx, observability.Event{
			Name:      "runtime.missing_information_unresolved",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Turn:      state.Turn,
			Stage:     string(StageCapabilityResolution),
			Level:     string(observability.LogLevelWarn),
			Detail: map[string]any{
				"outcome":              string(SupplementOutcomeUnableToProvide),
				"policy_allow_degrade": false,
			},
		})
		s.Observability.Inc("runtime_missing_information_unresolved_total", map[string]string{
			"decision": "finish",
		})
		s.Observability.RecordAudit(ctx, observability.AuditRecord{
			Action:    "missing_information_unresolved",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Detail: map[string]any{
				"policy_allow_degrade": false,
				"governance": map[string]any{
					"decision": GovernanceDecisionDeny,
					"reason":   "missing_information_unresolved",
				},
			},
		})
		return &PreparedExecution{
			Spec: spec,
			Initial: &TurnResult{
				Kind:  TurnResultError,
				Error: "required supplemental information could not be provided",
			},
			InitialError: &ProtocolError{
				Code:         string(RequestStatusMissingInformationUnresolved),
				Reason:       "policy_reject",
				Retryable:    false,
				ClientAction: "stop_and_surface_error",
				Detail: map[string]any{
					"close_reason":         string(SupplementOutcomeUnableToProvide),
					"policy_allow_degrade": false,
					"next_step":            "policy_decide",
				},
			},
			InitialStatus:    RequestStatusMissingInformationUnresolved,
			StructuredOutput: structuredOutputOutcome(spec, false, "missing_information_unresolved", nil),
			Governance: &GovernanceDecision{
				Decision: GovernanceDecisionDeny,
				Reason:   "missing_information_unresolved",
				Detail: map[string]any{
					"policy_allow_degrade": false,
				},
			},
		}, nil
	}
	if outcome := resolveSupplementOutcome(in); outcome == SupplementOutcomePendingHuman {
		spec.Metadata.Orchestration = &OrchestrationStatus{
			EntryState:   orchestration,
			CurrentState: OrchestrationStateAborted,
			Reason:       string(SupplementOutcomePendingHuman),
		}
		s.Observability.Emit(ctx, observability.Event{
			Name:      "runtime.pending_human_requested",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Turn:      state.Turn,
			Stage:     string(StageCapabilityResolution),
			Detail: map[string]any{
				"requested_outcome": string(SupplementOutcomePendingHuman),
			},
		})
		s.Observability.Inc("runtime_pending_human_total", map[string]string{
			"reason": string(SupplementOutcomePendingHuman),
		})
		s.Observability.RecordAudit(ctx, observability.AuditRecord{
			Action:    "pending_human_requested",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Detail: map[string]any{
				"requested_outcome": string(SupplementOutcomePendingHuman),
				"governance": map[string]any{
					"decision": GovernanceDecisionPendingHuman,
					"reason":   string(SupplementOutcomePendingHuman),
				},
			},
		})
		return &PreparedExecution{
			Spec: spec,
			Initial: &TurnResult{
				Kind: TurnResultAction,
				Action: &Action{
					Type:    ActionTypePendingHuman,
					Code:    string(ActionTypePendingHuman),
					Message: "the current gap should be handled outside the automatic flow",
					Target:  SupplementTargetClient,
					PendingHuman: &PendingHumanAction{
						Reason:          "required supplemental information will not be resolved in the current automatic flow",
						SuggestedAction: "let the client or its upstream system take over this pending gap and re-enter later if new information becomes available",
						Target:          SupplementTargetClient,
						Context: map[string]string{
							"requested_outcome": string(SupplementOutcomePendingHuman),
						},
					},
					Payload: map[string]any{
						"requested_outcome": string(SupplementOutcomePendingHuman),
					},
				},
			},
			InitialStatus:    RequestStatusPendingHuman,
			Orchestration:    OrchestrationStateAborted,
			StructuredOutput: structuredOutputOutcome(spec, false, "pending_human", nil),
			Governance: &GovernanceDecision{
				Decision: GovernanceDecisionPendingHuman,
				Reason:   string(SupplementOutcomePendingHuman),
				Detail: map[string]any{
					"requested_outcome": string(SupplementOutcomePendingHuman),
				},
			},
		}, nil
	}
	if action != nil {
		if spec.Metadata.Orchestration == nil {
			spec.Metadata.Orchestration = &OrchestrationStatus{
				EntryState:   orchestration,
				CurrentState: OrchestrationStateWaiting,
				Reason:       string(ActionTypeInformationRequest),
			}
		}
		result, err := s.TurnProcessor.ProcessAction(ctx, state, action)
		if err != nil {
			return nil, err
		}
		result.Orchestration = OrchestrationStateWaiting
		waitState, _ := s.LoopController.Evaluate(ctx, state, result)
		result.WaitState = waitState
		s.Observability.Inc("runtime_waiting_total", map[string]string{
			"stage": string(StageCapabilityResolution),
			"skill": spec.Skill.PrimarySkill,
		})
		s.Observability.Emit(ctx, observability.Event{
			Name:      "runtime.waiting_for_information",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Turn:      state.Turn,
			Stage:     string(StageCapabilityResolution),
			Detail: map[string]any{
				"primary_skill": spec.Skill.PrimarySkill,
				"timeout_at":    waitState.TimeoutAt,
			},
		})
		s.Observability.RecordAudit(ctx, observability.AuditRecord{
			Action:    "information_request_created",
			RequestID: in.RequestID,
			SessionID: in.SessionID,
			Detail: map[string]any{
				"primary_skill": spec.Skill.PrimarySkill,
				"missing_count": len(action.InformationRequest.Missing),
				"timeout_at":    waitState.TimeoutAt,
				"resume_token":  waitState.ResumeToken,
				"governance":    cloneGovernanceDetailMap(spec.Metadata.Governance),
			},
		})
		return &PreparedExecution{
			Spec:          spec,
			Messages:      messages,
			Initial:       result,
			TimeoutWait:   waitState,
			InitialStatus: RequestStatusWaitingForInformation,
			Orchestration: OrchestrationStateWaiting,
			StructuredOutput: structuredOutputOutcome(spec, false, "waiting_for_information", map[string]any{
				"missing_count": len(action.InformationRequest.Missing),
			}),
			Governance: cloneGovernanceDecision(spec.Metadata.Governance),
		}, nil
	}

	prepared, err := s.TurnExecutor.Prepare(ctx, state, spec, messages)
	if err != nil {
		return nil, err
	}
	prepared.Spec = spec
	prepared.Orchestration = resolvePreparedOrchestration(orchestration, spec.Metadata.Orchestration)
	s.Observability.Inc("runtime_prepare_completed_total", map[string]string{
		"skill": spec.Skill.PrimarySkill,
	})
	s.Observability.Emit(ctx, observability.Event{
		Name:      "runtime.prepare.completed",
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Turn:      state.Turn,
		Stage:     string(StageTurnExecution),
		Detail: map[string]any{
			"primary_skill":    spec.Skill.PrimarySkill,
			"auxiliary_skills": spec.Skill.AuxiliarySkills,
			"allowed_tools":    spec.Tools.AllowedTools,
			"persona_id":       stringValue(spec.Metadata.Constraints["persona_id"]),
			"persona_name":     stringValue(spec.Metadata.Constraints["persona_name"]),
		},
	})
	s.Observability.RecordAudit(ctx, observability.AuditRecord{
		Action:    "execution_spec_prepared",
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Detail: map[string]any{
			"primary_skill":     spec.Skill.PrimarySkill,
			"auxiliary_skills":  spec.Skill.AuxiliarySkills,
			"allowed_tools":     spec.Tools.AllowedTools,
			"persona_id":        stringValue(spec.Metadata.Constraints["persona_id"]),
			"persona_name":      stringValue(spec.Metadata.Constraints["persona_name"]),
			"structured_output": cloneStructuredOutputDetail(spec.Inference.StructuredOutput),
			"governance":        cloneGovernanceDetailMap(spec.Metadata.Governance),
		},
	})
	output := structuredOutputOutcome(spec, false, "text_stream_only", map[string]any{
		"transport_mode": "sse_text_stream",
	})
	s.Observability.RecordAudit(ctx, observability.AuditRecord{
		Action:    "structured_output_emission_planned",
		RequestID: in.RequestID,
		SessionID: in.SessionID,
		Detail: map[string]any{
			"structured_output": cloneStructuredOutputDetail(output),
		},
	})
	prepared.StructuredOutput = output
	prepared.Governance = cloneGovernanceDecision(spec.Metadata.Governance)
	return prepared, nil
}

func structuredOutputOutcome(spec *ExecutionSpec, emitted bool, fallbackReason string, detail map[string]any) *StructuredOutputContract {
	if spec == nil || spec.Inference.StructuredOutput == nil {
		return nil
	}
	contract := *spec.Inference.StructuredOutput
	contract.Emitted = emitted
	contract.FallbackReason = fallbackReason
	if len(detail) > 0 {
		contract.Detail = cloneStructuredMap(detail)
	}
	return &contract
}

func cloneStructuredOutputDetail(contract *StructuredOutputContract) map[string]any {
	if contract == nil {
		return nil
	}
	detail := map[string]any{
		"contract_id":    contract.ContractID,
		"mode":           contract.Mode,
		"schema_name":    contract.SchemaName,
		"schema_version": contract.SchemaVersion,
		"requested":      contract.Requested,
		"emitted":        contract.Emitted,
	}
	if contract.FallbackReason != "" {
		detail["fallback_reason"] = contract.FallbackReason
	}
	if len(contract.Detail) > 0 {
		detail["detail"] = cloneStructuredMap(contract.Detail)
	}
	if contract.Decision != nil {
		detail["decision"] = map[string]any{
			"verdict": contract.Decision.Verdict,
			"reason":  contract.Decision.Reason,
			"detail":  cloneStructuredMap(contract.Decision.Detail),
		}
	}
	return detail
}

func cloneStructuredMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func cloneGovernanceDecision(input *GovernanceDecision) *GovernanceDecision {
	if input == nil {
		return nil
	}
	return &GovernanceDecision{
		Decision: input.Decision,
		Reason:   input.Reason,
		Detail:   cloneGovernanceDetail(input.Detail),
	}
}

func cloneGovernanceDetail(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func cloneGovernanceDetailMap(input *GovernanceDecision) map[string]any {
	if input == nil {
		return nil
	}
	return map[string]any{
		"decision": input.Decision,
		"reason":   input.Reason,
		"detail":   cloneGovernanceDetail(input.Detail),
	}
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func restrictAllowedTools(allowedTools []string, sources map[string]string, explicitTools []string) ([]string, map[string]string) {
	if len(explicitTools) == 0 {
		return allowedTools, sources
	}
	allowedSet := make(map[string]struct{}, len(explicitTools))
	for _, toolName := range explicitTools {
		toolName = strings.TrimSpace(toolName)
		if toolName != "" {
			allowedSet[toolName] = struct{}{}
		}
	}
	filteredTools := make([]string, 0, len(allowedTools))
	filteredSources := make(map[string]string, len(sources))
	for _, toolName := range allowedTools {
		if _, ok := allowedSet[toolName]; !ok {
			continue
		}
		filteredTools = append(filteredTools, toolName)
		if source, ok := sources[toolName]; ok {
			filteredSources[toolName] = source
		}
	}
	return filteredTools, filteredSources
}

func ensureRuntimeTask(in Input) *runtimetask.RuntimeTask {
	if in.Task != nil {
		return in.Task
	}
	var knownFacts map[string]string
	if in.Supplement != nil && len(in.Supplement.Data) > 0 {
		knownFacts = in.Supplement.Data
	}
	return runtimetask.NormalizeChatRequest(in.RequestID, in.Query, knownFacts)
}

func resolveUserID(task *runtimetask.RuntimeTask, supplement *SupplementPayload) string {
	if supplement != nil {
		if value := strings.TrimSpace(supplement.Data["user_id"]); value != "" {
			return value
		}
	}
	if task != nil {
		if value := strings.TrimSpace(task.KnownFacts["user_id"]); value != "" {
			return value
		}
		if value := stringValue(task.GlobalContext["user_id"]); value != "" {
			return value
		}
		if value := stringValue(task.AppContext["user_id"]); value != "" {
			return value
		}
		return userIDPattern.FindString(task.UserGoal)
	}
	return ""
}

func hasSubjectContext(task *runtimetask.RuntimeTask, supplement *SupplementPayload) bool {
	if resolveUserID(task, supplement) != "" {
		return true
	}
	if supplement != nil && containsSubjectContextMap(stringAnyMap(supplement.Data)) {
		return true
	}
	if task == nil {
		return false
	}
	return containsSubjectContextMap(task.GlobalContext) || containsSubjectContextMap(task.AppContext) || containsSubjectContextMap(task.InputPayload)
}

func containsSubjectContextMap(values map[string]any) bool {
	if len(values) == 0 {
		return false
	}
	for _, key := range []string{"subject_context", "user_context", "user_profile"} {
		if value, ok := values[key]; ok {
			switch typed := value.(type) {
			case map[string]any:
				if len(typed) > 0 {
					return true
				}
			case []any:
				if len(typed) > 0 {
					return true
				}
			case string:
				if strings.TrimSpace(typed) != "" {
					return true
				}
			}
		}
	}
	return false
}

func stringAnyMap(values map[string]string) map[string]any {
	if len(values) == 0 {
		return nil
	}
	result := make(map[string]any, len(values))
	for key, value := range values {
		result[key] = value
	}
	return result
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func modelPolicyBoolValue(value any) bool {
	typed, _ := value.(bool)
	return typed
}

func taskRequestsSubjectContext(task *runtimetask.RuntimeTask) bool {
	if task == nil {
		return false
	}
	for _, values := range []map[string]any{task.GlobalContext, task.AppContext, task.InputPayload} {
		for _, key := range []string{"subject_context_required", "requires_user_context"} {
			if raw, ok := values[key]; ok {
				if required, ok := raw.(bool); ok && required {
					return true
				}
			}
		}
	}
	return false
}

func taskAllowsImplicitSubjectRequirement(task *runtimetask.RuntimeTask) bool {
	if task == nil {
		return true
	}
	taskType := strings.TrimSpace(task.TaskType)
	return taskType == "" || taskType == runtimetask.InputKindChat
}

func subjectContextReason(task *runtimetask.RuntimeTask, explicitSubjectCapability bool, resolvedSubjectCapability bool, usedAppSkillBundle bool) string {
	switch {
	case explicitSubjectCapability && usedAppSkillBundle:
		return "app_bundle_requires_subject_context"
	case explicitSubjectCapability:
		return "explicit_capability_requires_subject_context"
	case resolvedSubjectCapability:
		return "resolved_capability_requires_subject_context"
	case taskRequestsSubjectContext(task):
		return "task_context_requires_subject_context"
	default:
		return ""
	}
}

func defaultSubjectContextReason(reason string) string {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return "subject_dependent_runtime_path"
	}
	return reason
}

func isSubjectDependentSkill(skillName string) bool {
	return strings.TrimSpace(skillName) == "user_overview"
}

func isSubjectDependentTool(toolName string) bool {
	return strings.HasPrefix(strings.TrimSpace(toolName), "lookup_")
}

func resolveSupplementOutcome(in Input) SupplementOutcome {
	if in.Supplement != nil && in.Supplement.Outcome != "" {
		return in.Supplement.Outcome
	}
	if in.Pending != nil && !in.Pending.TimeoutAt.IsZero() && time.Now().After(in.Pending.TimeoutAt) {
		return SupplementOutcomeTimeoutExpired
	}
	return ""
}

func taskSkillBundle(task *runtimetask.RuntimeTask) []string {
	if task == nil || len(task.AppContext) == 0 {
		return nil
	}
	raw, ok := task.AppContext["skills"]
	if !ok {
		return nil
	}
	switch items := raw.(type) {
	case []string:
		return compactStrings(items)
	case []any:
		values := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return compactStrings(values)
	default:
		return nil
	}
}

func taskPersona(task *runtimetask.RuntimeTask) string {
	if task == nil || len(task.AppContext) == 0 {
		return ""
	}
	if text, ok := task.AppContext["persona"].(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func taskPersonaContext(task *runtimetask.RuntimeTask) *runtimepersona.Context {
	if task == nil || len(task.GlobalContext) == 0 {
		return nil
	}
	return runtimepersona.BuildContext(task.GlobalContext)
}

func taskUserLanguage(task *runtimetask.RuntimeTask) string {
	if task == nil {
		return ""
	}
	if strings.TrimSpace(task.UserLanguage) != "" {
		return strings.TrimSpace(task.UserLanguage)
	}
	if task.GlobalContext != nil {
		if text, ok := task.GlobalContext["user_language"].(string); ok {
			return strings.TrimSpace(text)
		}
		if text, ok := task.GlobalContext["language"].(string); ok {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

func taskGuideQuestions(task *runtimetask.RuntimeTask) []string {
	if task == nil || len(task.AppContext) == 0 {
		return nil
	}
	raw, ok := task.AppContext["guide_questions"]
	if !ok {
		return nil
	}
	switch items := raw.(type) {
	case []string:
		return compactStrings(items)
	case []any:
		values := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				values = append(values, text)
			}
		}
		return compactStrings(values)
	default:
		return nil
	}
}

func taskPlatformContextUsage(task *runtimetask.RuntimeTask) *platformcontext.UsageTrace {
	if task == nil || len(task.GlobalContext) == 0 {
		return nil
	}
	bundle := platformcontext.BuildBundle(task.GlobalContext)
	if bundle == nil {
		return nil
	}
	usage := bundle.ResolveUsage(platformcontext.UsageInput{
		Query:             strings.TrimSpace(task.UserGoal),
		TaskType:          strings.TrimSpace(task.TaskType),
		Scene:             strings.TrimSpace(task.Scene),
		DesiredOutputMode: strings.TrimSpace(task.OutputMode),
	})
	return &usage
}

func taskContextAssetsUsage(task *runtimetask.RuntimeTask) *contextassets.UsageTrace {
	if task == nil || len(task.GlobalContext) == 0 {
		return nil
	}
	bundle := contextassets.BuildBundle(task.GlobalContext)
	if bundle == nil {
		return nil
	}
	usage := bundle.ResolveUsage(contextassets.UsageInput{
		Query:             strings.TrimSpace(task.UserGoal),
		TaskType:          strings.TrimSpace(task.TaskType),
		Scene:             strings.TrimSpace(task.Scene),
		DesiredOutputMode: strings.TrimSpace(task.OutputMode),
	})
	return &usage
}

func taskContextAssetEffectiveViews(task *runtimetask.RuntimeTask) *contextassets.EffectiveViews {
	if task == nil || len(task.GlobalContext) == 0 {
		return nil
	}
	views := contextassets.BuildEffectiveViewsFromGlobalContext(task.GlobalContext)
	if views.Empty() {
		return nil
	}
	return &views
}

func defaultSceneSkillBundle(task *runtimetask.RuntimeTask, query string, catalog []scene.Definition) []string {
	if task == nil {
		return nil
	}
	sceneName := strings.TrimSpace(task.Scene)
	if sceneName == "" || sceneName == "default" {
		sceneName = scene.MatchWithCatalog(catalog, scene.MatchInput{
			TaskType:      strings.TrimSpace(task.TaskType),
			Query:         strings.TrimSpace(query),
			AppInstanceID: strings.TrimSpace(task.AppInstanceID),
		}).Scene
	}
	if def, ok := scene.FindDefinition(catalog, sceneName); ok && len(def.DefaultSkills) > 0 {
		return append([]string(nil), def.DefaultSkills...)
	}
	return nil
}

func defaultBuiltinSkillBundle() []string {
	return []string{"user_overview"}
}

func preservedContextFromPending(pending *session.PendingState) *session.PreservedContext {
	if pending == nil || pending.Preserved == nil {
		return nil
	}
	cloned := *pending.Preserved
	cloned.MissingFields = append([]string(nil), pending.Preserved.MissingFields...)
	if len(pending.Preserved.Facts) > 0 {
		cloned.Facts = make(map[string]string, len(pending.Preserved.Facts))
		for key, value := range pending.Preserved.Facts {
			cloned.Facts[key] = value
		}
	}
	if cloned.WaitStage == "" {
		cloned.WaitStage = pending.Stage
	}
	if cloned.ResumeToken == "" {
		cloned.ResumeToken = pending.ResumeToken
	}
	if cloned.TimeoutAt.IsZero() {
		cloned.TimeoutAt = pending.TimeoutAt
	}
	if cloned.TimeoutAfter == 0 {
		cloned.TimeoutAfter = pending.TimeoutAfter
	}
	if len(cloned.MissingFields) == 0 {
		cloned.MissingFields = append([]string(nil), pending.MissingFields...)
	}
	return &cloned
}

func buildPreservedContext(in Input, action *Action) *session.PreservedContext {
	preserved := preservedContextFromPending(in.Pending)
	if preserved == nil {
		preserved = &session.PreservedContext{}
	}
	if goal := resolveEffectiveQuery(in); goal != "" {
		preserved.Goal = goal
		preserved.LastUserIntent = goal
	}
	if in.Supplement != nil && len(in.Supplement.Data) > 0 {
		if preserved.Facts == nil {
			preserved.Facts = make(map[string]string, len(in.Supplement.Data))
		}
		for key, value := range in.Supplement.Data {
			if strings.TrimSpace(value) != "" {
				preserved.Facts[key] = value
			}
		}
	}
	if action != nil && action.InformationRequest != nil {
		preserved.MissingFields = preserved.MissingFields[:0]
		for _, item := range action.InformationRequest.Missing {
			preserved.MissingFields = append(preserved.MissingFields, item.Field)
		}
		preserved.WaitStage = string(StageCapabilityResolution)
		preserved.TimeoutAfter = action.InformationRequest.WaitPolicy.TimeoutAfter
	}
	switch outcome := resolveSupplementOutcome(in); outcome {
	case SupplementOutcomeTimeoutExpired, SupplementOutcomeUnableToProvide, SupplementOutcomeAbandonAndContinue:
		preserved.DegradeReason = string(outcome)
		preserved.CloseReason = string(outcome)
	case SupplementOutcomePendingHuman:
		preserved.PendingHumanReason = string(outcome)
		preserved.CloseReason = string(outcome)
	}
	if preserved.Goal == "" && preserved.LastUserIntent == "" && len(preserved.MissingFields) == 0 && len(preserved.Facts) == 0 && preserved.WaitStage == "" && preserved.ResumeToken == "" && preserved.TimeoutAt.IsZero() && preserved.TimeoutAfter == 0 && preserved.DegradeReason == "" && preserved.CloseReason == "" && preserved.PendingHumanReason == "" {
		return nil
	}
	return preserved
}

func resolveEffectiveQuery(in Input) string {
	if in.Task != nil {
		if goal := strings.TrimSpace(in.Task.UserGoal); goal != "" {
			return goal
		}
	}
	if goal := strings.TrimSpace(in.Query); goal != "" {
		return goal
	}
	if in.Pending != nil && in.Pending.Preserved != nil {
		if goal := strings.TrimSpace(in.Pending.Preserved.Goal); goal != "" {
			return goal
		}
	}
	return ""
}

func normalizeOrchestrationInput(in Input) OrchestrationState {
	if in.Orchestration != "" {
		return in.Orchestration
	}
	if in.Pending != nil {
		return OrchestrationStateResumed
	}
	return OrchestrationStateNormalized
}

func runnableOrchestrationState(input OrchestrationState) OrchestrationState {
	if input == OrchestrationStateResumed {
		return OrchestrationStateResumed
	}
	return OrchestrationStateExecuting
}

func runnableOrchestrationReason(input OrchestrationState) string {
	if input == OrchestrationStateResumed {
		return "resume_continue"
	}
	return "runnable_prepare"
}

func resolvePreparedOrchestration(input OrchestrationState, metadata *OrchestrationStatus) OrchestrationState {
	if metadata != nil {
		switch metadata.CurrentState {
		case OrchestrationStateWaiting, OrchestrationStateResumed, OrchestrationStateDegraded, OrchestrationStateAborted:
			return metadata.CurrentState
		}
	}
	if input == OrchestrationStateResumed {
		return OrchestrationStateResumed
	}
	return OrchestrationStateExecuting
}

func shouldDegradeMissingInformation(in Input) bool {
	switch resolveSupplementOutcome(in) {
	case SupplementOutcomeUnableToProvide, SupplementOutcomeTimeoutExpired, SupplementOutcomeAbandonAndContinue:
		return true
	default:
		return false
	}
}

func shouldContinueWithoutSupplement(in Input, p policy.CapabilityPolicy) bool {
	if resolveSupplementOutcome(in) == SupplementOutcomeAbandonAndContinue {
		return true
	}
	return shouldDegradeMissingInformation(in) && p.AllowDegradeWithoutResponse
}
