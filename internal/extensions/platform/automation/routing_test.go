// routing_test.go verifies platform-specific automation routing tightening remains deterministic.
// routing_test.go 用于验证 platform 专项自动化路由增强保持确定性。
package automation

import (
	"testing"

	intentpkg "moss/internal/runtime/intent"
)

// TestEnhanceResolutionReturnsChoiceRequiredForExplicitDraftChoice verifies explicit route-choice phrasing wins over generic automation draft routing.
// TestEnhanceResolutionReturnsChoiceRequiredForExplicitDraftChoice 用于验证用户显式要求在讨论与草案之间二选一时返回 choice_required。
func TestEnhanceResolutionReturnsChoiceRequiredForExplicitDraftChoice(t *testing.T) {
	resolution := EnhanceResolution(RouteInput{
		Query: "我想让系统以后定期帮我分析个人习惯，但这次先帮我判断接下来应该继续聊细节，还是直接进入待确认方案。",
	}, intentpkg.Resolution{
		InteractionMode: intentpkg.InteractionModeAutomationDraft,
		SelectedRoute:   "automation_draft",
		PrimarySkill:    "automation_planner",
	})
	if resolution.InteractionMode != intentpkg.InteractionModeChoiceRequired {
		t.Fatalf("interaction_mode = %q, want choice_required", resolution.InteractionMode)
	}
	if resolution.Reason != explicitChoiceReason {
		t.Fatalf("reason = %q", resolution.Reason)
	}
}

// TestEnhanceResolutionDoesNotOverrideExplicitEntry verifies platform explicit entry still wins over text heuristics.
// TestEnhanceResolutionDoesNotOverrideExplicitEntry 用于验证 platform 显式入口不会被二选一增强覆盖。
func TestEnhanceResolutionDoesNotOverrideExplicitEntry(t *testing.T) {
	resolution := EnhanceResolution(RouteInput{
		Query:     "还没决定是继续讨论还是直接进入待确认方案。",
		EntryMode: "automation_create",
	}, intentpkg.Resolution{
		InteractionMode: intentpkg.InteractionModeAutomationDraft,
		SelectedRoute:   "automation_draft",
	})
	if resolution.InteractionMode != intentpkg.InteractionModeAutomationDraft {
		t.Fatalf("interaction_mode = %q, want automation_draft", resolution.InteractionMode)
	}
}

// TestBuildInteractionProgressUsesStablePlatformSteps verifies the platform-facing interaction progress uses the centralized planning-step catalog.
// TestBuildInteractionProgressUsesStablePlatformSteps 用于验证 platform 面向的交互进度复用统一规划步骤目录。
func TestBuildInteractionProgressUsesStablePlatformSteps(t *testing.T) {
	progress := BuildInteractionProgress(intentpkg.InteractionModeChoiceRequired, true, 0)
	if progress == nil {
		t.Fatalf("BuildInteractionProgress() = nil")
	}
	if progress.CurrentStage != "detecting_interaction_mode" {
		t.Fatalf("current_stage = %q, want detecting_interaction_mode", progress.CurrentStage)
	}
	if len(progress.CompletedStages) != 1 || progress.CompletedStages[0] != "understanding_request" {
		t.Fatalf("completed_stages = %#v", progress.CompletedStages)
	}
}
