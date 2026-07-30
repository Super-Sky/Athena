package controlplane

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"moss/internal/systemtruth"
)

const (
	systemResourceMetaFile      = "meta.json"
	systemResourceSourceFile    = "source.txt"
	systemResourceParseFile     = "parse_result.json"
	systemResourceCompileFile   = "compile_result.json"
	systemResourcePipelineFile  = "pipeline.json"
	systemResourceVersionsDir   = ".versions"
	systemResourceVersionFile   = ".truth-version"
	systemResourceSourcesDir    = "sources"
	defaultSystemResourceType   = "scene"
	defaultSystemResourceScope  = "system"
	defaultSystemResourceSource = "control_plane_upload"
	truthDirSystemSource        = "truth_dir_source"
	defaultPipelineStatus       = "draft"
)

type storedSystemResource struct {
	SystemResourceSummary
	Metadata      map[string]any `json:"metadata,omitempty"`
	SourcePath    string         `json:"source_path,omitempty"`
	UpdatedByNote string         `json:"updated_by_note,omitempty"`
}

type systemManifest struct {
	TruthDirVersion string `json:"truth_dir_version,omitempty"`
}

type systemResourceVersionSnapshot struct {
	Version  SystemResourceVersionSummary `json:"version"`
	Resource SystemResourceDetail         `json:"resource"`
	Source   SystemResourceSource         `json:"source"`
}

func (m *Manager) TruthDirInfo(ctx context.Context) (TruthDirInfo, error) {
	if m == nil {
		return TruthDirInfo{}, nil
	}
	dir := strings.TrimSpace(m.truthDir)
	if dir == "" {
		return TruthDirInfo{}, nil
	}
	version, err := m.currentTruthDirVersion(ctx)
	if err != nil {
		return TruthDirInfo{}, err
	}
	return TruthDirInfo{
		Path:    dir,
		Version: version,
	}, nil
}

func (m *Manager) ListSystemResources(ctx context.Context) ([]SystemResourceSummary, error) {
	records, err := m.loadAllResources(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]SystemResourceSummary, 0, len(records))
	for _, record := range records {
		result = append(result, record.SystemResourceSummary)
	}
	return result, nil
}

// SyncSystemResources explicitly scans markdown sources and returns the refreshed detail catalog.
// SyncSystemResources 会显式遍历 markdown 主源，并返回同步后的详情目录。
func (m *Manager) SyncSystemResources(ctx context.Context) ([]SystemResourceDetail, error) {
	if err := m.SyncSystemSources(ctx); err != nil {
		return nil, err
	}
	return m.ListSystemResourceDetails(ctx)
}

// ListSystemResourceDetails returns detailed system-resource items for control-plane rendering.
// ListSystemResourceDetails 返回适合控制面渲染的 system-resource 详情列表。
func (m *Manager) ListSystemResourceDetails(ctx context.Context) ([]SystemResourceDetail, error) {
	items, err := m.ListSystemResources(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]SystemResourceDetail, 0, len(items))
	for _, item := range items {
		detail, err := m.GetSystemResource(ctx, item.AssetID)
		if err != nil {
			return nil, err
		}
		result = append(result, detail)
	}
	return result, nil
}

func (m *Manager) GetSystemResource(ctx context.Context, assetID string) (SystemResourceDetail, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceDetail{}, err
	}
	parseResult, _ := m.LoadSystemResourceParseResult(ctx, assetID)
	compileResult, _ := m.LoadSystemResourceCompileResult(ctx, assetID)
	pipeline, _ := m.LoadSystemResourcePipeline(ctx, assetID)
	return SystemResourceDetail{
		SystemResourceSummary: record.SystemResourceSummary,
		SourcePath:            record.SourcePath,
		Metadata:              cloneAnyMap(record.Metadata),
		ParseResult:           parseResult,
		CompileResult:         compileResult,
		Pipeline:              pipeline,
	}, nil
}

func (m *Manager) ListSystemResourceVersions(_ context.Context, assetID string) ([]SystemResourceVersionSummary, error) {
	entries, err := os.ReadDir(m.resourceVersionsDir(assetID))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]SystemResourceVersionSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		snapshot, err := m.loadSystemResourceVersionSnapshot(assetID, strings.TrimSuffix(entry.Name(), ".json"))
		if err != nil {
			continue
		}
		items = append(items, snapshot.Version)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].CreatedAt > items[j].CreatedAt })
	return items, nil
}

func (m *Manager) GetSystemResourceVersion(_ context.Context, assetID, versionID string) (SystemResourceVersionDetail, error) {
	snapshot, err := m.loadSystemResourceVersionSnapshot(assetID, versionID)
	if err != nil {
		return SystemResourceVersionDetail{}, err
	}
	return SystemResourceVersionDetail{
		SystemResourceVersionSummary: snapshot.Version,
		Resource:                     snapshot.Resource,
		SourceContent:                snapshot.Source.SourceContent,
	}, nil
}

func (m *Manager) ListSystemResourceAudit(ctx context.Context, assetID string) ([]SystemResourceAuditEntry, error) {
	versions, err := m.ListSystemResourceVersions(ctx, assetID)
	if err != nil {
		return nil, err
	}
	result := make([]SystemResourceAuditEntry, 0, len(versions))
	for _, item := range versions {
		result = append(result, SystemResourceAuditEntry{
			EventID:          item.VersionID,
			AssetID:          item.AssetID,
			Action:           item.Action,
			Summary:          item.Summary,
			CreatedAt:        item.CreatedAt,
			TruthDirVersion:  item.TruthDirVersion,
			CompiledVersion:  item.CompiledVersion,
			SourceChecksum:   item.SourceChecksum,
			CompiledChecksum: item.CompiledChecksum,
			RolledBackFrom:   item.RolledBackFrom,
			Detail: map[string]any{
				"version_id": item.VersionID,
			},
		})
	}
	return result, nil
}

func (m *Manager) CreateSystemResource(ctx context.Context, input SystemResourceCreateInput) (SystemResourceMutationResult, error) {
	assetID := sanitizeAssetID(input.AssetID)
	if assetID == "" {
		return SystemResourceMutationResult{}, fmt.Errorf("asset_id is required")
	}
	if _, err := m.loadResourceRecord(ctx, assetID); err == nil {
		return SystemResourceMutationResult{}, fmt.Errorf("system resource %q already exists", assetID)
	}
	now := time.Now().UTC()
	record := storedSystemResource{
		SystemResourceSummary: SystemResourceSummary{
			AssetID:    assetID,
			AssetType:  defaultString(strings.TrimSpace(input.AssetType), defaultAssetTypeForID(assetID)),
			AssetName:  strings.TrimSpace(input.AssetName),
			Scope:      defaultString(strings.TrimSpace(input.Scope), defaultSystemResourceScope),
			SourceKind: defaultString(strings.TrimSpace(input.SourceKind), defaultSystemResourceSource),
			Status:     defaultPipelineStatus,
			UpdatedAt:  now.Format(time.RFC3339),
			ReadOnly:   input.ReadOnly,
		},
		Metadata: cloneAnyMap(input.Metadata),
	}
	if record.AssetName == "" {
		record.AssetName = assetID
	}
	record.SourcePath = defaultSourceRelativePath(record)
	if err := m.saveResourceRecord(ctx, record, strings.TrimSpace(input.SourceContent)); err != nil {
		return SystemResourceMutationResult{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "created", defaultString(strings.TrimSpace(input.Message), "created system resource"), ""); err != nil {
		return SystemResourceMutationResult{}, err
	}
	result, err := m.runSystemResourcePipeline(ctx, assetID, "created")
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	return result, nil
}

func (m *Manager) DeleteSystemResource(ctx context.Context, assetID string) error {
	assetDir := m.resourceDir(assetID)
	if err := os.RemoveAll(assetDir); err != nil {
		return err
	}
	if _, err := m.bumpTruthDirVersion(ctx); err != nil {
		return err
	}
	return nil
}

func (m *Manager) RollbackSystemResourceVersion(ctx context.Context, assetID, versionID string) (SystemResourceMutationResult, error) {
	snapshot, err := m.loadSystemResourceVersionSnapshot(assetID, versionID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	record := storedSystemResource{
		SystemResourceSummary: snapshot.Resource.SystemResourceSummary,
		Metadata:              cloneAnyMap(snapshot.Resource.Metadata),
		SourcePath:            snapshot.Resource.SourcePath,
	}
	record.Status = "rollback_restored"
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if strings.TrimSpace(record.SourcePath) == "" {
		record.SourcePath = filepath.Join(record.AssetID, systemResourceSourceFile)
	}
	if err := m.saveResourceRecord(ctx, record, snapshot.Source.SourceContent); err != nil {
		return SystemResourceMutationResult{}, err
	}
	mutation, err := m.runSystemResourcePipeline(ctx, assetID, "rollback_restored")
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "rolled_back", fmt.Sprintf("rollback to %s", strings.TrimSpace(versionID)), versionID); err != nil {
		return SystemResourceMutationResult{}, err
	}
	return mutation, nil
}

func (m *Manager) LoadSystemResourceSource(ctx context.Context, assetID string) (SystemResourceSource, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceSource{}, err
	}
	content, err := os.ReadFile(m.sourceFilePath(record))
	if err != nil && !os.IsNotExist(err) {
		return SystemResourceSource{}, err
	}
	return SystemResourceSource{
		AssetID:       assetID,
		SourceContent: string(content),
		UpdatedAt:     record.UpdatedAt,
	}, nil
}

