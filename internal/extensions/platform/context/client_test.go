package context

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestHydrateDetailsLoadsKnowledgeDetailFromPlatform(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/platform-context/knowledge" {
			t.Fatalf("path = %s", r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer demo" {
			t.Fatalf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"近期重点关注供应链安全和依赖风险","items":[{"title":"依赖治理"}]}`))
	}))
	defer server.Close()

	result, err := HydrateDetails(context.Background(), ClientConfig{
		BaseURL:              server.URL,
		Timeout:              2 * time.Second,
		AuthHeader:           "Authorization",
		ForwardAuthorization: true,
	}, map[string]any{
		"platform_context_catalog": map[string]any{
			"knowledge": map[string]any{"available": true},
		},
	}, []string{"knowledge"}, RequestMetadata{Authorization: "Bearer demo", RequestID: "req-1"})
	if err != nil {
		t.Fatalf("HydrateDetails() error = %v", err)
	}
	if len(result.LoadedTypes) != 1 || result.LoadedTypes[0] != "knowledge" {
		t.Fatalf("LoadedTypes = %#v", result.LoadedTypes)
	}
	if result.GlobalContext["knowledge_detail"] == nil {
		t.Fatalf("knowledge_detail was not written back: %#v", result.GlobalContext)
	}
	if len(result.FailedTypes) != 0 {
		t.Fatalf("FailedTypes = %#v, want none", result.FailedTypes)
	}
}

func TestHydrateDetailsPrefersContextAccessToken(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer pct_demo" {
			t.Fatalf("authorization = %q, want Bearer pct_demo", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"ok"}`))
	}))
	defer server.Close()

	_, err := HydrateDetails(context.Background(), ClientConfig{
		BaseURL:              server.URL,
		Timeout:              2 * time.Second,
		AuthHeader:           "Authorization",
		AuthToken:            "Bearer static_config",
		ForwardAuthorization: true,
	}, map[string]any{
		"platform_context_catalog": map[string]any{
			"knowledge": map[string]any{"available": true},
		},
	}, []string{"knowledge"}, RequestMetadata{
		Authorization:      "Bearer user_header",
		ContextAccessToken: "pct_demo",
	})
	if err != nil {
		t.Fatalf("HydrateDetails() error = %v", err)
	}
}

func TestHydrateDetailsUsesRawTokenForCustomHeader(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("X-Platform-Context-Token"); got != "pct_demo" {
			t.Fatalf("token header = %q, want pct_demo", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"summary":"ok"}`))
	}))
	defer server.Close()

	_, err := HydrateDetails(context.Background(), ClientConfig{
		BaseURL:              server.URL,
		Timeout:              2 * time.Second,
		AuthHeader:           "X-Platform-Context-Token",
		ForwardAuthorization: true,
	}, map[string]any{
		"platform_context_catalog": map[string]any{
			"knowledge": map[string]any{"available": true},
		},
	}, []string{"knowledge"}, RequestMetadata{
		ContextAccessToken: "pct_demo",
	})
	if err != nil {
		t.Fatalf("HydrateDetails() error = %v", err)
	}
}
