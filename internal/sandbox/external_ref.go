// external_ref.go builds safe external sandbox reference validation results.
// external_ref.go 构造安全的 external sandbox reference 验证结果。
package sandbox

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	defaultMode      = "external_sandbox_ref"
	defaultProvider  = "athena-validation"
	defaultBoundary  = "validation_control_plane"
	defaultOperation = "validate"
	defaultResource  = "runtime_validation"
)

var nonSandboxRefChar = regexp.MustCompile(`[^a-zA-Z0-9_.-]+`)

// BuildExternalSandboxValidationResult creates a deterministic structured result for validation runs.
// BuildExternalSandboxValidationResult 为 validation run 创建确定性的结构化结果。
func BuildExternalSandboxValidationResult(req ValidationRequest) ExternalSandboxValidationResult {
	requestID := safeToken(req.RequestID, "runtime-validation")
	refID := "external_sandbox_ref." + requestID
	mode := safeToken(req.Mode, defaultMode)
	provider := safeToken(req.Provider, defaultProvider)
	boundary := safeToken(req.Boundary, defaultBoundary)
	operation := safeToken(req.Operation, defaultOperation)
	resource := safeToken(req.Resource, defaultResource)
	toolName := safeToken(req.ToolName, "validation_tool")
	decision := safeToken(req.GovernanceDecision, "allow")
	riskLevel := safeToken(req.RiskLevel, "unknown")
	signals := safeStringSlice(req.Signals)
	if len(signals) == 0 {
		signals = []string{"no_signal"}
	}

	ref := ExternalSandboxRef{
		RefID:     refID,
		Mode:      mode,
		Provider:  provider,
		Boundary:  boundary,
		Status:    "completed",
		Operation: operation,
		Resource:  resource,
		Metadata: map[string]any{
			"request_id": requestID,
			"run_id":     safeToken(req.RunID, ""),
			"step_id":    safeToken(req.StepID, ""),
		},
	}
	audit := ExternalSandboxAuditSummary{
		Summary:              fmt.Sprintf("%s completed with %s governance decision", defaultMode, decision),
		CredentialScope:      "none_persisted",
		ContextScope:         "safe_validation_summary_only",
		NestedExecution:      "disabled",
		StateIntegrity:       "input_snapshot_preserved",
		AllowedOutputClasses: []string{"risk_summary", "audit_summary", "candidate_projection"},
		SafeLabels: map[string]string{
			"mode":                mode,
			"provider":            provider,
			"boundary":            boundary,
			"operation":           operation,
			"resource":            resource,
			"governance_decision": decision,
		},
	}
	structured := ExternalSandboxStructuredResult{
		Status:     "success",
		ResultType: defaultMode,
		Summary:    fmt.Sprintf("%s produced structured validation result for %s", defaultMode, toolName),
		Output: map[string]any{
			"tool_name":           toolName,
			"governance_decision": decision,
			"risk_level":          riskLevel,
			"signals":             signals,
			"execution_ref":       refID,
		},
		RedactedInput: scrubMetadata(req.Metadata),
	}
	return ExternalSandboxValidationResult{
		SandboxRef:       ref,
		StructuredResult: structured,
		AuditSummary:     audit,
		Projection: map[string]any{
			"candidate_kind": "external_sandbox_ref",
			"status":         structured.Status,
			"summary":        structured.Summary,
			"execution_ref":  refID,
		},
	}
}

// RedactedPayload returns the safe trace/projection payload for persistence.
// RedactedPayload 返回可持久化的安全 trace/projection payload。
func (r ExternalSandboxValidationResult) RedactedPayload() map[string]any {
	return map[string]any{
		"sandbox_ref":       r.SandboxRef,
		"structured_result": r.StructuredResult,
		"audit_summary":     r.AuditSummary,
		"projection":        r.Projection,
	}
}

func safeToken(value string, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		value = strings.TrimSpace(fallback)
	}
	if value == "" {
		return ""
	}
	return strings.Trim(nonSandboxRefChar.ReplaceAllString(value, "_"), "_.-")
}

func safeStringSlice(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if trimmed := safeToken(value, ""); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func scrubMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any, len(metadata))
	for key, value := range metadata {
		lower := strings.ToLower(strings.TrimSpace(key))
		if strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password") || strings.Contains(lower, "credential") || strings.Contains(lower, "api_key") || strings.Contains(lower, "authorization") {
			out[key] = "[redacted]"
			continue
		}
		out[key] = scrubValue(value)
	}
	return out
}

func scrubValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return scrubMetadata(typed)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, scrubValue(item))
		}
		return out
	case string:
		if looksCredentialLike(typed) {
			return "[redacted]"
		}
		return typed
	default:
		return value
	}
}

func looksCredentialLike(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return strings.HasPrefix(lower, "bearer ") || strings.Contains(lower, "token") || strings.Contains(lower, "secret") || strings.Contains(lower, "password")
}
