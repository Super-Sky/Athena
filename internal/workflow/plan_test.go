package workflow

import "testing"

// TestPlanValidateAcceptsMinimalPlan verifies the minimum workflow contract can validate successfully.
// TestPlanValidateAcceptsMinimalPlan 用于验证最小工作流契约可以通过校验。
func TestPlanValidateAcceptsMinimalPlan(t *testing.T) {
	plan := Plan{
		PlanID:  "plan-1",
		TaskID:  "task-1",
		Title:   "investigate risk",
		Summary: "one-step plan",
		Steps: []Step{{
			StepID:               "step-1",
			Order:                1,
			Title:                "analyze current evidence",
			Description:          "review the current evidence and summarize risk",
			RequiredInputs:       []string{"task_context"},
			ParallelGroup:        "analysis",
			ConfirmationRequired: true,
			ExecutionMode:        StepExecutionModeReadonlyAnalysis,
			CompletionCondition:  "analysis finished",
			FailureGuidance:      "surface the failure details",
			StepType:             StepTypeAnalysis,
		}},
	}
	if err := plan.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// TestPlanValidateRejectsMissingFields verifies invalid workflow steps fail fast.
// TestPlanValidateRejectsMissingFields 用于验证缺少关键字段的工作流步骤会快速失败。
func TestPlanValidateRejectsMissingFields(t *testing.T) {
	plan := Plan{
		Steps: []Step{{StepID: "step-1"}},
	}
	if err := plan.Validate(); err == nil {
		t.Fatalf("expected validation error")
	}
}

// TestGenerateDefaultPlanAddsConfirmationStepForWorkflow verifies workflow tasks get a confirmation step by default.
// TestGenerateDefaultPlanAddsConfirmationStepForWorkflow 用于验证 workflow 任务默认会得到确认步骤。
func TestGenerateDefaultPlanAddsConfirmationStepForWorkflow(t *testing.T) {
	plan := GenerateDefaultPlan(GenerateInput{TaskID: "task-1", TaskType: "workflow_step_request", Scene: "workflow"})
	if plan == nil {
		t.Fatalf("expected generated plan")
	}
	if plan.Goal == "" {
		t.Fatalf("expected goal on generated plan")
	}
	if plan.RiskLevel == "" {
		t.Fatalf("expected risk_level on generated plan")
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("Steps length = %d, want 3", len(plan.Steps))
	}
	lastStep := plan.Steps[len(plan.Steps)-1]
	if !lastStep.ConfirmationRequired {
		t.Fatalf("expected confirmation_required on final step")
	}
	if !plan.RequiresConfirmation {
		t.Fatalf("expected top-level requires_confirmation on plan")
	}
	if lastStep.ParallelGroup == "" {
		t.Fatalf("expected explicit parallel_group on final step")
	}
	if len(lastStep.DependsOn) != 1 || lastStep.DependsOn[0] != "generate_plan" {
		t.Fatalf("DependsOn = %#v, want [generate_plan]", lastStep.DependsOn)
	}
}

// TestPlanValidateRejectsUnknownDependencies verifies step dependencies must point to declared steps.
// TestPlanValidateRejectsUnknownDependencies 用于验证步骤依赖必须引用已声明的步骤。
func TestPlanValidateRejectsUnknownDependencies(t *testing.T) {
	plan := Plan{
		Steps: []Step{{
			StepID:        "step-1",
			Order:         1,
			Title:         "analyze",
			ExecutionMode: StepExecutionModeReadonlyAnalysis,
			StepType:      StepTypeAnalysis,
			DependsOn:     []string{"step-missing"},
		}},
	}
	if err := plan.Validate(); err == nil {
		t.Fatalf("expected dependency validation error")
	}
}
