// generator.go provides the first-phase default workflow plan generator.
// generator.go 提供第一阶段默认工作流计划生成器。
package workflow

import (
	"fmt"
	"strings"

	"moss/internal/observability"
)

// GenerateInput captures the minimum fields needed to build one default workflow plan.
// GenerateInput 描述生成默认工作流计划所需的最小输入字段。
type GenerateInput struct {
	TaskID              string
	TaskType            string
	Scene               string
	SuggestedEntryAppID string
}

// GenerateDefaultPlan builds a minimal structured workflow plan for the current task.
// GenerateDefaultPlan 为当前任务生成最小结构化工作流计划。
func GenerateDefaultPlan(input GenerateInput) *Plan {
	if input.TaskType == "" && input.Scene == "" {
		observability.LogAction(observability.LogLevelDebug, observability.ActionLog{
			Module: "workflow",
			Action: "generate_default_plan",
			Step:   "skip",
			Status: "skipped",
			Reason: "empty_task_type_and_scene",
		})
		return nil
	}
	if def, ok := DefinitionForScene(strings.TrimSpace(input.Scene)); ok {
		if plan := buildPlanFromDefinition(input, def); plan != nil {
			observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
				Module: "workflow",
				Action: "generate_default_plan",
				Step:   "truth_definition",
				Status: "ok",
				Reason: "workflow_generated_from_truth",
				Detail: map[string]any{
					"task_id":    input.TaskID,
					"task_type":  input.TaskType,
					"scene":      input.Scene,
					"workflow_id": def.ID,
					"step_count": len(plan.Steps),
				},
			})
			return plan
		}
	}
	plan := &Plan{
		PlanID:                      fmt.Sprintf("plan-%s", input.TaskID),
		TaskID:                      input.TaskID,
		Goal:                        defaultGoal(input),
		Title:                       "默认工作流计划",
		Summary:                     "用于平台执行的最小结构化工作流计划",
		RiskLevel:                   defaultRiskLevel(input),
		SuggestedEntryAppInstanceID: input.SuggestedEntryAppID,
	}

	plan.Steps = []Step{
		{
			StepID:               "step-analyze",
			Order:                1,
			Title:                "分析当前上下文",
			Description:          "对当前任务上下文进行结构化分析并形成结论摘要。",
			RequiredInputs:       []string{"task_context"},
			ParallelGroup:        "analysis",
			ConfirmationRequired: false,
			ExecutionMode:        StepExecutionModeReadonlyAnalysis,
			CompletionCondition:  "analysis_ready",
			FailureGuidance:      "surface analysis failure details",
			StepType:             StepTypeAnalysis,
		},
	}

	if input.TaskType == "workflow_step_request" || input.Scene == "workflow" {
		plan.Steps = append(plan.Steps, Step{
			StepID:               "step-confirm",
			Order:                2,
			Title:                "确认后续执行",
			Description:          "对高风险或关键步骤进行确认后再继续执行。",
			RequiredInputs:       []string{"confirmation"},
			DependsOn:            []string{"step-analyze"},
			ParallelGroup:        "decision",
			ConfirmationRequired: true,
			ExecutionMode:        StepExecutionModeManualAction,
			CompletionCondition:  "confirmation_received",
			FailureGuidance:      "request explicit confirmation before continuing",
			StepType:             StepTypeAction,
		})
		plan.RequiresConfirmation = true
	}
	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "workflow",
		Action: "generate_default_plan",
		Step:   "completed",
		Status: "ok",
		Reason: "plan_generated",
		Detail: map[string]any{
			"task_id":         input.TaskID,
			"task_type":       input.TaskType,
			"scene":           input.Scene,
			"step_count":      len(plan.Steps),
			"suggested_entry": input.SuggestedEntryAppID,
		},
	})
	return plan
}

func defaultGoal(input GenerateInput) string {
	switch input.TaskType {
	case "scheduled_job":
		return "generate one reusable automation execution plan"
	case "workflow_step_request":
		return "generate one structured workflow plan for the current task"
	default:
		return "analyze the current task and prepare the next structured steps"
	}
}

func defaultRiskLevel(input GenerateInput) string {
	switch {
	case input.TaskType == "workflow_step_request" || input.Scene == "workflow":
		return "medium"
	case input.TaskType == "scheduled_job":
		return "low"
	default:
		return "low"
	}
}
