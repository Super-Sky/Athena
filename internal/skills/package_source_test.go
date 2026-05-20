package skills

import (
	"context"
	"testing"
)

// TestPackageSourceLoadsUploadedPackages ensures uploaded official skill packages can enter the unified loader chain.
// TestPackageSourceLoadsUploadedPackages 用于验证上传的官方 skill 包可以进入统一加载链。
func TestPackageSourceLoadsUploadedPackages(t *testing.T) {
	t.Parallel()

	store := NewMemoryPackageStore(10)
	if err := store.Put(context.Background(), Package{
		ID:   "pkg-uploaded-checklist",
		Name: "uploaded-checklist",
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-checklist
description: Uploaded checklist skill.
---

# Uploaded checklist

Guide the agent through a support checklist.`),
		},
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	source := NewPackageSource(store, NewPackageAdapter())
	defs, err := source.Load(context.Background())
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(defs) != 1 {
		t.Fatalf("defs len = %d, want 1", len(defs))
	}
	if defs[0].Name != "uploaded-checklist" {
		t.Fatalf("unexpected skill name = %q", defs[0].Name)
	}
	if defs[0].Description != "Uploaded checklist skill." {
		t.Fatalf("unexpected description = %q", defs[0].Description)
	}
	if defs[0].Guidance == "" {
		t.Fatalf("expected uploaded package guidance to be extracted")
	}
}

// TestDefaultPackageAdapterRequiresSkillMarkdown ensures uploaded packages must preserve official skill package shape.
// TestDefaultPackageAdapterRequiresSkillMarkdown 用于验证上传包必须保留官方 skill 包形态中的 SKILL.md。
func TestDefaultPackageAdapterRequiresSkillMarkdown(t *testing.T) {
	t.Parallel()

	_, err := NewPackageAdapter().AdaptPackage(Package{
		Name: "broken-skill",
		Files: map[string][]byte{
			"README.md": []byte("missing skill"),
		},
	})
	if err == nil {
		t.Fatalf("expected missing SKILL.md error")
	}
}