// GetSystemResourceSource returns one editable source body.
// GetSystemResourceSource 返回一份可编辑的 source 内容。
func (m *Manager) GetSystemResourceSource(ctx context.Context, assetID string) (SystemResourceSource, error) {
	return m.LoadSystemResourceSource(ctx, assetID)
}

func (m *Manager) SaveSystemResourceSource(ctx context.Context, assetID string, input SystemResourceSource) (SystemResourceMutationResult, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	record.Status = "source_saved"
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.saveResourceRecord(ctx, record, input.SourceContent); err != nil {
		return SystemResourceMutationResult{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "source_saved", defaultString(strings.TrimSpace(input.Message), "saved source content"), ""); err != nil {
		return SystemResourceMutationResult{}, err
	}
	return m.runSystemResourcePipeline(ctx, assetID, "source_saved")
}

// PutSystemResourceSource updates one source body and re-runs the default pipeline.
// PutSystemResourceSource 会更新 source 内容并重新执行默认 pipeline。
func (m *Manager) PutSystemResourceSource(ctx context.Context, assetID string, input SystemResourceSource) (SystemResourceMutationResult, error) {
	return m.SaveSystemResourceSource(ctx, assetID, input)
}

func (m *Manager) PatchSystemResourceMetadata(ctx context.Context, assetID string, patch SystemResourceMetadataPatch) (SystemResourceDetail, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceDetail{}, err
	}
	if strings.TrimSpace(patch.AssetType) != "" {
		record.AssetType = strings.TrimSpace(patch.AssetType)
	}
	if strings.TrimSpace(patch.AssetName) != "" {
		record.AssetName = strings.TrimSpace(patch.AssetName)
	}
	if strings.TrimSpace(patch.Scope) != "" {
		record.Scope = strings.TrimSpace(patch.Scope)
	}
	if strings.TrimSpace(patch.SourceKind) != "" {
		record.SourceKind = strings.TrimSpace(patch.SourceKind)
	}
	if patch.ReadOnly != nil {
		record.ReadOnly = *patch.ReadOnly
	}
	if patch.Metadata != nil {
		record.Metadata = cloneAnyMap(patch.Metadata)
	}
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	source, err := m.LoadSystemResourceSource(ctx, assetID)
	if err != nil {
		return SystemResourceDetail{}, err
	}
	if err := m.saveResourceRecord(ctx, record, source.SourceContent); err != nil {
		return SystemResourceDetail{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "metadata_updated", "updated system resource metadata", ""); err != nil {
		return SystemResourceDetail{}, err
	}
	if _, err := m.bumpTruthDirVersion(ctx); err != nil {
		return SystemResourceDetail{}, err
	}
	return m.GetSystemResource(ctx, assetID)
}

func (m *Manager) ParseSystemResource(ctx context.Context, assetID string) (SystemResourceMutationResult, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	source, err := m.LoadSystemResourceSource(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	now := time.Now().UTC()
	lines := compactLines(strings.Split(source.SourceContent, "\n"))
	parsed := map[string]any{
		"line_count":    len(lines),
		"heading_count": countHeadings(lines),
		"preview":       strings.Join(firstN(lines, 6), "\n"),
	}
	result := &SystemResourceParseResult{
		AssetID:    assetID,
		Status:     "parsed",
		Summary:    fmt.Sprintf("parsed %d non-empty lines from %s", len(lines), strings.TrimSpace(record.AssetName)),
		Parsed:     parsed,
		SourceHash: checksum(source.SourceContent),
		UpdatedAt:  now.Format(time.RFC3339),
	}
	if err := writeJSON(filepath.Join(m.resourceDir(assetID), systemResourceParseFile), result); err != nil {
		return SystemResourceMutationResult{}, err
	}
	pipeline := defaultPipeline(assetID, "parsed", 45)
	if err := m.savePipeline(ctx, assetID, pipeline); err != nil {
		return SystemResourceMutationResult{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "parsed", result.Summary, ""); err != nil {
		return SystemResourceMutationResult{}, err
	}
	return SystemResourceMutationResult{AssetID: assetID, Accepted: true, Pipeline: pipeline}, nil
}

func (m *Manager) CompileSystemResource(ctx context.Context, assetID string) (SystemResourceMutationResult, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	source, err := m.LoadSystemResourceSource(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	parseResult, _ := m.LoadSystemResourceParseResult(ctx, assetID)
	truthVersion, err := m.currentTruthDirVersion(ctx)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	payload := buildCompiledAssetPayload(record, source.SourceContent)
	guidanceText := buildCompiledGuidance(record, source.SourceContent, payload)
	result := &SystemResourceCompileResult{
		AssetID:          assetID,
		Status:           "compiled",
		Summary:          fmt.Sprintf("compiled %s for runtime injection", record.AssetType),
		GuidanceText:     guidanceText,
		SourceChecksum:   checksum(source.SourceContent),
		CompiledChecksum: checksum(guidanceText + truthVersion),
		CompiledVersion:  newResourceVersion(),
		TruthDirVersion:  truthVersion,
		Payload:          payload,
		UpdatedAt:        time.Now().UTC().Format(time.RFC3339),
	}
	if parseResult == nil {
		result.Summary = "compiled without parse result"
	}
	if err := writeJSON(filepath.Join(m.resourceDir(assetID), systemResourceCompileFile), result); err != nil {
		return SystemResourceMutationResult{}, err
	}
	record.CompiledVersion = result.CompiledVersion
	record.TruthDirVersion = truthVersion
	record.Status = "compiled"
	record.UpdatedAt = result.UpdatedAt
	if err := m.saveResourceRecord(ctx, record, source.SourceContent); err != nil {
		return SystemResourceMutationResult{}, err
	}
	pipeline := defaultPipeline(assetID, "compiled", 75)
	if err := m.savePipeline(ctx, assetID, pipeline); err != nil {
		return SystemResourceMutationResult{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "compiled", result.Summary, ""); err != nil {
		return SystemResourceMutationResult{}, err
	}
	return m.ActivateSystemResource(ctx, assetID)
}

func (m *Manager) ActivateSystemResource(ctx context.Context, assetID string) (SystemResourceMutationResult, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	source, err := m.LoadSystemResourceSource(ctx, assetID)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	record.Status = "active"
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.saveResourceRecord(ctx, record, source.SourceContent); err != nil {
		return SystemResourceMutationResult{}, err
	}
	version, err := m.bumpTruthDirVersion(ctx)
	if err != nil {
		return SystemResourceMutationResult{}, err
	}
	record.TruthDirVersion = version
	_ = m.saveResourceRecord(ctx, record, source.SourceContent)
	pipeline := defaultPipeline(assetID, "active", 100)
	pipeline.Status = "active"
	if err := m.savePipeline(ctx, assetID, pipeline); err != nil {
		return SystemResourceMutationResult{}, err
	}
	if err := m.recordSystemResourceVersion(ctx, assetID, "activated", "activated compiled system resource", ""); err != nil {
		return SystemResourceMutationResult{}, err
	}
	return SystemResourceMutationResult{AssetID: assetID, Accepted: true, Pipeline: pipeline}, nil
}

func (m *Manager) runSystemResourcePipeline(ctx context.Context, assetID string, initialStep string) (SystemResourceMutationResult, error) {
	pipeline := defaultPipeline(assetID, initialStep, 15)
	if err := m.savePipeline(ctx, assetID, pipeline); err != nil {
		return SystemResourceMutationResult{}, err
	}
	if _, err := m.ParseSystemResource(ctx, assetID); err != nil {
		return SystemResourceMutationResult{}, err
	}
	return m.CompileSystemResource(ctx, assetID)
}

func (m *Manager) LoadSystemResourcePipeline(_ context.Context, assetID string) (*SystemResourcePipeline, error) {
	var pipeline SystemResourcePipeline
	if err := readJSON(filepath.Join(m.resourceDir(assetID), systemResourcePipelineFile), &pipeline); err != nil {
		if errorsIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &pipeline, nil
}

// GetSystemResourcePipeline returns the latest pipeline state.
// GetSystemResourcePipeline 返回最近一次 pipeline 状态。
func (m *Manager) GetSystemResourcePipeline(ctx context.Context, assetID string) (SystemResourcePipeline, error) {
	item, err := m.LoadSystemResourcePipeline(ctx, assetID)
	if err != nil {
		return SystemResourcePipeline{}, err
	}
	if item == nil {
		return SystemResourcePipeline{}, nil
	}
	return *item, nil
}

func (m *Manager) LoadSystemResourceParseResult(_ context.Context, assetID string) (*SystemResourceParseResult, error) {
	var result SystemResourceParseResult
	if err := readJSON(filepath.Join(m.resourceDir(assetID), systemResourceParseFile), &result); err != nil {
		if errorsIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// GetSystemResourceParseResult returns the latest parse result.
// GetSystemResourceParseResult 返回最近一次 parse 结果。
func (m *Manager) GetSystemResourceParseResult(ctx context.Context, assetID string) (SystemResourceParseResult, error) {
	item, err := m.LoadSystemResourceParseResult(ctx, assetID)
	if err != nil {
		return SystemResourceParseResult{}, err
	}
	if item == nil {
		return SystemResourceParseResult{}, nil
	}
	return *item, nil
}

func (m *Manager) LoadSystemResourceCompileResult(_ context.Context, assetID string) (*SystemResourceCompileResult, error) {
	var result SystemResourceCompileResult
	if err := readJSON(filepath.Join(m.resourceDir(assetID), systemResourceCompileFile), &result); err != nil {
		if errorsIsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	return &result, nil
}

// GetSystemResourceCompileResult returns the latest compile result.
// GetSystemResourceCompileResult 返回最近一次 compile 结果。
func (m *Manager) GetSystemResourceCompileResult(ctx context.Context, assetID string) (SystemResourceCompileResult, error) {
	item, err := m.LoadSystemResourceCompileResult(ctx, assetID)
	if err != nil {
		return SystemResourceCompileResult{}, err
	}
	if item == nil {
		return SystemResourceCompileResult{}, nil
	}
	return *item, nil
}

func (m *Manager) BuildSystemResourceDebugPayload(ctx context.Context, assetID string, endpoint string) (SystemResourceDebugPayload, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return SystemResourceDebugPayload{}, err
	}
	compileResult, err := m.LoadSystemResourceCompileResult(ctx, assetID)
	if err != nil {
		return SystemResourceDebugPayload{}, err
	}
	if compileResult == nil {
		return SystemResourceDebugPayload{}, fmt.Errorf("resource %q must be compiled before debug payload is available", assetID)
	}
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		endpoint = "/api/chat/respond"
	}
	if endpoint == "chat_respond" {
		endpoint = "/api/chat/respond"
	}
	payload := map[string]any{
		"query":     "Explain how this context asset would affect the current answer.",
		"task_type": "chat",
		"global_context": map[string]any{
			"context_assets": []map[string]any{{
				"asset_id":    record.AssetID,
				"asset_type":  record.AssetType,
				"asset_name":  record.AssetName,
				"scope":       record.Scope,
				"source_kind": record.SourceKind,
				"mode":        "ref",
				"priority":    100,
				"ref": map[string]any{
					"ref_type":          "compiled_asset",
					"target":            record.AssetID,
					"version":           defaultString(compileResult.CompiledVersion, record.CompiledVersion),
					"checksum":          defaultString(compileResult.CompiledChecksum, compileResult.SourceChecksum),
					"truth_dir_version": defaultString(compileResult.TruthDirVersion, record.TruthDirVersion),
					"detail_endpoint":   fmt.Sprintf("/api/system-resources/%s/compile-result", record.AssetID),
				},
			}},
		},
	}
	if strings.Contains(record.AssetType, "skill") {
		payload["enabled_skills"] = []string{record.AssetID}
	}
	return SystemResourceDebugPayload{
		Endpoint: endpoint,
		Payload:  payload,
	}, nil
}

func (m *Manager) DownloadSystemResource(ctx context.Context, assetID string) ([]byte, string, error) {
	record, err := m.loadResourceRecord(ctx, assetID)
	if err != nil {
		return nil, "", err
	}
	source, err := m.LoadSystemResourceSource(ctx, assetID)
	if err != nil {
		return nil, "", err
	}
	filename := filepath.Base(m.sourceFilePath(record))
	if strings.TrimSpace(filename) == "" {
		filename = sanitizeAssetID(record.AssetID) + ".md"
	}
	return []byte(source.SourceContent), filename, nil
}

func (m *Manager) ExportSystemResources(ctx context.Context) ([]byte, SystemResourceExportInfo, error) {
	records, err := m.loadAllResources(ctx)
	if err != nil {
		return nil, SystemResourceExportInfo{}, err
	}
	version, err := m.currentTruthDirVersion(ctx)
	if err != nil {
		return nil, SystemResourceExportInfo{}, err
	}
	var buffer bytes.Buffer
	archive := zip.NewWriter(&buffer)
	for _, record := range records {
		source, err := m.LoadSystemResourceSource(ctx, record.AssetID)
		if err != nil {
			return nil, SystemResourceExportInfo{}, err
		}
		files := map[string][]byte{
			filepath.Join(record.AssetID, systemResourceMetaFile):                                                        mustJSON(record),
			defaultString(strings.TrimSpace(record.SourcePath), filepath.Join(record.AssetID, systemResourceSourceFile)): []byte(source.SourceContent),
		}
		if parseResult, _ := m.LoadSystemResourceParseResult(ctx, record.AssetID); parseResult != nil {
			files[filepath.Join(record.AssetID, systemResourceParseFile)] = mustJSON(parseResult)
		}
		if compileResult, _ := m.LoadSystemResourceCompileResult(ctx, record.AssetID); compileResult != nil {
			files[filepath.Join(record.AssetID, systemResourceCompileFile)] = mustJSON(compileResult)
		}
		if pipeline, _ := m.LoadSystemResourcePipeline(ctx, record.AssetID); pipeline != nil {
			files[filepath.Join(record.AssetID, systemResourcePipelineFile)] = mustJSON(pipeline)
		}
		versions, _ := m.ListSystemResourceVersions(ctx, record.AssetID)
		for _, version := range versions {
			snapshot, err := m.loadSystemResourceVersionSnapshot(record.AssetID, version.VersionID)
			if err != nil {
				continue
			}
			files[filepath.Join(record.AssetID, systemResourceVersionsDir, version.VersionID+".json")] = mustJSON(snapshot)
		}
		for name, payload := range files {
			writer, err := archive.Create(name)
			if err != nil {
				return nil, SystemResourceExportInfo{}, err
			}
			if _, err := writer.Write(payload); err != nil {
				return nil, SystemResourceExportInfo{}, err
			}
		}
	}
	manifestWriter, err := archive.Create("manifest.json")
	if err != nil {
		return nil, SystemResourceExportInfo{}, err
	}
	manifestPayload := map[string]any{
		"truth_dir_version": version,
		"asset_count":       len(records),
		"generated_at":      time.Now().UTC().Format(time.RFC3339),
	}
	if _, err := manifestWriter.Write(mustJSON(manifestPayload)); err != nil {
		return nil, SystemResourceExportInfo{}, err
	}
	if err := archive.Close(); err != nil {
		return nil, SystemResourceExportInfo{}, err
	}
	info := SystemResourceExportInfo{
		TruthDirVersion: version,
		ExportFile:      fmt.Sprintf("system-resources-%s.zip", strings.ReplaceAll(version, ":", "-")),
		AssetCount:      len(records),
	}
	return buffer.Bytes(), info, nil
}

func (m *Manager) loadAllResources(_ context.Context) ([]storedSystemResource, error) {
	root := strings.TrimSpace(m.activeStateRoot())
	if root == "" {
		return nil, nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := make([]storedSystemResource, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		record, err := m.loadResourceRecord(context.Background(), entry.Name())
		if err != nil {
			continue
		}
		result = append(result, record)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].AssetID < result[j].AssetID })
	return result, nil
}

func (m *Manager) loadResourceRecord(_ context.Context, assetID string) (storedSystemResource, error) {
	var record storedSystemResource
	if err := readJSON(filepath.Join(m.resourceDir(assetID), systemResourceMetaFile), &record); err != nil {
		return storedSystemResource{}, err
	}
	return record, nil
}

func (m *Manager) saveResourceRecord(_ context.Context, record storedSystemResource, sourceContent string) error {
	assetDir := m.resourceDir(record.AssetID)
	if strings.TrimSpace(record.SourcePath) == "" {
		record.SourcePath = defaultSourceRelativePath(record)
	}
	if err := os.MkdirAll(assetDir, 0o755); err != nil {
		return err
	}
	if err := writeJSON(filepath.Join(assetDir, systemResourceMetaFile), record); err != nil {
		return err
	}
	sourceFile := m.sourceFilePath(record)
	if err := os.MkdirAll(filepath.Dir(sourceFile), 0o755); err != nil {
		return err
	}
	return writeFileAtomically(sourceFile, []byte(sourceContent), 0o644)
}

func (m *Manager) savePipeline(_ context.Context, assetID string, pipeline SystemResourcePipeline) error {
	return writeJSON(filepath.Join(m.resourceDir(assetID), systemResourcePipelineFile), pipeline)
}

func (m *Manager) resourceDir(assetID string) string {
	return filepath.Join(strings.TrimSpace(m.activeStateRoot()), sanitizeAssetID(assetID))
}

func (m *Manager) activeStateRoot() string {
	if m == nil {
		return ""
	}
	if root := strings.TrimSpace(m.activeStateDir); root != "" {
		return root
	}
	return strings.TrimSpace(m.truthDir)
}

// SyncSystemSources scans truth-dir markdown sources and keeps the resource catalog compiled.
// SyncSystemSources 负责遍历 truth dir 下的新主源结构，并把它们同步进资源目录后完成编译。
func (m *Manager) SyncSystemSources(ctx context.Context) error {
	root := m.sourcesRoot()
	if strings.TrimSpace(root) == "" {
		return nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	seen := map[string]struct{}{}
	paths, err := m.collectSystemSourceFiles(root)
	if err != nil {
		return err
	}
	for _, path := range paths {
		record, _, err := m.recordFromSourceFile(path)
		if err != nil {
			return err
		}
		seen[record.AssetID] = struct{}{}
		if err := m.syncSystemSourceFile(ctx, path); err != nil {
			return err
		}
	}
	return m.deleteStaleSystemResources(ctx, seen)
}

func (m *Manager) syncSystemSourceFile(ctx context.Context, absolutePath string) error {
	record, sourceContent, err := m.recordFromSourceFile(absolutePath)
	if err != nil {
		return err
	}
	current, err := m.loadResourceRecord(ctx, record.AssetID)
	if err == nil {
		record.Metadata = mergeMetadata(current.Metadata, record.Metadata)
		if strings.TrimSpace(current.AssetName) != "" {
			record.AssetName = current.AssetName
		}
		if strings.TrimSpace(current.Scope) != "" {
			record.Scope = current.Scope
		}
		if strings.TrimSpace(current.SourceKind) != "" {
			record.SourceKind = current.SourceKind
		}
		record.ReadOnly = current.ReadOnly
		record.CompiledVersion = current.CompiledVersion
		record.TruthDirVersion = current.TruthDirVersion
		record.Status = current.Status
	}
	record.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	if err := m.saveResourceRecord(ctx, record, sourceContent); err != nil {
		return err
	}
	compileResult, _ := m.LoadSystemResourceCompileResult(ctx, record.AssetID)
	if compileResult != nil && compileResult.SourceChecksum == checksum(sourceContent) {
		return nil
	}
	if _, err := m.runSystemResourcePipeline(ctx, record.AssetID, "source_synced"); err != nil {
		return err
	}
	if err := m.recordSystemResourceVersion(ctx, record.AssetID, "source_synced", "synced markdown source from truth dir", ""); err != nil {
		return err
	}
	return nil
}

func (m *Manager) collectSystemSourceFiles(root string) ([]string, error) {
	result := make([]string, 0)
	appendIfExists := func(path string) error {
		info, err := os.Stat(path)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if info.IsDir() {
			return nil
		}
		result = append(result, path)
		return nil
	}

	if err := appendIfExists(filepath.Join(root, "core", "SOUL.md")); err != nil {
		return nil, err
	}
	if err := appendIfExists(filepath.Join(root, "core", "AGENTS.md")); err != nil {
		return nil, err
	}
	for _, dir := range []string{
		filepath.Join(root, "core", "policy_rule"),
		filepath.Join(root, "core", "tool_governance_policy"),
		filepath.Join(root, "core", "user_profile"),
		filepath.Join(root, "core", "memory_view"),
	} {
		if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				if os.IsNotExist(walkErr) {
					return nil
				}
				return walkErr
			}
			if entry.IsDir() {
				return nil
			}
			if isSystemResourceDirectoryReadme(path) {
				return nil
			}
			if strings.EqualFold(filepath.Ext(path), ".md") {
				result = append(result, path)
			}
			return nil
		}); err != nil {
			return nil, err
		}
	}
	scenesRoot := filepath.Join(root, "scenes")
	sceneEntries, err := os.ReadDir(scenesRoot)
	if err != nil {
		if os.IsNotExist(err) {
			sort.Strings(result)
			return result, nil
		}
		return nil, err
	}
	for _, sceneEntry := range sceneEntries {
		if !sceneEntry.IsDir() {
			continue
		}
		sceneRoot := filepath.Join(scenesRoot, sceneEntry.Name())
		if err := appendIfExists(filepath.Join(sceneRoot, "SCENE.md")); err != nil {
			return nil, err
		}
		if err := appendIfExists(filepath.Join(sceneRoot, "workflow.yaml")); err != nil {
			return nil, err
		}
		for _, dir := range []string{
			filepath.Join(sceneRoot, "contract"),
			filepath.Join(sceneRoot, "policy_rule"),
		} {
			if err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
				if walkErr != nil {
					if os.IsNotExist(walkErr) {
						return nil
					}
					return walkErr
				}
				if entry.IsDir() {
					return nil
				}
				if isSystemResourceDirectoryReadme(path) {
					return nil
				}
				switch strings.ToLower(filepath.Ext(path)) {
				case ".md", ".yaml", ".yml":
					result = append(result, path)
				}
				return nil
			}); err != nil {
				return nil, err
			}
		}
		skillsRoot := filepath.Join(sceneRoot, "skills")
		skillEntries, err := os.ReadDir(skillsRoot)
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() {
				continue
			}
			if err := appendIfExists(filepath.Join(skillsRoot, skillEntry.Name(), "SKILL.md")); err != nil {
				return nil, err
			}
		}
	}
	sort.Strings(result)
	return result, nil
}

// isSystemResourceDirectoryReadme keeps navigation docs out of source truth ingestion.
// isSystemResourceDirectoryReadme 避免把目录说明文档当成 system truth 资产导入。
func isSystemResourceDirectoryReadme(path string) bool {
	return strings.EqualFold(filepath.Base(path), "README.md")
}

func (m *Manager) deleteStaleSystemResources(ctx context.Context, active map[string]struct{}) error {
	records, err := m.loadAllResources(ctx)
	if err != nil {
		return err
	}
	for _, record := range records {
		if _, ok := active[strings.TrimSpace(record.AssetID)]; ok {
			continue
		}
		if err := os.RemoveAll(m.resourceDir(record.AssetID)); err != nil {
			return err
		}
	}
	if _, err := m.bumpTruthDirVersion(ctx); err != nil {
		return err
	}
	return nil
}

func (m *Manager) sourcesRoot() string {
	root := strings.TrimSpace(m.truthDir)
	if root == "" {
		return ""
	}
	return filepath.Join(root, systemResourceSourcesDir)
}

func (m *Manager) sourceFilePath(record storedSystemResource) string {
	relative := strings.TrimSpace(record.SourcePath)
	if relative == "" {
		relative = defaultSourceRelativePath(record)
	}
	cleaned := filepath.Clean(relative)
	if cleaned == "." {
		cleaned = defaultSourceRelativePath(record)
	}
	return filepath.Join(strings.TrimSpace(m.truthDir), cleaned)
}

func (m *Manager) recordFromSourceFile(absolutePath string) (storedSystemResource, string, error) {
	relative, err := filepath.Rel(strings.TrimSpace(m.truthDir), absolutePath)
	if err != nil {
		return storedSystemResource{}, "", err
	}
	assetType, assetID, assetName, err := assetIdentityFromSourceRelativePath(relative)
	if err != nil {
		return storedSystemResource{}, "", err
	}
	sourceContent, metadata, err := buildSourceContentAndMetadata(absolutePath, assetType)
	if err != nil {
		return storedSystemResource{}, "", err
	}
	if strings.TrimSpace(sourceContent) == "" {
		return storedSystemResource{}, "", fmt.Errorf("system truth source %q for %s is empty", filepath.ToSlash(filepath.Clean(relative)), assetID)
	}
	return storedSystemResource{
		SystemResourceSummary: SystemResourceSummary{
			AssetID:    assetID,
			AssetType:  assetType,
			AssetName:  assetName,
			Scope:      defaultSystemResourceScope,
			SourceKind: truthDirSystemSource,
			Status:     defaultPipelineStatus,
		},
		Metadata:   mergeMetadata(map[string]any{"managed_by": "system_sources"}, metadata),
		SourcePath: filepath.ToSlash(filepath.Clean(relative)),
	}, sourceContent, nil
}

func assetIdentityFromSourceRelativePath(relative string) (string, string, string, error) {
	cleaned := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relative)))
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "SOUL.md")) {
		return "persona", "persona.default", "SOUL", nil
	}
	if cleaned == filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "AGENTS.md")) {
		return "agent_profile", "agent_profile.default", "AGENTS", nil
	}
	prefix := systemResourceSourcesDir + "/"
	if !strings.HasPrefix(cleaned, prefix) {
		return "", "", "", fmt.Errorf("source path %q is outside %s", cleaned, systemResourceSourcesDir)
	}
	trimmed := strings.TrimPrefix(cleaned, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) < 3 {
		return "", "", "", fmt.Errorf("source path %q does not match new truth layout", cleaned)
	}
	switch parts[0] {
	case "core":
		switch parts[1] {
		case "policy_rule", "tool_governance_policy", "user_profile", "memory_view":
			last := strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1]))
			assetType := normalizeAssetType(parts[1])
			name := sanitizeAssetID(last)
			if name == "" {
				return "", "", "", fmt.Errorf("source path %q resolved to empty asset name", cleaned)
			}
			prefix := "core"
			if assetType == "user_profile" || assetType == "memory_view" {
				prefix = ""
			}
			assetID := assetType + "."
			if prefix != "" {
				assetID += prefix + "."
			}
			assetID += name
			return assetType, assetID, last, nil
		}
	case "scenes":
		sceneID := sanitizeAssetID(parts[1])
		if sceneID == "" {
			return "", "", "", fmt.Errorf("source path %q resolved to empty scene id", cleaned)
		}
		if len(parts) == 3 && parts[2] == "SCENE.md" {
			return "scene", "scene." + sceneID, sceneID, nil
		}
		if len(parts) == 3 && parts[2] == "workflow.yaml" {
			return "workflow", "workflow." + sceneID + ".main", sceneID, nil
		}
		if len(parts) >= 4 && parts[2] == "contract" {
			name := sanitizeAssetID(strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1])))
			return "contract", "contract." + sceneID + "." + name, name, nil
		}
		if len(parts) >= 4 && parts[2] == "policy_rule" {
			name := sanitizeAssetID(strings.TrimSuffix(parts[len(parts)-1], filepath.Ext(parts[len(parts)-1])))
			return "policy_rule", "policy_rule." + sceneID + "." + name, name, nil
		}
		if len(parts) >= 5 && parts[2] == "skills" && parts[4] == "SKILL.md" {
			skillID := sanitizeAssetID(parts[3])
			return "skill", "skill." + sceneID + "." + skillID, skillID, nil
		}
	}
	return "", "", "", fmt.Errorf("unsupported new source path %q", cleaned)
}

