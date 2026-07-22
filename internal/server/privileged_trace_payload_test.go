package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/runtime"
)

func TestPrivilegedTracePayloadEndpointRequiresAuthAndAuditsReads(t *testing.T) {
	store := &testRuntimeReadStore{}
	cfg := privilegedTracePayloadTestConfig(t)
	key := runtime.DerivePrivilegedTracePayloadKey(cfg.Security.TracePayloadEncryptionKey)
	record, status, err := runtime.BuildPrivilegedTracePayload(
		runtime.PrivilegedTracePayloadCoordinates{RunID: "run-private", StepID: "step-1", TraceType: "model_call", Source: "eino", CorrelationID: "trace-1"},
		map[string]any{"request": map[string]any{"messages": []any{map[string]any{"role": "user", "content": "private portfolio question"}}}},
		runtime.PrivilegedTracePayloadPolicy{Enabled: true, SampleRate: 1, Retention: time.Hour, MaxPayloadBytes: 4096, KeyID: "test-v1", EncryptionKey: key},
		time.Now().UTC(),
	)
	if err != nil || status != "captured" {
		t.Fatalf("build payload status=%q error=%v", status, err)
	}
	store.privilegedPayloads = append(store.privilegedPayloads, record)
	application := appcore.NewServiceWithRuntimeStore(cfg, nil, nil, nil, store)
	httpServer := NewHTTPServer(cfg, application)
	path := "/api/control-plane/runtime/runs/run-private/trace-payloads/" + record.PayloadRef

	unauthorized := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, path, nil)
	if unauthorized.Code != consts.StatusUnauthorized || unauthorized.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("unauthorized status=%d cache=%q body=%s", unauthorized.Code, unauthorized.Header().Get("Cache-Control"), unauthorized.Body.String())
	}
	if len(store.privilegedAudits) != 1 || store.privilegedAudits[0].Outcome != "denied" {
		t.Fatalf("unauthorized audits=%#v", store.privilegedAudits)
	}

	cookie := loginPrivilegedTraceControlPlane(t, httpServer, cfg.ControlPlane.AuthToken)
	authorized := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, path, nil, ut.Header{Key: "Cookie", Value: cookie})
	if authorized.Code != consts.StatusOK || !strings.Contains(authorized.Body.String(), "private portfolio question") {
		t.Fatalf("authorized status=%d body=%s", authorized.Code, authorized.Body.String())
	}
	if strings.Contains(authorized.Body.String(), cfg.Security.TracePayloadEncryptionKey) || len(store.privilegedAudits) != 2 || store.privilegedAudits[1].Outcome != "success" {
		t.Fatalf("authorized response or audit invalid: body=%s audits=%#v", authorized.Body.String(), store.privilegedAudits)
	}

	mismatch := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/control-plane/runtime/runs/other-run/trace-payloads/"+record.PayloadRef, nil, ut.Header{Key: "Cookie", Value: cookie})
	if mismatch.Code != consts.StatusNotFound || strings.Contains(mismatch.Body.String(), "run-private") {
		t.Fatalf("mismatch status=%d body=%s", mismatch.Code, mismatch.Body.String())
	}
	if store.privilegedAudits[len(store.privilegedAudits)-1].Outcome != "not_found" {
		t.Fatalf("mismatch audit=%#v", store.privilegedAudits[len(store.privilegedAudits)-1])
	}
}

func TestPrivilegedTracePayloadEndpointFailsClosedWhenAuditIsUnavailable(t *testing.T) {
	store := &testRuntimeReadStore{}
	cfg := privilegedTracePayloadTestConfig(t)
	key := runtime.DerivePrivilegedTracePayloadKey(cfg.Security.TracePayloadEncryptionKey)
	record, _, err := runtime.BuildPrivilegedTracePayload(
		runtime.PrivilegedTracePayloadCoordinates{RunID: "run-private", TraceType: "model_call", Source: "eino", CorrelationID: "trace-1"},
		map[string]any{"response": "must-not-return"},
		runtime.PrivilegedTracePayloadPolicy{Enabled: true, SampleRate: 1, Retention: time.Hour, MaxPayloadBytes: 4096, KeyID: "test-v1", EncryptionKey: key},
		time.Now().UTC(),
	)
	if err != nil {
		t.Fatal(err)
	}
	store.privilegedPayloads = append(store.privilegedPayloads, record)
	application := appcore.NewServiceWithRuntimeStore(cfg, nil, nil, nil, store)
	httpServer := NewHTTPServer(cfg, application)
	cookie := loginPrivilegedTraceControlPlane(t, httpServer, cfg.ControlPlane.AuthToken)
	store.privilegedAuditErr = errors.New("audit unavailable")
	path := "/api/control-plane/runtime/runs/run-private/trace-payloads/" + record.PayloadRef
	response := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, path, nil, ut.Header{Key: "Cookie", Value: cookie})
	if response.Code != consts.StatusServiceUnavailable || strings.Contains(response.Body.String(), "must-not-return") {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
}

