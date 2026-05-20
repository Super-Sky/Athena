package validationmcp

import (
	"context"
	"testing"
)

func TestServerListsDeterministicToolSchemas(t *testing.T) {
	server := NewServer()

	tools, err := server.ListTools(context.Background())
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	if len(tools) != 2 {
		t.Fatalf("len(tools) = %d, want 2", len(tools))
	}
	if tools[0].Name != "risk_signal_lookup" || tools[1].Name != "security_context_echo" {
		t.Fatalf("tool order = %#v, want sorted risk/security", tools)
	}
	if tools[0].InputSchema["type"] != "object" || tools[0].OutputSchema["type"] != "object" {
		t.Fatalf("risk_signal_lookup schemas = %#v / %#v, want object schemas", tools[0].InputSchema, tools[0].OutputSchema)
	}
}

func TestInvokeRiskSignalLookupRedactsCredentials(t *testing.T) {
	server := NewServer()

	result, err := server.Invoke(context.Background(), InvocationRequest{
		ToolName: "risk_signal_lookup",
		Input: map[string]any{
			"risk_key": "credential_export",
			"credentials": map[string]any{
				"authorization": "Bearer raw-token",
			},
		},
		Metadata: map[string]any{
			"api_key":    "sk-raw",
			"request_id": "req-1",
		},
	})
	if err != nil {
		t.Fatalf("Invoke() error = %v", err)
	}
	if result.Status != "success" || result.Output["risk_level"] != "high" {
		t.Fatalf("result = %#v, want successful high-risk output", result)
	}
	if !result.AppliedRedaction {
		t.Fatalf("AppliedRedaction = false, want true")
	}
	input, _ := result.Trace.RedactedPayload["input"].(map[string]any)
	if input["credentials"] != redactedValue {
		t.Fatalf("trace input credentials = %#v, want redacted", input["credentials"])
	}
	metadata, _ := result.Trace.RedactedPayload["metadata"].(map[string]any)
	if metadata["api_key"] != redactedValue {
		t.Fatalf("trace metadata api_key = %#v, want redacted", metadata["api_key"])
	}
}