func defaultSourceRelativePath(record storedSystemResource) string {
	assetType := normalizeAssetType(record.AssetType)
	switch {
	case assetType == "persona" && record.AssetID == "persona.default":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "SOUL.md"))
	case assetType == "agent_profile" && record.AssetID == "agent_profile.default":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "AGENTS.md"))
	case assetType == "policy_rule" && strings.HasPrefix(record.AssetID, "policy_rule.core."):
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "policy_rule", strings.TrimPrefix(record.AssetID, "policy_rule.core.")+".md"))
	case assetType == toolGovernancePolicyAssetType && strings.HasPrefix(record.AssetID, toolGovernancePolicyAssetType+".core."):
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", toolGovernancePolicyAssetType, strings.TrimPrefix(record.AssetID, toolGovernancePolicyAssetType+".core.")+".md"))
	case assetType == "user_profile":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "user_profile", defaultSourceFileName(record)+".md"))
	case assetType == "memory_view":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "core", "memory_view", defaultSourceFileName(record)+".md"))
	case assetType == "scene":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "scenes", sceneIDFromAssetID(record.AssetID), "SCENE.md"))
	case assetType == "workflow":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "scenes", sceneIDFromAssetID(record.AssetID), "workflow.yaml"))
	case assetType == "contract":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "scenes", sceneIDFromAssetID(record.AssetID), "contract", trailingAssetName(record.AssetID)+".yaml"))
	case assetType == "skill":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "scenes", sceneIDFromAssetID(record.AssetID), "skills", trailingAssetName(record.AssetID), "SKILL.md"))
	case assetType == "policy_rule":
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "scenes", sceneIDFromAssetID(record.AssetID), "policy_rule", trailingAssetName(record.AssetID)+".md"))
	default:
		return filepath.ToSlash(filepath.Join(systemResourceSourcesDir, "scenes", "default", defaultSourceFileName(record)+".md"))
	}
}

