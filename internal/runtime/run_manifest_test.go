// run_manifest_test.go verifies deterministic and redaction-safe run manifest capture.
// run_manifest_test.go 验证运行清单捕获具有确定性且不会保留敏感原文。
package runtime

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	runtimetask "moss/internal/runtime/task"
)

func TestBuildRunManifestIsDeterministicAndRedactionSafe(t *testing.T) {
	capturedAt := time.Date(2026, time.July, 16, 8, 0, 0, 0, time.UTC)
	spec := &ExecutionSpec{
		Skill: SkillSpec{
			PrimarySkill: "fund_analysis",
			Guidance:     "private prompt token=secret-value",
			RevisionRefs: []RunRevisionRef{
				NewRunRevisionRef("skill", "fund_analysis", "v2", "skill_registry", "private skill body"),
			},
		},
		Tools: ToolSpec{RevisionRefs: []RunRevisionRef{
			NewRunRevisionRef("tool", "market_quote", "v3", "tool_registry", map[string]any{"api_key": "private-tool-key"}),
		}},
		Model: ModelSpec{
			Requested:          ModelEndpoint{ProviderID: "provider-1", ProviderModelID: "model-1", Headers: map[string]string{"Authorization": "Bearer private"}},
			ExecutedConfig:     &model.ChatConfig{ProviderID: "provider-1", ProviderModelID: "model-1", BaseURL: "https://proxy-a.example/v1?token=private-url-token", APIKey: "private-model-key"},
			ResolvedParameters: &modelparams.ResolvedModelParameters{PolicyName: "balanced", PolicyVersion: "v1", Temperature: 0.2},
		},
		Metadata: ExecutionMetadata{ManifestRefs: RunManifestReferences{
			Governance: []RunRevisionRef{NewRunRevisionRef("governance_policy", "policy-1", "v4", "runtime_contract", "private policy")},
		}},
	}

	first := BuildRunManifest(spec, capturedAt)
	second := BuildRunManifest(spec, capturedAt)
	if first.ManifestSHA256 == "" || first.ManifestSHA256 != second.ManifestSHA256 {
		t.Fatalf("manifest digests = %q and %q, want equal non-empty values", first.ManifestSHA256, second.ManifestSHA256)
	}
	payload, err := json.Marshal(first)
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{"secret-value", "private skill body", "private-tool-key", "Bearer private", "private-model-key", "private-url-token", "private policy"} {
		if strings.Contains(string(payload), secret) {
			t.Fatalf("manifest contains raw sensitive content %q: %s", secret, payload)
		}
	}
	if first.Status != "complete" || first.Model == nil || first.Prompt == nil || len(first.Skills) != 1 || len(first.Tools) != 1 {
		t.Fatalf("manifest = %#v, want complete model/prompt/skill/tool refs", first)
	}
	if !ValidRunManifestDigest(first) {
		t.Fatal("fresh manifest digest is invalid")
	}

	spec.Skill.Guidance = "a revised prompt"
	revised := BuildRunManifest(spec, capturedAt)
	if revised.ManifestSHA256 == first.ManifestSHA256 || revised.Prompt.ContentSHA256 == first.Prompt.ContentSHA256 {
		t.Fatalf("revised prompt did not change manifest digest: first=%#v revised=%#v", first, revised)
	}
	spec.Skill.Guidance = "private prompt token=secret-value"
	spec.Model.ResolvedParameters.Temperature = 0.8
	revisedModel := BuildRunManifest(spec, capturedAt)
	if revisedModel.Model.ContentSHA256 == first.Model.ContentSHA256 {
		t.Fatal("revised model parameters did not change model digest")
	}
	spec.Model.ResolvedParameters.Temperature = 0.2
	spec.Model.ExecutedConfig.BaseURL = "https://proxy-b.example/v1"
	revisedEndpoint := BuildRunManifest(spec, capturedAt)
	if revisedEndpoint.Model.ContentSHA256 == first.Model.ContentSHA256 {
		t.Fatal("revised model endpoint did not change model digest")
	}
	first.Status = "tampered"
	if ValidRunManifestDigest(first) {
		t.Fatal("tampered manifest digest was accepted")
	}
}

