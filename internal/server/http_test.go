package server

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"github.com/hertz-contrib/sse"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/controlplane"
	"moss/internal/runtime"
	"moss/internal/session"
)

type skillPackageMetadataPayload struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	Revision   int      `json:"revision"`
	FileCount  int      `json:"file_count"`
	FilePaths  []string `json:"file_paths"`
	Enabled    bool     `json:"enabled"`
	Validation struct {
		Valid    bool     `json:"valid"`
		Errors   []string `json:"errors"`
		Warnings []string `json:"warnings"`
	} `json:"validation"`
	UploadedAt string `json:"uploaded_at"`
}

func TestParseChatStreamRequest(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"analyze","session_id":"sess-1","enabled_skills":["user_overview"],"supplement":{"data":{"user_id":"u1001"}},"resume_token":"resume-1","timeout_after_seconds":120}`)

	req, err := parseChatStreamRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatStreamRequest() error = %v", err)
	}
	if req.Query != "analyze" || req.SessionID != "sess-1" {
		t.Fatalf("unexpected request = %#v", req)
	}
	if req.Supplement == nil || req.Supplement.Data["user_id"] != "u1001" {
		t.Fatalf("unexpected supplement = %#v", req.Supplement)
	}
	if req.Supplement.Outcome != runtime.SupplementOutcomeProvided {
		t.Fatalf("supplement outcome = %q, want %q", req.Supplement.Outcome, runtime.SupplementOutcomeProvided)
	}
	if req.Supplement.Resume == nil || req.Supplement.Resume.ResumeToken != "resume-1" {
		t.Fatalf("unexpected resume context = %#v", req.Supplement.Resume)
	}
}

// TestHealthzEndpointStaysLivenessOnly verifies healthz does not depend on startup probe or runtime governance state.
// TestHealthzEndpointStaysLivenessOnly 用于验证 healthz 仍只表达活性状态，不依赖启动探测或运行时治理状态。
func TestHealthzEndpointStaysLivenessOnly(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/healthz", nil)
	if resp.Code != consts.StatusOK {
		t.Fatalf("healthz status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if body := strings.TrimSpace(resp.Body.String()); body != `{"status":"ok"}` {
		t.Fatalf("healthz body = %s, want {\"status\":\"ok\"}", body)
	}
}

func TestControlPlaneBootstrapEndpoint(t *testing.T) {
	truthDir := t.TempDir() + "/bootstrap/system-truth"
	application := appcore.NewService(config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath:      t.TempDir() + "/bootstrap/controlplane/overrides.json",
			AllowedOrigins: []string{"http://localhost:5173"},
		},
		System: config.SystemConfig{
			TruthDir: truthDir,
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	})
	_, err := application.CreateSystemResource(context.Background(), controlplane.SystemResourceCreateRequest{
		AssetID:       "spec.issue7.bootstrap",
		AssetName:     "Issue 7 Bootstrap",
		SourceContent: "# Issue 7 Bootstrap\n\nUse context assets.",
	})
	if err != nil {
		t.Fatalf("seed system resource failed: %v", err)
	}
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath:      t.TempDir() + "/controlplane/overrides.json",
			AllowedOrigins: []string{"http://localhost:5173"},
		},
		System: config.SystemConfig{
			TruthDir: truthDir,
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, application)
	resp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodGet,
		"/api/control-plane/bootstrap",
		nil,
		ut.Header{Key: "Origin", Value: "http://localhost:5173"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("bootstrap status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if header := string(resp.Header().Peek("Access-Control-Allow-Origin")); header != "http://localhost:5173" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want localhost dev origin", header)
	}
	if !strings.Contains(resp.Body.String(), `"swagger_spec_url":"/swagger/openapi.json"`) {
		t.Fatalf("bootstrap body = %s, want swagger spec url", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"system_resources"`) {
		t.Fatalf("bootstrap body = %s, want system resources", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"asset_id":"spec.issue7.bootstrap"`) {
		t.Fatalf("bootstrap body = %s, want seeded system resource", resp.Body.String())
	}
}

func TestControlPlaneBootstrapIncludesEmptySystemResourcesArray(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/bootstrap-empty/controlplane/overrides.json",
		},
		System: config.SystemConfig{
			TruthDir: t.TempDir() + "/bootstrap-empty/system-truth",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}
	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)
	resp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodGet,
		"/api/control-plane/bootstrap",
		nil,
		ut.Header{Key: "Origin", Value: "http://localhost:5173"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("bootstrap status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"system_resources":[]`) {
		t.Fatalf("bootstrap body = %s, want explicit empty system_resources array", resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"tools":[]`) {
		t.Fatalf("bootstrap body = %s, want no unreferenced control-plane tools", resp.Body.String())
	}
}

func TestControlPlaneAuthLifecycleAndProtectedRoutes(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath:         t.TempDir() + "/controlplane/overrides.json",
			AuthToken:         "issue7-control-plane-token",
			SessionTTLSecs:    3600,
			MaxFailedAttempts: 3,
		},
		System: config.SystemConfig{
			TruthDir: t.TempDir() + "/system-truth",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	unauthorized := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/bootstrap", nil)
	if unauthorized.Code != consts.StatusUnauthorized {
		t.Fatalf("unauthorized bootstrap status = %d, want %d; body=%s", unauthorized.Code, consts.StatusUnauthorized, unauthorized.Body.String())
	}

	statusBefore := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/auth/status", nil)
	if statusBefore.Code != consts.StatusOK {
		t.Fatalf("auth status before login = %d, want %d; body=%s", statusBefore.Code, consts.StatusOK, statusBefore.Body.String())
	}
	if !strings.Contains(statusBefore.Body.String(), `"authenticated":false`) {
		t.Fatalf("auth status before login body = %s, want unauthenticated", statusBefore.Body.String())
	}

	loginBody := `{"token":"issue7-control-plane-token"}`
	login := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/control-plane/login",
		&ut.Body{Body: strings.NewReader(loginBody), Len: len(loginBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if login.Code != consts.StatusOK {
		t.Fatalf("login status = %d, want %d; body=%s", login.Code, consts.StatusOK, login.Body.String())
	}
	cookie := firstCookieValue(login)
	if cookie == "" {
		t.Fatalf("login missing session cookie: headers=%v", login.Header())
	}
	if !strings.Contains(login.Body.String(), `"authenticated":true`) {
		t.Fatalf("login body = %s, want authenticated", login.Body.String())
	}

	protected := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodGet,
		"/api/control-plane/bootstrap",
		nil,
		ut.Header{Key: "Cookie", Value: cookie},
	)
	if protected.Code != consts.StatusOK {
		t.Fatalf("authorized bootstrap status = %d, want %d; body=%s", protected.Code, consts.StatusOK, protected.Body.String())
	}

	statusAfter := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodGet,
		"/api/control-plane/auth/status",
		nil,
		ut.Header{Key: "Cookie", Value: cookie},
	)
	if statusAfter.Code != consts.StatusOK {
		t.Fatalf("auth status after login = %d, want %d; body=%s", statusAfter.Code, consts.StatusOK, statusAfter.Body.String())
	}
	if !strings.Contains(statusAfter.Body.String(), `"authenticated":true`) {
		t.Fatalf("auth status after login body = %s, want authenticated", statusAfter.Body.String())
	}

	logout := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/control-plane/logout",
		nil,
		ut.Header{Key: "Cookie", Value: cookie},
	)
	if logout.Code != consts.StatusOK {
		t.Fatalf("logout status = %d, want %d; body=%s", logout.Code, consts.StatusOK, logout.Body.String())
	}
	if !strings.Contains(logout.Body.String(), `"authenticated":false`) {
		t.Fatalf("logout body = %s, want unauthenticated", logout.Body.String())
	}
}

func TestControlPlaneAuthLocksAfterRepeatedFailures(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath:         t.TempDir() + "/controlplane/overrides.json",
			AuthToken:         "issue7-control-plane-token",
			SessionTTLSecs:    3600,
			MaxFailedAttempts: 2,
		},
		System: config.SystemConfig{
			TruthDir: t.TempDir() + "/system-truth",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	for attempt := 1; attempt <= 2; attempt++ {
		body := `{"token":"wrong-token"}`
		resp := ut.PerformRequest(
			httpServer.engine.Engine,
			http.MethodPost,
			"/api/control-plane/login",
			&ut.Body{Body: strings.NewReader(body), Len: len(body)},
			ut.Header{Key: "Content-Type", Value: "application/json"},
		)
		want := consts.StatusUnauthorized
		if attempt == 2 {
			want = consts.StatusLocked
		}
		if resp.Code != want {
			t.Fatalf("attempt %d status = %d, want %d; body=%s", attempt, resp.Code, want, resp.Body.String())
		}
	}

	lockedStatus := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/auth/status", nil)
	if lockedStatus.Code != consts.StatusLocked {
		t.Fatalf("locked auth status = %d, want %d; body=%s", lockedStatus.Code, consts.StatusLocked, lockedStatus.Body.String())
	}
	if !strings.Contains(lockedStatus.Body.String(), `"lock_state":"locked"`) {
		t.Fatalf("locked auth status body = %s, want lock_state locked", lockedStatus.Body.String())
	}

	protected := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/bootstrap", nil)
	if protected.Code != consts.StatusLocked {
		t.Fatalf("locked protected route status = %d, want %d; body=%s", protected.Code, consts.StatusLocked, protected.Body.String())
	}
}

func TestSystemResourceEndpoints(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		System: config.SystemConfig{
			TruthDir: t.TempDir() + "/system-truth",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	createBody := `{"asset_id":"policy_rule.core.issue_7","asset_type":"policy_rule","asset_name":"Issue 7 Policy Rule","source_content":"---\nid: issue_7\nname: Issue 7 Rule\nsummary: Enforce context assets\nseverity: high\ncheckpoints:\n  - pre_inference\non_fail: ask\n---\n\n## Hard Gates\n- Enforce context assets.","message":"seed resource"}`
	create := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/system-resources",
		&ut.Body{Body: strings.NewReader(createBody), Len: len(createBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if create.Code != consts.StatusOK {
		t.Fatalf("create system resource status = %d, want %d; body=%s", create.Code, consts.StatusOK, create.Body.String())
	}
	if !strings.Contains(create.Body.String(), `"accepted":true`) {
		t.Fatalf("create system resource body = %s, want accepted mutation result", create.Body.String())
	}

	list := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources", nil)
	if list.Code != consts.StatusOK {
		t.Fatalf("list system resources status = %d, want %d; body=%s", list.Code, consts.StatusOK, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), `"asset_id":"policy_rule.core.issue_7"`) {
		t.Fatalf("list system resources body = %s, want asset id", list.Body.String())
	}

	detail := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7", nil)
	if detail.Code != consts.StatusOK {
		t.Fatalf("get system resource status = %d, want %d; body=%s", detail.Code, consts.StatusOK, detail.Body.String())
	}
	if !strings.Contains(detail.Body.String(), `"compile_result"`) {
		t.Fatalf("get system resource body = %s, want compile result", detail.Body.String())
	}

	source := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/source", nil)
	if source.Code != consts.StatusOK {
		t.Fatalf("get system resource source status = %d, want %d; body=%s", source.Code, consts.StatusOK, source.Body.String())
	}
	if !strings.Contains(source.Body.String(), `Enforce context assets.`) {
		t.Fatalf("get system resource source body = %s, want source content", source.Body.String())
	}

	debugPayload := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/debug-payload?endpoint=/api/chat/respond", nil)
	if debugPayload.Code != consts.StatusOK {
		t.Fatalf("get system resource debug payload status = %d, want %d; body=%s", debugPayload.Code, consts.StatusOK, debugPayload.Body.String())
	}
	if !strings.Contains(debugPayload.Body.String(), `"context_assets"`) || !strings.Contains(debugPayload.Body.String(), `"ref_type":"compiled_asset"`) {
		t.Fatalf("debug payload body = %s, want compiled context asset ref", debugPayload.Body.String())
	}

	parseResult := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/parse-result", nil)
	if parseResult.Code != consts.StatusOK {
		t.Fatalf("get parse result status = %d, want %d; body=%s", parseResult.Code, consts.StatusOK, parseResult.Body.String())
	}
	if !strings.Contains(parseResult.Body.String(), `"status":"parsed"`) {
		t.Fatalf("parse result body = %s, want parsed status", parseResult.Body.String())
	}

	compileResult := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/compile-result", nil)
	if compileResult.Code != consts.StatusOK {
		t.Fatalf("get compile result status = %d, want %d; body=%s", compileResult.Code, consts.StatusOK, compileResult.Body.String())
	}
	if !strings.Contains(compileResult.Body.String(), `"guidance_text"`) {
		t.Fatalf("compile result body = %s, want guidance text", compileResult.Body.String())
	}

	versions := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/versions", nil)
	if versions.Code != consts.StatusOK {
		t.Fatalf("get system resource versions status = %d, want %d; body=%s", versions.Code, consts.StatusOK, versions.Body.String())
	}
	if !strings.Contains(versions.Body.String(), `"version_id"`) {
		t.Fatalf("versions body = %s, want version id", versions.Body.String())
	}
	versionID := extractJSONField(t, versions.Body.String(), "version_id")
	if versionID == "" {
		t.Fatalf("versions body = %s, want extractable version_id", versions.Body.String())
	}

	versionDetail := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/versions/"+versionID, nil)
	if versionDetail.Code != consts.StatusOK {
		t.Fatalf("get system resource version detail status = %d, want %d; body=%s", versionDetail.Code, consts.StatusOK, versionDetail.Body.String())
	}
	if !strings.Contains(versionDetail.Body.String(), `"source_content"`) {
		t.Fatalf("version detail body = %s, want source_content", versionDetail.Body.String())
	}

	audit := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/audit", nil)
	if audit.Code != consts.StatusOK {
		t.Fatalf("get system resource audit status = %d, want %d; body=%s", audit.Code, consts.StatusOK, audit.Body.String())
	}
	if !strings.Contains(audit.Body.String(), `"event_id"`) {
		t.Fatalf("audit body = %s, want event_id", audit.Body.String())
	}

	rollback := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/system-resources/policy_rule.core.issue_7/versions/"+versionID+"/rollback", nil)
	if rollback.Code != consts.StatusOK {
		t.Fatalf("rollback system resource version status = %d, want %d; body=%s", rollback.Code, consts.StatusOK, rollback.Body.String())
	}
	if !strings.Contains(rollback.Body.String(), `"accepted":true`) {
		t.Fatalf("rollback body = %s, want accepted mutation", rollback.Body.String())
	}

	pipeline := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/pipeline", nil)
	if pipeline.Code != consts.StatusOK {
		t.Fatalf("get pipeline status = %d, want %d; body=%s", pipeline.Code, consts.StatusOK, pipeline.Body.String())
	}
	if !strings.Contains(pipeline.Body.String(), `"status":"active"`) {
		t.Fatalf("pipeline body = %s, want active status", pipeline.Body.String())
	}

	download := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/policy_rule.core.issue_7/download", nil)
	if download.Code != consts.StatusOK {
		t.Fatalf("download system resource status = %d, want %d; body=%s", download.Code, consts.StatusOK, download.Body.String())
	}
	if got := string(download.Header().Peek("Content-Disposition")); !strings.Contains(got, `issue_7.md`) {
		t.Fatalf("download content disposition = %q, want source filename", got)
	}
	if !strings.Contains(download.Body.String(), `Enforce context assets.`) {
		t.Fatalf("download body = %s, want source content", download.Body.String())
	}

	export := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/system-resources/export", nil)
	if export.Code != consts.StatusOK {
		t.Fatalf("export system resources status = %d, want %d; body=%s", export.Code, consts.StatusOK, export.Body.String())
	}
	if got := string(export.Header().Peek("X-Truth-Dir-Version")); got == "" {
		t.Fatalf("export X-Truth-Dir-Version header is empty")
	}
	reader, err := zip.NewReader(bytes.NewReader(export.Body.Bytes()), int64(export.Body.Len()))
	if err != nil {
		t.Fatalf("open export zip failed: %v", err)
	}
	var hasManifest bool
	for _, file := range reader.File {
		if file.Name == "manifest.json" {
			hasManifest = true
			break
		}
	}
	if !hasManifest {
		t.Fatalf("export zip missing manifest.json")
	}
}

func TestToolGovernanceEndpoints(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		System: config.SystemConfig{
			TruthDir: t.TempDir() + "/system-truth",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	createBody := `{"asset_id":"tool_governance_policy.core.default","asset_type":"tool_governance_policy","asset_name":"Default Tool Governance","source_content":"---\nid: default_tool_governance\nname: Default Tool Governance\ndefault_decision: allow\ndecision_model: first_match\nrules:\n  - id: redact_external_read\n    match_tool: demo_browser\n    match_scope: external_web\n    match_operation: read\n    match_risk: medium\n    decision: allow_with_redaction\n    reason: External reads may proceed with redaction.\n    redact_fields:\n      - headers.authorization\n---\n\n## Purpose\nGovern tool requests.","message":"seed tool governance"}`
	create := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/system-resources",
		&ut.Body{Body: strings.NewReader(createBody), Len: len(createBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if create.Code != consts.StatusOK {
		t.Fatalf("create tool governance resource status = %d, want %d; body=%s", create.Code, consts.StatusOK, create.Body.String())
	}

	policy := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/tool-governance/policy", nil)
	if policy.Code != consts.StatusOK {
		t.Fatalf("get tool governance policy status = %d, want %d; body=%s", policy.Code, consts.StatusOK, policy.Body.String())
	}
	if !strings.Contains(policy.Body.String(), `"decision_model":"first_match"`) || !strings.Contains(policy.Body.String(), `"rule_id":"redact_external_read"`) {
		t.Fatalf("tool governance policy body = %s, want compiled rule", policy.Body.String())
	}

	evaluateBody := `{"tool_name":"demo_browser","tool_scope":"external_web","operation":"read","risk_level":"medium","metadata":{"api_key":"secret","request_id":"req_1"}}`
	evaluate := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/control-plane/tool-governance/evaluate",
		&ut.Body{Body: strings.NewReader(evaluateBody), Len: len(evaluateBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if evaluate.Code != consts.StatusOK {
		t.Fatalf("evaluate tool governance status = %d, want %d; body=%s", evaluate.Code, consts.StatusOK, evaluate.Body.String())
	}
	if !strings.Contains(evaluate.Body.String(), `"decision":"allow_with_redaction"`) || !strings.Contains(evaluate.Body.String(), `"api_key":"[redacted]"`) {
		t.Fatalf("evaluate body = %s, want redaction decision and redacted metadata", evaluate.Body.String())
	}

	decisions := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/tool-governance/decisions", nil)
	if decisions.Code != consts.StatusOK {
		t.Fatalf("list tool governance decisions status = %d, want %d; body=%s", decisions.Code, consts.StatusOK, decisions.Body.String())
	}
	if !strings.Contains(decisions.Body.String(), `"matched_rule_id":"redact_external_read"`) {
		t.Fatalf("decisions body = %s, want persisted decision", decisions.Body.String())
	}
}

func TestValidationMCPEndpoints(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		System: config.SystemConfig{
			TruthDir: t.TempDir() + "/system-truth",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	createBody := `{"asset_id":"tool_governance_policy.validation_mcp","asset_type":"tool_governance_policy","asset_name":"Validation MCP Tool Governance","source_content":"---\nid: validation_mcp_tool_governance\nname: Validation MCP Tool Governance\ndefault_decision: allow\ndecision_model: first_match\nrules:\n  - id: redact_validation_mcp_risk_lookup\n    match_tool: risk_signal_lookup\n    match_scope: validation_mcp\n    match_operation: invoke\n    match_risk: medium\n    decision: allow_with_redaction\n    reason: Validation MCP risk lookups may proceed with redaction.\n    redact_fields:\n      - input.credentials\n---\n\n## Purpose\nGovern validation MCP requests.","message":"seed validation mcp governance"}`
	create := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/system-resources",
		&ut.Body{Body: strings.NewReader(createBody), Len: len(createBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if create.Code != consts.StatusOK {
		t.Fatalf("create validation mcp governance resource status = %d, want %d; body=%s", create.Code, consts.StatusOK, create.Body.String())
	}

	serverInfo := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/validation-mcp/server", nil)
	if serverInfo.Code != consts.StatusOK {
		t.Fatalf("get validation mcp server status = %d, want %d; body=%s", serverInfo.Code, consts.StatusOK, serverInfo.Body.String())
	}
	if !strings.Contains(serverInfo.Body.String(), `"server_id":"athena-validation-mcp"`) || !strings.Contains(serverInfo.Body.String(), `"name":"risk_signal_lookup"`) {
		t.Fatalf("server body = %s, want validation mcp server and tool schema", serverInfo.Body.String())
	}

	tools := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/validation-mcp/tools", nil)
	if tools.Code != consts.StatusOK {
		t.Fatalf("list validation mcp tools status = %d, want %d; body=%s", tools.Code, consts.StatusOK, tools.Body.String())
	}
	if !strings.Contains(tools.Body.String(), `"name":"security_context_echo"`) || !strings.Contains(tools.Body.String(), `"input_schema"`) {
		t.Fatalf("tools body = %s, want both schemas", tools.Body.String())
	}

	invokeBody := `{"tool_name":"risk_signal_lookup","input":{"risk_key":"credential_export","credentials":{"authorization":"Bearer raw-token"}},"metadata":{"authorization_token":"raw-token","request_id":"req-1"}}`
	invoke := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/control-plane/validation-mcp/invocations",
		&ut.Body{Body: strings.NewReader(invokeBody), Len: len(invokeBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if invoke.Code != consts.StatusOK {
		t.Fatalf("invoke validation mcp status = %d, want %d; body=%s", invoke.Code, consts.StatusOK, invoke.Body.String())
	}
	body := invoke.Body.String()
	if !strings.Contains(body, `"governance_decision"`) || !strings.Contains(body, `"matched_rule_id":"redact_validation_mcp_risk_lookup"`) {
		t.Fatalf("invoke body = %s, want governance decision with matched rule", body)
	}
	if !strings.Contains(body, `"result_summary":"risk signal credential_export classified as high"`) || !strings.Contains(body, `"trace_type":"validation_mcp_tool_invocation"`) {
		t.Fatalf("invoke body = %s, want result summary and trace", body)
	}
	if strings.Contains(body, "raw-token") || !strings.Contains(body, `"[redacted]"`) {
		t.Fatalf("invoke body = %s, want raw credentials redacted", body)
	}
}

func TestPutControlPlaneSceneEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	body := `{"id":"security_review","description":"custom security review","keywords":["安全评估"],"default_skills":["cso_review"],"suggested_questions":["是否需要我输出风险清单？"],"enabled":true,"match_score":91}`
	resp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/scenes/security_review",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("put scene status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"match_score":91`) {
		t.Fatalf("put scene body = %s, want persisted score", resp.Body.String())
	}
}

func TestPutControlPlaneToolEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	body := `{"name":"lookup_profile","description":"custom profile tool","tool_scope":"customer_profile","requires_confirmation":false,"side_effect_level":"none","input_schema_summary":"user_id:string","output_schema_summary":"profile summary","enabled":true}`
	resp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/tools/lookup_profile",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("put tool status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"tool_scope":"customer_profile"`) {
		t.Fatalf("put tool body = %s, want persisted tool_scope", resp.Body.String())
	}
}

func TestPutControlPlaneGovernanceEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	body := `{"choice_required_enabled":true,"automation_fallback_enabled":true,"planning_progress_enabled":true,"fact_quality_gate_enabled":false,"tool_hint_emission_enabled":true,"knowledge_retrieval_emission_enabled":false,"max_planning_steps":5,"max_tool_hints":2}`
	resp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/governance",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("put governance status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"max_tool_hints":2`) {
		t.Fatalf("put governance body = %s, want max_tool_hints", resp.Body.String())
	}
}

func TestRollbackControlPlaneConfigVersionEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}

	service := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, service)

	sceneV1 := `{"id":"security_review","description":"v1","keywords":["安全评估"],"default_skills":["cso_review"],"suggested_questions":["是否需要我输出风险清单？"],"enabled":true,"match_score":91}`
	resp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/scenes/security_review",
		&ut.Body{Body: strings.NewReader(sceneV1), Len: len(sceneV1)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("seed scene v1 status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}

	versionsResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/config-versions", nil)
	if versionsResp.Code != consts.StatusOK {
		t.Fatalf("list versions status = %d, want %d; body=%s", versionsResp.Code, consts.StatusOK, versionsResp.Body.String())
	}
	var versionsPayload struct {
		Items []struct {
			VersionID string `json:"version_id"`
		} `json:"items"`
	}
	if err := json.Unmarshal(versionsResp.Body.Bytes(), &versionsPayload); err != nil {
		t.Fatalf("unmarshal versions payload error = %v", err)
	}
	if len(versionsPayload.Items) == 0 {
		t.Fatal("expected at least one config version")
	}
	targetVersionID := versionsPayload.Items[0].VersionID

	sceneV2 := `{"id":"security_review","description":"v2","keywords":["安全评估"],"default_skills":["cso_review"],"suggested_questions":["是否需要我输出风险清单？"],"enabled":true,"match_score":91}`
	resp = ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/scenes/security_review",
		&ut.Body{Body: strings.NewReader(sceneV2), Len: len(sceneV2)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if resp.Code != consts.StatusOK {
		t.Fatalf("seed scene v2 status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}

	rollbackResp := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/control-plane/config-versions/"+targetVersionID+"/rollback",
		nil,
	)
	if rollbackResp.Code != consts.StatusOK {
		t.Fatalf("rollback status = %d, want %d; body=%s", rollbackResp.Code, consts.StatusOK, rollbackResp.Body.String())
	}
	if !strings.Contains(rollbackResp.Body.String(), `"summary":"rollback to `) {
		t.Fatalf("rollback body = %s, want rollback summary", rollbackResp.Body.String())
	}
}

func TestParseChatStreamRequestAppliesSupplementOutcome(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"retry","supplement_outcome":"unable_to_provide"}`)

	req, err := parseChatStreamRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatStreamRequest() error = %v", err)
	}
	if req.Supplement == nil || req.Supplement.Outcome != runtime.SupplementOutcomeUnableToProvide {
		t.Fatalf("unexpected supplement outcome = %#v", req.Supplement)
	}
}

func TestParseChatStreamRequestDisableFastPath(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"retry","disable_fast_path":true}`)

	req, err := parseChatStreamRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatStreamRequest() error = %v", err)
	}
	if !req.DisableFastPath {
		t.Fatalf("expected disable_fast_path to be true")
	}
}

func TestParseChatStreamRequestModelID(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"analyze","model_id":"model-selected"}`)

	req, err := parseChatStreamRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatStreamRequest() error = %v", err)
	}
	if req.ModelID != "model-selected" {
		t.Fatalf("model_id = %q, want model-selected", req.ModelID)
	}
}

func TestParseChatStreamRequestRejectsModelRecordIDAlias(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"analyze","model_record_id":"model-legacy"}`)

	if _, err := parseChatStreamRequest(ctx); err == nil {
		t.Fatalf("expected model_record_id rejection")
	}
}

func TestParseChatStreamRequestRequiresBody(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	if _, err := parseChatStreamRequest(ctx); err == nil {
		t.Fatalf("expected request body error")
	}
}

