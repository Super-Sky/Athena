// package.go defines uploaded skill package structures, revisions, and governance helpers.
// package.go 定义 uploaded skill package 结构、版本和治理辅助逻辑。
package skills

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Package stores one uploaded official skill package in its runtime file-bundle form.
// Package 保存一份上传后的官方 skill 包，并保留其运行时文件包形态。
type Package struct {
	ID         string
	Name       string
	Files      map[string][]byte
	Revision   int
	Enabled    bool
	Validation ValidationResult
	UploadedAt time.Time
}

// ValidationResult summarizes package governance checks without exposing file contents.
// ValidationResult 用于汇总 package 治理校验结果，但不暴露文件内容。
type ValidationResult struct {
	Valid    bool     `json:"valid"`
	Errors   []string `json:"errors,omitempty"`
	Warnings []string `json:"warnings,omitempty"`
}

// PackageMetadata describes one stored official skill package without exposing file contents.
// PackageMetadata 描述一份已保存的官方 skill 包元信息，但不暴露文件内容。
type PackageMetadata struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Revision   int              `json:"revision"`
	FileCount  int              `json:"file_count"`
	FilePaths  []string         `json:"file_paths,omitempty"`
	Enabled    bool             `json:"enabled"`
	Validation ValidationResult `json:"validation"`
	UploadedAt time.Time        `json:"uploaded_at,omitempty"`
}

// PackageStore persists uploaded official skill packages in their original file-bundle form.
// PackageStore 负责以原始文件包形态持久化上传的官方 skill 包。
type PackageStore interface {
	List(context.Context) ([]PackageMetadata, error)
	Get(context.Context, string) (Package, bool, error)
	ListRevisions(context.Context, string) ([]PackageMetadata, error)
	GetRevision(context.Context, string, int) (Package, bool, error)
	Put(context.Context, Package) error
	Rollback(context.Context, string, int) (PackageMetadata, error)
	Delete(context.Context, string) error
	SetEnabled(context.Context, string, bool) (PackageMetadata, error)
}

// MemoryPackageStore keeps uploaded official skill packages in memory.
// MemoryPackageStore 会在内存中保存上传的官方 skill 包。
type MemoryPackageStore struct {
	mu            sync.Mutex
	packages      map[string]Package
	nameToID      map[string]string
	revisions     map[string][]Package
	revisionLimit int
}

// NewMemoryPackageStore creates an in-memory package store for uploaded skills.
// NewMemoryPackageStore 创建一个用于上传 skill 的内存包存储。
func NewMemoryPackageStore(revisionLimit int) PackageStore {
	if revisionLimit <= 0 {
		revisionLimit = 10
	}
	return &MemoryPackageStore{
		packages:      make(map[string]Package),
		nameToID:      make(map[string]string),
		revisions:     make(map[string][]Package),
		revisionLimit: revisionLimit,
	}
}

// List returns all stored package metadata in stable name order.
// List 会按稳定名称顺序返回全部已保存包的元信息。
func (s *MemoryPackageStore) List(context.Context) ([]PackageMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	ids := make([]string, 0, len(s.packages))
	for id := range s.packages {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		left := s.packages[ids[i]]
		right := s.packages[ids[j]]
		if left.Name == right.Name {
			return left.ID < right.ID
		}
		return left.Name < right.Name
	})

	result := make([]PackageMetadata, 0, len(ids))
	for _, id := range ids {
		pkg := s.packages[id]
		filePaths := make([]string, 0, len(pkg.Files))
		for path := range pkg.Files {
			filePaths = append(filePaths, path)
		}
		sort.Strings(filePaths)
		result = append(result, PackageMetadata{
			ID:         pkg.ID,
			Name:       pkg.Name,
			Revision:   pkg.Revision,
			FileCount:  len(pkg.Files),
			FilePaths:  filePaths,
			Enabled:    pkg.Enabled,
			Validation: cloneValidation(pkg.Validation),
			UploadedAt: pkg.UploadedAt,
		})
	}
	return result, nil
}

// Get returns one stored package by id.
// Get 会按 id 返回一份已保存的 skill 包。
func (s *MemoryPackageStore) Get(_ context.Context, id string) (Package, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg, ok := s.packages[id]
	if !ok {
		return Package{}, false, nil
	}
	return clonePackage(pkg), true, nil
}

// ListRevisions returns all saved revisions for one uploaded skill package.
// ListRevisions 会返回一份 uploaded skill package 的全部已保存版本。
func (s *MemoryPackageStore) ListRevisions(_ context.Context, id string) ([]PackageMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	items := s.revisions[id]
	result := make([]PackageMetadata, 0, len(items))
	for i := len(items) - 1; i >= 0; i-- {
		pkg := items[i]
		result = append(result, metadataFromPackage(pkg))
	}
	return result, nil
}

// GetRevision returns one saved revision snapshot for an uploaded skill package.
// GetRevision 会返回一份 uploaded skill package 的指定版本快照。
func (s *MemoryPackageStore) GetRevision(_ context.Context, id string, revision int) (Package, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, pkg := range s.revisions[id] {
		if pkg.Revision == revision {
			return clonePackage(pkg), true, nil
		}
	}
	return Package{}, false, nil
}

