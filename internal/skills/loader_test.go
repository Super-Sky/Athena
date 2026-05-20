package skills

import (
	"context"
	"testing"
)

// TestMergedLoaderLoadsBuiltinAndStoredSkills ensures builtin and persisted declarations share one loading chain.
// TestMergedLoaderLoadsBuiltinAndStoredSkills 用于验证内置和持久化 skill 声明会进入同一条加载链。
func TestMergedLoaderLoadsBuiltinAndStoredSkills(t *testing.T) {
	t.Parallel()

	truthDir := t.TempDir()
	writeSkillFixture(t, truthDir, "default", "user_overview", "User Overview")

	store := NewMemoryStore()
	if err := store.Put(context.Background(), Definition{
		Name:        "uploaded_skill",
		Description: "uploaded description",
		ToolNames:   []string{"lookup_profile"},
		Guidance:    "uploaded guidance",
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	loader := NewLoader([]Source{NewBuiltinSourceWithTruthDir(truthDir)}, store)
	registry, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if _, ok := registry.Get("user_overview"); !ok {
		t.Fatalf("expected builtin skill to be loaded")
	}
	def, ok := registry.Get("uploaded_skill")
	if !ok {
		t.Fatalf("expected uploaded skill to be loaded")
	}
	if def.Guidance != "uploaded guidance" {
		t.Fatalf("unexpected uploaded skill = %#v", def)
	}
}

// TestMergedLoaderAllowsStoreOverride ensures persisted declarations can override builtin ones when names collide.
// TestMergedLoaderAllowsStoreOverride 用于验证持久化 skill 与内置 skill 重名时可以覆盖内置声明。
func TestMergedLoaderAllowsStoreOverride(t *testing.T) {
	t.Parallel()

	source := NewStaticSource("builtin", []Definition{{
		Name:        "override_me",
		Description: "source description",
		ToolNames:   []string{"lookup_profile"},
		Guidance:    "source guidance",
	}})
	store := NewMemoryStore()
	if err := store.Put(context.Background(), Definition{
		Name:        "override_me",
		Description: "store description",
		ToolNames:   []string{"lookup_orders"},
		Guidance:    "store guidance",
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	loader := NewLoader([]Source{source}, store)
	registry, err := loader.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	def, ok := registry.Get("override_me")
	if !ok {
		t.Fatalf("expected overriding skill to exist")
	}
	if def.Description != "store description" {
		t.Fatalf("expected store definition to override source, got %#v", def)
	}
	if len(def.ToolNames) != 1 || def.ToolNames[0] != "lookup_orders" {
		t.Fatalf("unexpected tool names = %#v", def.ToolNames)
	}
}