func TestParseChatRespondRequest(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"respond","model_id":"model-selected","strict_schema_validation":true,"schema_retry_count":3,"schema_repair_mode":"basic","schema_failure_action":"error","supplement":{"data":{"user_id":"u1001"}}}`)

	req, err := parseChatRespondRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatRespondRequest() error = %v", err)
	}
	if req.Query != "respond" || req.ModelID != "model-selected" {
		t.Fatalf("unexpected request = %#v", req)
	}
	if !req.StrictSchemaValidation || req.SchemaRetryCount != 3 || req.SchemaRepairMode != "basic" || req.SchemaFailureAction != "error" {
		t.Fatalf("unexpected schema controls = %#v", req)
	}
	if req.Supplement == nil || req.Supplement.Data["user_id"] != "u1001" {
		t.Fatalf("unexpected supplement = %#v", req.Supplement)
	}
}

func TestParseChatStreamRequestTaskFields(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"task_type":"inspection_task","workspace_id":"ws-1","main_session_id":"sess-1","integration_instance_id":"integration-1","trigger_type":"manual","input_payload":{"target":"asset-1"},"global_context":{"org_id":"org-1"},"app_context":{"mode":"inspection"}}`)

	req, err := parseChatStreamRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatStreamRequest() error = %v", err)
	}
	if req.TaskType != "inspection_task" || req.WorkspaceID != "ws-1" || req.MainSessionID != "sess-1" {
		t.Fatalf("unexpected task request = %#v", req)
	}
}

func TestParseChatRespondRequestRejectsModelRecordIDAlias(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(`{"query":"respond","model_record_id":"legacy-model"}`)

	if _, err := parseChatRespondRequest(ctx); err == nil {
		t.Fatalf("expected model_record_id rejection")
	}
}

// TestListRuntimeSkillsEndpoint verifies runtime skill metadata can be queried by source without exposing internal asset bundles.
// TestListRuntimeSkillsEndpoint 用于验证 runtime skill 元数据可以按来源查询，同时不会暴露内部资产 bundle。
func TestListRuntimeSkillsEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
	}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/runtime/skills?source=product_managed&task_type=runtime_event_analysis&task_subtype=openclaw_runtime_explanation&requested_output_mode=summary", nil)
	if resp.Code != consts.StatusOK {
		t.Fatalf("runtime skills status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"id":"skill_mosi_audit_operator_v1"`) {
		t.Fatalf("expected audit operator in response: %s", resp.Body.String())
	}
}

// TestRuntimeScenarioRespondEndpointReturnsEvidenceWait verifies scenario respond enters evidence supplement instead of forcing chat-style user supplement.
// TestRuntimeScenarioRespondEndpointReturnsEvidenceWait 用于验证 scenario respond 会进入 evidence supplement，而不是强行走 chat 风格用户补数。
func TestRuntimeScenarioRespondEndpointReturnsEvidenceWait(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
	}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	body := bytes.NewBufferString(`{
		"host_type":"openclaw",
		"hook_name":"before_tool_call",
		"event_type":"runtime_event",
		"raw_payload":{"params":{"command":"bash tools/deploy.sh"}}
	}`)
	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/runtime/scenario/respond", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusOK {
		t.Fatalf("runtime respond status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	if !strings.Contains(resp.Body.String(), `"status":"waiting_for_evidence"`) {
		t.Fatalf("expected waiting_for_evidence response: %s", resp.Body.String())
	}
}

func TestRuntimeScenarioRespondEndpointMapsInvalidSessionToNotFound(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
	}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	body := bytes.NewBufferString(`{
		"session_id":"sess-missing",
		"host_type":"openclaw",
		"hook_name":"before_tool_call",
		"event_type":"runtime_event"
	}`)
	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/runtime/scenario/respond", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusNotFound {
		t.Fatalf("runtime respond status = %d, want %d; body=%s", resp.Code, consts.StatusNotFound, resp.Body.String())
	}
}

func TestRuntimeScenarioRespondEndpointMapsInvalidResumeToBadRequest(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
	}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	body := bytes.NewBufferString(`{
		"host_type":"openclaw",
		"hook_name":"before_tool_call",
		"event_type":"runtime_event",
		"raw_payload":{"params":{"command":"bash tools/deploy.sh"}}
	}`)
	waiting := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/runtime/scenario/respond", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if waiting.Code != consts.StatusOK {
		t.Fatalf("runtime respond waiting status = %d, want %d; body=%s", waiting.Code, consts.StatusOK, waiting.Body.String())
	}
	var response struct {
		SessionID       string `json:"session_id"`
		EvidenceRequest struct {
			ResumeToken string `json:"resume_token"`
		} `json:"evidence_request"`
	}
	if err := json.Unmarshal(waiting.Body.Bytes(), &response); err != nil {
		t.Fatalf("unmarshal waiting response failed: %v", err)
	}

	resumeBody := bytes.NewBufferString(`{
		"session_id":"` + response.SessionID + `",
		"resume_token":"resume-invalid",
		"evidence_supplement":{"script_content":"echo ok"}
	}`)
	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/runtime/scenario/respond", &ut.Body{Body: resumeBody, Len: resumeBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusBadRequest {
		t.Fatalf("runtime respond status = %d, want %d; body=%s", resp.Code, consts.StatusBadRequest, resp.Body.String())
	}
}

func TestRuntimeRespondEndpointUsesCommonDirectRespondPath(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests: 1,
			RequestTimeoutSeconds: 30,
		},
	}
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))

	body := bytes.NewBufferString(`{"query":"hello from runtime respond","task_type":"custom_runtime_task","task_subtype":"direct_response"}`)
	resp := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/runtime/respond", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resp.Code != consts.StatusOK {
		t.Fatalf("runtime respond status = %d, want %d; body=%s", resp.Code, consts.StatusOK, resp.Body.String())
	}
	bodyText := resp.Body.String()
	if !strings.Contains(bodyText, `"status":"invalid_model"`) {
		t.Fatalf("runtime respond body = %s, want common invalid model response", bodyText)
	}
	if !strings.Contains(bodyText, `"code":"invalid_model"`) {
		t.Fatalf("runtime respond body = %s, want common runtime error detail", bodyText)
	}
	if strings.Contains(bodyText, `"evidence_request"`) {
		t.Fatalf("runtime respond body = %s, should not use scenario evidence wait contract", bodyText)
	}
	var envelope map[string]any
	if err := json.Unmarshal(resp.Body.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal runtime respond body failed: %v", err)
	}
	for _, key := range []string{"request_id", "status", "error_detail", "schema_validation"} {
		if _, ok := envelope[key]; !ok {
			t.Fatalf("runtime respond missing generic envelope key %q: %#v", key, envelope)
		}
	}
	for _, legacyKey := range []string{"evidence_request", "host_projection", "suggested_actions", "decision", "audit_summary"} {
		if _, ok := envelope[legacyKey]; ok {
			t.Fatalf("runtime respond leaked scenario key %q: %#v", legacyKey, envelope)
		}
	}
	if detail, ok := envelope["error_detail"].(map[string]any); !ok || detail["code"] != "invalid_model" {
		t.Fatalf("runtime respond error_detail = %#v, want invalid_model", envelope["error_detail"])
	}
}

func TestNewRequestIDKeepsPrefixAndEntropy(t *testing.T) {
	first := newRequestID()
	second := newRequestID()
	if !strings.HasPrefix(first, "req-") || !strings.HasPrefix(second, "req-") {
		t.Fatalf("unexpected request id prefix: %q %q", first, second)
	}
	if first == second {
		t.Fatalf("expected distinct request ids, got %q and %q", first, second)
	}
}

func TestProviderHeadersInputSupportsObjectAndList(t *testing.T) {
	var objectReq UpsertModelProviderRequest
	if err := json.Unmarshal([]byte(`{"name":"demo","protocol":"openai_compatible","headers":{"Accept-Encoding":"identity"}}`), &objectReq); err != nil {
		t.Fatalf("unmarshal object headers failed: %v", err)
	}
	if objectReq.Headers.Values["Accept-Encoding"] != "identity" {
		t.Fatalf("unexpected object headers = %#v", objectReq.Headers.Values)
	}

	var listReq UpsertModelProviderRequest
	if err := json.Unmarshal([]byte(`{"name":"demo","protocol":"openai_compatible","headers":[{"key":"Accept-Encoding","value":"identity"},{"key":"X-Trace","value":"demo"}]}`), &listReq); err != nil {
		t.Fatalf("unmarshal list headers failed: %v", err)
	}
	if listReq.Headers.Values["X-Trace"] != "demo" {
		t.Fatalf("unexpected list headers = %#v", listReq.Headers.Values)
	}
}

func TestChatStreamRequiresQuery(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	httpServer := NewHTTPServer(cfg, nil)
	recorder := ut.PerformRequest(httpServer.engine.Engine, "POST", "/api/chat/stream", &ut.Body{Body: strings.NewReader(`{}`), Len: 2}, ut.Header{Key: "Content-Type", Value: "application/json"})
	resp := recorder.Result()
	body := string(resp.Body())

	if resp.StatusCode() != 400 {
		t.Fatalf("status = %d, want 400; body=%s", resp.StatusCode(), body)
	}
	if !strings.Contains(body, `"error":"query is required unless supplement is provided"`) {
		t.Fatalf("unexpected body: %s", body)
	}
}

func TestParseChatStreamRequestKeepsLegacyTaskFieldsOptional(t *testing.T) {
	payload := `{"task_type":"workflow_step_request","workspace_id":"ws-1","main_session_id":"sess-1"}`
	ctx := hertzapp.NewContext(0)
	ctx.Request.SetBodyString(payload)

	req, err := parseChatStreamRequest(ctx)
	if err != nil {
		t.Fatalf("parseChatStreamRequest() error = %v", err)
	}
	if req.TaskType != "workflow_step_request" || req.WorkflowRunID != "" || req.StepID != "" {
		t.Fatalf("unexpected legacy task request = %#v", req)
	}
}

