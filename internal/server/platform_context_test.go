package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"moss/internal/config"
	platformcontext "moss/internal/extensions/platform/context"
)

func TestPreparePlatformContextLoadsDetailBeforeRuntime(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/platform-context/knowledge" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"近期重点关注供应链安全和依赖风险"}`))
	}))
	defer server.Close()

	prepared := preparePlatformContext(context.Background(), nil, config.Config{
		PlatformContext: config.PlatformContextConfig{
			BaseURL:              server.URL,
			DetailTimeoutSeconds: 3,
			AuthHeader:           "Authorization",
			ForwardAuthorization: true,
		},
	}, map[string]any{
		"platform_context_catalog": map[string]any{
			"knowledge": map[string]any{"available": true},
		},
		"knowledge_summary":       map[string]any{"summary": "供应链"},
		"platform_context_access": map[string]any{"token": "pct_demo"},
	}, platformcontext.UsageInput{
		Query: "结合我的知识详情，给我一个近期供应链安全关注点总结。",
		Scene: "security_review",
	})

	if prepared.Bundle == nil || !prepared.Bundle.HasDetail(platformcontext.TypeKnowledge) {
		t.Fatalf("preparePlatformContext() did not hydrate knowledge detail: %#v", prepared.Bundle)
	}
	if len(prepared.ContextDetailsLoaded) != 1 || prepared.ContextDetailsLoaded[0] != "knowledge" {
		t.Fatalf("ContextDetailsLoaded = %#v", prepared.ContextDetailsLoaded)
	}
	if len(prepared.Trace.ContextDetailsRequested) != 1 || prepared.Trace.ContextDetailsRequested[0] != "knowledge" {
		t.Fatalf("ContextDetailsRequested = %#v", prepared.Trace.ContextDetailsRequested)
	}
}
