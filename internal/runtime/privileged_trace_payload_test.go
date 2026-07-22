package runtime

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestBuildPrivilegedTracePayloadEncryptsRedactedContent(t *testing.T) {
	now := time.Date(2026, 7, 16, 8, 0, 0, 0, time.UTC)
	key := DerivePrivilegedTracePayloadKey("dedicated-secret")
	record, status, err := BuildPrivilegedTracePayload(PrivilegedTracePayloadCoordinates{
		RunID: "run-1", StepID: "step-1", TraceType: "model_call", Source: "eino", CorrelationID: "trace-1",
	}, map[string]any{
		"messages":       []any{map[string]any{"role": "user", "content": "review this portfolio"}},
		"empty_response": nil,
		"api_key":        "must-not-survive",
		"broker_account": "must-not-survive",
		"reasoning":      "must-not-survive",
		"note":           "Authorization: Bearer must-not-survive",
		"custom_private": "also-secret",
	}, PrivilegedTracePayloadPolicy{
		Enabled: true, SampleRate: 1, Retention: 24 * time.Hour, MaxPayloadBytes: 4096,
		MaxFieldBytes: 1024, KeyID: "test-v1", EncryptionKey: key,
		ExtraRedactedFields: []string{"custom_private"},
	}, now)
	if err != nil || status != "captured" {
		t.Fatalf("BuildPrivilegedTracePayload() status=%q error=%v", status, err)
	}
	if strings.Contains(string(record.Ciphertext), "review this portfolio") || record.RedactedFieldCount != 5 {
		t.Fatalf("ciphertext leaks plaintext or redaction count=%d", record.RedactedFieldCount)
	}
	decoded, err := DecryptPrivilegedTracePayload(record, key)
	if err != nil {
		t.Fatalf("DecryptPrivilegedTracePayload() error=%v", err)
	}
	if decoded["api_key"] != privilegedTraceRedactedValue || decoded["custom_private"] != privilegedTraceRedactedValue {
		t.Fatalf("decoded payload not redacted: %#v", decoded)
	}
	if decoded["empty_response"] != nil {
		t.Fatalf("decoded nil field=%#v, want nil", decoded["empty_response"])
	}
	if record.ExpiresAt.Sub(record.CreatedAt) != 24*time.Hour {
		t.Fatalf("retention=%s", record.ExpiresAt.Sub(record.CreatedAt))
	}
}

func BenchmarkBuildPrivilegedTracePayload(b *testing.B) {
	policy := PrivilegedTracePayloadPolicy{
		Enabled: true, SampleRate: 1, Retention: 24 * time.Hour, MaxPayloadBytes: 256 * 1024,
		MaxFieldBytes: 64 * 1024, KeyID: "benchmark-v1", EncryptionKey: DerivePrivilegedTracePayloadKey("benchmark-secret"),
	}
	payload := map[string]any{
		"request":  map[string]any{"messages": []any{map[string]any{"role": "user", "content": strings.Repeat("market context ", 128)}}},
		"response": map[string]any{"message": map[string]any{"role": "assistant", "content": strings.Repeat("scenario analysis ", 128)}},
	}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		coordinates := PrivilegedTracePayloadCoordinates{
			RunID: "benchmark-run", StepID: "step-1", TraceType: "model_call", Source: "benchmark",
			CorrelationID: fmt.Sprintf("trace-%d", index),
		}
		if _, _, err := BuildPrivilegedTracePayload(coordinates, payload, policy, time.Unix(0, int64(index))); err != nil {
			b.Fatal(err)
		}
	}
}

func TestBuildPrivilegedTracePayloadSamplingAndSizeStatuses(t *testing.T) {
	coordinates := PrivilegedTracePayloadCoordinates{RunID: "run-1", TraceType: "tool_call", Source: "canonical", CorrelationID: "call-1"}
	policy := PrivilegedTracePayloadPolicy{
		Enabled: true, SampleRate: 0, Retention: time.Hour, MaxPayloadBytes: 16,
		KeyID: "test-v1", EncryptionKey: DerivePrivilegedTracePayloadKey("secret"),
	}
	if _, status, err := BuildPrivilegedTracePayload(coordinates, map[string]any{"value": "short"}, policy, time.Now()); err != nil || status != "sampled_out" {
		t.Fatalf("zero sample status=%q error=%v", status, err)
	}
	policy.SampleRate = 1
	if _, status, err := BuildPrivilegedTracePayload(coordinates, map[string]any{"value": strings.Repeat("x", 100)}, policy, time.Now()); err != nil || status != "size_exceeded" {
		t.Fatalf("oversize status=%q error=%v", status, err)
	}
}

