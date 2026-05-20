package contextassets

import "testing"

func TestBuildBundleAndResolveUsage(t *testing.T) {
	t.Parallel()

	globalContext := map[string]any{
		"context_assets": []any{
			map[string]any{
				"asset_id":    "persona.default",
				"asset_type":  "persona",
				"asset_name":  "default persona",
				"mode":        "ref",
				"priority":    100,
				"source_kind": "repo_managed",
				"ref": map[string]any{
					"ref_type": "compiled_asset",
					"target":   "persona.default",
				},
				"resolution": map[string]any{
					"prefer_compiled":    true,
					"allow_detail_fetch": true,
					"resident_hint":      true,
				},
				"metadata": map[string]any{
					"tags": []string{"resident", "persona"},
				},
			},
			map[string]any{
				"asset_id":    "persona.override",
				"asset_type":  "persona",
				"asset_name":  "override persona",
				"mode":        "inline",
				"priority":    50,
				"source_kind": "platform_injected",
				"content": map[string]any{
					"summary": "override persona summary",
				},
				"metadata": map[string]any{
					"tags": []string{"persona"},
				},
			},
			map[string]any{
				"asset_id":    "policy_rule.core.safety_constitution",
				"asset_type":  "policy_rule",
				"asset_name":  "Safety Constitution",
				"mode":        "inline",
				"priority":    1000,
				"source_kind": "repo_managed",
				"content": map[string]any{
					"title":            "Safety Constitution",
					"guidance_lines":   []string{"Do not fabricate evidence"},
					"hard_gates":       []string{"No fabricated evidence"},
					"check_rules":      []string{"Require evidence for strong claims"},
					"operational_note": "core rule",
				},
				"resolution": map[string]any{
					"resident_hint": true,
				},
			},
			map[string]any{
				"asset_id":    "skill.security_review.risk_profiling",
				"asset_type":  "skill",
				"mode":        "ref",
				"priority":    80,
				"source_kind": "repo_managed",
				"ref": map[string]any{
					"ref_type": "compiled_asset",
					"target":   "skill.security_review.risk_profiling",
				},
				"resolution": map[string]any{
					"allow_detail_fetch": true,
				},
			},
		},
		"context_assets_resolved": []any{
			map[string]any{
				"asset": map[string]any{
					"asset_id":   "persona.default",
					"asset_type": "persona",
				},
				"summary":         "serious style",
				"guidance_text":   "use serious style",
				"loaded_from_ref": true,
			},
		},
	}

	bundle := BuildBundle(globalContext)
	if bundle == nil {
		t.Fatal("BuildBundle() returned nil")
	}
	trace := bundle.ResolveUsage(UsageInput{
		Query:    "请基于当前规则解释风险",
		TaskType: "chat",
		Scene:    "default",
	})
	if len(trace.UsedContextAssets) == 0 {
		t.Fatalf("ResolveUsage() used_context_assets is empty: %#v", trace)
	}
	if !containsString(trace.ResidentAssets, "persona.default") || !containsString(trace.ResidentAssets, "policy_rule.core.safety_constitution") {
		t.Fatalf("resident assets = %#v, want persona.default and policy_rule.core.safety_constitution", trace.ResidentAssets)
	}
	if !containsString(trace.SuppressedAssets, "persona.override") {
		t.Fatalf("suppressed assets = %#v, want lower-priority singleton persona suppressed", trace.SuppressedAssets)
	}
	if !containsString(trace.RequestedAssetDetails, "skill.security_review.risk_profiling") {
		t.Fatalf("requested asset details = %#v, want skill.security_review.risk_profiling", trace.RequestedAssetDetails)
	}
	if !containsString(trace.LoadedAssetDetails, "persona.default") {
		t.Fatalf("loaded asset details = %#v, want persona.default", trace.LoadedAssetDetails)
	}
}

func TestBuildCandidateTrace(t *testing.T) {
	t.Parallel()

	bundle := &Bundle{
		Assets: []Asset{
			{
				AssetID:           "memory.weekly-review",
				AssetType:         "memory_view",
				AssetName:         "weekly-review",
				Scope:             "session",
				SourceKind:        "system_truth",
				CandidateWritable: true,
				Ref: &Ref{
					RefType: "compiled_asset",
					Target:  "memory.weekly-review",
				},
			},
		},
		ResolvedAssets: []ResolvedAsset{
			{
				Asset: Asset{
					AssetID:   "memory.weekly-review",
					AssetType: "memory_view",
				},
				Summary:       "old summary",
				LoadedFromRef: true,
			},
		},
	}

	trace := BuildCandidateTrace(bundle, UsageTrace{}, CandidateInput{
		Query:      "请基于这轮对话更新我的长期记忆摘要",
		MainAnswer: "你当前更偏好直接的工程化表达，并要求单 issue 单提交。",
		TaskType:   "chat",
		Scene:      "default",
	})

	if len(trace.CandidateAssetTargets) != 1 {
		t.Fatalf("candidate targets len = %d, want 1; trace=%#v", len(trace.CandidateAssetTargets), trace)
	}
	if len(trace.CandidateAssetDiffs) != 1 {
		t.Fatalf("candidate diffs len = %d, want 1; trace=%#v", len(trace.CandidateAssetDiffs), trace)
	}
	if len(trace.CandidateAssetUpdates) != 1 {
		t.Fatalf("candidate updates len = %d, want 1; trace=%#v", len(trace.CandidateAssetUpdates), trace)
	}
	if got := trace.CandidateAssetTargets[0]["asset_id"]; got != "memory.weekly-review" {
		t.Fatalf("candidate target asset_id = %#v, want memory.weekly-review", got)
	}
	if got := trace.CandidateAssetDiffs[0]["before_summary"]; got != "old summary" {
		t.Fatalf("candidate diff before_summary = %#v, want old summary", got)
	}
	updateContent, _ := trace.CandidateAssetUpdates[0]["proposed_content"].(map[string]any)
	if updateContent == nil {
		t.Fatalf("candidate update missing proposed_content: %#v", trace.CandidateAssetUpdates[0])
	}
	if got := updateContent["summary"]; got == "" {
		t.Fatalf("candidate update summary = %#v, want non-empty", got)
	}
}

