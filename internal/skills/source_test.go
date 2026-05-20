package skills

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestBuiltinSourceLoadsEmbeddedDefinitions ensures built-in skills are loaded from embedded assets.
// TestBuiltinSourceLoadsEmbeddedDefinitions 用于验证内置 skill 会从嵌入式资产中加载。
func TestBuiltinSourceLoadsEmbeddedDefinitions(t *testing.T) {
	t.Parallel()

	truthDir := t.TempDir()
	writeSkillFixture(t, truthDir, "default", "user_overview", "User Overview")
	writeSkillFixture(t, truthDir, "security_review", "cso_review", "CSO Review")

	source := NewBuiltinSourceWithTruthDir(truthDir)
	defs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(defs) == 0 {
		t.Fatalf("expected at least one embedded builtin skill")
	}
	names := map[string]bool{}
	for _, def := range defs {
		names[def.Name] = true
	}
	for _, want := range []string{"user_overview", "cso_review"} {
		if !names[want] {
			t.Fatalf("builtin skills = %#v, missing %s", defs, want)
		}
	}
}

func writeSkillFixture(t *testing.T, truthDir, sceneID, skillID, name string) {
	t.Helper()

	dir := filepath.Join(truthDir, "sources", "scenes", sceneID, "skills", skillID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", dir, err)
	}
	payload := `---
id: ` + skillID + `
name: ` + name + `
summary: test skill
description: fixture description
scene: ` + sceneID + `
allowed_tools:
  - fixture_tool
---

## When to Use
Use this fixture skill.

## Process
Run the fixture process.

## Output
Return the fixture output.
`
	if err := os.WriteFile(filepath.Join(dir, "SKILL.md"), []byte(payload), 0o644); err != nil {
		t.Fatalf("WriteFile(SKILL.md) error = %v", err)
	}
}
