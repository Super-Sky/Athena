// resolver_test.go verifies stable interaction routing decisions for explicit and ambiguous requests.
// resolver_test.go 用于验证显式与模糊请求下的稳定交互路由决策。
package intent

import "testing"

// TestResolveHonorsExplicitAutomationEntry verifies explicit automation entry wins over text heuristics.
// TestResolveHonorsExplicitAutomationEntry 用于验证显式自动化入口优先于文本启发式。
func TestResolveHonorsExplicitAutomationEntry(t *testing.T) {
	resolution := Resolve(Context{
		Query:        "随便聊聊",
		EntryMode:    EntryModeAutomationCreate,
		Scene:        "main",
		PrimarySkill: "workflow_planning",
	})
	if resolution.InteractionMode != InteractionModeAutomationDraft {
		t.Fatalf("interaction_mode = %q, want automation_draft", resolution.InteractionMode)
	}
	if resolution.SelectedRoute != "automation_draft" {
		t.Fatalf("selected_route = %q, want automation_draft", resolution.SelectedRoute)
	}
	if resolution.PrimarySkill != "automation_planner" {
		t.Fatalf("primary_skill = %q, want automation_planner", resolution.PrimarySkill)
	}
}

// TestResolveReturnsChoiceRequiredForAmbiguousAutomationIntent verifies ambiguous recurring requests ask the user to choose.
// TestResolveReturnsChoiceRequiredForAmbiguousAutomationIntent 用于验证模糊周期性请求会返回用户选择。
func TestResolveReturnsChoiceRequiredForAmbiguousAutomationIntent(t *testing.T) {
	resolution := Resolve(Context{
		Query:        "我想让墨思每天帮我做点事情",
		Scene:        "main",
		PrimarySkill: "workflow_planning",
	})
	if resolution.InteractionMode != InteractionModeChoiceRequired {
		t.Fatalf("interaction_mode = %q, want choice_required", resolution.InteractionMode)
	}
	options := BuildOptions(resolution)
	if len(options) != 2 {
		t.Fatalf("options len = %d, want 2", len(options))
	}
	if options[0].ID != "continue_chat" {
		t.Fatalf("first option id = %q, want continue_chat", options[0].ID)
	}
}

// TestResolveReturnsChoiceRequiredWhenUserExplicitlyAsksToChoose verifies explicit route-choice phrasing stays on choice_required.
// TestResolveReturnsChoiceRequiredWhenUserExplicitlyAsksToChoose 用于验证用户明确要求在讨论与草案之间二选一时稳定返回 choice_required。
func TestResolveReturnsChoiceRequiredWhenUserExplicitlyAsksToChoose(t *testing.T) {
	resolution := Resolve(Context{
		Query:        "我想把习惯分析这件事做成可周期执行的任务，但还没决定是先继续讨论还是直接生成自动化草案。",
		Scene:        "main",
		PrimarySkill: "workflow_planning",
	})
	if resolution.InteractionMode != InteractionModeChoiceRequired {
		t.Fatalf("interaction_mode = %q, want choice_required", resolution.InteractionMode)
	}
	if resolution.Reason != "user explicitly asks to choose between continuing discussion and generating an automation draft" {
		t.Fatalf("reason = %q", resolution.Reason)
	}
}

// TestResolveUsesUserSelectedMode verifies user-selected mode overrides ambiguous text.
// TestResolveUsesUserSelectedMode 用于验证用户显式选择会覆盖模糊文本。
func TestResolveUsesUserSelectedMode(t *testing.T) {
	resolution := Resolve(Context{
		Query:            "我想让墨思每天帮我做点事情",
		UserSelectedMode: UserSelectedModeChat,
		Scene:            "main",
	})
	if resolution.InteractionMode != InteractionModeChat {
		t.Fatalf("interaction_mode = %q, want chat", resolution.InteractionMode)
	}
}
