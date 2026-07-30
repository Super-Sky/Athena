// app_auth_test.go verifies app identity authentication and tenant-safe Agent Run reads.
// app_auth_test.go 验证应用身份认证与 Agent Run 租户安全读取。
package server

import (
	"bytes"
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/runtime"
)

func TestAppFacingAgentRunRoutesRequireIdentity(t *testing.T) {
	httpServer := newAppAuthHTTPServer(t, &testRuntimeReadStore{})
	cases := []struct {
		method string
		path   string
		body   string
	}{
		{http.MethodPost, "/api/agent/runs", `{"goal":"analyze"}`},
		{http.MethodGet, "/api/agent/runs/run-a", ""},
		{http.MethodPost, "/api/agent/runs/run-a/resume", `{"goal":"continue"}`},
		{http.MethodPost, "/api/agent/runs/run-a/cancel", `{}`},
		{http.MethodGet, "/api/agent/runs/run-a/trace", ""},
		{http.MethodGet, "/api/agent/runs/run-a/timeline", ""},
	}
	for _, tc := range cases {
		var bodyOption *ut.Body
		if tc.body != "" {
			body := bytes.NewBufferString(tc.body)
			bodyOption = &ut.Body{Body: body, Len: body.Len()}
		}
		response := ut.PerformRequest(httpServer.engine.Engine, tc.method, tc.path, bodyOption)
		if response.Code != consts.StatusUnauthorized {
			t.Fatalf("%s %s status = %d, want 401; body=%s", tc.method, tc.path, response.Code, response.Body.String())
		}
	}
}

func TestAppFacingAgentRunReadEnforcesWorkspaceAndAppInstance(t *testing.T) {
	store := &testRuntimeReadStore{runs: []runtime.TaskRun{{
		ID: "run-a", TaskID: "task-a", Status: runtime.TaskRunStatusCompleted,
		WorkspaceID: "workspace-a", AppInstanceID: "fund-web-a", Metadata: map[string]any{
			"authorization_token": "private-run-token", "app_token": "private-app-token", "id_token": "private-id-token",
			"cookie": "private-cookie", "set-cookie": "private-set-cookie", "token_count": 12,
		}, CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}}, traces: []runtime.RuntimeTrace{{
		ID: "trace-a", RunID: "run-a", Summary: "safe trace", RedactedPayload: map[string]any{"headers": map[string]any{"Authorization": "private-header"}, "result": "safe"},
		Metadata: map[string]any{"nested": map[string]any{"api_key": "private-api-key"}}, CreatedAt: time.Now(),
	}}, projections: []runtime.ProjectionCandidate{{
		ID: "projection-a", RunID: "run-a", CandidateKind: "analysis", SemanticPayload: map[string]any{"tenant_secret_marker": "tenant-a-private"}, CreatedAt: time.Now(),
	}}}
	httpServer := newAppAuthHTTPServer(t, store)

	own := performAppAuthRequest(httpServer, http.MethodGet, "/api/agent/runs/run-a/timeline", "fund-secret", "fund-assistant", "workspace-a", "fund-web-a", "")
	if own.Code != consts.StatusOK {
		t.Fatalf("own scope status = %d, want 200; body=%s", own.Code, own.Body.String())
	}
	trace := performAppAuthRequest(httpServer, http.MethodGet, "/api/agent/runs/run-a/trace", "fund-secret", "fund-assistant", "workspace-a", "fund-web-a", "")
	traceBody := trace.Body.String()
	for _, forbidden := range []string{
		"private-run-token", "private-app-token", "private-id-token", "private-cookie", "private-set-cookie",
		"private-header", "private-api-key", "tenant-a-private", `"semantic_payload"`,
	} {
		if strings.Contains(traceBody, forbidden) {
			t.Fatalf("app trace leaks %q: %s", forbidden, traceBody)
		}
	}
	if !strings.Contains(traceBody, `"token_count":12`) || !strings.Contains(traceBody, `"authorization_token":"[redacted]"`) {
		t.Fatalf("app trace redaction body = %s", traceBody)
	}

	for _, tc := range []struct {
		name        string
		token       string
		appID       string
		workspaceID string
		instanceID  string
	}{
		{name: "other workspace", token: "fund-secret", appID: "fund-assistant", workspaceID: "workspace-b", instanceID: "fund-web-b"},
		{name: "other app instance", token: "fund-secret", appID: "fund-assistant", workspaceID: "workspace-a", instanceID: "fund-web-b"},
		{name: "other app identity", token: "research-secret", appID: "research-assistant", workspaceID: "workspace-b", instanceID: "research-web"},
	} {
		response := performAppAuthRequest(httpServer, http.MethodGet, "/api/agent/runs/run-a/timeline", tc.token, tc.appID, tc.workspaceID, tc.instanceID, "")
		if response.Code != consts.StatusNotFound || strings.Contains(response.Body.String(), "run-a") {
			t.Fatalf("%s status=%d body=%s, want generic 404", tc.name, response.Code, response.Body.String())
		}
	}
	missing := performAppAuthRequest(httpServer, http.MethodGet, "/api/agent/runs/run-missing/timeline", "fund-secret", "fund-assistant", "workspace-a", "fund-web-a", "")
	crossTenant := performAppAuthRequest(httpServer, http.MethodGet, "/api/agent/runs/run-a/timeline", "fund-secret", "fund-assistant", "workspace-b", "fund-web-b", "")
	if missing.Code != consts.StatusNotFound || missing.Body.String() != crossTenant.Body.String() {
		t.Fatalf("missing response=%d %s cross-tenant=%d %s, want identical 404", missing.Code, missing.Body.String(), crossTenant.Code, crossTenant.Body.String())
	}
}

