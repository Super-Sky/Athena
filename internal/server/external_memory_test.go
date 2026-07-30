// external_memory_test.go verifies the app-owned memory HTTP boundary.
// external_memory_test.go 验证应用拥有记忆的 HTTP 边界。
package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
)

func TestExternalMemoryAndContextAssetEndpoints(t *testing.T) {
	cfg := config.Config{Server: config.ServerConfig{HTTPPort: 8080}}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	write := performExternalMemoryJSON(t, httpServer, "/api/memory/write", map[string]any{
		"app_id":   "fund-assistant",
		"owner_id": "user-1",
		"scope":    "investor-profile",
		"kind":     "profile-summary",
		"summary":  "Balanced risk profile with a long horizon.",
	})
	if write.Code != consts.StatusCreated {
		t.Fatalf("write status = %d, want %d; body=%s", write.Code, consts.StatusCreated, write.Body.String())
	}
	if !bytes.Contains(write.Body.Bytes(), []byte(`"operation":"memory_write"`)) {
		t.Fatalf("write body = %s, want memory_write trace", write.Body.String())
	}

	queryPayload := map[string]any{"app_id": "fund-assistant", "owner_id": "user-1", "scope": "investor-profile"}
	query := performExternalMemoryJSON(t, httpServer, "/api/memory/query", queryPayload)
	if query.Code != consts.StatusOK {
		t.Fatalf("query status = %d, want %d; body=%s", query.Code, consts.StatusOK, query.Body.String())
	}
	if !bytes.Contains(query.Body.Bytes(), []byte(`"summary":"Balanced risk profile with a long horizon."`)) {
		t.Fatalf("query body = %s, want owned summary", query.Body.String())
	}

	isolated := performExternalMemoryJSON(t, httpServer, "/api/memory/query", map[string]any{"app_id": "fund-assistant", "owner_id": "user-2", "scope": "investor-profile"})
	if isolated.Code != consts.StatusOK || !bytes.Contains(isolated.Body.Bytes(), []byte(`"items":[]`)) {
		t.Fatalf("isolated query = status %d body=%s, want an empty result", isolated.Code, isolated.Body.String())
	}

	resolved := performExternalMemoryJSON(t, httpServer, "/api/context-assets/resolve", queryPayload)
	if resolved.Code != consts.StatusOK {
		t.Fatalf("resolve status = %d, want %d; body=%s", resolved.Code, consts.StatusOK, resolved.Body.String())
	}
	if !bytes.Contains(resolved.Body.Bytes(), []byte(`"asset_type":"memory_view"`)) || !bytes.Contains(resolved.Body.Bytes(), []byte(`"source_kind":"app_memory"`)) {
		t.Fatalf("resolve body = %s, want app-owned memory asset", resolved.Body.String())
	}

	assembled := performExternalMemoryJSON(t, httpServer, "/api/context-assets/assemble", map[string]any{
		"app_id": "fund-assistant", "owner_id": "user-1", "scope": "investor-profile", "query": "review risk preferences",
	})
	if assembled.Code != consts.StatusOK {
		t.Fatalf("assemble status = %d, want %d; body=%s", assembled.Code, consts.StatusOK, assembled.Body.String())
	}
	if !bytes.Contains(assembled.Body.Bytes(), []byte(`"schema_version":"external_context_compression.v1"`)) || !bytes.Contains(assembled.Body.Bytes(), []byte(`"effective_memory_view"`)) {
		t.Fatalf("assemble body = %s, want compression and effective memory view", assembled.Body.String())
	}
}

func TestExternalMemoryEndpointsRejectIncompleteOwnership(t *testing.T) {
	cfg := config.Config{Server: config.ServerConfig{HTTPPort: 8080}}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	resp := performExternalMemoryJSON(t, httpServer, "/api/memory/write", map[string]any{"app_id": "fund-assistant", "scope": "profile", "kind": "summary", "summary": "missing owner"})
	if resp.Code != consts.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body=%s", resp.Code, consts.StatusBadRequest, resp.Body.String())
	}
}

func TestExternalMemoryEndpointsAppearInOpenAPI(t *testing.T) {
	spec := buildOpenAPISpec(config.Config{Server: config.ServerConfig{HTTPPort: 8080}})
	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatal("OpenAPI paths missing")
	}
	for _, path := range []string{"/api/memory/write", "/api/memory/query", "/api/context-assets/resolve", "/api/context-assets/assemble"} {
		if _, ok := paths[path]; !ok {
			t.Fatalf("OpenAPI path %q missing", path)
		}
	}
}

func performExternalMemoryJSON(t *testing.T, httpServer *HTTPServer, path string, payload map[string]any) *ut.ResponseRecorder {
	t.Helper()
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	return ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, path, &ut.Body{Body: bytes.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
}
