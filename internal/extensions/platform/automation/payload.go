// payload.go builds platform-oriented automation payloads and planning progress descriptors.
// payload.go 负责构建面向 platform 的自动化创建 payload 与规划过程描述。
package automation

import (
	"strings"

	coreautomation "moss/internal/automation"
	platformcontext "moss/internal/extensions/platform/context"
	intentpkg "moss/internal/runtime/intent"
)

// CreatePayload captures the structured payload platform can directly consume after user confirmation.
// CreatePayload 描述用户确认后 platform 可直接消费的结构化自动化创建载荷。
type CreatePayload struct {
	Title                string             `json:"title,omitempty"`
	Goal                 string             `json:"goal,omitempty"`
	DraftOnly            bool               `json:"draft_only,omitempty"`
	RequiresConfirmation bool               `json:"requires_confirmation,omitempty"`
	Trigger              TriggerPayload     `json:"trigger,omitempty"`
	WorkflowSteps        []WorkflowStepSpec `json:"workflow_steps,omitempty"`
	Deliverables         []DeliverableSpec  `json:"deliverables,omitempty"`
	RiskNotes            []string           `json:"risk_notes,omitempty"`
	ConfirmationGates    []string           `json:"confirmation_gates,omitempty"`
}

// TriggerPayload captures the minimal structured trigger platform needs to create an automation.
// TriggerPayload 描述 platform 创建自动化所需的最小结构化触发器。
type TriggerPayload struct {
	Type     string `json:"type,omitempty"`
	Cadence  string `json:"cadence,omitempty"`
	Timezone string `json:"timezone,omitempty"`
}

// WorkflowStepSpec captures one structured workflow step that platform can persist directly.
// WorkflowStepSpec 描述一条 platform 可直接持久化的结构化工作流步骤。
type WorkflowStepSpec struct {
	StepID         string `json:"step_id,omitempty"`
	Type           string `json:"type,omitempty"`
	ExecutionOwner string `json:"execution_owner,omitempty"`
	Description    string `json:"description,omitempty"`
}

// DeliverableSpec captures one structured deliverable definition for platform storage and rendering.
// DeliverableSpec 描述一条可供 platform 存储和渲染的结构化交付物定义。
type DeliverableSpec struct {
	Kind           string `json:"kind,omitempty"`
	Title          string `json:"title,omitempty"`
	Summary        string `json:"summary,omitempty"`
	ArtifactPolicy string `json:"artifact_policy,omitempty"`
}

// PlanningProgressStep captures one platform-facing planning progress descriptor.
// PlanningProgressStep 描述一条面向 platform 的规划进度描述。
type PlanningProgressStep struct {
	StepID  string `json:"step_id,omitempty"`
	Label   string `json:"label,omitempty"`
	Summary string `json:"summary,omitempty"`
}

// BuildPayloadInput captures the minimal input used to build one structured automation create payload.
// BuildPayloadInput 描述构建结构化自动化创建载荷所需的最小输入。
type BuildPayloadInput struct {
	Query           string
	Timezone        string
	Draft           *coreautomation.PlanDraft
	PlatformContext *platformcontext.Bundle
}

// BuildCreatePayload returns one platform-consumable create payload from the current draft.
// BuildCreatePayload 会根据当前草案返回一份 platform 可消费的创建载荷。
func BuildCreatePayload(input BuildPayloadInput) *CreatePayload {
	if input.Draft == nil {
		return nil
	}
	planType := resolvePlanType(input.Query, input.Draft)
	if planType == "" && input.PlatformContext != nil {
		planType = input.PlatformContext.DomainHint()
	}
	title := buildAutomationTitle(input.Query, planType, input.Draft)
	goal := buildAutomationGoal(input.Query, planType, input.Draft)
	cadence := inferCadence(input.Query)
	triggerType := "manual"
	if cadence != "manual" {
		triggerType = "schedule"
	}
	payload := &CreatePayload{
		Title:                title,
		Goal:                 goal,
		DraftOnly:            true,
		RequiresConfirmation: true,
		Trigger: TriggerPayload{
			Type:     triggerType,
			Cadence:  cadence,
			Timezone: defaultString(input.Timezone, "Asia/Shanghai"),
		},
		WorkflowSteps:     buildWorkflowSteps(input.Draft.WorkflowSteps),
		Deliverables:      buildDeliverables(input.Draft.ExpectedOutputs),
		RiskNotes:         buildRiskNotes(input.Draft),
		ConfirmationGates: []string{"user_confirm_create"},
	}
	return payload
}