func TestChatRespondReturnsInvalidModelJSON(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
			MaxConcurrentRequests:     2,
			MaxConcurrentTools:        2,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "respond-invalid-model-test-key",
		},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	recorder := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/chat/respond",
		&ut.Body{Body: strings.NewReader(`{"query":"analyze risk","model_id":"model-missing","strict_schema_validation":true}`), Len: len(`{"query":"analyze risk","model_id":"model-missing","strict_schema_validation":true}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if recorder.Code != consts.StatusOK {
		t.Fatalf("chat respond status = %d, want %d; body=%s", recorder.Code, consts.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"invalid_model"`) {
		t.Fatalf("expected invalid_model status in body: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"model_id":"model-missing"`) {
		t.Fatalf("expected normalized model_id detail in body: %s", recorder.Body.String())
	}
}

func TestChatRespondReturnsWaitingJSON(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
			MaxConcurrentRequests:     2,
			MaxConcurrentTools:        2,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "respond-waiting-test-key",
		},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	createProvider := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers",
		&ut.Body{Body: strings.NewReader(`{"name":"respond-test-provider","protocol":"openai_compatible","base_url":"https://example.com/v1","api_key":"sk-demo","models":[{"model_id":"respond-model","display_name":"Respond Model","is_default":true}]}`), Len: len(`{"name":"respond-test-provider","protocol":"openai_compatible","base_url":"https://example.com/v1","api_key":"sk-demo","models":[{"model_id":"respond-model","display_name":"Respond Model","is_default":true}]}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if createProvider.Code != consts.StatusCreated {
		t.Fatalf("create provider status = %d, want %d; body=%s", createProvider.Code, consts.StatusCreated, createProvider.Body.String())
	}
	var providerPayload struct {
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(createProvider.Body.String()), &providerPayload); err != nil {
		t.Fatalf("unmarshal provider payload failed: %v", err)
	}
	if len(providerPayload.Models) != 1 || providerPayload.Models[0].ID == "" {
		t.Fatalf("unexpected provider payload = %#v", providerPayload)
	}

	recorder := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/chat/respond",
		&ut.Body{Body: strings.NewReader(`{"query":"show user profile","model_id":"` + providerPayload.Models[0].ID + `","enabled_skills":["user_overview"],"disable_fast_path":true,"strict_schema_validation":true}`), Len: len(`{"query":"show user profile","model_id":"` + providerPayload.Models[0].ID + `","enabled_skills":["user_overview"],"disable_fast_path":true,"strict_schema_validation":true}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if recorder.Code != consts.StatusOK {
		t.Fatalf("chat respond status = %d, want %d; body=%s", recorder.Code, consts.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"waiting_for_information"`) {
		t.Fatalf("expected waiting_for_information status in body: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"action_type":"information_request"`) {
		t.Fatalf("expected information_request action type in body: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"resume_token":"`) {
		t.Fatalf("expected resume_token in body: %s", recorder.Body.String())
	}
}

func TestSwaggerOpenAPISpecEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	httpServer := NewHTTPServer(cfg, nil)
	recorder := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/swagger/openapi.json", nil)
	resp := recorder.Result()
	body := string(resp.Body())

	if resp.StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode(), consts.StatusOK, body)
	}
	if !strings.Contains(body, `"openapi": "3.0.3"`) {
		t.Fatalf("expected openapi version in body: %s", body)
	}
	if !strings.Contains(body, `"/api/models/providers"`) {
		t.Fatalf("expected model providers path in body: %s", body)
	}
	if !strings.Contains(body, `"/api/chat/respond"`) {
		t.Fatalf("expected chat respond path in body: %s", body)
	}
	if !strings.Contains(body, `"/api/system-resources"`) {
		t.Fatalf("expected system resources path in body: %s", body)
	}
	if !strings.Contains(body, `"/api/control-plane/runtime/contracts/foundation"`) {
		t.Fatalf("expected runtime contract foundation path in body: %s", body)
	}

	var spec map[string]any
	if err := json.Unmarshal(resp.Body(), &spec); err != nil {
		t.Fatalf("unmarshal openapi spec failed: %v", err)
	}

	components, ok := spec["components"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing components: %#v", spec)
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing schemas: %#v", components)
	}
	createProviderSchema, ok := schemas["CreateModelProviderRequest"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing CreateModelProviderRequest: %#v", schemas)
	}
	properties, ok := createProviderSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("provider schema missing properties: %#v", createProviderSchema)
	}
	protocol, ok := properties["protocol"].(map[string]any)
	if !ok {
		t.Fatalf("provider schema missing protocol property: %#v", properties)
	}
	if got := protocol["description"]; got != "Athena 构建聊天客户端时使用的传输协议。留空时默认按 openai_compatible 处理。" {
		t.Fatalf("protocol description = %#v", got)
	}
	enumValues, ok := protocol["enum"].([]any)
	if !ok || len(enumValues) != 3 {
		t.Fatalf("protocol enum = %#v", protocol["enum"])
	}
	if enumValues[0] != "openai_compatible" || enumValues[1] != "ark" || enumValues[2] != "anthropic" {
		t.Fatalf("protocol enum values = %#v", enumValues)
	}

	paths, ok := spec["paths"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing paths: %#v", spec)
	}
	providerPath, ok := paths["/api/models/providers"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing provider path: %#v", paths)
	}
	createProviderOp, ok := providerPath["post"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing create provider operation: %#v", providerPath)
	}
	if got := createProviderOp["description"]; got != "创建一个供应商，并可在同一次请求中可选地一并创建其子模型。返回结果中的每个模型记录都可以继续通过 POST /api/models/providers/{id}/models/{record_id}/test 校验是否真实可用。" {
		t.Fatalf("create provider description = %#v", got)
	}
	if _, ok := properties["models"]; !ok {
		t.Fatalf("create provider schema missing models property: %#v", properties)
	}
	if _, ok := schemas["SessionResource"]; !ok {
		t.Fatalf("openapi spec missing SessionResource: %#v", schemas)
	}
	if _, ok := paths["/api/sessions"]; !ok {
		t.Fatalf("openapi spec missing /api/sessions path: %#v", paths)
	}
	if _, ok := paths["/api/system-resources"]; !ok {
		t.Fatalf("openapi spec missing /api/system-resources path: %#v", paths)
	}
	if _, ok := paths["/api/system-resources/{id}/debug-payload"]; !ok {
		t.Fatalf("openapi spec missing system resource debug-payload path: %#v", paths)
	}
	if _, ok := paths["/api/system-resources/{id}/versions"]; !ok {
		t.Fatalf("openapi spec missing system resource versions path: %#v", paths)
	}
	if _, ok := paths["/api/system-resources/{id}/versions/{versionID}/rollback"]; !ok {
		t.Fatalf("openapi spec missing system resource version rollback path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/contracts/foundation"]; !ok {
		t.Fatalf("openapi spec missing runtime contract foundation path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/contracts/{contractID}"]; !ok {
		t.Fatalf("openapi spec missing runtime contract write path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/task-types/{typeKey}"]; !ok {
		t.Fatalf("openapi spec missing runtime task type write path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/hook-bindings/{bindingID}"]; !ok {
		t.Fatalf("openapi spec missing runtime hook binding write path: %#v", paths)
	}
	if _, ok := schemas["RuntimeContractFoundationResponse"]; !ok {
		t.Fatalf("openapi spec missing RuntimeContractFoundationResponse: %#v", schemas)
	}
	if _, ok := schemas["RuntimeContract"]; !ok {
		t.Fatalf("openapi spec missing RuntimeContract: %#v", schemas)
	}
	if _, ok := schemas["RuntimeContractUpsertRequest"]; !ok {
		t.Fatalf("openapi spec missing RuntimeContractUpsertRequest: %#v", schemas)
	}
	if _, ok := schemas["RuntimeTaskTypeUpsertRequest"]; !ok {
		t.Fatalf("openapi spec missing RuntimeTaskTypeUpsertRequest: %#v", schemas)
	}
	if _, ok := schemas["RuntimeHookBindingUpsertRequest"]; !ok {
		t.Fatalf("openapi spec missing RuntimeHookBindingUpsertRequest: %#v", schemas)
	}
	if _, ok := paths["/api/system-resources/{id}/audit"]; !ok {
		t.Fatalf("openapi spec missing system resource audit path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/auth/status"]; !ok {
		t.Fatalf("openapi spec missing /api/control-plane/auth/status path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/login"]; !ok {
		t.Fatalf("openapi spec missing /api/control-plane/login path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/logout"]; !ok {
		t.Fatalf("openapi spec missing /api/control-plane/logout path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/runs"]; !ok {
		t.Fatalf("openapi spec missing /api/control-plane/runtime/runs path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/validation-runs"]; !ok {
		t.Fatalf("openapi spec missing /api/control-plane/runtime/validation-runs path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/runs/{runID}/traces"]; !ok {
		t.Fatalf("openapi spec missing runtime traces path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/runtime/runs/{runID}/checkpoints"]; !ok {
		t.Fatalf("openapi spec missing runtime checkpoints path: %#v", paths)
	}
	if _, ok := paths["/api/control-plane/validation-mcp/invocations"]; !ok {
		t.Fatalf("openapi spec missing validation mcp invocation path: %#v", paths)
	}
	if _, ok := schemas["RuntimeRun"]; !ok {
		t.Fatalf("openapi spec missing RuntimeRun schema: %#v", schemas)
	}
	if _, ok := schemas["RuntimeTraceListResponse"]; !ok {
		t.Fatalf("openapi spec missing RuntimeTraceListResponse schema: %#v", schemas)
	}
	if _, ok := schemas["RuntimeCheckpointReadoutListResponse"]; !ok {
		t.Fatalf("openapi spec missing RuntimeCheckpointReadoutListResponse schema: %#v", schemas)
	}
	if _, ok := schemas["RuntimeValidationRunResponse"]; !ok {
		t.Fatalf("openapi spec missing RuntimeValidationRunResponse schema: %#v", schemas)
	}
	if _, ok := schemas["ValidationMCPInvocationResponse"]; !ok {
		t.Fatalf("openapi spec missing ValidationMCPInvocationResponse schema: %#v", schemas)
	}

	respondPath, ok := paths["/api/chat/respond"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing chat respond path: %#v", paths)
	}
	respondOp, ok := respondPath["post"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing chat respond operation: %#v", respondPath)
	}
	if got := respondOp["summary"]; got != "获取结构化聊天结果" {
		t.Fatalf("chat respond summary = %#v", got)
	}

	testResultSchema, ok := schemas["ModelTestResponse"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing ModelTestResponse: %#v", schemas)
	}
	testResultProps, ok := testResultSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("model test schema missing properties: %#v", testResultSchema)
	}
	if _, ok := testResultProps["available"]; !ok {
		t.Fatalf("model test schema missing available: %#v", testResultProps)
	}
	if _, ok := testResultProps["duration_ms"]; !ok {
		t.Fatalf("model test schema missing duration_ms: %#v", testResultProps)
	}
	if _, ok := testResultProps["model_id"]; !ok {
		t.Fatalf("model test schema missing model_id: %#v", testResultProps)
	}
	if _, ok := schemas["SystemResourceSummary"]; !ok {
		t.Fatalf("openapi spec missing SystemResourceSummary: %#v", schemas)
	}
	if _, ok := schemas["SystemResourceVersionSummary"]; !ok {
		t.Fatalf("openapi spec missing SystemResourceVersionSummary: %#v", schemas)
	}
	if _, ok := schemas["SystemResourceVersionDetail"]; !ok {
		t.Fatalf("openapi spec missing SystemResourceVersionDetail: %#v", schemas)
	}
	if _, ok := schemas["SystemResourceAuditEntry"]; !ok {
		t.Fatalf("openapi spec missing SystemResourceAuditEntry: %#v", schemas)
	}
	if _, ok := schemas["ControlPlaneAuthStatus"]; !ok {
		t.Fatalf("openapi spec missing ControlPlaneAuthStatus: %#v", schemas)
	}
	if _, ok := schemas["ControlPlaneLoginRequest"]; !ok {
		t.Fatalf("openapi spec missing ControlPlaneLoginRequest: %#v", schemas)
	}
	bootstrapSchema, ok := schemas["ControlPlaneBootstrapResponse"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing ControlPlaneBootstrapResponse: %#v", schemas)
	}
	bootstrapProps, ok := bootstrapSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("bootstrap schema missing properties: %#v", bootstrapSchema)
	}
	if _, ok := bootstrapProps["system_resources"]; !ok {
		t.Fatalf("bootstrap schema missing system_resources: %#v", bootstrapProps)
	}

	skillDeleteSchema, ok := schemas["DeleteSkillPackageResponse"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing DeleteSkillPackageResponse: %#v", schemas)
	}
	skillDeleteProps, ok := skillDeleteSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("skill delete schema missing properties: %#v", skillDeleteSchema)
	}
	if _, ok := skillDeleteProps["id"]; !ok {
		t.Fatalf("skill delete schema missing id: %#v", skillDeleteProps)
	}
	if _, ok := skillDeleteProps["status"]; !ok {
		t.Fatalf("skill delete schema missing status: %#v", skillDeleteProps)
	}

	rollbackSchema, ok := schemas["SkillPackageRollbackResponse"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing SkillPackageRollbackResponse: %#v", schemas)
	}
	rollbackProps, ok := rollbackSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("rollback schema missing properties: %#v", rollbackSchema)
	}
	if _, ok := rollbackProps["metadata"]; !ok {
		t.Fatalf("rollback schema missing metadata: %#v", rollbackProps)
	}
	if _, ok := rollbackProps["rolled_back_from"]; !ok {
		t.Fatalf("rollback schema missing rolled_back_from: %#v", rollbackProps)
	}
	if _, ok := rollbackProps["current_revision"]; !ok {
		t.Fatalf("rollback schema missing current_revision: %#v", rollbackProps)
	}

	providerName, ok := properties["name"].(map[string]any)
	if !ok || providerName["example"] != "openrouter-main" {
		t.Fatalf("provider name example = %#v", providerName)
	}
	upsertProviderModel, ok := schemas["UpsertProviderModelRequest"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing UpsertProviderModelRequest: %#v", schemas)
	}
	upsertProviderModelProps, ok := upsertProviderModel["properties"].(map[string]any)
	if !ok {
		t.Fatalf("provider model request missing properties: %#v", upsertProviderModel)
	}
	if got := upsertProviderModelProps["model_id"].(map[string]any)["example"]; got != "gpt-4o-mini" {
		t.Fatalf("provider model_id example = %#v", got)
	}
	skillPackageSchema, ok := schemas["SkillPackageResponse"].(map[string]any)
	if !ok {
		t.Fatalf("openapi spec missing SkillPackageResponse: %#v", schemas)
	}
	skillPackageProps, ok := skillPackageSchema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("skill package schema missing properties: %#v", skillPackageSchema)
	}
	if _, ok := skillPackageProps["file_count"]; !ok {
		t.Fatalf("skill package schema missing file_count: %#v", skillPackageProps)
	}
	if _, ok := skillPackageProps["file_paths"]; !ok {
		t.Fatalf("skill package schema missing file_paths: %#v", skillPackageProps)
	}
	if _, ok := skillPackageProps["validation"]; !ok {
		t.Fatalf("skill package schema missing validation: %#v", skillPackageProps)
	}
	if got := string(resp.Header.ContentType()); got != "application/json; charset=utf-8" {
		t.Fatalf("content-type = %q, want %q", got, "application/json; charset=utf-8")
	}
}

func TestSwaggerUIEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	httpServer := NewHTTPServer(cfg, nil)
	recorder := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/swagger", nil)
	resp := recorder.Result()
	body := string(resp.Body())

	if resp.StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode(), consts.StatusOK, body)
	}
	if !strings.Contains(body, "/swagger/openapi.json") {
		t.Fatalf("expected swagger openapi url in body: %s", body)
	}
	if !strings.Contains(body, "/swagger/assets/swagger-ui.css") {
		t.Fatalf("expected local swagger css url in body: %s", body)
	}
	if !strings.Contains(body, "/swagger/assets/swagger-ui-bundle.js") {
		t.Fatalf("expected local swagger bundle url in body: %s", body)
	}
	if !strings.Contains(body, "stabilizeSwaggerBodyEditors") {
		t.Fatalf("expected swagger body editor sync shim in body: %s", body)
	}
	if !strings.Contains(body, "SwaggerUIBundle") {
		t.Fatalf("expected swagger ui bundle in body: %s", body)
	}
	if got := string(resp.Header.ContentType()); got != "text/html; charset=utf-8" {
		t.Fatalf("content-type = %q, want %q", got, "text/html; charset=utf-8")
	}
}

func TestSwaggerAssetsEndpoints(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	httpServer := NewHTTPServer(cfg, nil)

	css := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/swagger/assets/swagger-ui.css", nil)
	if css.Code != consts.StatusOK {
		t.Fatalf("css status = %d, want %d; body=%s", css.Code, consts.StatusOK, css.Body.String())
	}
	if !strings.Contains(css.Body.String(), ".swagger-ui") {
		t.Fatalf("expected swagger css body")
	}

	js := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/swagger/assets/swagger-ui-bundle.js", nil)
	if js.Code != consts.StatusOK {
		t.Fatalf("js status = %d, want %d; body=%s", js.Code, consts.StatusOK, js.Body.String())
	}
	if !strings.Contains(js.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("expected swagger bundle body")
	}
}

func TestListSkillsEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)
	recorder := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills", nil)

	resp := recorder.Result()
	if resp.StatusCode() != consts.StatusOK {
		t.Fatalf("status = %d, want %d; body=%s", resp.StatusCode(), consts.StatusOK, string(resp.Body()))
	}

	var payload struct {
		Items []struct {
			Name   string `json:"name"`
			Source string `json:"source"`
		} `json:"items"`
	}
	if err := json.Unmarshal(resp.Body(), &payload); err != nil {
		t.Fatalf("unmarshal response failed: %v", err)
	}
	if len(payload.Items) == 0 {
		t.Fatalf("expected at least one visible skill")
	}
}

func TestSessionResourceEndpoints(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
		},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	create := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/sessions",
		&ut.Body{Body: strings.NewReader(`{"title":"会话标题"}`), Len: len(`{"title":"会话标题"}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if create.Code != consts.StatusCreated {
		t.Fatalf("create session status = %d, want %d; body=%s", create.Code, consts.StatusCreated, create.Body.String())
	}
	var created struct {
		ID       string `json:"id"`
		Title    string `json:"title"`
		Status   string `json:"status"`
		Archived bool   `json:"archived"`
		Pending  bool   `json:"pending_wait"`
	}
	if err := json.Unmarshal([]byte(create.Body.String()), &created); err != nil {
		t.Fatalf("unmarshal create session failed: %v", err)
	}
	if !strings.HasPrefix(created.ID, "sess_") || created.Title != "会话标题" || created.Status != "active" || created.Archived || created.Pending {
		t.Fatalf("unexpected created session = %#v", created)
	}

	list := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/sessions", nil)
	if list.Code != consts.StatusOK {
		t.Fatalf("list sessions status = %d, want %d; body=%s", list.Code, consts.StatusOK, list.Body.String())
	}
	if !strings.Contains(list.Body.String(), created.ID) {
		t.Fatalf("expected created session in list: %s", list.Body.String())
	}

	get := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/sessions/"+created.ID, nil)
	if get.Code != consts.StatusOK {
		t.Fatalf("get session status = %d, want %d; body=%s", get.Code, consts.StatusOK, get.Body.String())
	}
	if !strings.Contains(get.Body.String(), `"title":"会话标题"`) {
		t.Fatalf("expected title in get response: %s", get.Body.String())
	}

	patch := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPatch,
		"/api/sessions/"+created.ID,
		&ut.Body{Body: strings.NewReader(`{"title":"会话标题-更新"}`), Len: len(`{"title":"会话标题-更新"}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if patch.Code != consts.StatusOK {
		t.Fatalf("patch session status = %d, want %d; body=%s", patch.Code, consts.StatusOK, patch.Body.String())
	}
	if !strings.Contains(patch.Body.String(), `"title":"会话标题-更新"`) {
		t.Fatalf("expected updated title in patch response: %s", patch.Body.String())
	}

	archive := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/sessions/"+created.ID+"/archive", nil)
	if archive.Code != consts.StatusOK {
		t.Fatalf("archive session status = %d, want %d; body=%s", archive.Code, consts.StatusOK, archive.Body.String())
	}
	if !strings.Contains(archive.Body.String(), `"archived":true`) || !strings.Contains(archive.Body.String(), `"status":"archived"`) {
		t.Fatalf("expected archived session in response: %s", archive.Body.String())
	}
}

func TestChatRespondReturnsInvalidSessionWhenExplicitSessionIsMissing(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
			MaxConcurrentRequests:     2,
			MaxConcurrentTools:        2,
		},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	recorder := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/chat/respond",
		&ut.Body{Body: strings.NewReader(`{"query":"analyze risk","session_id":"sess_missing_explicit","strict_schema_validation":true}`), Len: len(`{"query":"analyze risk","session_id":"sess_missing_explicit","strict_schema_validation":true}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if recorder.Code != consts.StatusOK {
		t.Fatalf("chat respond status = %d, want %d; body=%s", recorder.Code, consts.StatusOK, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"status":"invalid_session"`) {
		t.Fatalf("expected invalid_session status in body: %s", recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), `"reason":"not_found"`) {
		t.Fatalf("expected invalid_session reason in body: %s", recorder.Body.String())
	}
}

func TestModelProviderEndpoints(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
		},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	createProvider := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers",
		&ut.Body{Body: strings.NewReader(`{"name":"demo-provider","protocol":"openai_compatible","base_url":"https://example.com/v1","api_key":"sk-demo","request_timeout_seconds":35,"headers":[{"key":"Accept-Encoding","value":"identity"}],"models":[{"model_id":"gpt-4o-mini","display_name":"GPT-4o Mini","is_default":true}]}`), Len: len(`{"name":"demo-provider","protocol":"openai_compatible","base_url":"https://example.com/v1","api_key":"sk-demo","request_timeout_seconds":35,"headers":[{"key":"Accept-Encoding","value":"identity"}],"models":[{"model_id":"gpt-4o-mini","display_name":"GPT-4o Mini","is_default":true}]}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if createProvider.Code != consts.StatusCreated {
		t.Fatalf("create provider status = %d, want %d; body=%s", createProvider.Code, consts.StatusCreated, createProvider.Body.String())
	}

	var providerPayload struct {
		ID            string            `json:"id"`
		APIKeyMasked  string            `json:"api_key_masked"`
		APIKeyPresent bool              `json:"api_key_configured"`
		Headers       map[string]string `json:"headers"`
		Models        []struct {
			ID string `json:"id"`
		} `json:"models"`
		Protocol       string `json:"protocol"`
		BaseURL        string `json:"base_url"`
		RequestTimeout int    `json:"request_timeout_seconds"`
	}
	if err := json.Unmarshal([]byte(createProvider.Body.String()), &providerPayload); err != nil {
		t.Fatalf("unmarshal provider payload failed: %v", err)
	}
	if providerPayload.ID == "" || !providerPayload.APIKeyPresent || providerPayload.APIKeyMasked == "" {
		t.Fatalf("unexpected provider payload = %#v", providerPayload)
	}
	if len(providerPayload.Models) != 1 {
		t.Fatalf("expected provider create response to include nested child models, got %#v", providerPayload.Models)
	}
	if providerPayload.Models[0].ID == "" {
		t.Fatalf("expected nested provider model id, got %#v", providerPayload.Models)
	}
	if providerPayload.Headers["Accept-Encoding"] != "identity" {
		t.Fatalf("unexpected provider headers = %#v", providerPayload.Headers)
	}

	createModel := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers/"+providerPayload.ID+"/models",
		&ut.Body{Body: strings.NewReader(`{"model_id":"gpt-4.1-mini","display_name":"GPT-4.1 Mini","enabled":true}`), Len: len(`{"model_id":"gpt-4.1-mini","display_name":"GPT-4.1 Mini","enabled":true}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if createModel.Code != consts.StatusCreated {
		t.Fatalf("create provider model status = %d, want %d; body=%s", createModel.Code, consts.StatusCreated, createModel.Body.String())
	}
	var modelPayload struct {
		ID        string `json:"id"`
		ModelID   string `json:"model_id"`
		IsDefault bool   `json:"is_default"`
		Enabled   bool   `json:"enabled"`
	}
	if err := json.Unmarshal([]byte(createModel.Body.String()), &modelPayload); err != nil {
		t.Fatalf("unmarshal provider model payload failed: %v", err)
	}
	if modelPayload.ID == "" || modelPayload.ModelID != "gpt-4.1-mini" || modelPayload.IsDefault || !modelPayload.Enabled {
		t.Fatalf("unexpected model payload = %#v", modelPayload)
	}

	patchModel := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPatch,
		"/api/models/providers/"+providerPayload.ID+"/models/"+providerPayload.Models[0].ID,
		&ut.Body{Body: strings.NewReader(`{"enabled":false}`), Len: len(`{"enabled":false}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if patchModel.Code != consts.StatusBadRequest {
		t.Fatalf("patch protected model status = %d, want %d; body=%s", patchModel.Code, consts.StatusBadRequest, patchModel.Body.String())
	}

	createAuxModel := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers/"+providerPayload.ID+"/models",
		&ut.Body{Body: strings.NewReader(`{"model_id":"gpt-4.1-nano","display_name":"GPT-4.1 Nano","enabled":true}`), Len: len(`{"model_id":"gpt-4.1-nano","display_name":"GPT-4.1 Nano","enabled":true}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if createAuxModel.Code != consts.StatusCreated {
		t.Fatalf("create aux model status = %d, want %d; body=%s", createAuxModel.Code, consts.StatusCreated, createAuxModel.Body.String())
	}
	var auxModel struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal([]byte(createAuxModel.Body.String()), &auxModel); err != nil {
		t.Fatalf("unmarshal aux model payload failed: %v", err)
	}
	if auxModel.ID == "" {
		t.Fatalf("expected aux model id: %s", createAuxModel.Body.String())
	}

	deleteAuxModel := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodDelete,
		"/api/models/providers/"+providerPayload.ID+"/models/"+auxModel.ID,
		nil,
	)
	if deleteAuxModel.Code != consts.StatusOK {
		t.Fatalf("delete aux model status = %d, want %d; body=%s", deleteAuxModel.Code, consts.StatusOK, deleteAuxModel.Body.String())
	}
	var deleteAuxModelPayload struct {
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(deleteAuxModel.Body.String()), &deleteAuxModelPayload); err != nil {
		t.Fatalf("unmarshal delete aux model payload failed: %v", err)
	}
	if deleteAuxModelPayload.Status != "deleted" {
		t.Fatalf("unexpected delete aux model payload = %#v", deleteAuxModelPayload)
	}

	deleteProvider := ut.PerformRequest(httpServer.engine.Engine, http.MethodDelete, "/api/models/providers/"+providerPayload.ID, nil)
	if deleteProvider.Code != consts.StatusBadRequest {
		t.Fatalf("delete provider with protected model status = %d, want %d; body=%s", deleteProvider.Code, consts.StatusBadRequest, deleteProvider.Body.String())
	}

	deleteMissingProvider := ut.PerformRequest(httpServer.engine.Engine, http.MethodDelete, "/api/models/providers/missing-provider", nil)
	if deleteMissingProvider.Code != consts.StatusNotFound {
		t.Fatalf("delete missing provider status = %d, want %d; body=%s", deleteMissingProvider.Code, consts.StatusNotFound, deleteMissingProvider.Body.String())
	}

	deleteMissingModel := ut.PerformRequest(httpServer.engine.Engine, http.MethodDelete, "/api/models/providers/"+providerPayload.ID+"/models/missing-model", nil)
	if deleteMissingModel.Code != consts.StatusNotFound {
		t.Fatalf("delete missing provider model status = %d, want %d; body=%s", deleteMissingModel.Code, consts.StatusNotFound, deleteMissingModel.Body.String())
	}
}

func TestProviderModelTestEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
		},
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-test","object":"chat.completion","created":1712640000,"model":"gpt-4o-mini","choices":[{"index":0,"message":{"role":"assistant","content":"pong"},"finish_reason":"stop"}]}`))
	}))
	defer upstream.Close()

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	createProvider := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers",
		&ut.Body{Body: strings.NewReader(`{"name":"testable-provider","protocol":"openai_compatible","base_url":"` + upstream.URL + `/v1","api_key":"sk-demo","models":[{"model_id":"gpt-4o-mini","display_name":"GPT-4o Mini","is_default":true}]}`), Len: len(`{"name":"testable-provider","protocol":"openai_compatible","base_url":"` + upstream.URL + `/v1","api_key":"sk-demo","models":[{"model_id":"gpt-4o-mini","display_name":"GPT-4o Mini","is_default":true}]}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if createProvider.Code != consts.StatusCreated {
		t.Fatalf("create provider status = %d, want %d; body=%s", createProvider.Code, consts.StatusCreated, createProvider.Body.String())
	}

	var providerPayload struct {
		ID     string `json:"id"`
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(createProvider.Body.String()), &providerPayload); err != nil {
		t.Fatalf("unmarshal provider payload failed: %v", err)
	}
	if providerPayload.ID == "" || len(providerPayload.Models) != 1 || providerPayload.Models[0].ID == "" {
		t.Fatalf("unexpected provider payload = %#v", providerPayload)
	}

	testModel := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers/"+providerPayload.ID+"/models/"+providerPayload.Models[0].ID+"/test",
		nil,
	)
	if testModel.Code != consts.StatusOK {
		t.Fatalf("test model status = %d, want %d; body=%s", testModel.Code, consts.StatusOK, testModel.Body.String())
	}

	var result struct {
		ProviderID    string `json:"provider_id"`
		ModelRecordID string `json:"model_record_id"`
		ProviderName  string `json:"provider_name"`
		ModelID       string `json:"model_id"`
		DisplayName   string `json:"display_name"`
		Available     bool   `json:"available"`
		DurationMS    int64  `json:"duration_ms"`
	}
	if err := json.Unmarshal([]byte(testModel.Body.String()), &result); err != nil {
		t.Fatalf("unmarshal test model response failed: %v", err)
	}
	if result.ProviderID != providerPayload.ID || result.ModelRecordID != providerPayload.Models[0].ID || result.ModelID != "gpt-4o-mini" || !result.Available {
		t.Fatalf("unexpected test model response = %#v", result)
	}
}

func TestProviderModelAnthropicTestEndpoint(t *testing.T) {
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 5,
		},
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/v1/messages" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"msg_test","type":"message","role":"assistant","model":"claude-3-5-sonnet-20241022","content":[{"type":"text","text":"pong"}],"stop_reason":"end_turn","stop_sequence":null,"usage":{"input_tokens":8,"output_tokens":1}}`))
	}))
	defer upstream.Close()

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	createProvider := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers",
		&ut.Body{Body: strings.NewReader(`{"name":"anthropic-provider","protocol":"anthropic","base_url":"` + upstream.URL + `","api_key":"sk-ant-demo","models":[{"model_id":"claude-3-5-sonnet-20241022","display_name":"Claude Sonnet","is_default":true}]}`), Len: len(`{"name":"anthropic-provider","protocol":"anthropic","base_url":"` + upstream.URL + `","api_key":"sk-ant-demo","models":[{"model_id":"claude-3-5-sonnet-20241022","display_name":"Claude Sonnet","is_default":true}]}`)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if createProvider.Code != consts.StatusCreated {
		t.Fatalf("create provider status = %d, want %d; body=%s", createProvider.Code, consts.StatusCreated, createProvider.Body.String())
	}

	var providerPayload struct {
		ID     string `json:"id"`
		Models []struct {
			ID string `json:"id"`
		} `json:"models"`
	}
	if err := json.Unmarshal([]byte(createProvider.Body.String()), &providerPayload); err != nil {
		t.Fatalf("unmarshal provider payload failed: %v", err)
	}
	if providerPayload.ID == "" || len(providerPayload.Models) != 1 || providerPayload.Models[0].ID == "" {
		t.Fatalf("unexpected provider payload = %#v", providerPayload)
	}

	testModel := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/models/providers/"+providerPayload.ID+"/models/"+providerPayload.Models[0].ID+"/test",
		nil,
	)
	if testModel.Code != consts.StatusOK {
		t.Fatalf("test model status = %d, want %d; body=%s", testModel.Code, consts.StatusOK, testModel.Body.String())
	}

	var result struct {
		ModelID   string `json:"model_id"`
		Available bool   `json:"available"`
		Error     string `json:"error"`
	}
	if err := json.Unmarshal([]byte(testModel.Body.String()), &result); err != nil {
		t.Fatalf("unmarshal test model response failed: %v", err)
	}
	if result.ModelID != "claude-3-5-sonnet-20241022" || !result.Available || result.Error != "" {
		t.Fatalf("unexpected anthropic test model response = %#v", result)
	}
}

func TestUploadAndDeleteSkillPackageEndpoints(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body, contentType := buildSkillBundleMultipartBody(t, "bundle.zip", map[string]string{
		"uploaded-example/SKILL.md": `---
name: uploaded-example
description: Uploaded example skill.
---

# Uploaded example

Guide the agent through an uploaded example.`,
		"uploaded-example/scripts/check.sh": "#!/bin/sh\necho uploaded\n",
	})
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if upload.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusCreated, upload.Body.String())
	}

	listPackages := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills/packages", nil)
	if listPackages.Code != consts.StatusOK {
		t.Fatalf("list packages status = %d, want %d; body=%s", listPackages.Code, consts.StatusOK, listPackages.Body.String())
	}
	if !strings.Contains(listPackages.Body.String(), `"name":"uploaded-example"`) {
		t.Fatalf("expected uploaded-example in package list: %s", listPackages.Body.String())
	}
	var packagesPayload struct {
		Items []skillPackageMetadataPayload `json:"items"`
	}
	if err := json.Unmarshal([]byte(listPackages.Body.String()), &packagesPayload); err != nil {
		t.Fatalf("unmarshal package list failed: %v", err)
	}
	if len(packagesPayload.Items) != 1 || packagesPayload.Items[0].ID == "" {
		t.Fatalf("expected one package id, got %#v", packagesPayload.Items)
	}
	if packagesPayload.Items[0].FileCount != 2 || len(packagesPayload.Items[0].FilePaths) != 2 || !packagesPayload.Items[0].Validation.Valid {
		t.Fatalf("expected complete package metadata, got %#v", packagesPayload.Items[0])
	}
	packageID := packagesPayload.Items[0].ID

	listSkills := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills", nil)
	if listSkills.Code != consts.StatusOK {
		t.Fatalf("list skills status = %d, want %d; body=%s", listSkills.Code, consts.StatusOK, listSkills.Body.String())
	}
	if !strings.Contains(listSkills.Body.String(), `"source":"uploaded"`) {
		t.Fatalf("expected uploaded skill source in skill list: %s", listSkills.Body.String())
	}

	deleteResp := ut.PerformRequest(httpServer.engine.Engine, http.MethodDelete, "/api/skills/packages/"+packageID, nil)
	if deleteResp.Code != consts.StatusOK {
		t.Fatalf("delete status = %d, want %d; body=%s", deleteResp.Code, consts.StatusOK, deleteResp.Body.String())
	}
	var deletePayload struct {
		ID     string `json:"id"`
		Status string `json:"status"`
	}
	if err := json.Unmarshal([]byte(deleteResp.Body.String()), &deletePayload); err != nil {
		t.Fatalf("unmarshal delete payload failed: %v", err)
	}
	if deletePayload.ID != packageID || deletePayload.Status != "deleted" {
		t.Fatalf("unexpected delete payload = %#v", deletePayload)
	}
}

func TestUpdateSkillPackageStateEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body, contentType := buildSkillBundleMultipartBody(t, "toggle.zip", map[string]string{
		"toggle/SKILL.md": `---
name: uploaded-toggle
description: Uploaded toggle skill.
---

# Uploaded toggle

Guide the agent through an uploaded toggle.`,
	})
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if upload.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusCreated, upload.Body.String())
	}
	var metadata skillPackageMetadataPayload
	if err := json.Unmarshal([]byte(upload.Body.String()), &metadata); err != nil {
		t.Fatalf("unmarshal upload metadata failed: %v", err)
	}
	if metadata.ID == "" {
		t.Fatalf("expected package id in upload response: %s", upload.Body.String())
	}
	if metadata.FileCount != 1 || !metadata.Validation.Valid {
		t.Fatalf("expected full upload metadata in response: %#v", metadata)
	}

	updateBody := []byte(`{"enabled":false}`)
	update := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPatch,
		"/api/skills/packages/"+metadata.ID,
		&ut.Body{Body: bytes.NewReader(updateBody), Len: len(updateBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if update.Code != consts.StatusOK {
		t.Fatalf("update status = %d, want %d; body=%s", update.Code, consts.StatusOK, update.Body.String())
	}
	if !strings.Contains(update.Body.String(), `"enabled":false`) {
		t.Fatalf("expected disabled metadata in response: %s", update.Body.String())
	}
}

func TestReplaceSkillPackageEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body, contentType := buildSkillBundleMultipartBody(t, "replace.zip", map[string]string{
		"replace/SKILL.md": `---
name: uploaded-replace
description: Uploaded replace skill.
---

# Uploaded replace

Guide the agent through replace coverage.`,
	})
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if upload.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusCreated, upload.Body.String())
	}
	var metadata skillPackageMetadataPayload
	if err := json.Unmarshal([]byte(upload.Body.String()), &metadata); err != nil {
		t.Fatalf("unmarshal upload metadata failed: %v", err)
	}

	body, contentType = buildSkillBundleMultipartBody(t, "replace-v2.zip", map[string]string{
		"replace/SKILL.md": `---
name: uploaded-replace
description: Uploaded replace skill v2.
---

# Uploaded replace

Guide the agent through replace coverage v2.`,
	})
	replace := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/skills/packages/"+metadata.ID,
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if replace.Code != consts.StatusOK {
		t.Fatalf("replace status = %d, want %d; body=%s", replace.Code, consts.StatusOK, replace.Body.String())
	}
	var replacePayload skillPackageMetadataPayload
	if err := json.Unmarshal([]byte(replace.Body.String()), &replacePayload); err != nil {
		t.Fatalf("unmarshal replace payload failed: %v", err)
	}
	if replacePayload.Revision != 2 || replacePayload.FileCount != 1 || !replacePayload.Validation.Valid {
		t.Fatalf("unexpected replace payload = %#v", replacePayload)
	}

	listSkills := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills", nil)
	if listSkills.Code != consts.StatusOK {
		t.Fatalf("list skills status = %d, want %d; body=%s", listSkills.Code, consts.StatusOK, listSkills.Body.String())
	}
	if !strings.Contains(listSkills.Body.String(), `"description":"Uploaded replace skill v2."`) {
		t.Fatalf("expected skill list to reflect v2 description: %s", listSkills.Body.String())
	}

	revisions := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills/packages/"+metadata.ID+"/revisions", nil)
	if revisions.Code != consts.StatusOK {
		t.Fatalf("revisions status = %d, want %d; body=%s", revisions.Code, consts.StatusOK, revisions.Body.String())
	}
	if !strings.Contains(revisions.Body.String(), `"revision":2`) || !strings.Contains(revisions.Body.String(), `"revision":1`) {
		t.Fatalf("expected revisions response to include v2 and v1: %s", revisions.Body.String())
	}
}