func TestAppFacingCreateRejectsBodyScopeOverride(t *testing.T) {
	httpServer := newAppAuthHTTPServer(t, &testRuntimeReadStore{})
	response := performAppAuthRequest(httpServer, http.MethodPost, "/api/agent/runs", "fund-secret", "fund-assistant", "workspace-a", "fund-web-a", `{"goal":"analyze","workspace_id":"workspace-b"}`)
	if response.Code != consts.StatusForbidden || !strings.Contains(response.Body.String(), "app_scope_mismatch") {
		t.Fatalf("status=%d body=%s, want scope mismatch 403", response.Code, response.Body.String())
	}
}

func TestApplyAgentRunIdentityScopeInjectsAuthoritativeScope(t *testing.T) {
	ctx := context.WithValue(context.Background(), appAuthContextKey{}, AppRequestIdentity{AppID: "fund-assistant", WorkspaceID: "workspace-a", AppInstanceID: "fund-web-a"})
	req := &agentRunStartRequest{}
	if err := applyAgentRunIdentityScope(ctx, req); err != nil {
		t.Fatal(err)
	}
	if req.WorkspaceID != "workspace-a" || req.AppInstanceID != "fund-web-a" {
		t.Fatalf("request scope = %q/%q, want authoritative identity scope", req.WorkspaceID, req.AppInstanceID)
	}
}

func TestAppAuthUsesDedicatedTokenHeaderAndSupportsDisabledGrayConfig(t *testing.T) {
	if appAuthEnabled(config.AppAuthConfig{Required: false, Identities: []config.AppAuthIdentityConfig{{AppID: "fund"}}}) {
		t.Fatal("identities must not enable auth before APP_AUTH_REQUIRED is true")
	}
	httpServer := newAppAuthHTTPServer(t, &testRuntimeReadStore{})
	response := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/agent/runs/run-a", nil,
		ut.Header{Key: "Authorization", Value: "Bearer fund-secret"},
		ut.Header{Key: appAuthHeaderAppID, Value: "fund-assistant"},
		ut.Header{Key: appAuthHeaderWorkspaceID, Value: "workspace-a"},
		ut.Header{Key: appAuthHeaderAppInstanceID, Value: "fund-web-a"},
	)
	if response.Code != consts.StatusUnauthorized {
		t.Fatalf("Authorization bearer status = %d, want 401 because app token requires dedicated header", response.Code)
	}
}