func TestBuildRunManifestMarksMissingCoreRevisions(t *testing.T) {
	manifest := BuildRunManifest(&ExecutionSpec{Tools: ToolSpec{AllowedTools: []string{"quote_lookup"}}}, time.Unix(0, 0))
	if manifest.Status != "partial" {
		t.Fatalf("status = %q, want partial", manifest.Status)
	}
	joined := strings.Join(manifest.Missing, ",")
	for _, field := range []string{"model", "prompt", "skills", "tools"} {
		if !strings.Contains(joined, field) {
			t.Fatalf("missing = %#v, want %q", manifest.Missing, field)
		}
	}
}

func TestRunManifestReferencesMergeResolvedAndRequestedContextAssets(t *testing.T) {
	task := &runtimetask.RuntimeTask{GlobalContext: map[string]any{
		"context_assets": []any{
			map[string]any{"asset_id": "resolved", "asset_type": "knowledge", "source_kind": "system_truth"},
			map[string]any{"asset_id": "requested", "asset_type": "skill", "source_kind": "repo_managed", "ref": map[string]any{"version": "v2"}},
		},
		"context_assets_resolved": []any{
			map[string]any{"asset": map[string]any{"asset_id": "resolved", "asset_type": "knowledge", "source_kind": "system_truth"}, "compiled_version": "v1", "summary": "private resolved content"},
		},
	}}
	refs := runManifestReferences(task, nil).ContextAssets
	if len(refs) != 2 || refs[0].ID == refs[1].ID {
		t.Fatalf("context refs = %#v, want distinct resolved and requested refs", refs)
	}
	payload, err := json.Marshal(refs)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(payload), "private resolved content") {
		t.Fatalf("context refs contain raw content: %s", payload)
	}
}

func TestRunManifestSurvivesPostgresMetadataJSONRoundTrip(t *testing.T) {
	manifest := BuildRunManifest(&ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "analysis", Guidance: "analyze", RevisionRefs: []RunRevisionRef{
			NewRunRevisionRef("skill", "analysis", "v1", "registry", "skill"),
		}},
		Model: ModelSpec{Requested: ModelEndpoint{ProviderID: "provider", ProviderModelID: "model"}},
	}, time.Date(2026, time.July, 16, 8, 0, 0, 0, time.UTC))
	row, err := taskRunToRow(TaskRun{TaskID: "task-manifest", Metadata: map[string]any{"run_manifest": manifest}})
	if err != nil {
		t.Fatal(err)
	}
	roundTripped, err := taskRunFromRow(row)
	if err != nil {
		t.Fatal(err)
	}
	payload, err := json.Marshal(roundTripped.Metadata["run_manifest"])
	if err != nil {
		t.Fatal(err)
	}
	var restored RunManifest
	if err := json.Unmarshal(payload, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.ManifestSHA256 != manifest.ManifestSHA256 || !ValidRunManifestDigest(restored) {
		t.Fatalf("restored manifest = %#v, want valid original digest", restored)
	}
}

func BenchmarkBuildRunManifestWithOneHundredTools(b *testing.B) {
	spec := &ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "analysis", Guidance: "analyze evidence", RevisionRefs: []RunRevisionRef{
			NewRunRevisionRef("skill", "analysis", "v1", "registry", "skill body"),
		}},
		Model: ModelSpec{Requested: ModelEndpoint{ProviderID: "provider", ProviderModelID: "model"}},
	}
	for index := 0; index < 100; index++ {
		id := "tool-" + time.Duration(index).String()
		spec.Tools.RevisionRefs = append(spec.Tools.RevisionRefs, NewRunRevisionRef("tool", id, "v1", "registry", map[string]any{"name": id}))
	}
	capturedAt := time.Date(2026, time.July, 16, 8, 0, 0, 0, time.UTC)
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		_ = BuildRunManifest(spec, capturedAt)
	}
}
