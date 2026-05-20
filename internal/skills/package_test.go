package skills

import (
	"context"
	"testing"
)

// TestMemoryPackageStoreKeepsOfficialSkillPackages verifies uploaded skill packages stay in file-bundle form.
// TestMemoryPackageStoreKeepsOfficialSkillPackages 用于验证上传 skill 包会以文件包形态保存在 store 中。
func TestMemoryPackageStoreKeepsOfficialSkillPackages(t *testing.T) {
	t.Parallel()

	store := NewMemoryPackageStore(10)
	if err := store.Put(context.Background(), Package{
		ID:   "pkg-1",
		Name: "support-checklist",
		Files: map[string][]byte{
			"SKILL.md":           []byte("---\nname: support-checklist\n---"),
			"scripts/check.sh":   []byte("#!/bin/sh\necho ok"),
			"references/faq.txt": []byte("faq"),
		},
	}); err != nil {
		t.Fatalf("Put() error = %v", err)
	}

	metadata, err := store.List(context.Background())
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(metadata) != 1 {
		t.Fatalf("metadata len = %d, want 1", len(metadata))
	}
	if metadata[0].FileCount != 3 {
		t.Fatalf("file count = %d, want 3", metadata[0].FileCount)
	}

	pkg, ok, err := store.Get(context.Background(), "pkg-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected package to exist")
	}
	if _, ok := pkg.Files["SKILL.md"]; !ok {
		t.Fatalf("expected SKILL.md to be retained in uploaded package")
	}
}

// TestMemoryPackageStoreRejectsDuplicateName verifies uploaded package names stay unique across ids.
// TestMemoryPackageStoreRejectsDuplicateName 用于验证 uploaded package 名称在不同 id 间仍保持唯一。
func TestMemoryPackageStoreRejectsDuplicateName(t *testing.T) {
	t.Parallel()

	store := NewMemoryPackageStore(10)
	if err := store.Put(context.Background(), Package{
		ID:   "pkg-1",
		Name: "duplicate",
		Files: map[string][]byte{
			"SKILL.md": []byte("# duplicate"),
		},
	}); err != nil {
		t.Fatalf("first Put() error = %v", err)
	}

	if err := store.Put(context.Background(), Package{
		ID:   "pkg-2",
		Name: "duplicate",
		Files: map[string][]byte{
			"SKILL.md": []byte("# duplicate"),
		},
	}); err == nil {
		t.Fatalf("expected duplicate package name error")
	}
}

// TestMemoryPackageStoreTracksRevisions verifies each replace appends a new immutable revision.
// TestMemoryPackageStoreTracksRevisions 用于验证每次替换都会追加新的不可变版本。
func TestMemoryPackageStoreTracksRevisions(t *testing.T) {
	t.Parallel()

	store := NewMemoryPackageStore(10)
	ctx := context.Background()
	if err := store.Put(ctx, Package{
		ID:   "pkg-1",
		Name: "revisioned",
		Files: map[string][]byte{
			"SKILL.md": []byte("# v1"),
		},
	}); err != nil {
		t.Fatalf("first Put() error = %v", err)
	}
	if err := store.Put(ctx, Package{
		ID:   "pkg-1",
		Name: "revisioned",
		Files: map[string][]byte{
			"SKILL.md": []byte("# v2"),
		},
	}); err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	revisions, err := store.ListRevisions(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions len = %d, want 2", len(revisions))
	}
	if revisions[0].Revision != 2 || revisions[1].Revision != 1 {
		t.Fatalf("unexpected revisions order = %+v", revisions)
	}
}

// TestMemoryPackageStoreRollbackCreatesNewRevision verifies rollback creates a new head revision from history.
// TestMemoryPackageStoreRollbackCreatesNewRevision 用于验证回滚会基于历史版本生成新的头部版本。
func TestMemoryPackageStoreRollbackCreatesNewRevision(t *testing.T) {
	t.Parallel()

	store := NewMemoryPackageStore(10)
	ctx := context.Background()
	if err := store.Put(ctx, Package{
		ID:   "pkg-1",
		Name: "rollbackable",
		Files: map[string][]byte{
			"SKILL.md": []byte("# v1"),
		},
	}); err != nil {
		t.Fatalf("first Put() error = %v", err)
	}
	if err := store.Put(ctx, Package{
		ID:   "pkg-1",
		Name: "rollbackable",
		Files: map[string][]byte{
			"SKILL.md": []byte("# v2"),
		},
	}); err != nil {
		t.Fatalf("second Put() error = %v", err)
	}

	metadata, err := store.Rollback(ctx, "pkg-1", 1)
	if err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if metadata.Revision != 3 {
		t.Fatalf("metadata.Revision = %d, want 3", metadata.Revision)
	}

	current, ok, err := store.Get(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if !ok {
		t.Fatalf("expected current package")
	}
	if string(current.Files["SKILL.md"]) != "# v1" {
		t.Fatalf("current SKILL.md = %q, want v1 snapshot", string(current.Files["SKILL.md"]))
	}
}

// TestMemoryPackageStorePrunesOldRevisions verifies the store keeps only the configured newest revisions.
// TestMemoryPackageStorePrunesOldRevisions 用于验证 store 只保留配置允许的最新版本数量。
func TestMemoryPackageStorePrunesOldRevisions(t *testing.T) {
	t.Parallel()

	store := NewMemoryPackageStore(2)
	ctx := context.Background()
	for i, body := range []string{"# v1", "# v2", "# v3"} {
		if err := store.Put(ctx, Package{
			ID:   "pkg-1",
			Name: "pruned",
			Files: map[string][]byte{
				"SKILL.md": []byte(body),
			},
		}); err != nil {
			t.Fatalf("Put(%d) error = %v", i+1, err)
		}
	}

	revisions, err := store.ListRevisions(ctx, "pkg-1")
	if err != nil {
		t.Fatalf("ListRevisions() error = %v", err)
	}
	if len(revisions) != 2 {
		t.Fatalf("revisions len = %d, want 2", len(revisions))
	}
	if revisions[0].Revision != 3 || revisions[1].Revision != 2 {
		t.Fatalf("unexpected remaining revisions = %+v", revisions)
	}
	if _, ok, err := store.GetRevision(ctx, "pkg-1", 1); err != nil {
		t.Fatalf("GetRevision() error = %v", err)
	} else if ok {
		t.Fatalf("expected pruned revision 1 to be unavailable")
	}
}
