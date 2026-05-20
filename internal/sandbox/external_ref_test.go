package sandbox

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildExternalSandboxValidationResultReturnsStructuredResult(t *testing.T) {
	result := BuildExternalSandboxValidationResult(ValidationRequest{
		RequestID:          "runtime-validation-123",
		RunID:              "run-1",
		StepID:             "step-1",
		ToolName:           "risk_signal_lookup",
		GovernanceDecision: "allow_with_redaction",
		RiskLevel:          "high",
		Signals:            []string{"credential_like", "redaction_required"},
		Metadata:           map[string]any{"ui_surface": "system_validation"},
	})

	if result.SandboxRef.RefID != "external_sandbox_ref.runtime-validation-123" {
		t.Fatalf("RefID = %q, want deterministic external sandbox ref", result.SandboxRef.RefID)
	}
	if result.StructuredResult.Status != "success" || result.StructuredResult.Output["tool_name"] != "risk_signal_lookup" {
		t.Fatalf("structured result = %#v, want success tool result", result.StructuredResult)
	}
	if result.AuditSummary.CredentialScope != "none_persisted" || result.AuditSummary.NestedExecution != "disabled" {
		t.Fatalf("audit summary = %#v, want minimal boundary safeguards", result.AuditSummary)
	}
	if result.Projection["candidate_kind"] != "external_sandbox_ref" {
		t.Fatalf("projection = %#v, want external_sandbox_ref candidate", result.Projection)
	}
}

func TestExternalSandboxValidationResultRedactsCredentialLikeMetadata(t *testing.T) {
	result := BuildExternalSandboxValidationResult(ValidationRequest{
		RequestID: "req-secret",
		Metadata: map[string]any{
			"authorization_token": "Bearer raw-token",
			"nested": map[string]any{
				"api_key": "raw-key",
			},
		},
	})
	raw, err := json.Marshal(result.RedactedPayload())
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	payload := string(raw)
	if strings.Contains(payload, "raw-token") || strings.Contains(payload, "raw-key") || strings.Contains(payload, "Bearer ") {
		t.Fatalf("payload contains raw credential-like value: %s", payload)
	}
	if !strings.Contains(payload, "[redacted]") {
		t.Fatalf("payload = %s, want redaction marker", payload)
	}
}
