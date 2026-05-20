// skills.go implements the app-layer skill visibility and uploaded package governance use cases.
// skills.go 实现 app 层 skill 可见性和上传 package 治理用例。
package app

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"moss/internal/observability"
	"moss/internal/runtime"
	"moss/internal/skills"
)

// SkillView describes one visible skill plus its current source classification.
// SkillView 描述一条当前可见的 skill，以及它当前的来源分类。
type SkillView struct {
	Name        string   `json:"name"`
	Source      string   `json:"source"`
	Description string   `json:"description"`
	ToolNames   []string `json:"tool_names,omitempty"`
	Guidance    string   `json:"guidance,omitempty"`
}

// SkillPackageRollbackResult describes one rollback operation and its source revision.
// SkillPackageRollbackResult 描述一次 package 回滚结果以及它所基于的源版本。
type SkillPackageRollbackResult struct {
	Metadata        skills.PackageMetadata `json:"metadata"`
	RolledBackFrom  int                    `json:"rolled_back_from"`
	CurrentRevision int                    `json:"current_revision"`
}

type SkillPackageDetail struct {
	Metadata skills.PackageMetadata `json:"metadata"`
	Files    map[string]string      `json:"files"`
}

// ListVisibleSkills returns the currently visible skill set after the unified loader chain is applied.
// ListVisibleSkills 会在统一 loader 链生效后返回当前可见的 skill 集合。
func (s *Service) ListVisibleSkills(ctx context.Context) ([]SkillView, error) {
	registry, err := effectiveSkillRegistry(s.ControlPlane, s.SkillLoader)
	if err != nil {
		return nil, err
	}

	sourceByName, err := s.resolveSkillSources(ctx)
	if err != nil {
		return nil, err
	}

	defs := registry.List()
	result := make([]SkillView, 0, len(defs))
	for _, def := range defs {
		result = append(result, SkillView{
			Name:        def.Name,
			Source:      sourceByName[def.Name],
			Description: def.Description,
			ToolNames:   append([]string(nil), def.ToolNames...),
			Guidance:    def.Guidance,
		})
	}
	return result, nil
}

// ListSkillPackages returns metadata for all uploaded official skill packages.
// ListSkillPackages 会返回全部上传官方 skill 包的元信息。
func (s *Service) ListSkillPackages(ctx context.Context) ([]skills.PackageMetadata, error) {
	if s.PackageStore == nil {
		return nil, nil
	}
	return s.PackageStore.List(ctx)
}

// ListSkillPackageRevisions returns all saved revisions for one uploaded skill package.
// ListSkillPackageRevisions 会返回一份 uploaded skill package 的全部历史版本。
func (s *Service) ListSkillPackageRevisions(ctx context.Context, id string) ([]skills.PackageMetadata, error) {
	if s.PackageStore == nil {
		return nil, nil
	}
	return s.PackageStore.ListRevisions(ctx, strings.TrimSpace(id))
}

func (s *Service) GetSkillPackage(ctx context.Context, id string) (SkillPackageDetail, bool, error) {
	if s.PackageStore == nil {
		return SkillPackageDetail{}, false, nil
	}
	pkg, ok, err := s.PackageStore.Get(ctx, strings.TrimSpace(id))
	if err != nil || !ok {
		return SkillPackageDetail{}, ok, err
	}
	files := make(map[string]string, len(pkg.Files))
	filePaths := make([]string, 0, len(pkg.Files))
	for path, content := range pkg.Files {
		files[path] = string(content)
		filePaths = append(filePaths, path)
	}
	sort.Strings(filePaths)
	return SkillPackageDetail{
		Metadata: skills.PackageMetadata{
			ID:         pkg.ID,
			Name:       pkg.Name,
			Revision:   pkg.Revision,
			FileCount:  len(pkg.Files),
			FilePaths:  filePaths,
			Enabled:    pkg.Enabled,
			Validation: pkg.Validation,
			UploadedAt: pkg.UploadedAt,
		},
		Files: files,
	}, true, nil
}

