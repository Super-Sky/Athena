// trace_timeline_test.go verifies safe timeline projection without a second persistence model.
// trace_timeline_test.go 验证安全时间线投影，不引入第二套持久化模型。
package server

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"moss/internal/config"
	"moss/internal/runtime"
)

func TestProjectAgentTraceTimelineOrdersAndClassifiesRecords(t *testing.T) {
	start := time.Date(2026, time.July, 12, 9, 0, 0, 0, time.UTC)
	end := start.Add(1500 * time.Millisecond)
	readout := agentRunTraceReadout{
		Run:   runtimeRunDTO{ID: "run-1", StartedAt: &start, CompletedAt: &end},
		Steps: []runtimeStepDTO{{ID: "step-1", Name: "goal_plan", Status: "completed", CreatedAt: start, StartedAt: &start, CompletedAt: &end}},
		Traces: []runtimeTraceDTO{
			{ID: "tool-1", StepID: "step-1", TraceType: "eino_tool_callback", Summary: "safe tool call", SafeLabels: map[string]string{"provider": "remote-tool"}, CreatedAt: start.Add(300 * time.Millisecond)},
			{ID: "governance-1", StepID: "step-1", TraceType: "tool_governance_decision", Summary: "denied unsafe action", Metadata: map[string]any{"status": "denied", "reason": "policy"}, CreatedAt: start.Add(500 * time.Millisecond)},
			{ID: "model-1", StepID: "step-1", TraceType: "eino_model_callback", Summary: "safe model summary", CreatedAt: start.Add(200 * time.Millisecond)},
		},
		Usage: []runtimeUsageDTO{{ID: "usage-1", StepID: "step-1", ResourceType: "model", Provider: "openai-compatible", Unit: "tokens", Amount: 25, CreatedAt: start.Add(700 * time.Millisecond)}},
	}

	items := projectAgentTraceTimeline(readout)
	if len(items) != 5 {
		t.Fatalf("item count = %d, want 5", len(items))
	}
	if items[0].Kind != "loop_step" || items[1].Kind != "model_call" || items[2].Kind != "tool_call" || items[3].Kind != "governance" || items[4].Kind != "usage" {
		t.Fatalf("timeline kinds = %#v", []string{items[0].Kind, items[1].Kind, items[2].Kind, items[3].Kind, items[4].Kind})
	}
	if items[0].DurationMS == nil || *items[0].DurationMS != 1500 {
		t.Fatalf("step duration = %#v, want 1500ms", items[0].DurationMS)
	}
	if items[2].Source != "remote-tool" {
		t.Fatalf("tool source = %q, want remote-tool", items[2].Source)
	}
	if items[3].Error["reason"] != "policy" {
		t.Fatalf("governance item = %#v, want safe failure detail", items[3])
	}
	if _, found := items[3].Detail["raw_payload"]; found {
		t.Fatalf("governance detail = %#v, must not include raw payload", items[3].Detail)
	}
	if countTimelineFailures(items) != 1 {
		t.Fatalf("failure count = %d, want 1", countTimelineFailures(items))
	}
}

func TestTimelineErrorDoesNotMarkNormalStatusAsFailure(t *testing.T) {
	if value := timelineError("recorded", map[string]any{"error": "historical metadata"}); value != nil {
		t.Fatalf("timelineError(recorded) = %#v, want nil", value)
	}
}

func TestRunManifestProjectionUsesPersistedManifestAndRemovesMetadataDuplicate(t *testing.T) {
	manifest := runtime.BuildRunManifest(&runtime.ExecutionSpec{
		Skill: runtime.SkillSpec{PrimarySkill: "analysis", Guidance: "private prompt", RevisionRefs: []runtime.RunRevisionRef{
			runtime.NewRunRevisionRef("skill", "analysis", "v1", "registry", "private skill"),
		}},
		Model: runtime.ModelSpec{Requested: runtime.ModelEndpoint{ProviderID: "provider", ProviderModelID: "model"}},
	}, time.Date(2026, time.July, 16, 8, 0, 0, 0, time.UTC))
	persisted, err := json.Marshal(manifest)
	if err != nil {
		t.Fatal(err)
	}
	var persistedMap map[string]any
	if err := json.Unmarshal(persisted, &persistedMap); err != nil {
		t.Fatal(err)
	}

	projected, status := runManifestFromMetadata(map[string]any{"run_manifest": persistedMap})
	if status != manifest.Status || projected == nil || projected.ManifestSHA256 != manifest.ManifestSHA256 {
		t.Fatalf("projected=%#v status=%q, want persisted manifest", projected, status)
	}
	metadata := metadataWithoutRunManifest(map[string]any{"run_manifest": persistedMap, "writer": "graph"})
	if _, exists := metadata["run_manifest"]; exists || metadata["writer"] != "graph" {
		t.Fatalf("metadata = %#v, want manifest removed and other fields retained", metadata)
	}
}

func TestRunManifestProjectionMarksLegacyRunUnavailable(t *testing.T) {
	manifest, status := runManifestFromMetadata(map[string]any{"writer": "legacy"})
	if manifest != nil || status != "legacy_unavailable" {
		t.Fatalf("manifest=%#v status=%q, want legacy_unavailable", manifest, status)
	}
}

func TestRunManifestProjectionRejectsUnknownSchemaWithoutCurrentConfigInference(t *testing.T) {
	manifest, status := runManifestFromMetadata(map[string]any{"run_manifest": map[string]any{
		"schema_version":  "agent_run_manifest.v99",
		"manifest_sha256": "future-digest",
	}})
	if manifest != nil || status != "unsupported_schema" {
		t.Fatalf("manifest=%#v status=%q, want unsupported_schema", manifest, status)
	}
}

func TestAgentTimelineRoutesAreRegistered(t *testing.T) {
	cfg := config.Config{Server: config.ServerConfig{HTTPPort: 8080}}
	httpServer := NewHTTPServer(cfg, nil)
	for _, path := range []string{"/api/agent/runs/run-missing/timeline", "/api/control-plane/runtime/runs/run-missing/timeline"} {
		response := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, path, nil)
		if response.Code != consts.StatusInternalServerError {
			t.Fatalf("timeline route %q status = %d, want %d; body=%s", path, response.Code, consts.StatusInternalServerError, response.Body.String())
		}
	}
}

func BenchmarkProjectAgentTraceTimeline(b *testing.B) {
	start := time.Date(2026, time.July, 12, 9, 0, 0, 0, time.UTC)
	readout := agentRunTraceReadout{Run: runtimeRunDTO{ID: "run-benchmark"}}
	for index := 0; index < 100; index++ {
		at := start.Add(time.Duration(index) * time.Millisecond)
		readout.Traces = append(readout.Traces, runtimeTraceDTO{ID: "trace-" + time.Duration(index).String(), TraceType: "eino_tool_callback", Summary: "safe tool callback", CreatedAt: at})
		readout.Usage = append(readout.Usage, runtimeUsageDTO{ID: "usage-" + time.Duration(index).String(), ResourceType: "tool", Unit: "call", Amount: 1, CreatedAt: at.Add(500 * time.Microsecond)})
	}
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = projectAgentTraceTimeline(readout)
	}
}