// BuildPlanningProgressSteps returns stable planning progress descriptors for one interaction mode.
// BuildPlanningProgressSteps 会根据交互模式返回稳定的规划进度描述。
func BuildPlanningProgressSteps(mode intentpkg.InteractionMode, summary string) []PlanningProgressStep {
	switch mode {
	case intentpkg.InteractionModeAutomationDraft:
		return []PlanningProgressStep{
			{StepID: "understanding_request", Label: "Understanding the automation goal", Summary: "Athena is understanding the requested recurring workflow and its intent."},
			{StepID: "detecting_interaction_mode", Label: "Choosing the interaction route", Summary: "Athena confirmed this request should enter the automation draft flow."},
			{StepID: "collecting_dependencies", Label: "Collecting dependencies", Summary: "Athena is identifying the inputs, dependencies, and boundaries the draft will require."},
			{StepID: "defining_deliverables", Label: "Defining deliverables", Summary: "Athena is defining the outputs and artifacts the draft should produce."},
			{StepID: "assessing_risks", Label: "Assessing risks and confirmations", Summary: "Athena is evaluating risks, confirmations, and creation boundaries before the draft is shown."},
			{StepID: "building_draft", Label: "Building the draft plan", Summary: defaultString(summary, "Athena finished building a draft plan that the user can review before creation.")},
		}
	case intentpkg.InteractionModeChoiceRequired:
		return []PlanningProgressStep{
			{StepID: "understanding_request", Label: "Understanding the request", Summary: "Athena is understanding the goal and deciding which interaction route fits best."},
			{StepID: "detecting_interaction_mode", Label: "Asking the user to choose the route", Summary: defaultString(summary, "Athena needs the user to choose whether to continue normal discussion or generate an automation draft.")},
		}
	default:
		return nil
	}
}

func buildWorkflowSteps(steps []coreautomation.DraftStep) []WorkflowStepSpec {
	result := make([]WorkflowStepSpec, 0, len(steps))
	for _, step := range steps {
		result = append(result, WorkflowStepSpec{
			StepID:         strings.TrimSpace(step.StepID),
			Type:           strings.TrimSpace(step.Type),
			ExecutionOwner: inferExecutionOwner(step.Type),
			Description:    strings.TrimSpace(step.Title),
		})
	}
	return result
}

func buildDeliverables(outputs []string) []DeliverableSpec {
	result := make([]DeliverableSpec, 0, len(outputs))
	for _, output := range outputs {
		output = strings.TrimSpace(output)
		if output == "" {
			continue
		}
		result = append(result, DeliverableSpec{
			Kind:           output,
			Title:          output,
			Summary:        "platform should render or store this output after the workflow runs",
			ArtifactPolicy: "create_after_confirmation",
		})
	}
	return result
}

func buildRiskNotes(draft *coreautomation.PlanDraft) []string {
	notes := []string{}
	if strings.TrimSpace(draft.CannotExecuteReason) != "" {
		notes = append(notes, strings.TrimSpace(draft.CannotExecuteReason))
	}
	if strings.TrimSpace(draft.RiskLevel) != "" {
		notes = append(notes, "risk_level="+strings.TrimSpace(draft.RiskLevel))
	}
	return notes
}

func inferCadence(query string) string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.Contains(normalized, "daily") || strings.Contains(query, "每天") || strings.Contains(query, "每日"):
		return "daily"
	case strings.Contains(normalized, "weekly") || strings.Contains(query, "每周"):
		return "weekly"
	case strings.Contains(normalized, "monthly") || strings.Contains(query, "每月"):
		return "monthly"
	default:
		return "manual"
	}
}

func buildAutomationTitle(query, planType string, draft *coreautomation.PlanDraft) string {
	query = strings.TrimSpace(query)
	if draft == nil {
		return defaultString(query, "自动化草案")
	}
	switch strings.TrimSpace(planType) {
	case "habit_analysis":
		switch inferCadence(query) {
		case "daily":
			return "每日习惯分析"
		case "weekly":
			return "每周习惯分析"
		case "monthly":
			return "每月习惯分析"
		default:
			return "习惯分析"
		}
	case "runtime_daily_summary":
		return "运行情况定期分析"
	case "security_news_digest":
		return "安全动态定期摘要"
	case "profile_refresh":
		return "定期画像更新"
	case "supply_chain_security":
		switch inferCadence(query) {
		case "daily":
			return "每日供应链安全分析"
		case "weekly":
			return "每周供应链安全分析"
		case "monthly":
			return "每月供应链安全分析"
		default:
			return "供应链安全分析"
		}
	default:
		if strings.Contains(query, "计划") || strings.Contains(query, "自动化") {
			return "自动化计划草案"
		}
		return defaultString(draft.Summary, "自动化草案")
	}
}

func buildAutomationGoal(query, planType string, draft *coreautomation.PlanDraft) string {
	query = strings.TrimSpace(query)
	if draft == nil {
		return defaultString(query, "生成一份可确认的自动化草案")
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
	default:
		return defaultString(strings.TrimSpace(draft.Goal), defaultString(query, "生成一份可确认的自动化草案"))
	}
}

func resolvePlanType(query string, draft *coreautomation.PlanDraft) string {
	if draft != nil && strings.TrimSpace(draft.PlanType) != "" {
		return strings.TrimSpace(draft.PlanType)
	}
	normalized := strings.ToLower(strings.TrimSpace(query))
	switch {
	case strings.Contains(normalized, "habit") || strings.Contains(query, "习惯"):
		return "habit_analysis"
	case strings.Contains(normalized, "runtime") || strings.Contains(query, "运行情况"):
		return "runtime_daily_summary"
	case strings.Contains(normalized, "news") || strings.Contains(query, "动态"):
		return "security_news_digest"
	case strings.Contains(normalized, "profile") || strings.Contains(query, "画像"):
		return "profile_refresh"
	case strings.Contains(normalized, "supply chain") || strings.Contains(query, "供应链"):
		return "supply_chain_security"
	default:
		return ""
	}
}

func inferExecutionOwner(stepType string) string {
	switch strings.TrimSpace(stepType) {
	case "model_analysis":
		return "athena"
	default:
		return "platform"
	}
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}
