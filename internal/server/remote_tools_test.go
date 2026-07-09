package server

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/tools"
)

func TestRemoteToolRegistrationExecutionRestoreAndDelete(t *testing.T) {
	callback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input tools.RemoteExecutionRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Errorf("decode callback request: %v", err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(tools.RemoteExecutionResponse{
			ContractVersion: tools.RemoteToolContractVersion,
			RequestID:       input.RequestID,
			ToolCallID:      input.ToolCallID,
			Status:          "ok",
			Content:         `{"symbol":"510300","nav":4.12}`,
		})
	}))
	defer callback.Close()

	root := t.TempDir()
	cfg := remoteToolTestConfig(root, callback.URL)
	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)
	body := `{
		"registration_id":"fund-snapshot-v1",
		"app_id":"athena-fund-assistant",
		"name":"fund_market_snapshot",
		"description":"Read a normalized fund snapshot.",
		"parameters":{"type":"object","properties":{"symbol":{"type":"string"}}},
		"endpoint":"` + callback.URL + `",
		"tool_scope":"market_data_read",
		"operation":"read",
		"risk_level":"low",
		"side_effect_level":"none",
		"idempotent":true,
		"timeout_ms":500,
		"retry_max_attempts":1,
		"enabled":true
	}`
	put := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/remote-tools/fund_market_snapshot",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if put.Code != consts.StatusOK {
		t.Fatalf("put status = %d, body = %s", put.Code, put.Body.String())
	}

	definition, exists := application.ToolCatalog.Get("fund_market_snapshot")
	if !exists {
		t.Fatal("registered remote tool missing from live catalog")
	}
	invokable, ok := definition.BaseTool.(einotool.InvokableTool)
	if !ok {
		t.Fatal("registered remote tool is not invokable")
	}
	content, err := invokable.InvokableRun(context.Background(), `{"symbol":"510300","private":"must-not-appear-in-trace"}`)
	if err != nil {
		t.Fatalf("remote invocation error = %v", err)
	}
	if content != `{"symbol":"510300","nav":4.12}` {
		t.Fatalf("remote content = %q", content)
	}
	decisions, err := application.ListToolGovernanceDecisions(context.Background())
	if err != nil || len(decisions) == 0 || decisions[0].ToolName != "fund_market_snapshot" {
		t.Fatalf("governance decisions = %#v, error = %v", decisions, err)
	}
	foundTrace := false
	for _, trace := range application.Observability.SnapshotTraces() {
		if trace.Name != "remote_tool_invocation" {
			continue
		}
		foundTrace = true
		encoded, _ := json.Marshal(trace)
		if strings.Contains(string(encoded), "must-not-appear-in-trace") {
			t.Fatalf("trace leaked raw arguments: %s", encoded)
		}
	}
	if !foundTrace {
		t.Fatal("remote invocation trace was not recorded")
	}

	restored := appcore.NewService(cfg)
	if _, exists := restored.ToolCatalog.Get("fund_market_snapshot"); !exists {
		t.Fatal("remote tool was not restored after service restart")
	}

	deleteResponse := ut.PerformRequest(httpServer.engine.Engine, http.MethodDelete, "/api/control-plane/remote-tools/fund_market_snapshot", nil)
	if deleteResponse.Code != consts.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, exists := application.ToolCatalog.Get("fund_market_snapshot"); exists {
		t.Fatal("deleted remote tool remains in live catalog")
	}
}

