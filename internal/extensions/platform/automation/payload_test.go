// payload_test.go verifies platform automation payload and planning progress remain structured and deterministic.
// payload_test.go 用于验证 platform 自动化载荷与规划进度保持结构化且可确定。
package automation

import (
	"testing"

	coreautomation "moss/internal/automation"
	intentpkg "moss/internal/runtime/intent"
)

// TestBuildCreatePayloadReturnsStructuredFields verifies the create payload does not depend on natural-language parsing by platform.
// TestBuildCreatePayloadReturnsStructuredFields 用于验证创建载荷不依赖 platform 对自然语言做反解析。
func TestBuildCreatePayloadReturnsStructuredFields(t *testing.T) {
	payload := BuildCreatePayload(BuildPayloadInput{
		Query:    "请帮我每天分析我的个人习惯",
		Timezone: "Asia/Shanghai",
		Draft: &coreautomation.PlanDraft{
			PlanType:             "habit_analysis",
			Goal:                 "每日分析一次",
			Summary:              "每日分析草案",
			ExpectedOutputs:      []string{"daily_summary"},
			RiskLevel:            "low",
			RequiresConfirmation: true,
			WorkflowSteps: []coreautomation.DraftStep{
				{StepID: "collect_context", Title: "收集近期上下文", Type: "read"},
				{StepID: "analyze_pattern", Title: "分析当前模式", Type: "model_analysis"},
			},
		},
	})
	if payload == nil {
		t.Fatalf("BuildCreatePayload() = nil")
	}
	if payload.Trigger.Cadence != "daily" {
		t.Fatalf("cadence = %q, want daily", payload.Trigger.Cadence)
	}
	if payload.Trigger.Type != "schedule" {
		t.Fatalf("trigger.type = %q, want schedule", payload.Trigger.Type)
	}
	if payload.Title != "每日习惯分析" {
		t.Fatalf("title = %q, want 每日习惯分析", payload.Title)
	}
	if payload.Goal != "定期分析用户的个人习惯并生成摘要" {
		t.Fatalf("goal = %q", payload.Goal)
	}
	if len(payload.WorkflowSteps) != 2 || payload.WorkflowSteps[1].ExecutionOwner != "athena" {
		t.Fatalf("workflow_steps = %#v", payload.WorkflowSteps)
	}
	if len(payload.Deliverables) != 1 || payload.Deliverables[0].Kind != "daily_summary" {
		t.Fatalf("deliverables = %#v", payload.Deliverables)
	}
}

// TestBuildPlanningProgressStepsCoversAutomationDraft verifies automation draft planning steps stay stable.
// TestBuildPlanningProgressStepsCoversAutomationDraft 用于验证自动化草案规划步骤保持稳定。
func TestBuildPlanningProgressStepsCoversAutomationDraft(t *testing.T) {
	steps := BuildPlanningProgressSteps(intentpkg.InteractionModeAutomationDraft, "done")
	if len(steps) != 6 {
		t.Fatalf("steps len = %d, want 6", len(steps))
	}
	if steps[0].StepID != "understanding_request" || steps[len(steps)-1].StepID != "building_draft" {
		t.Fatalf("steps = %#v", steps)
	}
}