func TestApplyOverrides(t *testing.T) {
	t.Parallel()

	bundle := &Bundle{
		Assets: []Asset{
			{AssetID: "persona.default", AssetType: "persona", Priority: 10},
			{AssetID: "policy_rule.core.safety_constitution", AssetType: "policy_rule", Priority: 20},
		},
		ResolvedAssets: []ResolvedAsset{
			{Asset: Asset{AssetID: "persona.default", AssetType: "persona"}, LoadedFromRef: true},
			{Asset: Asset{AssetID: "policy_rule.core.safety_constitution", AssetType: "policy_rule"}, LoadedFromRef: true},
		},
	}

	overridden := bundle.ApplyOverrides(BindingOverrides{
		ContextAssetOverrides: []Asset{
			{AssetID: "policy_rule.core.safety_constitution", AssetType: "policy_rule", Priority: 99, Scope: "session"},
			{AssetID: "memory.weekly", AssetType: "memory_view", Priority: 40},
		},
		DisabledAssetTypes: []string{"persona"},
		AssetPriorityOverrides: map[string]int{
			"memory.weekly": 88,
		},
	})
	if overridden == nil {
		t.Fatal("ApplyOverrides() returned nil")
	}
	if len(overridden.Assets) != 2 {
		t.Fatalf("assets len = %d, want 2", len(overridden.Assets))
	}
	if containsString([]string{overridden.Assets[0].AssetType, overridden.Assets[1].AssetType}, "persona") {
		t.Fatalf("assets = %#v, persona should be disabled", overridden.Assets)
	}
	var memory Asset
	for _, item := range overridden.Assets {
		if item.AssetID == "memory.weekly" {
			memory = item
		}
	}
	if memory.AssetID == "" || memory.Priority != 88 {
		t.Fatalf("memory asset = %#v, want inserted asset with priority override 88", memory)
	}
	for _, item := range overridden.ResolvedAssets {
		if item.Asset.AssetID == "persona.default" || item.Asset.AssetID == "policy_rule.core.safety_constitution" {
			t.Fatalf("resolved assets = %#v, overridden/disabled assets should be removed for re-hydration", overridden.ResolvedAssets)
		}
	}
}

func TestBuildEffectiveViews(t *testing.T) {
	t.Parallel()

	bundle := &Bundle{
		Assets: []Asset{
			{
				AssetID:    "persona.default",
				AssetType:  "persona",
				AssetName:  "Default Persona",
				Priority:   100,
				SourceKind: "repo_managed",
				Content: map[string]any{
					"summary":      "严谨表达",
					"bottom_lines": []string{"不虚构"},
				},
			},
			{
				AssetID:    "rule.core",
				AssetType:  "policy_rule",
				AssetName:  "Core Rules",
				Priority:   90,
				SourceKind: "repo_managed",
				Content: map[string]any{
					"title":          "Core Rules",
					"guidance_lines": []string{"先核对事实"},
					"hard_gates":     []string{"禁止跨租户泄漏"},
				},
			},
			{
				AssetID:    "skill.issue-intake",
				AssetType:  "skill",
				AssetName:  "issue-intake",
				Priority:   80,
				SourceKind: "repo_managed",
				Content: map[string]any{
					"name":     "issue-intake",
					"skill_id": "issue_intake",
				},
			},
		},
	}

	trace := UsageTrace{
		UsedContextAssets: []string{"persona.default", "rule.core", "skill.issue-intake"},
		ResidentAssets:    []string{"persona.default", "rule.core"},
		OnDemandAssets:    []string{"skill.issue-intake"},
	}

	views := BuildEffectiveViews(bundle, trace)
	if got := firstNonEmptyString(views.EffectivePersona["summary"]); got != "严谨表达" {
		t.Fatalf("effective persona summary = %q, want 严谨表达", got)
	}
	if len(views.EffectivePolicyRules) != 1 {
		t.Fatalf("effective policy rules len = %d, want 1", len(views.EffectivePolicyRules))
	}
	if got := firstNonEmptyString(views.EffectivePolicyRules[0]["title"]); got != "Core Rules" {
		t.Fatalf("effective policy rule title = %q, want Core Rules", got)
	}
	if len(views.EffectiveSkills) != 1 {
		t.Fatalf("effective skills len = %d, want 1", len(views.EffectiveSkills))
	}

	globalContext := views.AsGlobalContext()
	decoded := BuildEffectiveViewsFromGlobalContext(globalContext)
	if got := firstNonEmptyString(decoded.EffectivePersona["summary"]); got != "严谨表达" {
		t.Fatalf("decoded effective persona summary = %q, want 严谨表达", got)
	}
	if len(decoded.GuidanceLines()) == 0 {
		t.Fatalf("guidance lines = %#v, want non-empty", decoded.GuidanceLines())
	}
}