func TestGetAndReplaceSkillPackageWithFilesEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body := []byte(`{"name":"uploaded-files","files":{"SKILL.md":"---\nname: uploaded-files\ndescription: Uploaded files skill.\n---\n\n# Uploaded files\n\nGuide the agent through files coverage.","references/guide.md":"reference"}}`)
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if upload.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusCreated, upload.Body.String())
	}
	var metadata skillPackageMetadataPayload
	if err := json.Unmarshal([]byte(upload.Body.String()), &metadata); err != nil {
		t.Fatalf("unmarshal upload metadata failed: %v", err)
	}

	detail := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills/packages/"+metadata.ID, nil)
	if detail.Code != consts.StatusOK {
		t.Fatalf("detail status = %d, want %d; body=%s", detail.Code, consts.StatusOK, detail.Body.String())
	}
	if !strings.Contains(detail.Body.String(), `"SKILL.md"`) || !strings.Contains(detail.Body.String(), `"references/guide.md"`) {
		t.Fatalf("expected package files in detail response: %s", detail.Body.String())
	}

	replaceBody := []byte(`{"files":{"SKILL.md":"---\nname: uploaded-files\ndescription: Uploaded files skill v2.\n---\n\n# Uploaded files\n\nGuide the agent through files coverage v2.","scripts/check.sh":"#!/bin/sh\necho ok\n"}}`)
	replace := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/skills/packages/"+metadata.ID,
		&ut.Body{Body: bytes.NewReader(replaceBody), Len: len(replaceBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if replace.Code != consts.StatusOK {
		t.Fatalf("replace status = %d, want %d; body=%s", replace.Code, consts.StatusOK, replace.Body.String())
	}
	if !strings.Contains(replace.Body.String(), `"revision":2`) {
		t.Fatalf("expected replacement revision: %s", replace.Body.String())
	}
}

func TestRollbackSkillPackageEndpoint(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body, contentType := buildSkillBundleMultipartBody(t, "rollback.zip", map[string]string{
		"rollback/SKILL.md": `---
name: uploaded-rollback
description: Uploaded rollback skill.
---

# Uploaded rollback

Guide the agent through rollback coverage.`,
	})
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if upload.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusCreated, upload.Body.String())
	}
	var metadata skillPackageMetadataPayload
	if err := json.Unmarshal([]byte(upload.Body.String()), &metadata); err != nil {
		t.Fatalf("unmarshal upload metadata failed: %v", err)
	}

	body, contentType = buildSkillBundleMultipartBody(t, "rollback-v2.zip", map[string]string{
		"rollback/SKILL.md": `---
name: uploaded-rollback
description: Uploaded rollback skill v2.
---

# Uploaded rollback

Guide the agent through rollback coverage v2.`,
	})
	replace := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/skills/packages/"+metadata.ID,
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if replace.Code != consts.StatusOK {
		t.Fatalf("replace status = %d, want %d; body=%s", replace.Code, consts.StatusOK, replace.Body.String())
	}

	rollbackBody := []byte(`{"revision":1}`)
	rollback := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages/"+metadata.ID+"/rollback",
		&ut.Body{Body: bytes.NewReader(rollbackBody), Len: len(rollbackBody)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if rollback.Code != consts.StatusOK {
		t.Fatalf("rollback status = %d, want %d; body=%s", rollback.Code, consts.StatusOK, rollback.Body.String())
	}
	var rollbackPayload struct {
		Metadata        skillPackageMetadataPayload `json:"metadata"`
		RolledBackFrom  int                         `json:"rolled_back_from"`
		CurrentRevision int                         `json:"current_revision"`
	}
	if err := json.Unmarshal([]byte(rollback.Body.String()), &rollbackPayload); err != nil {
		t.Fatalf("unmarshal rollback payload failed: %v", err)
	}
	if rollbackPayload.RolledBackFrom != 1 || rollbackPayload.CurrentRevision != 3 || rollbackPayload.Metadata.Revision != 3 {
		t.Fatalf("unexpected rollback payload = %#v", rollbackPayload)
	}

	listSkills := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills", nil)
	if listSkills.Code != consts.StatusOK {
		t.Fatalf("list skills status = %d, want %d; body=%s", listSkills.Code, consts.StatusOK, listSkills.Body.String())
	}
	if !strings.Contains(listSkills.Body.String(), `"description":"Uploaded rollback skill."`) {
		t.Fatalf("expected skill list to reflect rolled back description: %s", listSkills.Body.String())
	}
}

