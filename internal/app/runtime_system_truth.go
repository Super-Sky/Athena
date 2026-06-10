// runtime_system_truth.go coordinates app-layer System Truth lifecycle writes.
// runtime_system_truth.go 负责 app 层 System Truth 生命周期写入编排。
package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"moss/internal/runtime"
)

const (
	defaultSystemTruthLifecycleReadLimit = 50
	maxSystemTruthLifecycleReadLimit     = 200
)

// SystemTruthLifecycleReadQuery filters System Truth lifecycle readout records.
// SystemTruthLifecycleReadQuery 用于筛选 System Truth 生命周期 readout 记录。
type SystemTruthLifecycleReadQuery struct {
	AssetID  string
	SourceID string
	DraftID  string
	Status   string
	Limit    int
}

// SystemTruthLifecycleReadout groups lifecycle records for Control Plane reads.
// SystemTruthLifecycleReadout 汇总供控制面读取的 System Truth 生命周期记录。
type SystemTruthLifecycleReadout struct {
	Sources        []runtime.SystemTruthSource
	Drafts         []runtime.SystemTruthDraft
	CompileResults []runtime.SystemTruthCompileResult
	ActiveVersions []runtime.SystemTruthActiveVersion
}

// CreateSystemTruthSource appends one source record to the System Truth lifecycle.
// CreateSystemTruthSource 向 System Truth 生命周期追加一条 source 记录。
func (s *Service) CreateSystemTruthSource(ctx context.Context, input runtime.SystemTruthSource) (runtime.SystemTruthSource, error) {
	store, err := s.systemTruthLifecycleStore()
	if err != nil {
		return runtime.SystemTruthSource{}, err
	}
	input.AssetID = strings.TrimSpace(input.AssetID)
	input.SourceKind = strings.TrimSpace(input.SourceKind)
	input.SourceRef = strings.TrimSpace(input.SourceRef)
	input.Status = defaultRuntimeReadString(input.Status, runtime.SystemTruthSourceStatusImported)
	input.ContentHash = defaultRuntimeReadString(input.ContentHash, systemTruthContentHash(input.Content))
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if err := runtime.ValidateSystemTruthSource(input); err != nil {
		return runtime.SystemTruthSource{}, err
	}
	return store.CreateSystemTruthSource(ctx, input)
}

// CreateSystemTruthDraft appends one editable draft for a source.
// CreateSystemTruthDraft 为一条 source 追加可编辑 draft。
func (s *Service) CreateSystemTruthDraft(ctx context.Context, input runtime.SystemTruthDraft) (runtime.SystemTruthDraft, error) {
	store, err := s.systemTruthLifecycleStore()
	if err != nil {
		return runtime.SystemTruthDraft{}, err
	}
	source, ok, err := store.GetSystemTruthSource(ctx, input.SourceID)
	if err != nil {
		return runtime.SystemTruthDraft{}, err
	}
	if !ok {
		return runtime.SystemTruthDraft{}, runtimeInvalidInput("system truth source %q not found", input.SourceID)
	}
	input.SourceID = source.ID
	input.AssetID = defaultRuntimeReadString(input.AssetID, source.AssetID)
	if input.AssetID != source.AssetID {
		return runtime.SystemTruthDraft{}, runtimeInvalidInput("system truth draft asset_id must match source asset_id")
	}
	input.Status = defaultRuntimeReadString(input.Status, runtime.SystemTruthDraftStatusDraft)
	now := time.Now().UTC()
	if input.CreatedAt.IsZero() {
		input.CreatedAt = now
	}
	input.UpdatedAt = now
	if err := runtime.ValidateSystemTruthDraft(input); err != nil {
		return runtime.SystemTruthDraft{}, err
	}
	return store.CreateSystemTruthDraft(ctx, input)
}