func TestRemoteToolRollbackRestoresLiveCatalog(t *testing.T) {
	callback := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		var input tools.RemoteExecutionRequest
		if err := json.NewDecoder(request.Body).Decode(&input); err != nil {
			t.Errorf("decode callback request: %v", err)
			return
		}
		writer.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(writer).Encode(tools.RemoteExecutionResponse{
			ContractVersion: tools.RemoteToolContractVersion,
			RequestID:       input.RequestID,
			ToolCallID:      input.ToolCallID,
			Status:          "ok",
			Content:         `{"symbol":"SPY","price":600.12}`,
		})
	}))
	defer callback.Close()

	root := t.TempDir()
	cfg := remoteToolTestConfig(root, callback.URL)
	application := appcore.NewService(cfg)
	httpServer := NewHTTPServer(cfg, application)
	body := `{
		"registration_id":"fund-snapshot-rollback-v1",
		"app_id":"athena-fund-assistant",
		"name":"fund_market_snapshot",
		"description":"Read a normalized fund snapshot.",
		"parameters":{"type":"object","properties":{"symbol":{"type":"string"}}},
		"endpoint":"` + callback.URL + `",
		"tool_scope":"market_data_read",
		"operation":"read",
		"risk_level":"low",
		"side_effect_level":"none",
		"idempotent":true,
		"timeout_ms":500,
		"retry_max_attempts":1,
		"enabled":true
	}`
	put := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/remote-tools/fund_market_snapshot",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if put.Code != consts.StatusOK {
		t.Fatalf("put status = %d, body = %s", put.Code, put.Body.String())
	}

	versions, err := application.ListControlPlaneConfigVersions(context.Background())
	if err != nil {
		t.Fatalf("list versions error = %v", err)
	}
	var targetVersionID string
	for _, version := range versions {
		detail, err := application.GetControlPlaneConfigVersion(context.Background(), version.VersionID)
		if err != nil {
			t.Fatalf("get version %q error = %v", version.VersionID, err)
		}
		if len(detail.Document.RemoteTools) == 1 && detail.Document.RemoteTools[0].Name == "fund_market_snapshot" {
			targetVersionID = version.VersionID
			break
		}
	}
	if targetVersionID == "" {
		t.Fatalf("expected a config version containing the remote tool, versions = %#v", versions)
	}

	deleteResponse := ut.PerformRequest(httpServer.engine.Engine, http.MethodDelete, "/api/control-plane/remote-tools/fund_market_snapshot", nil)
	if deleteResponse.Code != consts.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", deleteResponse.Code, deleteResponse.Body.String())
	}
	if _, exists := application.ToolCatalog.Get("fund_market_snapshot"); exists {
		t.Fatal("deleted remote tool remains in live catalog before rollback")
	}

	rollbackResponse := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/control-plane/config-versions/"+targetVersionID+"/rollback", nil)
	if rollbackResponse.Code != consts.StatusOK {
		t.Fatalf("rollback status = %d, body = %s", rollbackResponse.Code, rollbackResponse.Body.String())
	}
	if _, exists := application.ToolCatalog.Get("fund_market_snapshot"); !exists {
		t.Fatal("remote tool was not restored to the live catalog after rollback")
	}
}

func TestRemoteToolRegistrationRejectsUnlistedOrigin(t *testing.T) {
	root := t.TempDir()
	cfg := remoteToolTestConfig(root, "http://allowed.invalid")
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	body := `{
		"registration_id":"blocked-v1",
		"app_id":"athena-fund-assistant",
		"name":"blocked_tool",
		"parameters":{"type":"object"},
		"endpoint":"http://blocked.invalid/execute",
		"timeout_ms":500,
		"enabled":true
	}`
	response := ut.PerformRequest(
		httpServer.engine.Engine,
		http.MethodPut,
		"/api/control-plane/remote-tools/blocked_tool",
		&ut.Body{Body: strings.NewReader(body), Len: len(body)},
		ut.Header{Key: "Content-Type", Value: "application/json"},
	)
	if response.Code != consts.StatusBadRequest || !strings.Contains(response.Body.String(), "not allowed") {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRemoteToolRoutesRequireControlPlaneAuthentication(t *testing.T) {
	root := t.TempDir()
	cfg := remoteToolTestConfig(root, "http://allowed.invalid")
	cfg.ControlPlane.AuthToken = "required-token"
	httpServer := NewHTTPServer(cfg, appcore.NewService(cfg))
	response := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/remote-tools", nil)
	if response.Code != consts.StatusUnauthorized {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
}

func TestRemoteToolOpenAPIContractIsTyped(t *testing.T) {
	paths := buildOpenAPIPaths()
	if _, ok := paths["/api/control-plane/remote-tools/{name}"]; !ok {
		t.Fatal("OpenAPI missing remote tool mutation path")
	}
	schemas := buildOpenAPISchemas()
	if _, ok := schemas["RemoteToolRegistration"]; !ok {
		t.Fatal("OpenAPI missing RemoteToolRegistration")
	}
	if _, ok := schemas["RemoteToolRegistrationListResponse"]; !ok {
		t.Fatal("OpenAPI missing RemoteToolRegistrationListResponse")
	}
}

func remoteToolTestConfig(root, allowedOrigin string) config.Config {
	return config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             root + "/shared",
		},
		RemoteTools: config.RemoteToolsConfig{
			AllowedOrigins:   []string{allowedOrigin},
			MaxResponseBytes: 1 << 20,
		},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: root + "/controlplane/overrides.json",
		},
		System: config.SystemConfig{
			TruthDir:          root + "/truth",
			ActiveStateDir:    root + "/active",
			CompiledAssetsDir: root + "/compiled",
		},
	}
}
