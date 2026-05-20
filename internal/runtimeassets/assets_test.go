package runtimeassets

import (
	"context"
	"testing"
)

// TestRegistrySelectTaskBundle verifies runtime task bundles are narrowed by task_type and task_subtype first.
// TestRegistrySelectTaskBundle 用于验证 runtime 任务资产会优先按 task_type 和 task_subtype 收紧。
func TestRegistrySelectTaskBundle(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	bundle, ok := registry.SelectTaskBundle("runtime_event_analysis", "openclaw_before_tool_call", []string{"judgment", "decision"})
	if !ok {
		t.Fatalf("expected task bundle")
	}
	if bundle.ID != "runtime_event_analysis.openclaw_before_tool_call" {
		t.Fatalf("bundle.ID = %q", bundle.ID)
	}
}

// TestRegistryResolveVisibleSkillsRejectsOutOfScopeSkills verifies allowlisted skills still obey task narrowing.
// TestRegistryResolveVisibleSkillsRejectsOutOfScopeSkills 用于验证 allowlist skill 仍需遵守任务范围约束。
func TestRegistryResolveVisibleSkillsRejectsOutOfScopeSkills(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	if _, err := registry.ResolveVisibleSkills([]string{"skill_mosi_audit_operator_v1"}, "runtime_event_analysis", "openclaw_before_tool_call", []string{"judgment", "decision"}); err == nil {
		t.Fatalf("expected out-of-scope skill rejection")
	}
}

// TestRegistryListSkillsFiltersBySource verifies skill queries can be narrowed by source.
// TestRegistryListSkillsFiltersBySource 用于验证 skill 查询可以按来源过滤。
func TestRegistryListSkillsFiltersBySource(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	items := registry.ListSkills(context.Background(), SkillFilter{Source: SkillSourceProductManaged})
	if len(items) != 2 {
		t.Fatalf("product_managed skills len = %d, want 2", len(items))
	}
}

// TestRegistryBuiltinAssetsRemainLegacyCompatibilityIsland prevents new core runtime assets from being added here by accident.
// TestRegistryBuiltinAssetsRemainLegacyCompatibilityIsland 用于防止新的 core runtime 资产误加到当前 legacy 兼容包。
func TestRegistryBuiltinAssetsRemainLegacyCompatibilityIsland(t *testing.T) {
	registry, err := NewRegistry()
	if err != nil {
		t.Fatalf("NewRegistry() error = %v", err)
	}
	wantTasks := map[string]bool{
		"runtime_event_analysis::openclaw_before_tool_call":    true,
		"runtime_event_analysis::openclaw_message_sending":     true,
		"runtime_event_analysis::openclaw_runtime_explanation": true,
	}
	if len(registry.taskBundles) != len(wantTasks) {
		t.Fatalf("task bundle count = %d, want legacy-only %d: %#v", len(registry.taskBundles), len(wantTasks), registry.taskBundles)
	}
	for key := range registry.taskBundles {
		if !wantTasks[key] {
			t.Fatalf("unexpected runtime asset %q; add new assets through the v2.0.0 system truth or graph boundary", key)
		}
	}

	wantSkills := map[string]bool{
		"skill_mosi_audit_operator_v1": true,
		"skill_mosi_email_sender_v1":   true,
	}
	if len(registry.skills) != len(wantSkills) {
		t.Fatalf("skill count = %d, want legacy-only %d: %#v", len(registry.skills), len(wantSkills), registry.skills)
	}
	for id := range registry.skills {
		if !wantSkills[id] {
			t.Fatalf("unexpected runtime skill %q; add new skills through the v2.0.0 system truth or graph boundary", id)
		}
	}
}