// CompileSystemTruthDraft appends a deterministic compile result for one draft.
// CompileSystemTruthDraft 为一条 draft 追加确定性的 compile result。
func (s *Service) CompileSystemTruthDraft(ctx context.Context, draftID string, input runtime.SystemTruthCompileResult) (runtime.SystemTruthCompileResult, error) {
	store, err := s.systemTruthLifecycleStore()
	if err != nil {
		return runtime.SystemTruthCompileResult{}, err
	}
	draft, ok, err := store.GetSystemTruthDraft(ctx, draftID)
	if err != nil {
		return runtime.SystemTruthCompileResult{}, err
	}
	if !ok {
		return runtime.SystemTruthCompileResult{}, runtimeInvalidInput("system truth draft %q not found", draftID)
	}
	input.DraftID = draft.ID
	input.AssetID = defaultRuntimeReadString(input.AssetID, draft.AssetID)
	if input.AssetID != draft.AssetID {
		return runtime.SystemTruthCompileResult{}, runtimeInvalidInput("system truth compile asset_id must match draft asset_id")
	}
	input.Status = defaultRuntimeReadString(input.Status, runtime.SystemTruthCompileStatusSucceeded)
	if len(input.CompiledPayload) == 0 && input.Status == runtime.SystemTruthCompileStatusSucceeded {
		input.CompiledPayload = cloneRuntimeAnyMap(draft.Content)
	}
	input.ContentHash = defaultRuntimeReadString(input.ContentHash, systemTruthContentHash(input.CompiledPayload))
	if input.Summary == "" {
		input.Summary = "compiled system truth draft"
	}
	if input.CreatedAt.IsZero() {
		input.CreatedAt = time.Now().UTC()
	}
	if err := runtime.ValidateSystemTruthCompileResult(input); err != nil {
		return runtime.SystemTruthCompileResult{}, err
	}
	return store.CreateSystemTruthCompileResult(ctx, input)
}

// ActivateSystemTruthCompileResult appends an audited active pointer for a successful compile result.
// ActivateSystemTruthCompileResult 为成功 compile result 追加可审计 active pointer。
func (s *Service) ActivateSystemTruthCompileResult(ctx context.Context, compileID string, input runtime.SystemTruthActiveVersion) (runtime.SystemTruthActiveVersion, error) {
	store, err := s.systemTruthLifecycleStore()
	if err != nil {
		return runtime.SystemTruthActiveVersion{}, err
	}
	compiled, ok, err := store.GetSystemTruthCompileResult(ctx, compileID)
	if err != nil {
		return runtime.SystemTruthActiveVersion{}, err
	}
	if !ok {
		return runtime.SystemTruthActiveVersion{}, runtimeInvalidInput("system truth compile result %q not found", compileID)
	}
	if compiled.Status != runtime.SystemTruthCompileStatusSucceeded {
		return runtime.SystemTruthActiveVersion{}, runtimeInvalidInput("system truth compile result %q is not successful", compileID)
	}
	input.CompileResultID = compiled.ID
	input.DraftID = defaultRuntimeReadString(input.DraftID, compiled.DraftID)
	input.AssetID = defaultRuntimeReadString(input.AssetID, compiled.AssetID)
	if input.AssetID != compiled.AssetID || input.DraftID != compiled.DraftID {
		return runtime.SystemTruthActiveVersion{}, runtimeInvalidInput("system truth active pointer must match compile result asset_id and draft_id")
	}
	if input.ActivatedAt.IsZero() {
		input.ActivatedAt = time.Now().UTC()
	}
	if err := runtime.ValidateSystemTruthActiveVersion(input); err != nil {
		return runtime.SystemTruthActiveVersion{}, err
	}
	return store.ActivateSystemTruthVersion(ctx, input)
}

// RollbackSystemTruthActiveVersion appends a new active pointer that restores an earlier active version.
// RollbackSystemTruthActiveVersion 追加一个恢复早期 active version 的 active pointer。
func (s *Service) RollbackSystemTruthActiveVersion(ctx context.Context, activeID string, input runtime.SystemTruthActiveVersion) (runtime.SystemTruthActiveVersion, error) {
	store, err := s.systemTruthLifecycleStore()
	if err != nil {
		return runtime.SystemTruthActiveVersion{}, err
	}
	target, ok, err := store.GetSystemTruthActiveVersion(ctx, activeID)
	if err != nil {
		return runtime.SystemTruthActiveVersion{}, err
	}
	if !ok {
		return runtime.SystemTruthActiveVersion{}, runtimeInvalidInput("system truth active version %q not found", activeID)
	}
	current, ok, err := store.GetActiveSystemTruthVersion(ctx, target.AssetID)
	if err != nil {
		return runtime.SystemTruthActiveVersion{}, err
	}
	if !ok {
		return runtime.SystemTruthActiveVersion{}, runtimeInvalidInput("system truth asset %q has no active pointer", target.AssetID)
	}
	if current.ID == target.ID {
		return runtime.SystemTruthActiveVersion{}, runtimeInvalidInput("system truth active version %q is already current", activeID)
	}
	input.AssetID = target.AssetID
	input.CompileResultID = target.CompileResultID
	input.DraftID = target.DraftID
	input.RollbackFromID = current.ID
	if input.Reason == "" {
		input.Reason = "rollback system truth active pointer"
	}
	return s.ActivateSystemTruthCompileResult(ctx, target.CompileResultID, input)
}