func defaultSourceFileName(record storedSystemResource) string {
	assetType := normalizeAssetType(record.AssetType)
	assetID := strings.TrimSpace(record.AssetID)
	if assetID != "" && strings.HasPrefix(assetID, assetType+".") {
		return strings.TrimPrefix(assetID, assetType+".")
	}
	if name := sanitizeAssetID(strings.TrimSpace(record.AssetName)); name != "" {
		return name
	}
	return sanitizeAssetID(assetID)
}

func isSupportedSystemAssetType(input string) bool {
	switch normalizeAssetType(input) {
	case "persona", "agent_profile", "policy_rule", toolGovernancePolicyAssetType, "user_profile", "memory_view", "scene", "workflow", "contract", "skill":
		return true
	default:
		return false
	}
}

func defaultAssetTypeForID(assetID string) string {
	assetID = strings.TrimSpace(strings.ToLower(assetID))
	switch {
	case strings.HasPrefix(assetID, "persona."):
		return "persona"
	case strings.HasPrefix(assetID, "agent_profile."):
		return "agent_profile"
	case strings.HasPrefix(assetID, "policy_rule."):
		return "policy_rule"
	case strings.HasPrefix(assetID, toolGovernancePolicyAssetType+"."):
		return toolGovernancePolicyAssetType
	case strings.HasPrefix(assetID, "scene."):
		return "scene"
	case strings.HasPrefix(assetID, "workflow."):
		return "workflow"
	case strings.HasPrefix(assetID, "contract."):
		return "contract"
	case strings.HasPrefix(assetID, "skill."):
		return "skill"
	case strings.HasPrefix(assetID, "user_profile."):
		return "user_profile"
	case strings.HasPrefix(assetID, "memory_view."):
		return "memory_view"
	default:
		return defaultSystemResourceType
	}
}

