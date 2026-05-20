package app

import (
	"context"
	"testing"

	"moss/internal/config"
	"moss/internal/observability"
	"moss/internal/skills"
)

func TestPutSkillPackageRefreshesVisibleSkills(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-checklist
description: Uploaded checklist skill.
---

# Uploaded checklist

Guide the agent through a support checklist.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}
	if metadata.Name != "uploaded-checklist" {
		t.Fatalf("metadata name = %q, want uploaded-checklist", metadata.Name)
	}

	items, err := svc.ListVisibleSkills(context.Background())
	if err != nil {
		t.Fatalf("ListVisibleSkills() error = %v", err)
	}
	found := false
	for _, item := range items {
		if item.Name == "uploaded-checklist" {
			found = true
			if item.Source != "uploaded" {
				t.Fatalf("source = %q, want uploaded", item.Source)
			}
		}
	}
	if !found {
		t.Fatalf("expected uploaded skill to be visible")
	}
}

func TestPutSkillPackageRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	first := skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-duplicate
description: Uploaded duplicate skill.
---

# Uploaded duplicate

Guide the agent through duplicate coverage.`),
		},
	}
	if _, err := svc.PutSkillPackage(context.Background(), first); err != nil {
		t.Fatalf("first PutSkillPackage() error = %v", err)
	}

	second := skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-duplicate
description: Uploaded duplicate skill.
---

# Uploaded duplicate

Guide the agent through duplicate coverage again.`),
		},
	}
	if _, err := svc.PutSkillPackage(context.Background(), second); err == nil {
		t.Fatalf("expected duplicate package name error")
	}
}

func TestDeleteSkillPackageRefreshesVisibleSkills(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-delete
description: Uploaded delete skill.
---

# Uploaded delete

Guide the agent through deletion coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}

	if err := svc.DeleteSkillPackage(context.Background(), metadata.ID); err != nil {
		t.Fatalf("DeleteSkillPackage() error = %v", err)
	}

	packages, err := svc.ListSkillPackages(context.Background())
	if err != nil {
		t.Fatalf("ListSkillPackages() error = %v", err)
	}
	if len(packages) != 0 {
		t.Fatalf("packages len = %d, want 0", len(packages))
	}
}

func TestSetSkillPackageEnabledRemovesVisibleSkillWhenDisabled(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-toggle
description: Uploaded toggle skill.
---

# Uploaded toggle

Guide the agent through toggle coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}

	if _, err := svc.SetSkillPackageEnabled(context.Background(), metadata.ID, false); err != nil {
		t.Fatalf("SetSkillPackageEnabled() error = %v", err)
	}

	items, err := svc.ListVisibleSkills(context.Background())
	if err != nil {
		t.Fatalf("ListVisibleSkills() error = %v", err)
	}
	for _, item := range items {
		if item.Name == "uploaded-toggle" {
			t.Fatalf("expected disabled uploaded skill to disappear from visible registry")
		}
	}
}

// TestSkillPackageLifecycleSeparatesRegisteredAndVisible verifies governance metadata can remain registered while visibility tracks enabled state.
// TestSkillPackageLifecycleSeparatesRegisteredAndVisible 用于验证治理元数据可以继续保留注册态，而运行时可见性会随 enabled 状态变化。
func TestSkillPackageLifecycleSeparatesRegisteredAndVisible(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-lifecycle
description: Uploaded lifecycle skill.
---

# Uploaded lifecycle

Guide the agent through lifecycle coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}

	if _, err := svc.SetSkillPackageEnabled(context.Background(), metadata.ID, false); err != nil {
		t.Fatalf("SetSkillPackageEnabled(false) error = %v", err)
	}

	packages, err := svc.ListSkillPackages(context.Background())
	if err != nil {
		t.Fatalf("ListSkillPackages() error = %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("packages len = %d, want 1", len(packages))
	}
	if packages[0].ID != metadata.ID || packages[0].Enabled {
		t.Fatalf("unexpected disabled package metadata = %#v", packages[0])
	}

	items, err := svc.ListVisibleSkills(context.Background())
	if err != nil {
		t.Fatalf("ListVisibleSkills() error = %v", err)
	}
	for _, item := range items {
		if item.Name == "uploaded-lifecycle" {
			t.Fatalf("expected disabled package to stay registered but disappear from visible skills")
		}
	}

	if _, err := svc.SetSkillPackageEnabled(context.Background(), metadata.ID, true); err != nil {
		t.Fatalf("SetSkillPackageEnabled(true) error = %v", err)
	}

	items, err = svc.ListVisibleSkills(context.Background())
	if err != nil {
		t.Fatalf("ListVisibleSkills() error = %v", err)
	}
	found := false
	for _, item := range items {
		if item.Name == "uploaded-lifecycle" {
			found = true
			if item.Source != "uploaded" {
				t.Fatalf("source = %q, want uploaded", item.Source)
			}
		}
	}
	if !found {
		t.Fatalf("expected re-enabled package to become visible again")
	}
}