func privilegedTracePayloadTestConfig(t *testing.T) config.Config {
	t.Helper()
	return config.Config{
		Server:        config.ServerConfig{HTTPPort: 8080},
		ControlPlane:  config.ControlPlaneConfig{StorePath: t.TempDir() + "/controlplane/overrides.json", AuthToken: "trace-admin", SessionTTLSecs: 3600, MaxFailedAttempts: 3},
		System:        config.SystemConfig{TruthDir: t.TempDir() + "/truth"},
		Runtime:       config.RuntimeConfig{MaxConcurrentRequests: 4, MaxConcurrentTools: 2, RequestTimeoutSeconds: 30, DeferredQueueLimit: 4, ClosedTokenTTLSecs: 3600, SkillPackageRevisionLimit: 4, SharedRootDir: "shared"},
		Security:      config.SecurityConfig{TracePayloadEncryptionKey: "trace-secret", TracePayloadEncryptionKeyID: "test-v1"},
		Observability: config.ObservabilityConfig{PrivilegedTracePayload: config.PrivilegedTracePayloadConfig{ReadEnabled: true}},
	}
}

func loginPrivilegedTraceControlPlane(t *testing.T, server *HTTPServer, token string) string {
	t.Helper()
	body := `{"token":"` + token + `"}`
	response := ut.PerformRequest(server.engine.Engine, http.MethodPost, "/api/control-plane/login", &ut.Body{Body: strings.NewReader(body), Len: len(body)}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if response.Code != consts.StatusOK {
		t.Fatalf("login status=%d body=%s", response.Code, response.Body.String())
	}
	return firstCookieValue(response)
}

func (s *testRuntimeReadStore) CreatePrivilegedTracePayload(_ context.Context, input runtime.PrivilegedTracePayload) (runtime.PrivilegedTracePayload, error) {
	s.privilegedPayloads = append(s.privilegedPayloads, input)
	return input, nil
}

func (s *testRuntimeReadStore) GetPrivilegedTracePayload(_ context.Context, runID, payloadRef string) (runtime.PrivilegedTracePayload, bool, error) {
	for _, item := range s.privilegedPayloads {
		if item.RunID == strings.TrimSpace(runID) && item.PayloadRef == strings.TrimSpace(payloadRef) {
			return item, true, nil
		}
	}
	return runtime.PrivilegedTracePayload{}, false, nil
}

func (s *testRuntimeReadStore) CreatePrivilegedTracePayloadAccessAudit(_ context.Context, input runtime.PrivilegedTracePayloadAccessAudit) error {
	if s.privilegedAuditErr != nil {
		return s.privilegedAuditErr
	}
	s.privilegedAudits = append(s.privilegedAudits, input)
	return nil
}

func (s *testRuntimeReadStore) DeletePrivilegedTracePayload(_ context.Context, payloadRef string) error {
	return nil
}

func (s *testRuntimeReadStore) DeleteExpiredPrivilegedTracePayloads(_ context.Context, before time.Time) (int64, error) {
	return 0, nil
}

func (s *testRuntimeReadStore) ListPrivilegedTracePayloadAccessAudits(_ context.Context, payloadRef string, limit int) ([]runtime.PrivilegedTracePayloadAccessAudit, error) {
	return append([]runtime.PrivilegedTracePayloadAccessAudit(nil), s.privilegedAudits...), nil
}

var _ runtime.PrivilegedTracePayloadStore = (*testRuntimeReadStore)(nil)