// PutSkillPackage stores one uploaded official skill package and refreshes the active registry view.
// PutSkillPackage 会保存一份上传官方 skill 包，并刷新当前生效的 registry 视图。
func (s *Service) PutSkillPackage(ctx context.Context, pkg skills.Package) (skills.PackageMetadata, error) {
	if s.PackageStore == nil {
		return skills.PackageMetadata{}, fmt.Errorf("skill package store is not configured")
	}

	adapter := skills.NewPackageAdapter()
	pkg.Validation = skills.ValidatePackage(pkg, adapter)
	def, err := adapter.AdaptPackage(pkg)
	if err != nil && !pkg.Validation.Valid {
		return skills.PackageMetadata{}, err
	}
	pkg.Name = strings.TrimSpace(def.Name)
	if pkg.Name == "" {
		return skills.PackageMetadata{}, fmt.Errorf("uploaded skill package is missing a skill name")
	}
	packages, err := s.ListSkillPackages(ctx)
	if err != nil {
		return skills.PackageMetadata{}, err
	}
	for _, item := range packages {
		if item.Name == pkg.Name {
			return skills.PackageMetadata{}, fmt.Errorf("skill package name %q already exists", pkg.Name)
		}
	}
	pkg.ID = newPackageID()
	pkg.Enabled = pkg.Validation.Valid
	if err := s.PackageStore.Put(ctx, pkg); err != nil {
		return skills.PackageMetadata{}, err
	}
	if s.SkillStore != nil {
		if err := s.SkillStore.Delete(ctx, pkg.Name); err != nil {
			return skills.PackageMetadata{}, err
		}
	}
	if err := s.reloadSkillRegistry(ctx); err != nil {
		return skills.PackageMetadata{}, err
	}

	items, err := s.PackageStore.List(ctx)
	if err != nil {
		return skills.PackageMetadata{}, err
	}
	for _, item := range items {
		if item.ID == pkg.ID {
			s.recordSkillPackageAudit(ctx, "skill_package_created", item)
			return item, nil
		}
	}
	metadata := skills.PackageMetadata{ID: pkg.ID, Name: pkg.Name}
	s.recordSkillPackageAudit(ctx, "skill_package_created", metadata)
	return metadata, nil
}

// SetSkillPackageEnabled toggles one uploaded package and refreshes the active registry view.
// SetSkillPackageEnabled 会切换一份 uploaded package 的启用状态，并刷新当前生效的 registry。
func (s *Service) SetSkillPackageEnabled(ctx context.Context, id string, enabled bool) (skills.PackageMetadata, error) {
	if s.PackageStore == nil {
		return skills.PackageMetadata{}, fmt.Errorf("skill package store is not configured")
	}
	metadata, err := s.PackageStore.SetEnabled(ctx, strings.TrimSpace(id), enabled)
	if err != nil {
		return skills.PackageMetadata{}, err
	}
	if err := s.reloadSkillRegistry(ctx); err != nil {
		return skills.PackageMetadata{}, err
	}
	s.recordSkillPackageAudit(ctx, "skill_package_toggled", metadata)
	return metadata, nil
}

// DeleteSkillPackage removes one uploaded official skill package and refreshes the active registry view.
// DeleteSkillPackage 会删除一份上传官方 skill 包，并刷新当前生效的 registry 视图。
func (s *Service) DeleteSkillPackage(ctx context.Context, id string) error {
	if s.PackageStore == nil {
		return fmt.Errorf("skill package store is not configured")
	}
	trimmed := strings.TrimSpace(id)
	if trimmed == "" {
		return fmt.Errorf("skill package id is required")
	}
	if err := s.PackageStore.Delete(ctx, trimmed); err != nil {
		return err
	}
	if err := s.reloadSkillRegistry(ctx); err != nil {
		return err
	}
	s.recordSkillPackageAudit(ctx, "skill_package_deleted", skills.PackageMetadata{
		ID: trimmed,
	})
	return nil
}

// ReplaceSkillPackage replaces one uploaded official skill package by id and refreshes the active registry view.
// ReplaceSkillPackage 会按 id 替换一份上传官方 skill 包，并刷新当前生效的 registry 视图。
func (s *Service) ReplaceSkillPackage(ctx context.Context, id string, pkg skills.Package) (skills.PackageMetadata, error) {
	if s.PackageStore == nil {
		return skills.PackageMetadata{}, fmt.Errorf("skill package store is not configured")
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return skills.PackageMetadata{}, fmt.Errorf("skill package id is required")
	}
	existing, ok, err := s.PackageStore.Get(ctx, trimmedID)
	if err != nil {
		return skills.PackageMetadata{}, err
	}
	if !ok {
		return skills.PackageMetadata{}, fmt.Errorf("skill package %q not found", trimmedID)
	}

	adapter := skills.NewPackageAdapter()
	pkg.Validation = skills.ValidatePackage(pkg, adapter)
	def, err := adapter.AdaptPackage(pkg)
	if err != nil && !pkg.Validation.Valid {
		return skills.PackageMetadata{}, err
	}
	pkg.ID = trimmedID
	pkg.Name = strings.TrimSpace(def.Name)
	if pkg.Name == "" {
		return skills.PackageMetadata{}, fmt.Errorf("uploaded skill package is missing a skill name")
	}
	if err := s.ensureSkillPackageNameAvailable(ctx, pkg.Name, trimmedID); err != nil {
		return skills.PackageMetadata{}, err
	}
	if pkg.Validation.Valid {
		pkg.Enabled = existing.Enabled
	} else {
		pkg.Enabled = false
	}
	if err := s.PackageStore.Put(ctx, pkg); err != nil {
		return skills.PackageMetadata{}, err
	}
	if pkg.Validation.Valid && !existing.Enabled {
		if _, err := s.PackageStore.SetEnabled(ctx, pkg.ID, false); err != nil {
			return skills.PackageMetadata{}, err
		}
	}
	if err := s.reloadSkillRegistry(ctx); err != nil {
		return skills.PackageMetadata{}, err
	}
	items, err := s.PackageStore.List(ctx)
	if err != nil {
		return skills.PackageMetadata{}, err
	}
	for _, item := range items {
		if item.ID == pkg.ID {
			s.recordSkillPackageAudit(ctx, "skill_package_replaced", item)
			return item, nil
		}
	}
	metadata := skills.PackageMetadata{ID: pkg.ID, Name: pkg.Name}
	s.recordSkillPackageAudit(ctx, "skill_package_replaced", metadata)
	return metadata, nil
}

