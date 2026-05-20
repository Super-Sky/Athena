package scene

import "testing"

// TestClassifyScore verifies the three-stage scene confidence thresholds remain stable.
// TestClassifyScore 用于验证三段式场景置信度阈值保持稳定。
func TestClassifyScore(t *testing.T) {
	if got := ClassifyScore(85); got != MatchStrengthHigh {
		t.Fatalf("ClassifyScore(85) = %q, want %q", got, MatchStrengthHigh)
	}
	if got := ClassifyScore(70); got != MatchStrengthWeak {
		t.Fatalf("ClassifyScore(70) = %q, want %q", got, MatchStrengthWeak)
	}
	if got := ClassifyScore(59); got != MatchStrengthNone {
		t.Fatalf("ClassifyScore(59) = %q, want %q", got, MatchStrengthNone)
	}
}

// TestBuildSwitchSuggestionSuppressesLowConfidence verifies low-confidence matches never emit switch suggestions.
// TestBuildSwitchSuggestionSuppressesLowConfidence 用于验证低置信命中不会发出切换建议。
func TestBuildSwitchSuggestionSuppressesLowConfidence(t *testing.T) {
	if suggestion := BuildSwitchSuggestion(40, "app-1", "weak"); suggestion != nil {
		t.Fatalf("expected nil suggestion, got %#v", suggestion)
	}
}

// TestMatchRespectsExplicitTaskTypes verifies explicit task types bypass weak heuristic matching.
// TestMatchRespectsExplicitTaskTypes 用于验证显式 task_type 会绕过弱启发式匹配。
func TestMatchRespectsExplicitTaskTypes(t *testing.T) {
	result := Match(MatchInput{TaskType: "workflow_step_request"})
	if result.Scene != "workflow" || result.Strength != MatchStrengthHigh {
		t.Fatalf("Match() = %#v, want explicit workflow match", result)
	}
}

// TestMatchPrefersExplicitTaskTypeOverAppContext verifies explicit task types stay canonical even with app context.
// TestMatchPrefersExplicitTaskTypeOverAppContext 用于验证显式 task_type 在存在 app 上下文时仍保持最高优先级。
func TestMatchPrefersExplicitTaskTypeOverAppContext(t *testing.T) {
	result := Match(MatchInput{
		TaskType:      "workflow_step_request",
		AppInstanceID: "app-1",
	})
	if result.Scene != "workflow" || result.TargetAppInstanceID != "" {
		t.Fatalf("Match() = %#v, want explicit workflow task to win over app context", result)
	}
}

// TestMatchDetectsSecurityReviewKeywords verifies security-review phrasing hits the dedicated security_review scene.
// TestMatchDetectsSecurityReviewKeywords 用于验证安全审计类表达会命中专用 security_review 场景。
func TestMatchDetectsSecurityReviewKeywords(t *testing.T) {
	result := Match(MatchInput{Query: "请做一次 CSO 安全审计并给我 OWASP 风险分析"})
	if result.Scene != "security_review" || result.Strength != MatchStrengthHigh {
		t.Fatalf("Match() = %#v, want security_review high match", result)
	}
}