// GetSystemTruthLifecycleReadout returns lifecycle records grouped for Control Plane readout.
// GetSystemTruthLifecycleReadout 返回供控制面 readout 使用的生命周期记录。
func (s *Service) GetSystemTruthLifecycleReadout(ctx context.Context, query SystemTruthLifecycleReadQuery) (SystemTruthLifecycleReadout, error) {
	store, err := s.systemTruthLifecycleStore()
	if err != nil {
		return SystemTruthLifecycleReadout{}, err
	}
	limit := normalizeSystemTruthLifecycleReadLimit(query.Limit)
	sources, err := store.ListSystemTruthSources(ctx, runtime.SystemTruthSourceListFilter{
		AssetID: strings.TrimSpace(query.AssetID),
		Status:  strings.TrimSpace(query.Status),
		Limit:   limit,
	})
	if err != nil {
		return SystemTruthLifecycleReadout{}, err
	}
	drafts, err := store.ListSystemTruthDrafts(ctx, runtime.SystemTruthDraftListFilter{
		SourceID: strings.TrimSpace(query.SourceID),
		AssetID:  strings.TrimSpace(query.AssetID),
		Status:   strings.TrimSpace(query.Status),
		Limit:    limit,
	})
	if err != nil {
		return SystemTruthLifecycleReadout{}, err
	}
	compileResults, err := store.ListSystemTruthCompileResults(ctx, runtime.SystemTruthCompileResultListFilter{
		DraftID: strings.TrimSpace(query.DraftID),
		AssetID: strings.TrimSpace(query.AssetID),
		Status:  strings.TrimSpace(query.Status),
		Limit:   limit,
	})
	if err != nil {
		return SystemTruthLifecycleReadout{}, err
	}
	activeVersions, err := store.ListSystemTruthActiveVersions(ctx, strings.TrimSpace(query.AssetID))
	if err != nil {
		return SystemTruthLifecycleReadout{}, err
	}
	return SystemTruthLifecycleReadout{Sources: sources, Drafts: drafts, CompileResults: compileResults, ActiveVersions: limitSystemTruthActiveVersions(activeVersions, limit)}, nil
}

func (s *Service) systemTruthLifecycleStore() (runtime.SystemTruthLifecycleStore, error) {
	store, err := s.runtimePersistenceStore()
	if err != nil {
		return nil, err
	}
	truthStore, ok := store.(runtime.SystemTruthLifecycleStore)
	if !ok {
		return nil, ErrRuntimeFoundationWriteUnsupported
	}
	return truthStore, nil
}

func normalizeSystemTruthLifecycleReadLimit(limit int) int {
	switch {
	case limit <= 0:
		return defaultSystemTruthLifecycleReadLimit
	case limit > maxSystemTruthLifecycleReadLimit:
		return maxSystemTruthLifecycleReadLimit
	default:
		return limit
	}
}

func runtimeInvalidInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", runtime.ErrInvalidRuntimePersistenceInput, fmt.Sprintf(format, args...))
}

func systemTruthContentHash(value map[string]any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func cloneRuntimeAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	raw, err := json.Marshal(input)
	if err != nil {
		out := make(map[string]any, len(input))
		for key, value := range input {
			out[key] = value
		}
		return out
	}
	var output map[string]any
	if err := json.Unmarshal(raw, &output); err != nil || output == nil {
		return map[string]any{}
	}
	return output
}

func limitSystemTruthActiveVersions(items []runtime.SystemTruthActiveVersion, limit int) []runtime.SystemTruthActiveVersion {
	if limit <= 0 || len(items) <= limit {
		return items
	}
	return items[:limit]
}
