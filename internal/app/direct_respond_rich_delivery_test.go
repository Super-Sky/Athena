package app

import (
	"testing"

	"moss/internal/contextassets"
)

func TestAttachDirectRespondContextAssetArtifactsIncludesUsageAndEffectiveViews(t *testing.T) {
	result := &DirectRespondRichResult{
		StructuredResult: map[string]any{},
	}
	trace := contextassets.UsageTrace{
		UsedContextAssets: []string{"persona.default", "policy_rule.core.safety_constitution"},
		ResidentAssets:    []string{"persona.default"},
		OnDemandAssets:    []string{"memory.weekly"},
		SuppressedAssets:  []string{"skill.experimental"},
		AssetConflictsResolved: []map[string]any{
			{"asset_type": "persona", "winner": "persona.default"},
		},
		RequestedAssetDetails: []string{"memory.weekly"},
		LoadedAssetDetails:    []string{"memory.weekly"},
		CandidateAssetTargets: []map[string]any{
			{"asset_id": "memory.weekly", "target_type": "memory_view"},
		},
		CandidateAssetDiffs: []map[string]any{
			{"asset_id": "memory.weekly", "summary": "new preference discovered"},
		},
		CandidateAssetUpdates: []map[string]any{
			{"asset_id": "memory.weekly", "mode": "candidate_update"},
		},
		AssetUsageTrace: []map[string]any{
			{"asset_id": "persona.default", "reason": "resident persona guidance applied"},
		},
	}
	views := contextassets.EffectiveViews{
		EffectivePersona: map[string]any{
			"summary": "直接、严格、证据优先",
		},
		EffectivePolicyRules: []map[string]any{
			{"rule_id": "safety_constitution", "title": "Safety Constitution"},
		},
		EffectiveSkills: []map[string]any{
			{"skill_id": "user_overview", "name": "User Overview"},
		},
	}

	AttachDirectRespondContextAssetArtifacts(result, trace, views)

	if got := result.StructuredResult["used_context_assets"]; got == nil {
		t.Fatalf("used_context_assets missing: %#v", result.StructuredResult)
	}
	if got := result.StructuredResult["asset_usage_trace"]; got == nil {
		t.Fatalf("asset_usage_trace missing: %#v", result.StructuredResult)
	}
	if got := result.StructuredResult["candidate_asset_updates"]; got == nil {
		t.Fatalf("candidate_asset_updates missing: %#v", result.StructuredResult)
	}
	persona, ok := result.StructuredResult["effective_persona"].(map[string]any)
	if !ok || persona["summary"] != "直接、严格、证据优先" {
		t.Fatalf("effective_persona = %#v, want injected effective persona", result.StructuredResult["effective_persona"])
	}
	rules, ok := result.StructuredResult["effective_policy_rules"].([]map[string]any)
	if !ok || len(rules) != 1 || rules[0]["rule_id"] != "safety_constitution" {
		t.Fatalf("effective_policy_rules = %#v, want policy rule", result.StructuredResult["effective_policy_rules"])
	}
	skills, ok := result.StructuredResult["effective_skills"].([]map[string]any)
	if !ok || len(skills) != 1 || skills[0]["skill_id"] != "user_overview" {
		t.Fatalf("effective_skills = %#v, want skill", result.StructuredResult["effective_skills"])
	}
}