func TestUploadSkillPackageRejectsDuplicateName(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	build := func() ([]byte, string) {
		return buildSkillBundleMultipartBody(t, "dup.zip", map[string]string{
			"dup/SKILL.md": `---
name: uploaded-duplicate
description: Uploaded duplicate skill.
---

# Uploaded duplicate

Guide the agent through duplicate coverage.`,
		})
	}

	body, contentType := build()
	first := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if first.Code != consts.StatusCreated {
		t.Fatalf("first upload status = %d, want %d; body=%s", first.Code, consts.StatusCreated, first.Body.String())
	}

	body, contentType = build()
	second := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if second.Code != consts.StatusBadRequest {
		t.Fatalf("second upload status = %d, want %d; body=%s", second.Code, consts.StatusBadRequest, second.Body.String())
	}
	if !strings.Contains(second.Body.String(), `skill package name \"uploaded-duplicate\" already exists`) {
		t.Fatalf("expected duplicate package name error, got %s", second.Body.String())
	}
}

func TestUploadSkillPackageCanOverrideBuiltinName(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)
	body, contentType := buildSkillBundleMultipartBody(t, "builtin.zip", map[string]string{
		"builtin/SKILL.md": `---
name: cso_review
description: Debug override skill.
---

# Builtin debug override

Override builtin guidance from the management console.`,
	})

	response := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if response.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", response.Code, consts.StatusCreated, response.Body.String())
	}

	listSkills := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/skills", nil)
	if listSkills.Code != consts.StatusOK {
		t.Fatalf("skill list status = %d, want %d; body=%s", listSkills.Code, consts.StatusOK, listSkills.Body.String())
	}
	if !strings.Contains(listSkills.Body.String(), `"name":"cso_review"`) || !strings.Contains(listSkills.Body.String(), `"source":"uploaded"`) || !strings.Contains(listSkills.Body.String(), `"description":"Debug override skill."`) {
		t.Fatalf("expected uploaded builtin override in skill list: %s", listSkills.Body.String())
	}
}

