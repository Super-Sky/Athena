// plan.go defines the structured workflow plan contract consumed by downstream platform execution.
// plan.go 定义下游平台执行所消费的结构化工作流规划契约。
package workflow

import "fmt"

// StepExecutionMode captures how one workflow step should be executed.
// StepExecutionMode 描述单个工作流步骤应如何执行。
type StepExecutionMode string

const (
	// StepExecutionModeReadonlyAnalysis means the step only analyzes and never performs external side effects.
	// StepExecutionModeReadonlyAnalysis 表示该步骤只做分析，不执行外部副作用。
	StepExecutionModeReadonlyAnalysis StepExecutionMode = "readonly_analysis"

	// StepExecutionModeManualAction means the step requires manual action outside Athena.
	// StepExecutionModeManualAction 表示该步骤需要在 Athena 外部由人工执行。
	StepExecutionModeManualAction StepExecutionMode = "manual_action"

	// StepExecutionModeAutomationCandidate means the step can be turned into an automation candidate.
	// StepExecutionModeAutomationCandidate 表示该步骤可被转化为自动化候选步骤。
	StepExecutionModeAutomationCandidate StepExecutionMode = "automation_candidate"
)

// StepType captures the semantic category of one workflow step.
// StepType 描述单个工作流步骤的语义类别。
type StepType string

const (
	// StepTypeAnalysis marks one analytical step.
	// StepTypeAnalysis 表示分析型步骤。
	StepTypeAnalysis StepType = "analysis"

	// StepTypeInvestigation marks one investigation step.
	// StepTypeInvestigation 表示调查型步骤。
	StepTypeInvestigation StepType = "investigation"

	// StepTypeAction marks one action-oriented step.
	// StepTypeAction 表示动作型步骤。
	StepTypeAction StepType = "action"
)

// Step captures one platform-consumable workflow step.
// Step 描述一个平台可消费的工作流步骤。
type Step struct {
	StepID               string            `json:"step_id,omitempty"`
	Order                int               `json:"order,omitempty"`
	Title                string            `json:"title,omitempty"`
	Description          string            `json:"description,omitempty"`
	RequiredInputs       []string          `json:"required_inputs,omitempty"`
	DependsOn            []string          `json:"depends_on,omitempty"`
	ParallelGroup        string            `json:"parallel_group,omitempty"`
	ConfirmationRequired bool              `json:"confirmation_required,omitempty"`
	ExecutionMode        StepExecutionMode `json:"execution_mode,omitempty"`
	CompletionCondition  string            `json:"completion_condition,omitempty"`
	FailureGuidance      string            `json:"failure_guidance,omitempty"`
	StepType             StepType          `json:"step_type,omitempty"`
}

// StepResult captures the current-step judgment returned on workflow callbacks.
// StepResult 描述 workflow 回调时返回的当前步骤判断结果。
type StepResult struct {
	WorkflowRunID      string   `json:"workflow_run_id,omitempty"`
	StepID             string   `json:"step_id,omitempty"`
	Summary            string   `json:"summary,omitempty"`
	Decision           string   `json:"decision,omitempty"`
	RecommendedAction  string   `json:"recommended_action,omitempty"`
	ParallelGroup      string   `json:"parallel_group,omitempty"`
	DependsOn          []string `json:"depends_on,omitempty"`
	PlanRefreshAllowed bool     `json:"plan_refresh_allowed,omitempty"`
}

// Plan captures the structured workflow plan returned by Athena.
// Plan 描述 Athena 返回的结构化工作流计划。
type Plan struct {
	PlanID                      string `json:"plan_id,omitempty"`
	TaskID                      string `json:"task_id,omitempty"`
	Goal                        string `json:"goal,omitempty"`
	Title                       string `json:"title,omitempty"`
	Summary                     string `json:"summary,omitempty"`
	RiskLevel                   string `json:"risk_level,omitempty"`
	RequiresConfirmation        bool   `json:"requires_confirmation,omitempty"`
	SuggestedEntryAppInstanceID string `json:"suggested_entry_app_instance_id,omitempty"`
	Steps                       []Step `json:"steps,omitempty"`
}

// Validate checks whether the workflow plan contains the minimum platform-consumable fields.
// Validate 检查工作流计划是否包含平台可消费的最小字段。
func (p Plan) Validate() error {
	if len(p.Steps) == 0 {
		return fmt.Errorf("workflow plan must contain at least one step")
	}
	stepIDs := make(map[string]struct{}, len(p.Steps))
	for idx, step := range p.Steps {
		if step.StepID == "" {
			return fmt.Errorf("workflow step %d is missing step_id", idx)
		}
		stepIDs[step.StepID] = struct{}{}
		if step.Order <= 0 {
			return fmt.Errorf("workflow step %s is missing a positive order", step.StepID)
		}
		if step.Title == "" {
			return fmt.Errorf("workflow step %s is missing title", step.StepID)
		}
		if step.ExecutionMode == "" {
			return fmt.Errorf("workflow step %s is missing execution_mode", step.StepID)
		}
		if step.StepType == "" {
			return fmt.Errorf("workflow step %s is missing step_type", step.StepID)
		}
	}
	for _, step := range p.Steps {
		for _, dependency := range step.DependsOn {
			if _, ok := stepIDs[dependency]; !ok {
				return fmt.Errorf("workflow step %s depends_on unknown step %s", step.StepID, dependency)
			}
		}
	}
	return nil
}