func mergeMetadata(existing map[string]any, incoming map[string]any) map[string]any {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	result := cloneAnyMap(existing)
	if result == nil {
		result = map[string]any{}
	}
	for key, value := range incoming {
		result[key] = value
	}
	return result
}

func buildSourceContentAndMetadata(path string, assetType string) (string, map[string]any, error) {
	normalizedType := normalizeAssetType(assetType)
	switch normalizedType {
	case "workflow", "contract":
		payload, err := systemtruth.ReadYAMLMap(path)
		if err != nil {
			return "", nil, err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		return string(content), map[string]any{"parsed_yaml": payload}, nil
	case "skill":
		doc, err := systemtruth.ReadMarkdownDocument(path)
		if err != nil {
			return "", nil, err
		}
		skillRoot := filepath.Dir(path)
		metadata := map[string]any{
			"frontmatter":         cloneAnyMap(doc.Frontmatter),
			"scripts_manifest":    directoryManifest(filepath.Join(skillRoot, "scripts")),
			"references_manifest": directoryManifest(filepath.Join(skillRoot, "references")),
			"assets_manifest":     directoryManifest(filepath.Join(skillRoot, "assets")),
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		return string(content), metadata, nil
	default:
		doc, err := systemtruth.ReadMarkdownDocument(path)
		if err != nil {
			return "", nil, err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return "", nil, err
		}
		if len(doc.Frontmatter) == 0 {
			return string(content), nil, nil
		}
		return string(content), map[string]any{"frontmatter": cloneAnyMap(doc.Frontmatter)}, nil
	}
}

func directoryManifest(dir string) []map[string]any {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	result := make([]map[string]any, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		result = append(result, map[string]any{
			"name": entry.Name(),
			"path": filepath.ToSlash(filepath.Join(filepath.Base(dir), entry.Name())),
		})
	}
	if len(result) == 0 {
		return nil
	}
	sort.Slice(result, func(i, j int) bool {
		return valueAsString(result[i]["name"]) < valueAsString(result[j]["name"])
	})
	return result
}

func sceneIDFromAssetID(assetID string) string {
	parts := strings.Split(strings.TrimSpace(assetID), ".")
	if len(parts) < 2 {
		return "default"
	}
	return sanitizeAssetID(parts[1])
}

func trailingAssetName(assetID string) string {
	parts := strings.Split(strings.TrimSpace(assetID), ".")
	if len(parts) == 0 {
		return ""
	}
	return sanitizeAssetID(parts[len(parts)-1])
}

func (m *Manager) currentTruthDirVersion(_ context.Context) (string, error) {
	root := strings.TrimSpace(m.activeStateRoot())
	if root == "" {
		return "", nil
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	payload, err := os.ReadFile(filepath.Join(root, systemResourceVersionFile))
	if err != nil {
		if os.IsNotExist(err) {
			version := newResourceVersion()
			if err := os.WriteFile(filepath.Join(root, systemResourceVersionFile), []byte(version), 0o644); err != nil {
				return "", err
			}
			return version, nil
		}
		return "", err
	}
	return strings.TrimSpace(string(payload)), nil
}

func (m *Manager) bumpTruthDirVersion(ctx context.Context) (string, error) {
	root := strings.TrimSpace(m.activeStateRoot())
	if root == "" {
		return "", nil
	}
	version := newResourceVersion()
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(root, systemResourceVersionFile), []byte(version), 0o644); err != nil {
		return "", err
	}
	return version, nil
}

func (m *Manager) loadSystemManifest() (systemManifest, error) {
	version, err := m.currentTruthDirVersion(context.Background())
	if err != nil {
		return systemManifest{}, err
	}
	return systemManifest{TruthDirVersion: version}, nil
}

func (m *Manager) resourceVersionsDir(assetID string) string {
	return filepath.Join(m.resourceDir(assetID), systemResourceVersionsDir)
}

func (m *Manager) systemResourceSnapshot(ctx context.Context, assetID string) (systemResourceVersionSnapshot, error) {
	detail, err := m.GetSystemResource(ctx, assetID)
	if err != nil {
		return systemResourceVersionSnapshot{}, err
	}
	source, err := m.LoadSystemResourceSource(ctx, assetID)
	if err != nil {
		return systemResourceVersionSnapshot{}, err
	}
	return systemResourceVersionSnapshot{
		Resource: detail,
		Source:   source,
	}, nil
}

func (m *Manager) recordSystemResourceVersion(ctx context.Context, assetID, action, summary, rolledBackFrom string) error {
	snapshot, err := m.systemResourceSnapshot(ctx, assetID)
	if err != nil {
		return err
	}
	snapshot.Version = SystemResourceVersionSummary{
		VersionID:        fmt.Sprintf("asset_%s", newResourceVersion()),
		AssetID:          assetID,
		Action:           strings.TrimSpace(action),
		Summary:          strings.TrimSpace(summary),
		CreatedAt:        time.Now().UTC().Format(time.RFC3339),
		TruthDirVersion:  defaultString(snapshot.Resource.TruthDirVersion, compileResultField(snapshot.Resource.CompileResult, "truth_dir_version")),
		CompiledVersion:  defaultString(snapshot.Resource.CompiledVersion, compileResultField(snapshot.Resource.CompileResult, "compiled_version")),
		SourceChecksum:   checksum(snapshot.Source.SourceContent),
		CompiledChecksum: compileResultField(snapshot.Resource.CompileResult, "compiled_checksum"),
		RolledBackFrom:   strings.TrimSpace(rolledBackFrom),
	}
	dir := m.resourceVersionsDir(assetID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return writeJSON(filepath.Join(dir, snapshot.Version.VersionID+".json"), snapshot)
}

func (m *Manager) loadSystemResourceVersionSnapshot(assetID, versionID string) (systemResourceVersionSnapshot, error) {
	var snapshot systemResourceVersionSnapshot
	if err := readJSON(filepath.Join(m.resourceVersionsDir(assetID), strings.TrimSpace(versionID)+".json"), &snapshot); err != nil {
		return systemResourceVersionSnapshot{}, err
	}
	return snapshot, nil
}

func compileResultField(result *SystemResourceCompileResult, field string) string {
	if result == nil {
		return ""
	}
	switch strings.TrimSpace(field) {
	case "truth_dir_version":
		return strings.TrimSpace(result.TruthDirVersion)
	case "compiled_version":
		return strings.TrimSpace(result.CompiledVersion)
	case "compiled_checksum":
		return strings.TrimSpace(result.CompiledChecksum)
	default:
		return ""
	}
}

func defaultPipeline(assetID string, step string, progress int) SystemResourcePipeline {
	now := time.Now().UTC().Format(time.RFC3339)
	return SystemResourcePipeline{
		PipelineID:      fmt.Sprintf("pipe_%s_%d", sanitizeAssetID(assetID), time.Now().UTC().UnixNano()),
		AssetID:         sanitizeAssetID(assetID),
		Status:          "running",
		CurrentStep:     step,
		ProgressPercent: progress,
		StartedAt:       now,
		UpdatedAt:       now,
	}
}

func buildCompiledGuidance(record storedSystemResource, source string, payload map[string]any) string {
	summary := strings.TrimSpace(record.AssetName)
	if summary == "" {
		summary = record.AssetID
	}
	if len(payload) > 0 {
		if compiledSummary := strings.TrimSpace(valueAsString(payload["summary"])); compiledSummary != "" {
			return fmt.Sprintf("Compiled context asset [%s/%s]: %s\n%s", record.AssetType, record.AssetID, summary, compiledSummary)
		}
	}
	content := strings.TrimSpace(source)
	if len(content) > 600 {
		content = content[:600]
	}
	return fmt.Sprintf("Authorized context asset [%s/%s]: %s\n%s", record.AssetType, record.AssetID, summary, content)
}

func buildCompiledAssetPayload(record storedSystemResource, source string) map[string]any {
	normalizedContent := sourceContentForPayload(record, source)
	base := map[string]any{
		"asset_id":           record.AssetID,
		"asset_type":         record.AssetType,
		"asset_name":         record.AssetName,
		"scope":              record.Scope,
		"source_kind":        record.SourceKind,
		"read_only":          record.ReadOnly,
		"candidate_writable": !record.ReadOnly,
		"resident":           record.Scope == "system" || record.Scope == "workspace",
		"summary":            summarizeSource(normalizedContent),
		"content":            strings.TrimSpace(normalizedContent),
		"metadata":           cloneAnyMap(record.Metadata),
		"version":            record.CompiledVersion,
	}
	switch normalizeAssetType(record.AssetType) {
	case "persona":
		return mergePayload(base, buildPersonaPayload(record, normalizedContent))
	case "agent_profile":
		return mergePayload(base, buildAgentProfilePayload(record, normalizedContent))
	case "policy_rule":
		return mergePayload(base, buildPolicyRulePayload(record, normalizedContent))
	case toolGovernancePolicyAssetType:
		return mergePayload(base, buildToolGovernancePolicyPayload(record, source))
	case "user_profile":
		return mergePayload(base, buildUserProfilePayload(record, normalizedContent))
	case "memory_view":
		return mergePayload(base, buildMemoryViewPayload(record, normalizedContent))
	case "scene":
		return mergePayload(base, buildScenePayload(record, normalizedContent))
	case "workflow":
		return mergePayload(base, buildWorkflowPayload(record))
	case "contract":
		return mergePayload(base, buildContractPayload(record))
	case "skill":
		return mergePayload(base, buildSkillPayload(record, normalizedContent))
	default:
		base["guidance"] = fmt.Sprintf("Authorized context asset [%s/%s]", record.AssetType, record.AssetID)
		return base
	}
}

func buildPersonaPayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	return map[string]any{
		"id":                  defaultString(record.AssetID, "persona.default"),
		"name":                defaultString(record.AssetName, record.AssetID),
		"role":                firstNonEmptySectionText(sections, "role", "角色", "定位"),
		"communication_style": firstNonEmptySectionText(sections, "communication_style", "style", "语气", "表达风格"),
		"thinking_framework":  firstNonEmptySectionText(sections, "thinking_framework", "framework", "思考框架"),
		"bottom_lines":        uniqueNonEmpty(append(sectionBullets(sections, "bottom_lines", "底线", "边界"), hardConstraintLines(source)...)),
		"examples":            sectionBullets(sections, "examples", "示例", "例子"),
		"summary":             summarizeSource(source),
	}
}

func buildAgentProfilePayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	return map[string]any{
		"id":                     defaultString(record.AssetID, "agent_profile.default"),
		"operational_discipline": uniqueNonEmpty(append(append(sectionBullets(sections, "operational_discipline", "discipline", "rules", "守则"), sectionBullets(sections, "process", "流程", "步骤")...), allBullets(sections)...)),
		"spec_references":        sectionRefs(sections, "spec_references", "specs", "references", "引用"),
		"uncertainty_policy": map[string]any{
			"required_fields": uniqueNonEmpty(sectionBullets(sections, "uncertainty_policy", "uncertainty", "不确定性处理")),
		},
		"orchestration_flow": map[string]any{
			"resident_first": true,
			"phases":         sectionBullets(sections, "orchestration_flow", "flow", "phases", "流程", "阶段"),
		},
		"context_loading_policy": map[string]any{
			"resident":   sectionBullets(sections, "resident", "resident_assets", "常驻"),
			"on_demand":  sectionBullets(sections, "on_demand", "按需"),
			"background": sectionBullets(sections, "background", "后台"),
		},
		"skill_trigger_map":    sectionRows(sections, "skill_trigger_map", "skills", "skill_triggers"),
		"degradation_policy":   sectionKeyValueMap(sections, "degradation_policy", "degradation", "降级"),
		"non_degradable_items": uniqueNonEmpty(append(sectionBullets(sections, "non_degradable_items", "不可降级", "must_keep"), hardConstraintLines(source)...)),
		"summary":              summarizeSource(source),
	}
}

func buildPolicyRulePayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	return map[string]any{
		"rule_id":        defaultString(record.AssetID, record.AssetName),
		"title":          defaultString(record.AssetName, record.AssetID),
		"scope":          ruleScopeFromAssetID(record.AssetID),
		"severity":       defaultString(valueAsString(frontmatterMap(record)["severity"]), "medium"),
		"checkpoints":    uniqueNonEmpty(append(stringSliceFromAny(frontmatterMap(record)["checkpoints"]), sectionBullets(sections, "scope")...)),
		"on_fail":        defaultString(valueAsString(frontmatterMap(record)["on_fail"]), "ask"),
		"hard_gates":     uniqueNonEmpty(append(sectionBullets(sections, "hard_gates"), hardConstraintLines(source)...)),
		"check_rules":    uniqueNonEmpty(sectionBullets(sections, "check_rules")),
		"guidance_lines": uniqueNonEmpty(sectionBullets(sections, "guidance")),
		"examples":       uniqueNonEmpty(sectionBullets(sections, "examples")),
		"summary":        summarizeSource(source),
	}
}

func buildUserProfilePayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	return map[string]any{
		"id":                 defaultString(record.AssetID, record.AssetName),
		"identity_summary":   defaultString(firstNonEmptySectionText(sections, "identity_summary", "identity", "身份"), rootSummary(sections)),
		"role_summary":       firstNonEmptySectionText(sections, "role_summary", "role", "角色"),
		"preference_summary": firstNonEmptySectionText(sections, "preference_summary", "preferences", "偏好"),
		"domain_context":     firstNonEmptySectionText(sections, "domain_context", "context", "领域背景"),
		"long_term_goals":    sectionBullets(sections, "long_term_goals", "goals", "长期目标"),
		"constraints":        uniqueNonEmpty(append(sectionBullets(sections, "constraints", "限制"), hardConstraintLines(source)...)),
		"summary":            summarizeSource(source),
	}
}

func buildMemoryViewPayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	return map[string]any{
		"id":               defaultString(record.AssetID, record.AssetName),
		"summary":          defaultString(firstNonEmptySectionText(sections, "summary", "overview", "概述", "摘要", "recent summary"), rootSummary(sections)),
		"facts":            sectionBullets(sections, "facts", "事实"),
		"preferences":      sectionBullets(sections, "preferences", "偏好"),
		"constraints":      uniqueNonEmpty(append(sectionBullets(sections, "constraints", "限制"), hardConstraintLines(source)...)),
		"recent_decisions": sectionBullets(sections, "recent_decisions", "decisions", "近期决策"),
		"retrieval_refs":   sectionRefs(sections, "retrieval_refs", "refs", "references", "检索引用"),
	}
}

func buildScenePayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	return map[string]any{
		"scene_id":                 strings.TrimPrefix(strings.TrimSpace(record.AssetID), "scene."),
		"title":                    defaultString(record.AssetName, record.AssetID),
		"description":              summarizeSource(source),
		"summary":                  defaultString(valueAsString(frontmatterMap(record)["summary"]), summarizeSource(source)),
		"keywords":                 uniqueNonEmpty(sectionBullets(sections, "when_it_applies")),
		"suggested_questions":      firstNStrings(sectionBullets(sections, "examples"), 3),
		"default_workflow_ref":     "workflow." + strings.TrimPrefix(strings.TrimSpace(record.AssetID), "scene.") + ".main",
		"default_contract_refs":    filterRefs(sectionBullets(sections, "default_assets"), "contract."),
		"default_skill_refs":       filterRefs(sectionBullets(sections, "default_assets"), "skill."),
		"default_policy_rule_refs": filterRefs(sectionBullets(sections, "default_assets"), "policy_rule."),
		"fallback_allowed":         strings.HasSuffix(strings.TrimSpace(record.AssetID), ".default"),
	}
}

func buildWorkflowPayload(record storedSystemResource) map[string]any {
	payload := cloneAnyMap(yamlMap(record))
	if payload == nil {
		payload = map[string]any{}
	}
	if _, ok := payload["workflow_id"]; !ok {
		payload["workflow_id"] = strings.TrimSpace(record.AssetID)
	}
	payload["scene_id"] = sceneIDFromAssetID(record.AssetID)
	payload["stage_order"] = stageOrderFromWorkflow(payload)
	return payload
}

func buildContractPayload(record storedSystemResource) map[string]any {
	payload := cloneAnyMap(yamlMap(record))
	if payload == nil {
		payload = map[string]any{}
	}
	payload["contract_id"] = strings.TrimSpace(record.AssetID)
	payload["scene_id"] = sceneIDFromAssetID(record.AssetID)
	return payload
}

func buildSkillPayload(record storedSystemResource, source string) map[string]any {
	sections := parseMarkdownSections(source)
	meta := frontmatterMap(record)
	return map[string]any{
		"skill_id":            strings.TrimSpace(record.AssetID),
		"scene_id":            sceneIDFromAssetID(record.AssetID),
		"name":                defaultString(valueAsString(meta["name"]), record.AssetName),
		"description":         defaultString(valueAsString(meta["description"]), valueAsString(meta["summary"])),
		"allowed_tools":       stringSliceFromAny(meta["allowed_tools"]),
		"depends_on":          stringSliceFromAny(meta["depends_on"]),
		"guidance":            summarizeSource(source),
		"trigger_conditions":  sectionBullets(sections, "when_to_use"),
		"process":             uniqueNonEmpty(sectionBullets(sections, "process")),
		"output_contracts":    filterRefs(append(stringSliceFromAny(meta["depends_on"]), sectionBullets(sections, "output")...), "contract."),
		"red_flags":           uniqueNonEmpty(sectionBullets(sections, "red_flags")),
		"scripts_manifest":    cloneAnySliceMap(record.Metadata["scripts_manifest"]),
		"references_manifest": cloneAnySliceMap(record.Metadata["references_manifest"]),
		"assets_manifest":     cloneAnySliceMap(record.Metadata["assets_manifest"]),
	}
}

func summarizeSource(source string) string {
	lines := compactLines(strings.Split(source, "\n"))
	if len(lines) == 0 {
		return ""
	}
	preview := strings.Join(firstN(lines, 3), " ")
	if len(preview) > 220 {
		return preview[:220]
	}
	return preview
}

func sourceContentForPayload(record storedSystemResource, source string) string {
	switch normalizeAssetType(record.AssetType) {
	case "workflow", "contract":
		return strings.TrimSpace(source)
	default:
		doc := systemtruth.ParseMarkdownDocumentText(record.SourcePath, source)
		if strings.TrimSpace(doc.Body) != "" {
			return doc.Body
		}
		return strings.TrimSpace(source)
	}
}

func normalizeAssetType(input string) string {
	return strings.TrimSpace(strings.ToLower(input))
}

type markdownSection struct {
	Title   string
	Content []string
}

func parseMarkdownSections(source string) map[string]markdownSection {
	result := map[string]markdownSection{}
	currentKey := "__root__"
	current := markdownSection{Title: "__root__"}
	lines := strings.Split(source, "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if len(current.Content) > 0 || current.Title != "__root__" {
				result[currentKey] = current
			}
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			currentKey = normalizeHeadingKey(title)
			current = markdownSection{Title: title}
			continue
		}
		current.Content = append(current.Content, line)
	}
	if len(current.Content) > 0 || current.Title != "__root__" {
		result[currentKey] = current
	}
	return result
}