func TestDecryptPrivilegedTracePayloadRejectsTampering(t *testing.T) {
	key := DerivePrivilegedTracePayloadKey("secret")
	buildRecord := func(t *testing.T) PrivilegedTracePayload {
		t.Helper()
		record, _, err := BuildPrivilegedTracePayload(
			PrivilegedTracePayloadCoordinates{RunID: "run-1", TraceType: "model_call", Source: "eino", CorrelationID: "trace-1"},
			map[string]any{"response": "ok"},
			PrivilegedTracePayloadPolicy{Enabled: true, SampleRate: 1, Retention: time.Hour, MaxPayloadBytes: 1024, KeyID: "v1", EncryptionKey: key},
			time.Now(),
		)
		if err != nil {
			t.Fatal(err)
		}
		return record
	}
	tests := map[string]func(*PrivilegedTracePayload){
		"ciphertext":           func(record *PrivilegedTracePayload) { record.Ciphertext[0] ^= 0xff },
		"created_at":           func(record *PrivilegedTracePayload) { record.CreatedAt = record.CreatedAt.Add(time.Second) },
		"expires_at":           func(record *PrivilegedTracePayload) { record.ExpiresAt = record.ExpiresAt.Add(time.Hour) },
		"payload_size":         func(record *PrivilegedTracePayload) { record.PayloadSize++ },
		"redacted_field_count": func(record *PrivilegedTracePayload) { record.RedactedFieldCount++ },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			record := buildRecord(t)
			mutate(&record)
			if _, err := DecryptPrivilegedTracePayload(record, key); err == nil {
				t.Fatal("DecryptPrivilegedTracePayload() expected authentication error")
			}
		})
	}
}

// TestRedactPrivilegedTracePayloadRejectsSensitiveFreeText verifies forbidden content cannot bypass field-name redaction.
// TestRedactPrivilegedTracePayloadRejectsSensitiveFreeText 验证禁止内容不能通过通用字段名绕过脱敏。
func TestRedactPrivilegedTracePayloadRejectsSensitiveFreeText(t *testing.T) {
	tests := []string{
		"broker_account: 123456789012",
		"证券账户：A123456789",
		"data:application/pdf;base64,VGhpcyBpcyBhIHByaXZhdGUgZmlsZQ==",
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiJ1c2VyLTEifQ.signature123456",
		"-----BEGIN PRIVATE KEY----- private material",
	}
	for _, input := range tests {
		redacted, count := RedactPrivilegedTracePayloadWithFields(map[string]any{"content": input}, 4096, nil)
		output := redacted.(map[string]any)
		if output["content"] != privilegedTraceRedactedValue || count != 1 {
			t.Fatalf("content %q was not redacted: %#v count=%d", input, output, count)
		}
	}
}

func TestPrivilegedContextAssemblyPayloadHonorsCapturePolicy(t *testing.T) {
	spec := &ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "fund-analysis", AuxiliarySkills: []string{"market-data"}, Guidance: "compare risk-adjusted scenarios"},
		Metadata: ExecutionMetadata{
			Constraints: map[string]any{"used_context_assets": []string{"policy-v1"}, "workspace_id": "workspace-1"},
			ManifestRefs: RunManifestReferences{
				ContextAssets: []RunRevisionRef{{Kind: "context_asset", ID: "policy-v1", ContentSHA256: "sha256-policy"}},
			},
		},
	}
	payload := privilegedContextAssemblyPayload(spec)
	skill, _ := payload["skill"].(map[string]any)
	if skill["primary"] != "fund-analysis" || skill["guidance"] != "compare risk-adjusted scenarios" {
		t.Fatalf("context payload skill=%#v", skill)
	}
	contextSummary, _ := payload["context"].(map[string]any)
	if refs, _ := contextSummary["context_assets"].([]RunRevisionRef); len(refs) != 1 || refs[0].ID != "policy-v1" {
		t.Fatalf("context payload refs=%#v", contextSummary["context_assets"])
	}

	policy := PrivilegedTracePayloadPolicy{Enabled: true, CaptureContextSummary: false}
	if status, reason := privilegedTraceCaptureEligibility(policy, "workspace-1", privilegedTraceComponentContext); status != "disabled" || reason != "context_capture_disabled" {
		t.Fatalf("disabled context eligibility=(%q, %q)", status, reason)
	}
	policy.CaptureContextSummary = true
	if status, reason := privilegedTraceCaptureEligibility(policy, "workspace-1", privilegedTraceComponentContext); status != "eligible" || reason != "" {
		t.Fatalf("enabled context eligibility=(%q, %q)", status, reason)
	}
}
