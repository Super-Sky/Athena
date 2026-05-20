// task.go defines automation-oriented task and plan-draft contracts emitted by the workflow/runtime path.
// task.go 定义由 workflow/runtime 路径发出的自动化任务与计划草案契约。
package automation

// DraftStep captures one minimal automation planning step shown to users and platform.
// DraftStep 描述一条面向用户和平台展示的最小自动化计划步骤。
type DraftStep struct {
	StepID string `json:"step_id,omitempty"`
	Title  string `json:"title,omitempty"`
	Type   string `json:"type,omitempty"`
}

// PlanDraft captures one structured automation planning draft returned before a real automation is created.
// PlanDraft 描述正式自动化创建前返回的一份结构化自动化计划草案。
type PlanDraft struct {
	PlanType               string      `json:"plan_type,omitempty"`
	Goal                   string      `json:"goal,omitempty"`
	Summary                string      `json:"summary,omitempty"`
	Scenario               string      `json:"scenario,omitempty"`
	RequiredCapabilities   []string    `json:"required_capabilities,omitempty"`
	RequiredIntegrations   []string    `json:"required_integrations,omitempty"`
	WorkflowSteps          []DraftStep `json:"workflow_steps,omitempty"`
	ExpectedOutputs        []string    `json:"expected_outputs,omitempty"`
	RiskLevel              string      `json:"risk_level,omitempty"`
	RequiresConfirmation   bool        `json:"requires_confirmation,omitempty"`
	CannotExecuteReason    string      `json:"cannot_execute_reason,omitempty"`
	UserVisibleExplanation string      `json:"user_visible_explanation,omitempty"`
}

// Task captures one automation candidate or scheduled execution request.
// Task 描述一个自动化候选或定时执行请求。
type Task struct {
	TaskID      string         `json:"task_id,omitempty"`
	Title       string         `json:"title,omitempty"`
	Summary     string         `json:"summary,omitempty"`
	ExecuteMode string         `json:"execute_mode,omitempty"`
	Payload     map[string]any `json:"payload,omitempty"`
}

// BuildTask returns a minimal automation task from summary inputs.
// BuildTask 会根据摘要输入构建最小自动化任务对象。
func BuildTask(taskID, title, summary string, payload map[string]any) *Task {
	return &Task{
		TaskID:      taskID,
		Title:       title,
		Summary:     summary,
		ExecuteMode: "manual_action",
		Payload:     payload,
	}
}

// BuildPlanDraft returns a minimal automation plan draft from normalized summary inputs.
// BuildPlanDraft 会根据归一化摘要输入构建最小自动化计划草案。
func BuildPlanDraft(planType, goal, summary, scenario string, requiredCapabilities, requiredIntegrations, expectedOutputs []string, riskLevel string, requiresConfirmation bool, cannotExecuteReason, explanation string) *PlanDraft {
	return &PlanDraft{
		PlanType:             planType,
		Goal:                 goal,
		Summary:              summary,
		Scenario:             scenario,
		RequiredCapabilities: append([]string(nil), requiredCapabilities...),
		RequiredIntegrations: append([]string(nil), requiredIntegrations...),
		WorkflowSteps: []DraftStep{
			{StepID: "collect_context", Title: "收集近期上下文", Type: "read"},
			{StepID: "analyze_pattern", Title: "分析当前模式", Type: "model_analysis"},
			{StepID: "generate_summary", Title: "生成结果摘要", Type: "artifact_output"},
		},
		ExpectedOutputs:        append([]string(nil), expectedOutputs...),
		RiskLevel:              riskLevel,
		RequiresConfirmation:   requiresConfirmation,
		CannotExecuteReason:    cannotExecuteReason,
		UserVisibleExplanation: explanation,
	}
}