func TestReplaceSkillPackagePreservesDisabledStateWhenValidationPasses(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-replace
description: Uploaded replace skill.
---

# Uploaded replace

Guide the agent through replace coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}
	if _, err := svc.SetSkillPackageEnabled(context.Background(), metadata.ID, false); err != nil {
		t.Fatalf("SetSkillPackageEnabled() error = %v", err)
	}

	replaced, err := svc.ReplaceSkillPackage(context.Background(), metadata.ID, skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-replace
description: Uploaded replace skill v2.
---

# Uploaded replace

Guide the agent through replace coverage v2.`),
		},
	})
	if err != nil {
		t.Fatalf("ReplaceSkillPackage() error = %v", err)
	}
	if replaced.Enabled {
		t.Fatalf("expected disabled state to be preserved after replace")
	}
	if replaced.Revision != 2 {
		t.Fatalf("replaced.Revision = %d, want 2", replaced.Revision)
	}
}

// TestListSkillPackageRevisionsReturnsNewestFirst verifies revision history is exposed in reverse chronological order.
// TestListSkillPackageRevisionsReturnsNewestFirst 用于验证版本历史会按从新到旧顺序暴露。
func TestListSkillPackageRevisionsReturnsNewestFirst(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-history
description: Uploaded history skill.
---

# Uploaded history

Guide the agent through history coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}
	if _, err := svc.ReplaceSkillPackage(context.Background(), metadata.ID, skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-history
description: Uploaded history skill v2.
---

# Uploaded history

Guide the agent through history coverage v2.`),
		},
	}); err != nil {
		t.Fatalf("ReplaceSkillPackage() error = %v", err)
	}

	revisions, err := svc.ListSkillPackageRevisions(context.Background(), metadata.ID)
	if err != nil {
		t.Fatalf("ListSkillPackageRevisions() error = %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions len = %d, want 2", len(revisions))
	}
	if revisions[0].Revision != 2 || revisions[1].Revision != 1 {
		t.Fatalf("unexpected revisions order = %+v", revisions)
	}
}

// TestRollbackSkillPackageRestoresHistoricalRevision verifies rollback creates a fresh current revision from history.
// TestRollbackSkillPackageRestoresHistoricalRevision 用于验证回滚会从历史版本恢复出新的当前版本。
func TestRollbackSkillPackageRestoresHistoricalRevision(t *testing.T) {
	t.Parallel()

	svc := NewService(config.Config{})
	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-rollback
description: Uploaded rollback skill.
---

# Uploaded rollback

Guide the agent through rollback coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}
	if _, err := svc.ReplaceSkillPackage(context.Background(), metadata.ID, skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-rollback
description: Uploaded rollback skill v2.
---

# Uploaded rollback

Guide the agent through rollback coverage v2.`),
		},
	}); err != nil {
		t.Fatalf("ReplaceSkillPackage() error = %v", err)
	}

	result, err := svc.RollbackSkillPackage(context.Background(), metadata.ID, 1)
	if err != nil {
		t.Fatalf("RollbackSkillPackage() error = %v", err)
	}
	if result.RolledBackFrom != 1 {
		t.Fatalf("RolledBackFrom = %d, want 1", result.RolledBackFrom)
	}
	if result.CurrentRevision != 3 {
		t.Fatalf("CurrentRevision = %d, want 3", result.CurrentRevision)
	}

	packages, err := svc.ListSkillPackages(context.Background())
	if err != nil {
		t.Fatalf("ListSkillPackages() error = %v", err)
	}
	if len(packages) != 1 {
		t.Fatalf("packages len = %d, want 1", len(packages))
	}
	if packages[0].Revision != 3 {
		t.Fatalf("packages[0].Revision = %d, want 3", packages[0].Revision)
	}
}

func TestSkillPackageAuditsAreRecorded(t *testing.T) {
	t.Parallel()

	obs := observability.NewDefaultManager()
	svc := NewServiceWithObservability(config.Config{}, obs)

	metadata, err := svc.PutSkillPackage(context.Background(), skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-audit
description: Uploaded audit skill.
---

# Uploaded audit

Guide the agent through audit coverage.`),
		},
	})
	if err != nil {
		t.Fatalf("PutSkillPackage() error = %v", err)
	}
	if _, err := svc.SetSkillPackageEnabled(context.Background(), metadata.ID, false); err != nil {
		t.Fatalf("SetSkillPackageEnabled() error = %v", err)
	}
	if _, err := svc.ReplaceSkillPackage(context.Background(), metadata.ID, skills.Package{
		Files: map[string][]byte{
			"SKILL.md": []byte(`---
name: uploaded-audit
description: Uploaded audit skill v2.
---

# Uploaded audit

Guide the agent through audit coverage v2.`),
		},
	}); err != nil {
		t.Fatalf("ReplaceSkillPackage() error = %v", err)
	}
	if _, err := svc.RollbackSkillPackage(context.Background(), metadata.ID, 1); err != nil {
		t.Fatalf("RollbackSkillPackage() error = %v", err)
	}
	if err := svc.DeleteSkillPackage(context.Background(), metadata.ID); err != nil {
		t.Fatalf("DeleteSkillPackage() error = %v", err)
	}

	audits := obs.SnapshotAudits()
	if len(audits) < 5 {
		t.Fatalf("expected at least 5 skill package audits, got %d", len(audits))
	}
}
