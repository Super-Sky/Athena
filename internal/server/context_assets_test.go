package server

import (
	"context"
	"testing"

	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/contextassets"
	"moss/internal/controlplane"
	"moss/internal/customization"
	"moss/internal/session"
)

func TestPrepareContextAssetsInjectsDefaultSystemBindings(t *testing.T) {
	application := newContextAssetsTestService(t)
	seedDefaultSystemResource(t, application, controlplane.SystemResourceCreateRequest{
		AssetID:       "persona.default",
		AssetType:     "persona",
		AssetName:     "SOUL",
		SourceKind:    "truth_dir_source",
		SourceContent: "# SOUL\n\n## Summary\n\nDirect, strict, evidence first.\n",
	})
	seedDefaultSystemResource(t, application, controlplane.SystemResourceCreateRequest{
		AssetID:       "agent_profile.default",
		AssetType:     "agent_profile",
		AssetName:     "AGENTS",
		SourceKind:    "truth_dir_source",
		SourceContent: "# AGENTS\n\n## Operational Discipline\n\n- Keep contracts stable.\n",
	})
	seedDefaultSystemResource(t, application, controlplane.SystemResourceCreateRequest{
		AssetID:       "policy_rule.core.safety_constitution",
		AssetType:     "policy_rule",
		AssetName:     "Safety Constitution",
		SourceKind:    "truth_dir_source",
		SourceContent: "# Hard Gates\n\n- Verify before conclude.\n",
	})

	prepared := prepareContextAssets(context.Background(), application, map[string]any{}, customization.UserCustomization{}, contextassets.UsageInput{
		Query:    "Assess current risk",
		TaskType: "chat",
		Scene:    "default",
	})

	if prepared.Bundle == nil {
		t.Fatal("expected default system context assets to be injected")
	}
	if got := prepared.Trace.UsedContextAssets; len(got) < 3 {
		t.Fatalf("used_context_assets = %#v, want persona.default/agent_profile.default/policy_rule.core.safety_constitution", got)
	}
	assertContainsAsset(t, prepared.Trace.UsedContextAssets, "persona.default")
	assertContainsAsset(t, prepared.Trace.UsedContextAssets, "agent_profile.default")
	assertContainsAsset(t, prepared.Trace.UsedContextAssets, "policy_rule.core.safety_constitution")
	assertContainsAsset(t, prepared.Trace.ResidentAssets, "persona.default")
	assertContainsAsset(t, prepared.Trace.ResidentAssets, "agent_profile.default")
	assertContainsAsset(t, prepared.Trace.ResidentAssets, "policy_rule.core.safety_constitution")
	if _, ok := prepared.GlobalContext["effective_persona"].(map[string]any); !ok {
		t.Fatalf("effective_persona missing from global_context: %#v", prepared.GlobalContext)
	}
}

func TestPrepareContextAssetsExplicitSingletonOverridesDefaultBinding(t *testing.T) {
	application := newContextAssetsTestService(t)
	seedDefaultSystemResource(t, application, controlplane.SystemResourceCreateRequest{
		AssetID:       "persona.default",
		AssetType:     "persona",
		AssetName:     "SOUL",
		SourceKind:    "truth_dir_source",
		SourceContent: "# SOUL\n\n## Summary\n\nDefault persona summary.\n",
	})
	seedDefaultSystemResource(t, application, controlplane.SystemResourceCreateRequest{
		AssetID:       "agent_profile.default",
		AssetType:     "agent_profile",
		AssetName:     "AGENTS",
		SourceKind:    "truth_dir_source",
		SourceContent: "# AGENTS\n\n## Operational Discipline\n\n- Stay concise.\n",
	})

	globalContext := map[string]any{
		"context_assets": toAnySlice(contextassets.AssetMaps([]contextassets.Asset{
			{
				AssetID:    "persona.custom",
				AssetType:  "persona",
				AssetName:  "custom",
				SourceKind: "request_inline",
				Mode:       "inline",
				Priority:   999,
				Content: map[string]any{
					"summary": "Custom persona from request",
				},
				Resolution: contextassets.Resolution{
					ResidentHint: true,
				},
				Metadata: contextassets.Metadata{
					Tags: []string{"resident"},
				},
			},
		})),
	}

	prepared := prepareContextAssets(context.Background(), application, globalContext, customization.UserCustomization{}, contextassets.UsageInput{
		Query:    "Use my custom persona",
		TaskType: "chat",
		Scene:    "default",
	})

	assertContainsAsset(t, prepared.Trace.UsedContextAssets, "persona.custom")
	assertContainsAsset(t, prepared.Trace.UsedContextAssets, "agent_profile.default")
	assertNotContainsAsset(t, prepared.Trace.UsedContextAssets, "persona.default")
	persona, _ := prepared.GlobalContext["effective_persona"].(map[string]any)
	if got, _ := persona["summary"].(string); got != "Custom persona from request" {
		t.Fatalf("effective_persona.summary = %q, want custom request persona", got)
	}
}

func newContextAssetsTestService(t *testing.T) *appcore.Service {
	t.Helper()
	truthDir := t.TempDir() + "/truth"
	cfg := config.Config{
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		System: config.SystemConfig{
			TruthDir: truthDir,
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     2,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        session.DefaultDeferredQueueLimit,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL.Seconds()),
			SkillPackageRevisionLimit: 4,
		},
	}
	return appcore.NewService(cfg)
}

func seedDefaultSystemResource(t *testing.T, application *appcore.Service, input controlplane.SystemResourceCreateRequest) {
	t.Helper()
	if _, err := application.CreateSystemResource(context.Background(), input); err != nil {
		t.Fatalf("CreateSystemResource(%s) error = %v", input.AssetID, err)
	}
}

func assertContainsAsset(t *testing.T, items []string, target string) {
	t.Helper()
	for _, item := range items {
		if item == target {
			return
		}
	}
	t.Fatalf("asset %q not found in %#v", target, items)
}

func assertNotContainsAsset(t *testing.T, items []string, target string) {
	t.Helper()
	for _, item := range items {
		if item == target {
			t.Fatalf("asset %q unexpectedly found in %#v", target, items)
		}
	}
}
