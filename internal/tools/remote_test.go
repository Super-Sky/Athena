package tools

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
)

func TestRemoteToolExecutesAfterGovernanceAndRedactsArguments(t *testing.T) {
	var governanceDone atomic.Bool
	var received RemoteExecutionRequest
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !governanceDone.Load() {
			t.Error("remote endpoint reached before governance")
		}
		if err := json.NewDecoder(r.Body).Decode(&received); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		writeRemoteResponse(t, w, received, http.StatusOK, "ok", `{"price":1}`, nil)
	}))
	defer server.Close()

	var events []RemoteInvocationEvent
	definition := mustRemoteDefinition(t, remoteRegistration(server.URL), RemoteToolOptions{
		AllowedOrigins: []string{server.URL},
		Evaluate: func(context.Context, RemoteGovernanceRequest) (RemoteGovernanceDecision, error) {
			governanceDone.Store(true)
			return RemoteGovernanceDecision{
				DecisionID:   "decision-1",
				Decision:     "allow_with_redaction",
				RedactFields: []string{"account.token"},
			}, nil
		},
		Observe: func(_ context.Context, event RemoteInvocationEvent) {
			events = append(events, event)
		},
	})

	content, err := invokeRemoteDefinition(t, definition, `{"symbol":"510300","account":{"token":"secret"}}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if content != `{"price":1}` {
		t.Fatalf("content = %q", content)
	}
	var arguments map[string]any
	if err := json.Unmarshal(received.Arguments, &arguments); err != nil {
		t.Fatal(err)
	}
	account := arguments["account"].(map[string]any)
	if account["token"] != "[redacted]" {
		t.Fatalf("redacted token = %#v", account["token"])
	}
	if len(events) != 1 || events[0].DecisionID != "decision-1" || events[0].Status != "ok" {
		t.Fatalf("events = %#v", events)
	}
}

func TestRemoteToolRetriesOnlyBoundedIdempotentCalls(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request RemoteExecutionRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Errorf("decode request: %v", err)
			return
		}
		if attempts.Add(1) == 1 {
			writeRemoteResponse(t, w, request, http.StatusServiceUnavailable, "error", "", &RemoteExecutionError{
				Code: "upstream_busy", Message: "retry", Retryable: true,
			})
			return
		}
		writeRemoteResponse(t, w, request, http.StatusOK, "ok", "fresh", nil)
	}))
	defer server.Close()

	registration := remoteRegistration(server.URL)
	registration.RetryMaxAttempts = 1
	definition := mustRemoteDefinition(t, registration, RemoteToolOptions{AllowedOrigins: []string{server.URL}})
	content, err := invokeRemoteDefinition(t, definition, `{"symbol":"SPY"}`)
	if err != nil || content != "fresh" {
		t.Fatalf("content = %q, error = %v", content, err)
	}
	if attempts.Load() != 2 {
		t.Fatalf("attempts = %d", attempts.Load())
	}

	registration.SideEffectLevel = "write"
	registration.Idempotent = false
	if _, err := NewRemoteDefinition(registration, RemoteToolOptions{AllowedOrigins: []string{server.URL}}); err == nil {
		t.Fatal("expected non-idempotent retry validation error")
	}
}

func TestRemoteToolNormalizesTimeoutCorrelationAndSizeErrors(t *testing.T) {
	tests := []struct {
		name      string
		handler   func(http.ResponseWriter, *http.Request, RemoteExecutionRequest)
		timeoutMS int
		maxBytes  int64
		wantCode  string
	}{
		{
			name: "timeout",
			handler: func(_ http.ResponseWriter, _ *http.Request, _ RemoteExecutionRequest) {
				time.Sleep(40 * time.Millisecond)
			},
			timeoutMS: 5,
			wantCode:  "remote_timeout",
		},
		{
			name: "request correlation",
			handler: func(w http.ResponseWriter, _ *http.Request, request RemoteExecutionRequest) {
				request.RequestID = "wrong"
				writeRemoteResponse(t, w, request, http.StatusOK, "ok", "bad", nil)
			},
			wantCode: "request_id_mismatch",
		},
		{
			name: "response budget",
			handler: func(w http.ResponseWriter, _ *http.Request, request RemoteExecutionRequest) {
				writeRemoteResponse(t, w, request, http.StatusOK, "ok", strings.Repeat("x", 256), nil)
			},
			maxBytes: 64,
			wantCode: "response_too_large",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var request RemoteExecutionRequest
				_ = json.NewDecoder(r.Body).Decode(&request)
				test.handler(w, r, request)
			}))
			defer server.Close()
			registration := remoteRegistration(server.URL)
			if test.timeoutMS > 0 {
				registration.TimeoutMS = test.timeoutMS
			}
			definition := mustRemoteDefinition(t, registration, RemoteToolOptions{
				AllowedOrigins:   []string{server.URL},
				MaxResponseBytes: test.maxBytes,
			})
			_, err := invokeRemoteDefinition(t, definition, `{}`)
			var remoteErr *RemoteExecutionError
			if !errors.As(err, &remoteErr) || remoteErr.Code != test.wantCode {
				t.Fatalf("error = %#v, want code %q", err, test.wantCode)
			}
		})
	}
}

func TestRemoteToolRejectsGovernanceDenialBeforeNetwork(t *testing.T) {
	var hits atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	defer server.Close()
	definition := mustRemoteDefinition(t, remoteRegistration(server.URL), RemoteToolOptions{
		AllowedOrigins: []string{server.URL},
		Evaluate: func(context.Context, RemoteGovernanceRequest) (RemoteGovernanceDecision, error) {
			return RemoteGovernanceDecision{Decision: "deny", Reason: "policy"}, nil
		},
	})
	_, err := invokeRemoteDefinition(t, definition, `{}`)
	var remoteErr *RemoteExecutionError
	if !errors.As(err, &remoteErr) || remoteErr.Code != "governance_denied" {
		t.Fatalf("error = %#v", err)
	}
	if hits.Load() != 0 {
		t.Fatalf("network hits = %d", hits.Load())
	}
}

func TestRemoteToolRejectsDisallowedOriginAndRedirect(t *testing.T) {
	targetHits := atomic.Int32{}
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		targetHits.Add(1)
	}))
	defer target.Close()
	redirect := httptest.NewServer(http.RedirectHandler(target.URL, http.StatusTemporaryRedirect))
	defer redirect.Close()

	if _, err := NewRemoteDefinition(remoteRegistration(target.URL), RemoteToolOptions{AllowedOrigins: []string{redirect.URL}}); err == nil {
		t.Fatal("expected disallowed origin error")
	}
	definition := mustRemoteDefinition(t, remoteRegistration(redirect.URL), RemoteToolOptions{AllowedOrigins: []string{redirect.URL}})
	if _, err := invokeRemoteDefinition(t, definition, `{}`); err == nil {
		t.Fatal("expected redirect response error")
	}
	if targetHits.Load() != 0 {
		t.Fatalf("redirect target hits = %d", targetHits.Load())
	}
}

func TestRemoteToolRejectsNullArguments(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer server.Close()
	definition := mustRemoteDefinition(t, remoteRegistration(server.URL), RemoteToolOptions{AllowedOrigins: []string{server.URL}})
	_, err := invokeRemoteDefinition(t, definition, `null`)
	var remoteErr *RemoteExecutionError
	if !errors.As(err, &remoteErr) || remoteErr.Code != "invalid_arguments" {
		t.Fatalf("error = %#v", err)
	}
}

func remoteRegistration(endpoint string) RemoteRegistration {
	return RemoteRegistration{
		RegistrationID: "fund-snapshot-v1",
		AppID:          "athena-fund-assistant",
		Name:           "fund_market_snapshot",
		Description:    "Read a normalized market snapshot.",
		Parameters: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"symbol": map[string]any{"type": "string"},
			},
		},
		Endpoint:        endpoint,
		ToolScope:       "market_data_read",
		Operation:       "read",
		RiskLevel:       "low",
		SideEffectLevel: "none",
		Idempotent:      true,
		TimeoutMS:       500,
		Enabled:         true,
	}
}

func mustRemoteDefinition(t *testing.T, registration RemoteRegistration, options RemoteToolOptions) Definition {
	t.Helper()
	definition, err := NewRemoteDefinition(registration, options)
	if err != nil {
		t.Fatalf("NewRemoteDefinition() error = %v", err)
	}
	return definition
}

func invokeRemoteDefinition(t *testing.T, definition Definition, arguments string) (string, error) {
	t.Helper()
	invokable, ok := definition.BaseTool.(einotool.InvokableTool)
	if !ok {
		t.Fatalf("tool %q is not invokable", definition.Name)
	}
	return invokable.InvokableRun(context.Background(), arguments)
}

func writeRemoteResponse(t *testing.T, writer http.ResponseWriter, request RemoteExecutionRequest, statusCode int, status, content string, remoteErr *RemoteExecutionError) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(statusCode)
	if err := json.NewEncoder(writer).Encode(RemoteExecutionResponse{
		ContractVersion: RemoteToolContractVersion,
		RequestID:       request.RequestID,
		ToolCallID:      request.ToolCallID,
		Status:          status,
		Content:         content,
		Error:           remoteErr,
	}); err != nil {
		t.Errorf("encode response: %v", err)
	}
}
