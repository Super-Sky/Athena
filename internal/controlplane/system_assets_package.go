package controlplane

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CompiledAssetsPackageManifest captures one build-time compiled asset package snapshot.
// CompiledAssetsPackageManifest 描述一份构建期编译资产包快照。
type CompiledAssetsPackageManifest struct {
	OutputDir       string                      `json:"output_dir,omitempty"`
	TruthDirPath    string                      `json:"truth_dir_path,omitempty"`
	TruthDirVersion string                      `json:"truth_dir_version,omitempty"`
	BuiltAt         string                      `json:"built_at,omitempty"`
	AssetCount      int                         `json:"asset_count,omitempty"`
	Assets          []CompiledAssetsPackageItem `json:"assets,omitempty"`
}

// CompiledAssetsPackageItem captures one asset entry inside the build-time package.
// CompiledAssetsPackageItem 描述构建期资产包中的单条资产条目。
type CompiledAssetsPackageItem struct {
	AssetID          string `json:"asset_id,omitempty"`
	AssetType        string `json:"asset_type,omitempty"`
	AssetName        string `json:"asset_name,omitempty"`
	SourcePath       string `json:"source_path,omitempty"`
	CompiledVersion  string `json:"compiled_version,omitempty"`
	CompiledChecksum string `json:"compiled_checksum,omitempty"`
	TruthDirVersion  string `json:"truth_dir_version,omitempty"`
	PackageFile      string `json:"package_file,omitempty"`
}

type compiledAssetPackageFile struct {
	Detail        SystemResourceDetail        `json:"detail"`
	CompileResult SystemResourceCompileResult `json:"compile_result"`
}

// BuildCompiledAssetsPackage syncs source markdown, verifies compiled results, and writes one build artifact package.
// BuildCompiledAssetsPackage 会同步 markdown 主源、校验编译结果，并写出一份构建期资产包。
func (m *Manager) BuildCompiledAssetsPackage(ctx context.Context, outputDir string) (CompiledAssetsPackageManifest, error) {
	if m == nil {
		return CompiledAssetsPackageManifest{}, nil
	}
	outputDir = strings.TrimSpace(outputDir)
	if outputDir == "" {
		return CompiledAssetsPackageManifest{}, fmt.Errorf("output dir is required")
	}
	if err := m.SyncSystemSources(ctx); err != nil {
		return CompiledAssetsPackageManifest{}, err
	}
	if err := os.RemoveAll(outputDir); err != nil {
		return CompiledAssetsPackageManifest{}, err
	}
	assetsDir := filepath.Join(outputDir, "assets")
	if err := os.MkdirAll(assetsDir, 0o755); err != nil {
		return CompiledAssetsPackageManifest{}, err
	}
	items, err := m.ListSystemResources(ctx)
	if err != nil {
		return CompiledAssetsPackageManifest{}, err
	}
	manifest := CompiledAssetsPackageManifest{
		OutputDir:    outputDir,
		TruthDirPath: strings.TrimSpace(m.truthDir),
		BuiltAt:      time.Now().UTC().Format(time.RFC3339),
		Assets:       make([]CompiledAssetsPackageItem, 0, len(items)),
	}
	truthInfo, err := m.TruthDirInfo(ctx)
	if err == nil {
		manifest.TruthDirVersion = truthInfo.Version
	}
	for _, item := range items {
		detail, err := m.GetSystemResource(ctx, item.AssetID)
		if err != nil {
			return CompiledAssetsPackageManifest{}, err
		}
		compileResult, err := m.GetSystemResourceCompileResult(ctx, item.AssetID)
		if err != nil {
			return CompiledAssetsPackageManifest{}, err
		}
		if strings.TrimSpace(compileResult.Status) == "" {
			return CompiledAssetsPackageManifest{}, fmt.Errorf("system resource %q has no compiled result", item.AssetID)
		}
		fileName := sanitizeAssetID(item.AssetID) + ".json"
		filePath := filepath.Join(assetsDir, fileName)
		payload := compiledAssetPackageFile{
			Detail:        detail,
			CompileResult: compileResult,
		}
		if err := writeJSON(filePath, payload); err != nil {
			return CompiledAssetsPackageManifest{}, err
		}
		manifest.Assets = append(manifest.Assets, CompiledAssetsPackageItem{
			AssetID:          item.AssetID,
			AssetType:        item.AssetType,
			AssetName:        item.AssetName,
			SourcePath:       detail.SourcePath,
			CompiledVersion:  compileResult.CompiledVersion,
			CompiledChecksum: compileResult.CompiledChecksum,
			TruthDirVersion:  compileResult.TruthDirVersion,
			PackageFile:      filepath.ToSlash(filepath.Join("assets", fileName)),
		})
	}
	manifest.AssetCount = len(manifest.Assets)
	if err := writeJSON(filepath.Join(outputDir, "manifest.json"), manifest); err != nil {
		return CompiledAssetsPackageManifest{}, err
	}
	return manifest, nil
}
