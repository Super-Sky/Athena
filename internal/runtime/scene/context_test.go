package scene

import "testing"

// TestBuildContextExtractsApplicationFields verifies scene context can extract app definition and user profile fields.
// TestBuildContextExtractsApplicationFields 用于验证场景上下文能提取 App 定义和用户画像字段。
func TestBuildContextExtractsApplicationFields(t *testing.T) {
	ctx := BuildContext(
		map[string]any{
			"application_id":  "app-def-1",
			"title":           "安全专家",
			"persona":         "security expert",
			"guide_questions": []any{"问题1", "问题2"},
			"skills":          []any{"skill-1"},
		},
		map[string]any{
			"user_id":       "user-1",
			"user_language": "zh-CN",
		},
		"app-inst-1",
		"",
	)
	if ctx == nil || ctx.ApplicationDefinition == nil || ctx.ApplicationInstance == nil || ctx.UserProfile == nil {
		t.Fatalf("unexpected context = %#v", ctx)
	}
	if ctx.ApplicationDefinition.Title != "安全专家" {
		t.Fatalf("Title = %q", ctx.ApplicationDefinition.Title)
	}
	if ctx.ApplicationInstance.AppInstanceID != "app-inst-1" {
		t.Fatalf("AppInstanceID = %q", ctx.ApplicationInstance.AppInstanceID)
	}
	if ctx.UserProfile.Language != "zh-CN" {
		t.Fatalf("Language = %q", ctx.UserProfile.Language)
	}
}

// TestResolveGuideQuestionsUsesAppContext verifies app-specific guide questions override fallback questions.
// TestResolveGuideQuestionsUsesAppContext 用于验证 App 自带 guide questions 会覆盖默认推荐问题。
func TestResolveGuideQuestionsUsesAppContext(t *testing.T) {
	ctx := &Context{
		ApplicationDefinition: &ApplicationDefinition{
			GuideQuestions: []string{"Q1", "Q2", "Q3", "Q4"},
		},
	}
	questions := ResolveGuideQuestions(ctx, []string{"fallback"}, "zh-CN")
	if len(questions) != 3 {
		t.Fatalf("questions len = %d, want 3", len(questions))
	}
	if questions[0] != "Q1" {
		t.Fatalf("questions = %#v", questions)
	}
}
