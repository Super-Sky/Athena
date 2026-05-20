// context_test.go verifies platform context parsing and usage selection remain deterministic.
// context_test.go 用于验证 platform 上下文解析与使用选择保持确定性。
package context

import "testing"

func TestResolveUsageUsesIdentityMemoryAndPersonaForChat(t *testing.T) {
	bundle := BuildBundle(map[string]any{
		"platform_context_catalog": map[string]any{
			"identity": map[string]any{"available": true},
			"memory":   map[string]any{"available": true},
			"persona":  map[string]any{"available": true},
		},
		"identity_summary": map[string]any{"summary": "安全负责人，关注供应链治理"},
		"memory_summary":   map[string]any{"summary": "偏好结构化回答和长期连续性"},
		"persona_summary":  map[string]any{"summary": "用词严谨，重依据"},
	})
	trace := bundle.ResolveUsage(UsageInput{
		Query: "结合我的身份和长期记忆，介绍一下你目前如何理解我。",
		Scene: "default",
	})
	if !trace.ContextUsage["identity"] || !trace.ContextUsage["memory"] || !trace.ContextUsage["persona"] {
		t.Fatalf("context_usage = %#v", trace.ContextUsage)
	}
	if len(trace.ContextDetailsRequested) != 0 {
		t.Fatalf("context_details_requested = %#v, want none", trace.ContextDetailsRequested)
	}
}

func TestResolveUsageRequestsKnowledgeDetailWhenSummaryIsThin(t *testing.T) {
	bundle := BuildBundle(map[string]any{
		"platform_context_catalog": map[string]any{
			"knowledge": map[string]any{"available": true},
		},
		"knowledge_summary": map[string]any{"summary": "供应链"},
	})
	trace := bundle.ResolveUsage(UsageInput{
		Query: "结合我的知识详情，给我一个近期供应链安全关注点总结。",
		Scene: "security_review",
	})
	if !trace.ContextUsage["knowledge"] {
		t.Fatalf("context_usage = %#v, want knowledge used", trace.ContextUsage)
	}
	if len(trace.ContextDetailsRequested) != 1 || trace.ContextDetailsRequested[0] != "knowledge" {
		t.Fatalf("context_details_requested = %#v, want knowledge", trace.ContextDetailsRequested)
	}
}

func TestDomainHintFallsBackToSupplyChain(t *testing.T) {
	bundle := BuildBundle(map[string]any{
		"knowledge_summary": map[string]any{"summary": "近期重点关注供应链安全和依赖风险"},
	})
	if got := bundle.DomainHint(); got != "supply_chain_security" {
		t.Fatalf("DomainHint() = %q, want supply_chain_security", got)
	}
}

func TestBuildBundleParsesPlatformContextAccess(t *testing.T) {
	bundle := BuildBundle(map[string]any{
		"platform_context_access": map[string]any{
			"token":         "pct_demo",
			"subject_id":    "u_1",
			"tenant_id":     "t_1",
			"allowed_types": []any{"identity", "knowledge"},
			"session_id":    "sess_1",
			"expires_at":    "2026-04-21T18:00:00+08:00",
		},
	})
	if bundle == nil || bundle.Access == nil {
		t.Fatalf("BuildBundle() access = %#v", bundle)
	}
	if bundle.ContextAccessToken() != "pct_demo" {
		t.Fatalf("ContextAccessToken() = %q, want pct_demo", bundle.ContextAccessToken())
	}
	if len(bundle.Access.AllowedTypes) != 2 || bundle.Access.AllowedTypes[0] != "identity" || bundle.Access.AllowedTypes[1] != "knowledge" {
		t.Fatalf("AllowedTypes = %#v", bundle.Access.AllowedTypes)
	}
}