func TestUploadSkillPackageRequiresSkillMarkdown(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body, contentType := buildSkillBundleMultipartBody(t, "broken.zip", map[string]string{
		"broken/README.md": "missing skill markdown",
	})
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if upload.Code != consts.StatusBadRequest {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusBadRequest, upload.Body.String())
	}
	if !strings.Contains(upload.Body.String(), "SKILL.md") {
		t.Fatalf("expected SKILL.md validation error: %s", upload.Body.String())
	}
}

func TestUploadSkillPackageReturnsValidationWarnings(t *testing.T) {
	cfg := config.Config{
		Server:  config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{RequestTimeoutSeconds: 30},
	}

	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)

	body, contentType := buildSkillBundleMultipartBody(t, "warn.zip", map[string]string{
		"warn/SKILL.md": `---
name: uploaded-warn
description: Uploaded warn skill.
---

# Uploaded warn

Guide the agent through warning coverage.`,
		"warn/misc/note.txt": "note",
	})
	upload := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPost,
		"/api/skills/packages",
		&ut.Body{Body: bytes.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: contentType},
	)
	if upload.Code != consts.StatusCreated {
		t.Fatalf("upload status = %d, want %d; body=%s", upload.Code, consts.StatusCreated, upload.Body.String())
	}
	if !strings.Contains(upload.Body.String(), `"warnings"`) {
		t.Fatalf("expected validation warnings in response: %s", upload.Body.String())
	}
}

func TestWriteInitialActionAndFinish(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	prepared := &runtime.PreparedExecution{
		Spec: &runtime.ExecutionSpec{
			Skill: runtime.SkillSpec{PrimarySkill: "user_overview"},
		},
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID:     "structured-output.v1",
			Requested:      true,
			Emitted:        false,
			FallbackReason: "waiting_for_information",
		},
		Initial: &runtime.TurnResult{
			Kind: runtime.TurnResultAction,
			Action: &runtime.Action{
				Type: runtime.ActionTypeInformationRequest,
				InformationRequest: &runtime.InformationRequestAction{
					Missing: []runtime.MissingInformationItem{{
						Field: "user_id",
					}},
				},
			},
			WaitState: &runtime.WaitState{
				Stage:        runtime.StageCapabilityResolution,
				StartedAt:    time.Now(),
				TimeoutAt:    time.Now().Add(5 * time.Minute),
				TimeoutAfter: 5 * time.Minute,
			},
		},
		TimeoutWait: &runtime.WaitState{
			Stage:        runtime.StageCapabilityResolution,
			StartedAt:    time.Now(),
			TimeoutAt:    time.Now().Add(5 * time.Minute),
			TimeoutAfter: 5 * time.Minute,
		},
		InitialStatus: runtime.RequestStatusWaitingForInformation,
	}

	if err := writeInitialActionAndFinish(stream, "req-1", "sess-1", prepared, nil); err != nil {
		t.Fatalf("writeInitialActionAndFinish() error = %v", err)
	}

	body := writer.String()
	if !strings.Contains(body, `"type":"action"`) {
		t.Fatalf("expected action event in body: %s", body)
	}
	if !strings.Contains(body, `"action_type":"information_request"`) {
		t.Fatalf("expected information_request action in body: %s", body)
	}
	if !strings.Contains(body, `"status":"waiting_for_information"`) {
		t.Fatalf("expected waiting_for_information status in body: %s", body)
	}
	if !strings.Contains(body, `"type":"done"`) {
		t.Fatalf("expected done event in body: %s", body)
	}

	events := extractSSEEvents(t, body)
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3; body=%s", len(events), body)
	}
	if events[0].Type != progressStepType || events[0].ProgressStep == nil {
		t.Fatalf("expected progress_step event first, got %#v", events[0])
	}
	if events[0].Status != progressStatusWaiting || events[0].ProgressStep.StepID != string(runtime.StageCapabilityResolution) {
		t.Fatalf("unexpected progress event = %#v", events[0])
	}
	if events[1].Action == nil || events[1].Action.InformationRequest == nil {
		t.Fatalf("expected information request payload in second event: %#v", events[1])
	}
	if events[1].StructuredOutput == nil || events[2].StructuredOutput == nil {
		t.Fatalf("expected structured output contract in action and done events")
	}
	if events[1].WaitState == nil || events[2].WaitState == nil {
		t.Fatalf("expected wait state in action and done events")
	}
	stepFlow, ok := events[2].Detail["step_flow"].(map[string]any)
	if !ok {
		t.Fatalf("expected step_flow summary in done detail, got %#v", events[2].Detail)
	}
	if got := stepFlow["terminal_event"]; got != "done" {
		t.Fatalf("terminal_event = %#v, want done", got)
	}
	if got := stepFlow["final_status"]; got != string(runtime.RequestStatusWaitingForInformation) {
		t.Fatalf("final_status = %#v, want waiting_for_information", got)
	}
	if got := stepFlow["auto_fold"]; got != false {
		t.Fatalf("auto_fold = %#v, want false", got)
	}
}

func TestWriteInitialErrorAndFinish(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID:     "structured-output.v1",
			Requested:      true,
			Emitted:        false,
			FallbackReason: "policy_reject",
		},
		Initial: &runtime.TurnResult{
			Kind:  runtime.TurnResultError,
			Error: "required supplemental information could not be provided",
		},
		InitialError: &runtime.ProtocolError{
			Code:         string(runtime.RequestStatusMissingInformationUnresolved),
			Reason:       "policy_reject",
			Retryable:    false,
			ClientAction: "stop_and_surface_error",
			Detail: map[string]any{
				"close_reason": "unable_to_provide",
			},
		},
		InitialStatus: runtime.RequestStatusMissingInformationUnresolved,
	}

	if err := writeInitialErrorAndFinish(stream, "req-2", "sess-2", prepared, nil); err != nil {
		t.Fatalf("writeInitialErrorAndFinish() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3; body=%s", len(events), writer.String())
	}
	if events[0].Type != progressStepType || events[0].Status != progressStatusFailed {
		t.Fatalf("expected failed progress step first, got %#v", events[0])
	}
	if events[1].Type != "error" || events[1].Error == "" {
		t.Fatalf("expected initial error event, got %#v", events[1])
	}
	if events[1].ErrorDetail == nil || events[1].ErrorDetail.Reason != "policy_reject" {
		t.Fatalf("expected structured error detail, got %#v", events[1].ErrorDetail)
	}
	if events[1].StructuredOutput == nil || events[2].StructuredOutput == nil {
		t.Fatalf("expected structured output contract in error and done events")
	}
	if events[2].Status != string(runtime.RequestStatusMissingInformationUnresolved) {
		t.Fatalf("final status = %q, want %q", events[2].Status, runtime.RequestStatusMissingInformationUnresolved)
	}
	if events[2].Detail["reason"] != "policy_reject" {
		t.Fatalf("expected done detail reason, got %#v", events[2].Detail)
	}
	stepFlow, ok := events[2].Detail["step_flow"].(map[string]any)
	if !ok {
		t.Fatalf("expected step_flow summary in done detail, got %#v", events[2].Detail)
	}
	if got := stepFlow["current_stage"]; got != "initial_error" {
		t.Fatalf("current_stage = %#v, want initial_error", got)
	}
	if got := stepFlow["auto_fold"]; got != false {
		t.Fatalf("auto_fold = %#v, want false", got)
	}
}

func TestWriteInitialActionAndFinishPendingHuman(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	prepared := &runtime.PreparedExecution{
		Spec: &runtime.ExecutionSpec{
			Skill: runtime.SkillSpec{PrimarySkill: "user_overview"},
		},
		Initial: &runtime.TurnResult{
			Kind: runtime.TurnResultAction,
			Action: &runtime.Action{
				Type: runtime.ActionTypePendingHuman,
				PendingHuman: &runtime.PendingHumanAction{
					Reason: "missing information will be handled outside the current automatic flow",
					Target: runtime.SupplementTargetClient,
				},
			},
		},
		InitialStatus: runtime.RequestStatusPendingHuman,
	}

	if err := writeInitialActionAndFinish(stream, "req-pending", "sess-pending", prepared, nil); err != nil {
		t.Fatalf("writeInitialActionAndFinish() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3; body=%s", len(events), writer.String())
	}
	if events[0].Type != progressStepType || events[0].ProgressStep == nil {
		t.Fatalf("expected progress_step first, got %#v", events[0])
	}
	if events[0].ProgressStep.StepID != string(runtime.ActionTypePendingHuman) {
		t.Fatalf("unexpected progress step = %#v", events[0].ProgressStep)
	}
	if events[1].ActionType != string(runtime.ActionTypePendingHuman) {
		t.Fatalf("action_type = %q, want %q", events[1].ActionType, runtime.ActionTypePendingHuman)
	}
	if events[1].Status != string(runtime.RequestStatusPendingHuman) {
		t.Fatalf("action status = %q, want %q", events[1].Status, runtime.RequestStatusPendingHuman)
	}
	if events[2].Status != string(runtime.RequestStatusPendingHuman) {
		t.Fatalf("final status = %q, want %q", events[2].Status, runtime.RequestStatusPendingHuman)
	}
	stepFlow, ok := events[2].Detail["step_flow"].(map[string]any)
	if !ok {
		t.Fatalf("expected step_flow summary in done detail, got %#v", events[2].Detail)
	}
	if got := stepFlow["final_status"]; got != string(runtime.RequestStatusPendingHuman) {
		t.Fatalf("final_status = %#v, want pending_human", got)
	}
	if got := stepFlow["auto_fold"]; got != false {
		t.Fatalf("auto_fold = %#v, want false", got)
	}
}

func TestWriteCompletedAndFinishIncludesStructuredOutput(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID:     "structured-output.v1",
			Requested:      true,
			Emitted:        false,
			FallbackReason: "text_stream_only",
		},
	}

	if err := writeCompletedAndFinish(stream, "req-done", "sess-done", prepared, map[string]any{"source": "test"}); err != nil {
		t.Fatalf("writeCompletedAndFinish() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1; body=%s", len(events), writer.String())
	}
	if events[0].Type != "done" || events[0].Status != "completed" {
		t.Fatalf("unexpected done event = %#v", events[0])
	}
	if events[0].StructuredOutput == nil {
		t.Fatalf("expected structured output contract in done event")
	}
	if events[0].StructuredOutput.FallbackReason != "text_stream_only" {
		t.Fatalf("fallback_reason = %q, want text_stream_only", events[0].StructuredOutput.FallbackReason)
	}
}

func TestEmitStructuredCompletionEvents(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID: "structured-output.v1",
			Requested:  true,
		},
	}

	req := ChatStreamRequest{
		TaskType:          "workflow_step_request",
		DesiredOutputMode: "workflow_plan",
		WorkflowRunID:     "run-1",
	}
	if err := emitStructuredCompletionEvents(context.Background(), stream, "req-structured", "sess-structured", prepared, req, "workflow summary", "", nil); err != nil {
		t.Fatalf("emitStructuredCompletionEvents() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) == 0 {
		t.Fatalf("expected completion events, got none")
	}
	types := make(map[string]bool)
	for _, event := range events {
		types[event.Type] = true
	}
	for _, want := range []string{"workflow_plan", "card_created", "right_panel_view", "next_questions", "completed"} {
		if !types[want] {
			t.Fatalf("missing %s event in %#v", want, types)
		}
	}
	for _, event := range events {
		if event.Type == "card_created" {
			if event.Detail["card_type"] == nil || event.Detail["title"] == nil || event.Detail["summary"] == nil || event.Detail["source"] == nil {
				t.Fatalf("card_created detail missing stable fields = %#v", event.Detail)
			}
		}
	}
}

func TestWritePendingWaitAndFinish(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	pending := &session.PendingState{
		Stage:         string(runtime.StageCapabilityResolution),
		Status:        string(runtime.RequestStatusWaitingForInformation),
		ActionType:    string(runtime.ActionTypeInformationRequest),
		ResumeToken:   "resume-99",
		TimeoutAfter:  2 * time.Minute,
		TimeoutAt:     time.Now().Add(2 * time.Minute),
		MissingFields: []string{"user_id"},
	}

	if err := writePendingWaitAndFinish(stream, "req-3", "sess-3", pending, nil, nil); err != nil {
		t.Fatalf("writePendingWaitAndFinish() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3; body=%s", len(events), writer.String())
	}
	if events[0].Type != progressStepType || events[0].Status != progressStatusWaiting {
		t.Fatalf("expected waiting progress step first, got %#v", events[0])
	}
	if events[1].Action == nil || events[1].Action.InformationRequest == nil {
		t.Fatalf("expected information request payload, got %#v", events[1])
	}
	if events[1].WaitState == nil || events[1].WaitState.ResumeToken != "resume-99" {
		t.Fatalf("unexpected wait state = %#v", events[1].WaitState)
	}
	if accepted, ok := events[1].Detail["accepted"].(bool); !ok || accepted {
		t.Fatalf("expected accepted=false detail, got %#v", events[1].Detail)
	}
	if events[2].Status != string(runtime.RequestStatusWaitingForInformation) {
		t.Fatalf("final status = %q, want %q", events[2].Status, runtime.RequestStatusWaitingForInformation)
	}
}