// Put creates or replaces one stored package.
// Put 会创建或替换一份已保存的 skill 包。
func (s *MemoryPackageStore) Put(_ context.Context, pkg Package) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if pkg.UploadedAt.IsZero() {
		pkg.UploadedAt = time.Now()
	}
	if !pkg.Validation.Valid && len(pkg.Validation.Errors) == 0 && len(pkg.Validation.Warnings) == 0 {
		pkg.Validation.Valid = true
	}
	if !pkg.Validation.Valid {
		pkg.Enabled = false
	} else {
		pkg.Enabled = true
	}
	if existingID, ok := s.nameToID[pkg.Name]; ok && existingID != pkg.ID {
		return fmt.Errorf("skill package name %q already exists", pkg.Name)
	}
	if pkg.ID == "" {
		return fmt.Errorf("skill package id is required")
	}
	pkg.Revision = len(s.revisions[pkg.ID]) + 1
	if existing, ok := s.packages[pkg.ID]; ok && existing.Name != pkg.Name {
		delete(s.nameToID, existing.Name)
	}
	cloned := clonePackage(pkg)
	s.packages[pkg.ID] = cloned
	s.nameToID[pkg.Name] = pkg.ID
	s.revisions[pkg.ID] = append(s.revisions[pkg.ID], cloned)
	s.pruneRevisionsLocked(pkg.ID)
	return nil
}

// Rollback restores one historical revision as the latest active revision.
// Rollback 会把一个历史版本恢复为最新生效版本。
func (s *MemoryPackageStore) Rollback(_ context.Context, id string, revision int) (PackageMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	versions := s.revisions[id]
	if len(versions) == 0 {
		return PackageMetadata{}, fmt.Errorf("skill package %q not found", id)
	}

	var target Package
	found := false
	for _, item := range versions {
		if item.Revision == revision {
			target = clonePackage(item)
			found = true
			break
		}
	}
	if !found {
		return PackageMetadata{}, fmt.Errorf("skill package %q revision %d not found", id, revision)
	}

	if existingID, ok := s.nameToID[target.Name]; ok && existingID != id {
		return PackageMetadata{}, fmt.Errorf("skill package name %q already exists", target.Name)
	}

	target.ID = id
	target.Revision = len(versions) + 1
	target.UploadedAt = time.Now()
	cloned := clonePackage(target)
	s.packages[id] = cloned
	s.nameToID[target.Name] = id
	s.revisions[id] = append(s.revisions[id], cloned)
	s.pruneRevisionsLocked(id)
	return metadataFromPackage(cloned), nil
}

// Delete removes one stored package by id.
// Delete 会按 id 删除一份已保存的 skill 包。
func (s *MemoryPackageStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.packages[id]; ok {
		delete(s.nameToID, existing.Name)
	}
	delete(s.packages, id)
	delete(s.revisions, id)
	return nil
}

// SetEnabled toggles one stored package without changing its original file bundle.
// SetEnabled 会在不修改原始文件包的前提下切换一份已存 package 的启用状态。
func (s *MemoryPackageStore) SetEnabled(_ context.Context, id string, enabled bool) (PackageMetadata, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	pkg, ok := s.packages[id]
	if !ok {
		return PackageMetadata{}, fmt.Errorf("skill package %q not found", id)
	}
	if enabled && !pkg.Validation.Valid {
		return PackageMetadata{}, fmt.Errorf("skill package %q cannot be enabled until validation errors are resolved", id)
	}
	pkg.Enabled = enabled
	s.packages[id] = clonePackage(pkg)
	return metadataFromPackage(pkg), nil
}

func clonePackage(pkg Package) Package {
	clonedFiles := make(map[string][]byte, len(pkg.Files))
	for path, payload := range pkg.Files {
		clonedFiles[path] = append([]byte(nil), payload...)
	}
	return Package{
		ID:         pkg.ID,
		Name:       pkg.Name,
		Files:      clonedFiles,
		Revision:   pkg.Revision,
		Enabled:    pkg.Enabled,
		Validation: cloneValidation(pkg.Validation),
		UploadedAt: pkg.UploadedAt,
	}
}

func metadataFromPackage(pkg Package) PackageMetadata {
	return PackageMetadata{
		ID:         pkg.ID,
		Name:       pkg.Name,
		Revision:   pkg.Revision,
		FileCount:  len(pkg.Files),
		FilePaths:  sortedFilePaths(pkg.Files),
		Enabled:    pkg.Enabled,
		Validation: cloneValidation(pkg.Validation),
		UploadedAt: pkg.UploadedAt,
	}
}

func (s *MemoryPackageStore) pruneRevisionsLocked(id string) {
	if s.revisionLimit <= 0 {
		return
	}
	items := s.revisions[id]
	if len(items) <= s.revisionLimit {
		return
	}
	s.revisions[id] = append([]Package(nil), items[len(items)-s.revisionLimit:]...)
}

func cloneValidation(validation ValidationResult) ValidationResult {
	return ValidationResult{
		Valid:    validation.Valid,
		Errors:   append([]string(nil), validation.Errors...),
		Warnings: append([]string(nil), validation.Warnings...),
	}
}

func sortedFilePaths(files map[string][]byte) []string {
	paths := make([]string, 0, len(files))
	for path := range files {
		paths = append(paths, path)
	}
	sort.Strings(paths)
	return paths
}