func normalizeHeadingKey(input string) string {
	key := strings.ToLower(strings.TrimSpace(input))
	replacer := strings.NewReplacer(" ", "_", "-", "_", "/", "_", "\\", "_", "：", "_", ":", "_")
	return replacer.Replace(key)
}

func sectionContent(sections map[string]markdownSection, keys ...string) []string {
	for _, key := range keys {
		if section, ok := sections[normalizeHeadingKey(key)]; ok && len(section.Content) > 0 {
			return append([]string(nil), section.Content...)
		}
	}
	return nil
}

func rootSummary(sections map[string]markdownSection) string {
	lines := sectionContent(sections, "__root__")
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " ")
}

func rootBullets(sections map[string]markdownSection) []string {
	return sectionBullets(sections, "__root__")
}

func allBullets(sections map[string]markdownSection) []string {
	result := make([]string, 0)
	for key := range sections {
		result = append(result, sectionBullets(sections, key)...)
	}
	return uniqueNonEmpty(result)
}

func sectionBullets(sections map[string]markdownSection, keys ...string) []string {
	lines := sectionContent(sections, keys...)
	if len(lines) == 0 {
		return nil
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") {
			result = append(result, strings.TrimSpace(line[2:]))
			continue
		}
		if numberedPrefixLength(line) > 0 {
			result = append(result, strings.TrimSpace(line[numberedPrefixLength(line):]))
		}
	}
	if len(result) == 0 {
		return lines
	}
	return uniqueNonEmpty(result)
}

func numberedPrefixLength(line string) int {
	for idx, ch := range line {
		if ch < '0' || ch > '9' {
			if ch == '.' && idx > 0 && idx+1 < len(line) && line[idx+1] == ' ' {
				return idx + 2
			}
			return 0
		}
	}
	return 0
}

func firstNonEmptySectionText(sections map[string]markdownSection, keys ...string) string {
	lines := sectionContent(sections, keys...)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " ")
}

func sectionRefs(sections map[string]markdownSection, keys ...string) []map[string]any {
	items := sectionBullets(sections, keys...)
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"ref": item})
	}
	return result
}

func sectionRows(sections map[string]markdownSection, keys ...string) []map[string]any {
	items := sectionBullets(sections, keys...)
	if len(items) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(items))
	for _, item := range items {
		result = append(result, map[string]any{"value": item})
	}
	return result
}

func sectionKeyValueMap(sections map[string]markdownSection, keys ...string) map[string]any {
	lines := sectionContent(sections, keys...)
	if len(lines) == 0 {
		return nil
	}
	result := map[string]any{}
	for _, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok {
			key, value, ok = strings.Cut(line, "：")
		}
		if !ok {
			continue
		}
		key = normalizeHeadingKey(key)
		value = strings.TrimSpace(value)
		if key != "" && value != "" {
			result[key] = value
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func buildSectionObjects(sections map[string]markdownSection) []map[string]any {
	result := make([]map[string]any, 0, len(sections))
	for key, section := range sections {
		if key == "__root__" {
			continue
		}
		result = append(result, map[string]any{
			"section_id": key,
			"title":      section.Title,
			"content":    strings.Join(section.Content, "\n"),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		return valueAsString(result[i]["section_id"]) < valueAsString(result[j]["section_id"])
	})
	return result
}

func hardConstraintLines(source string) []string {
	lines := compactLines(strings.Split(source, "\n"))
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "must ") || strings.HasPrefix(lower, "must not ") || strings.HasPrefix(lower, "no ") || strings.HasPrefix(lower, "cannot ") {
			result = append(result, line)
		}
	}
	return uniqueNonEmpty(result)
}

func uniqueNonEmpty(items []string) []string {
	if len(items) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func stringSliceFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return uniqueNonEmpty(typed)
	case []any:
		result := make([]string, 0, len(typed))
		for _, item := range typed {
			if text := strings.TrimSpace(valueAsString(item)); text != "" {
				result = append(result, text)
			}
		}
		return uniqueNonEmpty(result)
	default:
		return nil
	}
}

func valueAsString(value any) string {
	if value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", value))
	}
}

func frontmatterMap(record storedSystemResource) map[string]any {
	return decodeStoredMap(record.Metadata["frontmatter"])
}

func yamlMap(record storedSystemResource) map[string]any {
	return decodeStoredMap(record.Metadata["parsed_yaml"])
}

func decodeStoredMap(value any) map[string]any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case nil:
		return nil
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var result map[string]any
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil
		}
		return result
	}
}

func cloneAnySliceMap(value any) []map[string]any {
	switch typed := value.(type) {
	case []map[string]any:
		return cloneAnySlice(typed)
	case nil:
		return nil
	default:
		payload, err := json.Marshal(typed)
		if err != nil {
			return nil
		}
		var result []map[string]any
		if err := json.Unmarshal(payload, &result); err != nil {
			return nil
		}
		return result
	}
}

func valueAsInt(value any, fallback int) int {
	switch typed := value.(type) {
	case int:
		return typed
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if number, err := typed.Int64(); err == nil {
			return int(number)
		}
	case string:
		if typed = strings.TrimSpace(typed); typed != "" {
			var parsed int
			if _, err := fmt.Sscanf(typed, "%d", &parsed); err == nil {
				return parsed
			}
		}
	}
	return fallback
}

func mergePayload(base map[string]any, extra map[string]any) map[string]any {
	if len(extra) == 0 {
		return base
	}
	for key, value := range extra {
		base[key] = value
	}
	base["guidance"] = fmt.Sprintf("Compiled context asset [%s/%s]", valueAsString(base["asset_type"]), valueAsString(base["asset_id"]))
	return base
}

func filterRefs(items []string, prefix string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if strings.HasPrefix(item, prefix) {
			result = append(result, item)
		}
	}
	return uniqueNonEmpty(result)
}

func ruleScopeFromAssetID(assetID string) string {
	if strings.HasPrefix(strings.TrimSpace(assetID), "policy_rule.core.") {
		return "core"
	}
	return "scene"
}

func stageOrderFromWorkflow(payload map[string]any) []string {
	items, ok := payload["stages"].([]any)
	if !ok {
		return nil
	}
	result := make([]string, 0, len(items))
	for _, item := range items {
		stage, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if id := strings.TrimSpace(valueAsString(stage["id"])); id != "" {
			result = append(result, id)
		}
	}
	return uniqueNonEmpty(result)
}

func firstNStrings(items []string, limit int) []string {
	if limit <= 0 || len(items) <= limit {
		return append([]string(nil), items...)
	}
	return append([]string(nil), items[:limit]...)
}

func sanitizeAssetID(input string) string {
	input = strings.TrimSpace(strings.ToLower(input))
	input = strings.ReplaceAll(input, " ", "_")
	input = strings.ReplaceAll(input, "/", "_")
	input = strings.ReplaceAll(input, "\\", "_")
	return input
}

func newResourceVersion() string {
	return time.Now().UTC().Format("20060102T150405.000000000Z")
}

func compactLines(lines []string) []string {
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result = append(result, line)
		}
	}
	return result
}

func countHeadings(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "#") {
			count++
		}
	}
	return count
}

func firstN(lines []string, limit int) []string {
	if limit <= 0 || len(lines) <= limit {
		return append([]string(nil), lines...)
	}
	return append([]string(nil), lines[:limit]...)
}

func checksum(input string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(input)))
	return hex.EncodeToString(sum[:])
}

func writeJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomically(path, payload, 0o644)
}

func readJSON(path string, target any) error {
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(payload)) == 0 {
		return os.ErrNotExist
	}
	if err := json.Unmarshal(payload, target); err != nil {
		return fmt.Errorf("read json %s: %w", filepath.Clean(path), err)
	}
	return nil
}

func writeFileAtomically(path string, payload []byte, mode os.FileMode) error {
	path = filepath.Clean(path)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(mode); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpPath, path)
}

func mustJSON(value any) []byte {
	payload, _ := json.MarshalIndent(value, "", "  ")
	return payload
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

func cloneAnySlice(input []map[string]any) []map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(input))
	for _, item := range input {
		result = append(result, cloneAnyMap(item))
	}
	return result
}

func defaultString(value, fallback string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return fallback
	}
	return value
}

func errorsIsNotExist(err error) bool {
	return err != nil && errorsOrContains(err, fs.ErrNotExist)
}

func errorsOrContains(err error, target error) bool {
	if err == nil {
		return false
	}
	if os.IsNotExist(err) || strings.Contains(strings.ToLower(err.Error()), "no such file") {
		return true
	}
	return false
}