func TestEmitStructuredCompletionEventsAddsPlanningProgressForAutomationDraft(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	prepared := &runtime.PreparedExecution{
		StructuredOutput: &runtime.StructuredOutputContract{
			ContractID: "structured-output.v1",
			Requested:  true,
		},
	}

	req := ChatStreamRequest{
		TaskType: "chat",
		Query:    "请为我设计一个每天执行的计划，先给我草案，确认后再执行",
		InputPayload: map[string]any{
			"interaction_context": map[string]any{
				"entry_mode": "automation_create",
			},
		},
	}
	if err := emitStructuredCompletionEvents(context.Background(), stream, "req-auto-progress", "sess-auto-progress", prepared, req, "自动化计划说明", "", nil); err != nil {
		t.Fatalf("emitStructuredCompletionEvents() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	steps := findEventsByType(events, progressStepType)
	found := map[string]bool{}
	for _, event := range steps {
		if event.ProgressStep != nil {
			found[event.ProgressStep.StepID] = true
		}
	}
	for _, stepID := range []string{"understanding_request", "detecting_interaction_mode", "collecting_dependencies", "defining_deliverables", "assessing_risks", "building_draft"} {
		if !found[stepID] {
			t.Fatalf("missing planning progress step %q in %#v", stepID, found)
		}
	}
	last := events[len(events)-1]
	structured, ok := last.Detail["structured_result"].(map[string]any)
	if !ok {
		t.Fatalf("completed event missing structured_result detail: %#v", last.Detail)
	}
	if structured["interaction_mode"] == nil {
		t.Fatalf("completed event missing interaction_mode: %#v", structured)
	}
	if structured["intent_resolution"] == nil {
		t.Fatalf("completed event missing intent_resolution: %#v", structured)
	}
	if structured["automation_create_payload"] == nil {
		t.Fatalf("completed event missing automation_create_payload: %#v", structured)
	}
}

func TestWritePendingWaitAndFinishReportsQueueOverflow(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	pending := &session.PendingState{
		Stage:         string(runtime.StageCapabilityResolution),
		Status:        string(runtime.RequestStatusWaitingForInformation),
		ActionType:    string(runtime.ActionTypeInformationRequest),
		ResumeToken:   "resume-overflow",
		TimeoutAfter:  time.Minute,
		TimeoutAt:     time.Now().Add(time.Minute),
		MissingFields: []string{"user_id"},
	}
	dropped := &session.DeferredMessage{
		Query:      "oldest-queued-message",
		ReceivedAt: time.Unix(123, 0),
	}

	if err := writePendingWaitAndFinish(stream, "req-overflow", "sess-overflow", pending, nil, dropped); err != nil {
		t.Fatalf("writePendingWaitAndFinish() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3; body=%s", len(events), writer.String())
	}
	queueOverflow, ok := events[1].Detail["queue_overflow"].(map[string]any)
	if !ok {
		t.Fatalf("expected queue_overflow detail, got %#v", events[1].Detail)
	}
	if droppedOldest, ok := queueOverflow["dropped_oldest"].(bool); !ok || !droppedOldest {
		t.Fatalf("expected dropped_oldest=true, got %#v", queueOverflow)
	}
	if query, ok := queueOverflow["query"].(string); !ok || query != "oldest-queued-message" {
		t.Fatalf("unexpected dropped query detail = %#v", queueOverflow)
	}
}

func TestWriteGapClosedNotification(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	if err := writeGapClosedNotification(stream, "req-gap", "sess-gap", &runtime.GapClosedAction{
		ResumeToken:  "resume-gap",
		CloseReason:  "timeout_expired",
		NextStep:     "consume_deferred",
		TokenInvalid: true,
	}); err != nil {
		t.Fatalf("writeGapClosedNotification() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 1 {
		t.Fatalf("event count = %d, want 1; body=%s", len(events), writer.String())
	}
	if events[0].Type != "notification" {
		t.Fatalf("type = %q, want notification", events[0].Type)
	}
	if events[0].Notification == nil || events[0].Notification.Code != "gap_closed" {
		t.Fatalf("unexpected notification = %#v", events[0].Notification)
	}
	if events[0].Notification.ResumeToken != "resume-gap" {
		t.Fatalf("unexpected resume token = %#v", events[0].Notification)
	}
}

func TestWriteInvalidResumeTokenAndFinish(t *testing.T) {
	ctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(ctx, writer)

	err := writeInvalidResumeTokenAndFinish(stream, "req-invalid", "sess-invalid", &appcore.InvalidResumeTokenError{
		SessionID:   "sess-invalid",
		ResumeToken: "resume-old",
		Reason:      "closed",
	})
	if err != nil {
		t.Fatalf("writeInvalidResumeTokenAndFinish() error = %v", err)
	}

	events := extractSSEEvents(t, writer.String())
	if len(events) != 3 {
		t.Fatalf("event count = %d, want 3; body=%s", len(events), writer.String())
	}
	if events[0].Type != progressStepType || events[0].Status != progressStatusFailed {
		t.Fatalf("expected failed progress step first, got %#v", events[0])
	}
	if events[1].ErrorDetail == nil || events[1].ErrorDetail.Code != string(runtime.RequestStatusInvalidResumeToken) {
		t.Fatalf("unexpected error detail = %#v", events[1].ErrorDetail)
	}
	if events[1].ErrorDetail.ClientAction != "start_new_request" {
		t.Fatalf("client_action = %q, want start_new_request", events[1].ErrorDetail.ClientAction)
	}
	if events[2].Status != string(runtime.RequestStatusInvalidResumeToken) {
		t.Fatalf("final status = %q, want %q", events[2].Status, runtime.RequestStatusInvalidResumeToken)
	}
	if reason := events[2].Detail["reason"]; reason != "closed" {
		t.Fatalf("detail reason = %#v, want closed", reason)
	}
}

func TestProcessAgentEventEmitsProgressSteps(t *testing.T) {
	ctx := withRequestID(context.Background(), "req-progress")
	hctx := hertzapp.NewContext(0)
	writer := &mockExtWriter{}
	stream := sse.NewStreamWithWriter(hctx, writer)
	tracker := newProgressStepTracker("req-progress", "sess-progress", stream)

	if err := tracker.emitContextReady(); err != nil {
		t.Fatalf("emitContextReady() error = %v", err)
	}
	if err := tracker.startAnalysis(); err != nil {
		t.Fatalf("startAnalysis() error = %v", err)
	}

	toolStarted := schema.AssistantMessage("", nil)
	toolStarted.Extra = map[string]any{
		"event_type":   "tool_call_started",
		"tool_call_id": "call-1",
		"tool_name":    "lookup_profile",
		"status":       "running",
	}
	if err := processAgentEvent(ctx, stream, &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: toolStarted,
			},
		},
	}, &strings.Builder{}, "sess-progress", tracker); err != nil {
		t.Fatalf("processAgentEvent(tool start) error = %v", err)
	}

	toolFinished := schema.AssistantMessage("", nil)
	toolFinished.Extra = map[string]any{
		"event_type":   "tool_call_finished",
		"tool_call_id": "call-1",
		"tool_name":    "lookup_profile",
		"status":       "ok",
	}
	if err := processAgentEvent(ctx, stream, &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: toolFinished,
			},
		},
	}, &strings.Builder{}, "sess-progress", tracker); err != nil {
		t.Fatalf("processAgentEvent(tool finish) error = %v", err)
	}

	if err := processAgentEvent(ctx, stream, &adk.AgentEvent{
		Output: &adk.AgentOutput{
			MessageOutput: &adk.MessageVariant{
				Message: schema.AssistantMessage("final answer", nil),
			},
		},
	}, &strings.Builder{}, "sess-progress", tracker); err != nil {
		t.Fatalf("processAgentEvent(message) error = %v", err)
	}

	if err := tracker.completeResponse(""); err != nil {
		t.Fatalf("completeResponse() error = %v", err)
	}

	progressEvents := findEventsByType(extractSSEEvents(t, writer.String()), progressStepType)
	if len(progressEvents) < 6 {
		t.Fatalf("progress_step count = %d, want at least 6; body=%s", len(progressEvents), writer.String())
	}
	if progressEvents[0].ProgressStep == nil || progressEvents[0].ProgressStep.StepID != string(runtime.StageContextAssembly) {
		t.Fatalf("unexpected first progress step = %#v", progressEvents[0])
	}
	if progressEvents[1].Status != progressStatusRunning || progressEvents[1].ProgressStep.StepID != string(runtime.StageCapabilityResolution) {
		t.Fatalf("unexpected analysis running step = %#v", progressEvents[1])
	}
	if progressEvents[2].Status != progressStatusCompleted || progressEvents[2].ProgressStep.StepID != string(runtime.StageCapabilityResolution) {
		t.Fatalf("unexpected analysis completed step = %#v", progressEvents[2])
	}
	if progressEvents[3].Status != progressStatusRunning || progressEvents[3].ProgressStep.StepID != "tool:call-1" {
		t.Fatalf("unexpected tool running step = %#v", progressEvents[3])
	}
	if progressEvents[4].Status != progressStatusCompleted || progressEvents[4].ProgressStep.StepID != "tool:call-1" {
		t.Fatalf("unexpected tool completed step = %#v", progressEvents[4])
	}
	if progressEvents[5].Status != progressStatusRunning || progressEvents[5].ProgressStep.StepID != string(runtime.StageTurnProcessing) {
		t.Fatalf("unexpected response running step = %#v", progressEvents[5])
	}
	if progressEvents[len(progressEvents)-1].Status != progressStatusCompleted || progressEvents[len(progressEvents)-1].ProgressStep.StepID != string(runtime.StageTurnProcessing) {
		t.Fatalf("unexpected response completed step = %#v", progressEvents[len(progressEvents)-1])
	}

	stepFlow := tracker.stepFlowSummary("completed", "Athena finished this request.")
	if !stepFlow.AutoFold {
		t.Fatalf("expected completed step flow to auto-fold")
	}
	if stepFlow.CurrentStage != string(runtime.StageTurnProcessing) {
		t.Fatalf("current_stage = %q, want %q", stepFlow.CurrentStage, runtime.StageTurnProcessing)
	}
	if len(stepFlow.CompletedStages) < 4 {
		t.Fatalf("completed_stages = %#v, want at least 4 entries", stepFlow.CompletedStages)
	}
	if stepFlow.TerminalEvent != "done" {
		t.Fatalf("terminal_event = %q, want done", stepFlow.TerminalEvent)
	}
}

func extractSSEEvents(t *testing.T, body string) []StreamEvent {
	t.Helper()

	lines := strings.Split(body, "\n")
	events := make([]StreamEvent, 0)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		var event StreamEvent
		payload := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if err := json.Unmarshal([]byte(payload), &event); err != nil {
			t.Fatalf("unmarshal SSE event failed: %v; line=%s", err, line)
		}
		events = append(events, event)
	}
	return events
}

func findEventsByType(events []StreamEvent, eventType string) []StreamEvent {
	filtered := make([]StreamEvent, 0)
	for _, event := range events {
		if event.Type == eventType {
			filtered = append(filtered, event)
		}
	}
	return filtered
}

type mockExtWriter struct {
	bytes.Buffer
}

func (m *mockExtWriter) Flush() error {
	return nil
}

func (m *mockExtWriter) Finalize() error {
	return nil
}

func BenchmarkWriteInitialActionAndFinish(b *testing.B) {
	prepared := &runtime.PreparedExecution{
		Spec: &runtime.ExecutionSpec{
			Skill: runtime.SkillSpec{PrimarySkill: "user_overview"},
		},
		Initial: &runtime.TurnResult{
			Kind: runtime.TurnResultAction,
			Action: &runtime.Action{
				Type: runtime.ActionTypeInformationRequest,
				InformationRequest: &runtime.InformationRequestAction{
					Missing: []runtime.MissingInformationItem{{Field: "user_id"}},
				},
			},
			WaitState: &runtime.WaitState{
				Stage:        runtime.StageCapabilityResolution,
				StartedAt:    time.Now(),
				TimeoutAt:    time.Now().Add(5 * time.Minute),
				TimeoutAfter: 5 * time.Minute,
			},
		},
		TimeoutWait: &runtime.WaitState{
			Stage:        runtime.StageCapabilityResolution,
			StartedAt:    time.Now(),
			TimeoutAt:    time.Now().Add(5 * time.Minute),
			TimeoutAfter: 5 * time.Minute,
		},
		InitialStatus: runtime.RequestStatusWaitingForInformation,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ctx := hertzapp.NewContext(0)
		writer := &mockExtWriter{}
		stream := sse.NewStreamWithWriter(ctx, writer)
		if err := writeInitialActionAndFinish(stream, "req-bench", "sess-bench", prepared, nil); err != nil {
			b.Fatalf("writeInitialActionAndFinish() error = %v", err)
		}
	}
}

func buildSkillBundleMultipartBody(t *testing.T, fileName string, files map[string]string) ([]byte, string) {
	t.Helper()

	zipPayload := buildSkillBundleZip(t, files)
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("bundle", fileName)
	if err != nil {
		t.Fatalf("CreateFormFile() error = %v", err)
	}
	if _, err := part.Write(zipPayload); err != nil {
		t.Fatalf("part.Write() error = %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("writer.Close() error = %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func firstCookieValue(resp *ut.ResponseRecorder) string {
	if resp == nil {
		return ""
	}
	raw := strings.TrimSpace(string(resp.Header().Peek("Set-Cookie")))
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, ";")
	return strings.TrimSpace(parts[0])
}

func extractJSONField(t *testing.T, body string, field string) string {
	t.Helper()
	var payload map[string]any
	if err := json.Unmarshal([]byte(body), &payload); err == nil {
		if value, ok := payload[field].(string); ok {
			return value
		}
		if items, ok := payload["items"].([]any); ok && len(items) > 0 {
			if first, ok := items[0].(map[string]any); ok {
				if value, ok := first[field].(string); ok {
					return value
				}
			}
		}
	}
	return ""
}

func buildSkillBundleZip(t *testing.T, files map[string]string) []byte {
	t.Helper()

	var body bytes.Buffer
	writer := zip.NewWriter(&body)
	for path, content := range files {
		entry, err := writer.Create(path)
		if err != nil {
			t.Fatalf("Create(%q) error = %v", path, err)
		}
		if _, err := entry.Write([]byte(content)); err != nil {
			t.Fatalf("entry.Write(%q) error = %v", path, err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("zip writer.Close() error = %v", err)
	}
	return body.Bytes()
}