// RollbackSkillPackage restores one historical revision as the latest active revision.
// RollbackSkillPackage 会把一个历史版本恢复为最新生效版本。
func (s *Service) RollbackSkillPackage(ctx context.Context, id string, revision int) (SkillPackageRollbackResult, error) {
	if s.PackageStore == nil {
		return SkillPackageRollbackResult{}, fmt.Errorf("skill package store is not configured")
	}
	trimmedID := strings.TrimSpace(id)
	if trimmedID == "" {
		return SkillPackageRollbackResult{}, fmt.Errorf("skill package id is required")
	}
	target, ok, err := s.PackageStore.GetRevision(ctx, trimmedID, revision)
	if err != nil {
		return SkillPackageRollbackResult{}, err
	}
	if !ok {
		return SkillPackageRollbackResult{}, fmt.Errorf("skill package %q revision %d not found", trimmedID, revision)
	}
	if err := s.ensureSkillPackageNameAvailable(ctx, target.Name, trimmedID); err != nil {
		return SkillPackageRollbackResult{}, err
	}
	metadata, err := s.PackageStore.Rollback(ctx, trimmedID, revision)
	if err != nil {
		return SkillPackageRollbackResult{}, err
	}
	if err := s.reloadSkillRegistry(ctx); err != nil {
		return SkillPackageRollbackResult{}, err
	}
	s.recordSkillPackageAudit(ctx, "skill_package_rolled_back", metadata)
	return SkillPackageRollbackResult{
		Metadata:        metadata,
		RolledBackFrom:  revision,
		CurrentRevision: metadata.Revision,
	}, nil
}

func (s *Service) reloadSkillRegistry(ctx context.Context) error {
	registry, err := effectiveSkillRegistry(s.ControlPlane, s.SkillLoader)
	if err != nil {
		return err
	}
	resolver, ok := s.Runtime.CapabilityResolver.(runtime.DefaultCapabilityResolver)
	if !ok {
		return fmt.Errorf("runtime capability resolver does not support registry reload")
	}
	resolver.Registry = *registry
	s.Runtime.CapabilityResolver = resolver
	return nil
}

func (s *Service) resolveSkillSources(ctx context.Context) (map[string]string, error) {
	result := make(map[string]string)

	builtinDefs, err := skills.NewBuiltinSource().Load(ctx)
	if err != nil {
		return nil, err
	}
	for _, def := range builtinDefs {
		result[def.Name] = "builtin"
	}

	if s.PackageStore == nil {
		return result, nil
	}
	uploaded, err := s.PackageStore.List(ctx)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(uploaded))
	for _, item := range uploaded {
		names = append(names, item.Name)
	}
	sort.Strings(names)
	for _, name := range names {
		result[name] = "uploaded"
	}
	return result, nil
}

func newPackageID() string {
	return fmt.Sprintf("spkg-%d", time.Now().UnixNano())
}

func (s *Service) ensureSkillPackageNameAvailable(ctx context.Context, name string, currentID string) error {
	packages, err := s.ListSkillPackages(ctx)
	if err != nil {
		return err
	}
	for _, item := range packages {
		if item.Name == name && item.ID != currentID {
			return fmt.Errorf("skill package name %q already exists", name)
		}
	}
	return nil
}

func (s *Service) recordSkillPackageAudit(ctx context.Context, action string, metadata skills.PackageMetadata) {
	if s == nil || s.Observability == nil {
		return
	}
	s.Observability.Inc("app_skill_package_action_total", map[string]string{
		"action": action,
	})
	s.Observability.RecordAudit(ctx, observability.AuditRecord{
		Action: action,
		Detail: map[string]any{
			"id":         metadata.ID,
			"name":       metadata.Name,
			"revision":   metadata.Revision,
			"enabled":    metadata.Enabled,
			"valid":      metadata.Validation.Valid,
			"file_count": metadata.FileCount,
		},
	})
}