func TestAgentRunOpenAPIOperationsDeclareAppAuth(t *testing.T) {
	paths := buildOpenAPIPaths()
	cases := []struct {
		path   string
		method string
	}{
		{path: "/api/agent/runs", method: "post"},
		{path: "/api/agent/runs/{runID}", method: "get"},
		{path: "/api/agent/runs/{runID}/resume", method: "post"},
		{path: "/api/agent/runs/{runID}/cancel", method: "post"},
		{path: "/api/agent/runs/{runID}/events", method: "get"},
		{path: "/api/agent/runs/{runID}/trace", method: "get"},
		{path: "/api/agent/runs/{runID}/timeline", method: "get"},
	}
	for _, tc := range cases {
		pathItem, ok := paths[tc.path].(map[string]any)
		if !ok {
			t.Fatalf("path %s missing", tc.path)
		}
		operation, ok := pathItem[tc.method].(map[string]any)
		if !ok || operation["security"] == nil {
			t.Fatalf("%s %s missing AppToken security", tc.method, tc.path)
		}
		parameters, ok := operation["parameters"].([]map[string]any)
		if !ok {
			t.Fatalf("%s %s parameters = %#v", tc.method, tc.path, operation["parameters"])
		}
		for _, headerName := range []string{appAuthHeaderAppID, appAuthHeaderWorkspaceID, appAuthHeaderAppInstanceID} {
			found := false
			for _, parameter := range parameters {
				if parameter["name"] == headerName && parameter["in"] == "header" && parameter["required"] == true {
					found = true
					break
				}
			}
			if !found {
				t.Fatalf("%s %s missing required header %s", tc.method, tc.path, headerName)
			}
		}
	}
}

func BenchmarkAuthenticateAppRequestWithOneHundredScopes(b *testing.B) {
	instanceIDs := make([]string, 100)
	for index := range instanceIDs {
		instanceIDs[index] = "fund-web-" + strconv.Itoa(index)
	}
	cfg := config.AppAuthConfig{Required: true, Identities: []config.AppAuthIdentityConfig{{
		AppID: "fund-assistant", Token: "fund-secret",
		Scopes: []config.AppAuthScopeConfig{{WorkspaceID: "workspace-a", AppInstanceIDs: instanceIDs}},
	}}}
	c := hertzapp.NewContext(0)
	c.Request.Header.Set(appAuthHeaderToken, "fund-secret")
	c.Request.Header.Set(appAuthHeaderAppID, "fund-assistant")
	c.Request.Header.Set(appAuthHeaderWorkspaceID, "workspace-a")
	c.Request.Header.Set(appAuthHeaderAppInstanceID, "fund-web-99")
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		if _, err := authenticateAppRequest(c, cfg); err != nil {
			b.Fatal(err)
		}
	}
}

func newAppAuthHTTPServer(t *testing.T, store runtime.RuntimePersistenceStore) *HTTPServer {
	t.Helper()
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
		AppAuth: config.AppAuthConfig{Required: true, Identities: []config.AppAuthIdentityConfig{
			{AppID: "fund-assistant", Token: "fund-secret", Scopes: []config.AppAuthScopeConfig{
				{WorkspaceID: "workspace-a", AppInstanceIDs: []string{"fund-web-a", "fund-web-b"}},
				{WorkspaceID: "workspace-b", AppInstanceIDs: []string{"fund-web-b"}},
			}},
			{AppID: "research-assistant", Token: "research-secret", Scopes: []config.AppAuthScopeConfig{
				{WorkspaceID: "workspace-b", AppInstanceIDs: []string{"research-web"}},
			}},
		}},
	}
	application := appcore.NewServiceWithRuntimeStore(cfg, nil, nil, nil, store)
	return NewHTTPServer(cfg, application)
}

func performAppAuthRequest(httpServer *HTTPServer, method, path, token, appID, workspaceID, instanceID, bodyValue string) *ut.ResponseRecorder {
	headers := []ut.Header{
		ut.Header{Key: appAuthHeaderToken, Value: token},
		ut.Header{Key: appAuthHeaderAppID, Value: appID},
		ut.Header{Key: appAuthHeaderWorkspaceID, Value: workspaceID},
		ut.Header{Key: appAuthHeaderAppInstanceID, Value: instanceID},
	}
	var bodyOption *ut.Body
	if bodyValue != "" {
		body := bytes.NewBufferString(bodyValue)
		bodyOption = &ut.Body{Body: body, Len: body.Len()}
		headers = append(headers, ut.Header{Key: "Content-Type", Value: "application/json"})
	}
	return ut.PerformRequest(httpServer.engine.Engine, method, path, bodyOption, headers...)
}
