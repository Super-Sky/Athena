package app

import (
	"testing"

	"moss/internal/runtime"
)

// TestResolveOrchestrationStateDefaultsToNormalized verifies new chat requests enter runtime as normalized tasks.
// TestResolveOrchestrationStateDefaultsToNormalized 用于验证新 chat 请求会以 normalized 语义进入 runtime。
func TestResolveOrchestrationStateDefaultsToNormalized(t *testing.T) {
	got := resolveOrchestrationState(ChatRequest{Query: "analyze risk posture"})
	if got != runtime.OrchestrationStateNormalized {
		t.Fatalf("resolveOrchestrationState() = %q, want %q", got, runtime.OrchestrationStateNormalized)
	}
}

// TestResolveOrchestrationStateMarksResumed verifies resume-token requests re-enter runtime as resumed tasks.
// TestResolveOrchestrationStateMarksResumed 用于验证带 resume token 的请求会以 resumed 语义重入 runtime。
func TestResolveOrchestrationStateMarksResumed(t *testing.T) {
	got := resolveOrchestrationState(ChatRequest{
		Query: "continue",
		Supplement: &runtime.SupplementPayload{
			Resume: &runtime.ResumeContext{ResumeToken: "resume-1"},
		},
	})
	if got != runtime.OrchestrationStateResumed {
		t.Fatalf("resolveOrchestrationState() = %q, want %q", got, runtime.OrchestrationStateResumed)
	}
}
