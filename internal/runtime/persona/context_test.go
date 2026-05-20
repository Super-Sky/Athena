// context_test.go verifies persona context extraction and guidance generation for runtime usage.
// context_test.go 用于验证 runtime persona context 的解析与 guidance 生成行为。
package persona

import (
	"strings"
	"testing"
)

// TestBuildContextExtractsPersonaContext verifies platform persona context is parsed from global context.
// TestBuildContextExtractsPersonaContext 用于验证 platform persona context 能从 global context 中解析出来。
func TestBuildContextExtractsPersonaContext(t *testing.T) {
	ctx := BuildContext(map[string]any{
		"persona_context": map[string]any{
			"id":             "persona_serious",
			"name":           "严肃",
			"description":    "用词严谨，重数据和证据",
			"style_rules":    []any{"优先给出依据和边界", "不要夸张渲染风险"},
			"example_dialog": "用户:这个风险严重吗？\n墨思:依据当前证据，这个问题属于高风险。",
		},
	})
	if ctx == nil {
		t.Fatalf("BuildContext() = nil, want context")
	}
	if ctx.ID != "persona_serious" {
		t.Fatalf("ID = %q, want persona_serious", ctx.ID)
	}
	if ctx.Name != "严肃" {
		t.Fatalf("Name = %q, want 严肃", ctx.Name)
	}
	if len(ctx.StyleRules) != 2 {
		t.Fatalf("StyleRules len = %d, want 2", len(ctx.StyleRules))
	}
}

// TestGuidanceLinesPreserveExpressionBoundary verifies persona guidance stays on expression style and not facts.
// TestGuidanceLinesPreserveExpressionBoundary 用于验证 persona guidance 只约束表达风格而不影响事实边界。
func TestGuidanceLinesPreserveExpressionBoundary(t *testing.T) {
	ctx := &Context{
		ID:            "persona_concise",
		Name:          "简洁",
		Description:   "更短，直奔主题",
		StyleRules:    []string{"少铺垫", "先结论后说明"},
		ExampleDialog: "用户:怎么处理？\n墨思:先冻结风险订单，再复核。",
	}
	lines := ctx.GuidanceLines()
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "Current user persona id: persona_concise") {
		t.Fatalf("guidance = %q, want persona id", joined)
	}
	if !strings.Contains(joined, "Current user persona style rules:") {
		t.Fatalf("guidance = %q, want style rules header", joined)
	}
	if !strings.Contains(joined, "must not change facts") {
		t.Fatalf("guidance = %q, want fact boundary", joined)
	}
}
