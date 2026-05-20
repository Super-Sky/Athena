package controlplane

import (
	"context"
	"path/filepath"
	"testing"

	"moss/internal/skills"
	"moss/internal/tools"
)

func TestManagerOverridesScenesAndRuntime(t *testing.T) {
	t.Parallel()

	store := NewFileStore(filepath.Join(t.TempDir(), "overrides.json"))
	manager := NewManager(store)

	savedScene, err := manager.PutScene(context.Background(), "security_review", SceneConfig{
		ID:                 "security_review",
		Description:        "custom security review",
		Keywords:           []string{"安全评估", "threat drill"},
		DefaultSkills:      []string{"cso_review"},
		SuggestedQuestions: []string{"是否需要我输出风险清单？"},
		Enabled:            true,
		MatchScore:         91,
	})
	if err != nil {
		t.Fatalf("PutScene() error = %v", err)
	}
	if savedScene.MatchScore != 91 {
		t.Fatalf("PutScene() match score = %d, want 91", savedScene.MatchScore)
	}

	runtimeView, err := manager.PutRuntime(context.Background(), RuntimeTuning{
		ChoiceRequiredEnabled:     false,
		AutomationFallbackEnabled: true,
		PlanningProgressEnabled:   false,
	})
	if err != nil {
		t.Fatalf("PutRuntime() error = %v", err)
	}
	if runtimeView.ChoiceRequiredEnabled {
		t.Fatalf("PutRuntime() choice_required_enabled = true, want false")
	}

	scenes, err := manager.ListScenes(context.Background())
	if err != nil {
		t.Fatalf("ListScenes() error = %v", err)
	}
	found := false
	for _, item := range scenes {
		if item.ID == "security_review" {
			found = true
			if item.Description != "custom security review" {
				t.Fatalf("security_review description = %q, want custom override", item.Description)
			}
			if len(item.Keywords) != 2 || item.Keywords[0] != "安全评估" {
				t.Fatalf("security_review keywords = %#v, want override", item.Keywords)
			}
		}
	}
	if !found {
		t.Fatal("security_review override not found in effective scene list")
	}
}

func TestManagerOverridesSkills(t *testing.T) {
	t.Parallel()

	store := NewFileStore(filepath.Join(t.TempDir(), "overrides.json"))
	manager := NewManager(store)

	_, err := manager.PutSkill(context.Background(), "cso_review", SkillConfig{
		Name:        "cso_review",
		Description: "custom cso description",
		Guidance:    "focus on risk summary first",
		ToolNames:   []string{"query_runtime_state"},
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("PutSkill() error = %v", err)
	}

	defs, err := skills.NewBuiltinSource().Load(context.Background())
	if err != nil {
		t.Fatalf("Load builtin skills error = %v", err)
	}
	items, err := manager.ListSkills(context.Background(), defs)
	if err != nil {
		t.Fatalf("ListSkills() error = %v", err)
	}

	found := false
	for _, item := range items {
		if item.Name == "cso_review" {
			found = true
			if item.Description != "custom cso description" {
				t.Fatalf("cso_review description = %q, want override", item.Description)
			}
			if item.Guidance != "focus on risk summary first" {
				t.Fatalf("cso_review guidance = %q, want override", item.Guidance)
			}
		}
	}
	if !found {
		t.Fatal("cso_review not found in effective skill list")
	}
}

func TestManagerOverridesToolsAndVersions(t *testing.T) {
	t.Parallel()

	store := NewFileStore(filepath.Join(t.TempDir(), "overrides.json"))
	manager := NewManager(store)

	saved, err := manager.PutTool(context.Background(), "lookup_profile", ToolConfig{
		Name:                 "lookup_profile",
		Description:          "custom profile tool",
		ToolScope:            "customer_profile",
		RequiresConfirmation: false,
		SideEffectLevel:      "none",
		InputSchemaSummary:   "user_id:string",
		OutputSchemaSummary:  "profile summary",
		Enabled:              true,
	})
	if err != nil {
		t.Fatalf("PutTool() error = %v", err)
	}
	if saved.Description != "custom profile tool" {
		t.Fatalf("PutTool() description = %q, want override", saved.Description)
	}

	defs, err := tools.DemoDefinitionList()
	if err != nil {
		t.Fatalf("DemoDefinitionList() error = %v", err)
	}
	items, err := manager.ListTools(context.Background(), defs)
	if err != nil {
		t.Fatalf("ListTools() error = %v", err)
	}
	found := false
	for _, item := range items {
		if item.Name == "lookup_profile" {
			found = true
			if item.Description != "custom profile tool" {
				t.Fatalf("lookup_profile description = %q, want override", item.Description)
			}
		}
	}
	if !found {
		t.Fatal("lookup_profile not found in effective tool list")
	}

	versions, err := manager.ListVersions(context.Background())
	if err != nil {
		t.Fatalf("ListVersions() error = %v", err)
	}
	if len(versions) == 0 {
		t.Fatal("expected version snapshot after tool update")
	}

	detail, err := manager.GetVersion(context.Background(), versions[0].VersionID)
	if err != nil {
		t.Fatalf("GetVersion() error = %v", err)
	}
	if len(detail.Document.Tools) == 0 || detail.Document.Tools[0].Name != "lookup_profile" {
		t.Fatalf("version document tools = %#v, want lookup_profile snapshot", detail.Document.Tools)
	}
}

func TestManagerRollbackVersion(t *testing.T) {
	t.Parallel()

	store := NewFileStore(filepath.Join(t.TempDir(), "overrides.json"))
	manager := NewManager(store)

	_, err := manager.PutScene(context.Background(), "security_review", SceneConfig{
		ID:          "security_review",
		Description: "v1",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("PutScene(v1) error = %v", err)
	}
	versions, err := manager.ListVersions(context.Background())
	if err != nil || len(versions) == 0 {
		t.Fatalf("ListVersions() after v1 error = %v, len = %d", err, len(versions))
	}
	targetVersionID := versions[0].VersionID

	_, err = manager.PutScene(context.Background(), "security_review", SceneConfig{
		ID:          "security_review",
		Description: "v2",
		Enabled:     true,
	})
	if err != nil {
		t.Fatalf("PutScene(v2) error = %v", err)
	}

	rolledBack, err := manager.RollbackVersion(context.Background(), targetVersionID)
	if err != nil {
		t.Fatalf("RollbackVersion() error = %v", err)
	}
	if rolledBack.Summary == "" {
		t.Fatal("RollbackVersion() summary is empty")
	}

	scenes, err := manager.ListScenes(context.Background())
	if err != nil {
		t.Fatalf("ListScenes() error = %v", err)
	}
	for _, item := range scenes {
		if item.ID == "security_review" && item.Description != "v1" {
			t.Fatalf("security_review description after rollback = %q, want v1", item.Description)
		}
	}
}
