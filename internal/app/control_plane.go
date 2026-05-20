// control_plane.go exposes app-layer helpers and use cases for the standalone control plane.
// control_plane.go 负责暴露独立控制面的 app 层辅助方法和用例。
package app

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"moss/internal/contextassets"
	"moss/internal/controlplane"
	runtimescene "moss/internal/runtime/scene"
	"moss/internal/skills"
	"moss/internal/tools"
)

// ControlPlaneBootstrap returns the combined control-plane bootstrap payload.
// ControlPlaneBootstrap 返回组合后的控制面启动载荷。
func (s *Service) ControlPlaneBootstrap(ctx context.Context, swaggerSpecURL string) (*controlplane.BootstrapPayload, error) {
	if s.ControlPlane == nil {
		return &controlplane.BootstrapPayload{
			Scenes:          []controlplane.SceneConfig{},
			Skills:          []controlplane.SkillConfig{},
			Tools:           []controlplane.ToolConfig{},
			SystemResources: []controlplane.SystemResourceSummary{},
			ConfigVersions:  []controlplane.ConfigVersionSummary{},
			Runtime:         controlplane.DefaultRuntimeTuning(),
			Governance:      controlplane.DefaultRuntimeTuning(),
			SwaggerSpecURL:  swaggerSpecURL,
		}, nil
	}
	skillRegistry, err := effectiveSkillRegistry(s.ControlPlane, s.SkillLoader)
	if err != nil {
		return nil, err
	}
	toolDefs, err := visibleToolDefinitions(skillRegistry.List())
	if err != nil {
		return nil, err
	}
	systemResources, err := s.ControlPlane.ListSystemResources(ctx)
	if err != nil {
		return nil, err
	}
	payload, err := s.ControlPlane.Bootstrap(ctx, skillRegistry.List(), toolDefs, swaggerSpecURL)
	if err != nil {
		return nil, err
	}
	payload.SystemResources = systemResources
	return payload, nil
}

// GetControlPlaneAuthStatus returns the current auth and lock state for one client.
// GetControlPlaneAuthStatus 返回当前客户端的认证与锁定状态。
func (s *Service) GetControlPlaneAuthStatus(ctx context.Context, sessionID, remoteIP string) (controlplane.AuthStatus, error) {
	if s.ControlPlane == nil {
		return controlplane.AuthStatus{}, nil
	}
	return s.ControlPlane.AuthStatus(ctx, sessionID, remoteIP)
}

// LoginControlPlane validates the configured control-plane token and opens one auth session.
// LoginControlPlane 校验控制面 token 并创建一条认证会话。
func (s *Service) LoginControlPlane(ctx context.Context, token, remoteIP string) (controlplane.AuthStatus, string, error) {
	if s.ControlPlane == nil {
		return controlplane.AuthStatus{}, "", nil
	}
	return s.ControlPlane.Login(ctx, token, remoteIP)
}

// LogoutControlPlane closes one existing control-plane auth session.
// LogoutControlPlane 会关闭一条已有控制面认证会话。
func (s *Service) LogoutControlPlane(ctx context.Context, sessionID, remoteIP string) (controlplane.AuthStatus, error) {
	if s.ControlPlane == nil {
		return controlplane.AuthStatus{}, nil
	}
	return s.ControlPlane.Logout(ctx, sessionID, remoteIP)
}

// ListControlPlaneScenes returns the effective scene catalog.
// ListControlPlaneScenes 返回有效场景目录。
func (s *Service) ListControlPlaneScenes(ctx context.Context) ([]controlplane.SceneConfig, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListScenes(ctx)
}

// UpdateControlPlaneScene updates one scene override and reloads runtime state.
// UpdateControlPlaneScene 会更新一条场景 override 并刷新 runtime 状态。
func (s *Service) UpdateControlPlaneScene(ctx context.Context, id string, input controlplane.SceneConfig) (controlplane.SceneConfig, error) {
	item, err := s.ControlPlane.PutScene(ctx, id, input)
	if err != nil {
		return controlplane.SceneConfig{}, err
	}
	return item, s.reloadSkillRegistry(ctx)
}

// ListControlPlaneSkills returns the effective skill list.
// ListControlPlaneSkills 返回有效 skill 列表。
func (s *Service) ListControlPlaneSkills(ctx context.Context) ([]controlplane.SkillConfig, error) {
	registry, err := effectiveSkillRegistry(s.ControlPlane, s.SkillLoader)
	if err != nil {
		return nil, err
	}
	return s.ControlPlane.ListSkills(ctx, registry.List())
}

// ListControlPlaneTools returns the effective tool registry metadata.
// ListControlPlaneTools 返回有效 tool registry 元数据。
func (s *Service) ListControlPlaneTools(ctx context.Context) ([]controlplane.ToolConfig, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	skillRegistry, err := effectiveSkillRegistry(s.ControlPlane, s.SkillLoader)
	if err != nil {
		return nil, err
	}
	defs, err := visibleToolDefinitions(skillRegistry.List())
	if err != nil {
		return nil, err
	}
	return s.ControlPlane.ListTools(ctx, defs)
}

// UpdateControlPlaneTool updates one tool override.
// UpdateControlPlaneTool 会更新一条 tool override。
func (s *Service) UpdateControlPlaneTool(ctx context.Context, name string, input controlplane.ToolConfig) (controlplane.ToolConfig, error) {
	if s.ControlPlane == nil {
		return controlplane.ToolConfig{}, nil
	}
	return s.ControlPlane.PutTool(ctx, name, input)
}

// UpdateControlPlaneSkill updates one skill override and reloads runtime state.
// UpdateControlPlaneSkill 会更新一条 skill override 并刷新 runtime 状态。
func (s *Service) UpdateControlPlaneSkill(ctx context.Context, name string, input controlplane.SkillConfig) (controlplane.SkillConfig, error) {
	item, err := s.ControlPlane.PutSkill(ctx, name, input)
	if err != nil {
		return controlplane.SkillConfig{}, err
	}
	return item, s.reloadSkillRegistry(ctx)
}

// GetControlPlaneRuntime returns the effective runtime tuning.
// GetControlPlaneRuntime 返回有效运行开关。
func (s *Service) GetControlPlaneRuntime(ctx context.Context) (controlplane.RuntimeTuning, error) {
	if s.ControlPlane == nil {
		return controlplane.DefaultRuntimeTuning(), nil
	}
	return s.ControlPlane.Runtime(ctx)
}

// UpdateControlPlaneRuntime updates the runtime tuning.
// UpdateControlPlaneRuntime 会更新运行开关。
func (s *Service) UpdateControlPlaneRuntime(ctx context.Context, input controlplane.RuntimeTuning) (controlplane.RuntimeTuning, error) {
	if s.ControlPlane == nil {
		return controlplane.DefaultRuntimeTuning(), nil
	}
	return s.ControlPlane.PutRuntime(ctx, input)
}

// GetControlPlaneGovernance returns the effective governance configuration.
// GetControlPlaneGovernance 返回有效治理配置。
func (s *Service) GetControlPlaneGovernance(ctx context.Context) (controlplane.GovernanceConfig, error) {
	if s.ControlPlane == nil {
		return controlplane.DefaultRuntimeTuning(), nil
	}
	return s.ControlPlane.Governance(ctx)
}

// UpdateControlPlaneGovernance updates the governance configuration.
// UpdateControlPlaneGovernance 会更新治理配置。
func (s *Service) UpdateControlPlaneGovernance(ctx context.Context, input controlplane.GovernanceConfig) (controlplane.GovernanceConfig, error) {
	if s.ControlPlane == nil {
		return controlplane.DefaultRuntimeTuning(), nil
	}
	return s.ControlPlane.PutGovernance(ctx, input)
}

// EffectiveToolGovernancePolicy returns the active compiled tool governance policy.
// EffectiveToolGovernancePolicy 返回当前生效的编译后 tool 治理策略。
func (s *Service) EffectiveToolGovernancePolicy(ctx context.Context) (controlplane.ToolGovernancePolicy, error) {
	if s.ControlPlane == nil {
		return controlplane.ToolGovernancePolicy{}, nil
	}
	return s.ControlPlane.EffectiveToolGovernancePolicy(ctx)
}

// EvaluateToolGovernance evaluates and persists one pre-execution tool governance decision.
// EvaluateToolGovernance 会判定并持久化一次 tool 执行前治理决策。
func (s *Service) EvaluateToolGovernance(ctx context.Context, input controlplane.ToolGovernanceDecisionRequest) (controlplane.ToolGovernanceDecision, error) {
	if s.ControlPlane == nil {
		return controlplane.ToolGovernanceDecision{}, nil
	}
	return s.ControlPlane.EvaluateToolGovernance(ctx, input)
}

// ListToolGovernanceDecisions returns recent persisted tool governance decisions.
// ListToolGovernanceDecisions 返回最近持久化的 tool 治理判定。
func (s *Service) ListToolGovernanceDecisions(ctx context.Context) ([]controlplane.ToolGovernanceDecision, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListToolGovernanceDecisions(ctx)
}

// ListControlPlaneConfigVersions returns the persisted version summaries.
// ListControlPlaneConfigVersions 返回持久化版本摘要。
func (s *Service) ListControlPlaneConfigVersions(ctx context.Context) ([]controlplane.ConfigVersionSummary, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListVersions(ctx)
}

// GetControlPlaneConfigVersion returns one persisted version detail.
// GetControlPlaneConfigVersion 返回单个持久化版本详情。
func (s *Service) GetControlPlaneConfigVersion(ctx context.Context, versionID string) (controlplane.ConfigVersionDetail, error) {
	if s.ControlPlane == nil {
		return controlplane.ConfigVersionDetail{}, nil
	}
	return s.ControlPlane.GetVersion(ctx, versionID)
}

// RollbackControlPlaneConfigVersion restores one historical control-plane version.
// RollbackControlPlaneConfigVersion 会恢复一条历史控制面版本。
func (s *Service) RollbackControlPlaneConfigVersion(ctx context.Context, versionID string) (controlplane.ConfigVersionDetail, error) {
	if s.ControlPlane == nil {
		return controlplane.ConfigVersionDetail{}, nil
	}
	return s.ControlPlane.RollbackVersion(ctx, versionID)
}

// ListSystemResources returns the active truth-dir system resource catalog with detail payloads.
// ListSystemResources 返回 active truth dir 的 system resource 详情目录。
func (s *Service) ListSystemResources(ctx context.Context) ([]controlplane.SystemResourceDetail, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListSystemResourceDetails(ctx)
}

// SyncSystemResources explicitly scans truth-dir markdown sources and refreshes the catalog.
// SyncSystemResources 会显式遍历 truth-dir markdown 主源并刷新资源目录。
func (s *Service) SyncSystemResources(ctx context.Context) ([]controlplane.SystemResourceDetail, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	items, err := s.ControlPlane.SyncSystemResources(ctx)
	if err != nil {
		return nil, err
	}
	if err := s.syncRuntimeContractFoundation(ctx); err != nil {
		return nil, err
	}
	return items, nil
}

// BuildSystemAssetsPackage writes one build-time compiled asset package.
// BuildSystemAssetsPackage 会写出一份构建期编译资产包。
func (s *Service) BuildSystemAssetsPackage(ctx context.Context) (controlplane.CompiledAssetsPackageManifest, error) {
	if s.ControlPlane == nil {
		return controlplane.CompiledAssetsPackageManifest{}, nil
	}
	outputDir := s.Config.System.CompiledAssetsDir
	if outputDir == "" {
		outputDir = filepath.Join("output", "system-assets")
	}
	return s.ControlPlane.BuildCompiledAssetsPackage(ctx, outputDir)
}

// DefaultSystemContextAssets returns the default system asset bindings sourced from the active truth dir.
// DefaultSystemContextAssets 返回来自 active truth dir 的默认 system asset bindings。
func (s *Service) DefaultSystemContextAssets(ctx context.Context) ([]contextassets.Asset, error) {
	if s == nil || s.ControlPlane == nil {
		return nil, nil
	}
	items, err := s.ControlPlane.ListSystemResourceDetails(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(items, func(i, j int) bool {
		left := strings.TrimSpace(items[i].SourcePath)
		right := strings.TrimSpace(items[j].SourcePath)
		if left == right {
			return strings.TrimSpace(items[i].AssetID) < strings.TrimSpace(items[j].AssetID)
		}
		if left == "" {
			return false
		}
		if right == "" {
			return true
		}
		return left < right
	})

	result := make([]contextassets.Asset, 0, len(items))
	for _, item := range items {
		if !shouldDefaultBindSystemResource(item) {
			continue
		}
		compileResult, err := s.ControlPlane.GetSystemResourceCompileResult(ctx, item.AssetID)
		if err != nil {
			return nil, err
		}
		if strings.TrimSpace(compileResult.CompiledVersion) == "" {
			continue
		}
		result = append(result, buildDefaultSystemContextAsset(item, compileResult))
	}
	return result, nil
}

// GetSystemResource returns one system resource detail.
// GetSystemResource 返回单条 system resource 详情。
func (s *Service) GetSystemResource(ctx context.Context, assetID string) (controlplane.SystemResourceDetail, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceDetail{}, nil
	}
	return s.ControlPlane.GetSystemResource(ctx, assetID)
}

func shouldDefaultBindSystemResource(item controlplane.SystemResourceDetail) bool {
	if strings.TrimSpace(item.AssetID) == "" {
		return false
	}
	if strings.TrimSpace(item.SourceKind) != "truth_dir_source" {
		return false
	}
	switch strings.TrimSpace(item.AssetType) {
	case "persona", "agent_profile", "user_profile", "memory_view":
		return true
	case "policy_rule":
		return strings.HasPrefix(strings.TrimSpace(item.AssetID), "policy_rule.core.")
	default:
		return false
	}
}

func buildDefaultSystemContextAsset(item controlplane.SystemResourceDetail, compileResult controlplane.SystemResourceCompileResult) contextassets.Asset {
	priority, residentHint, allowDetailFetch := defaultSystemContextAssetPolicy(item.AssetType)
	tags := []string{"system_default"}
	if residentHint {
		tags = append(tags, "resident")
	}
	return contextassets.Asset{
		AssetID:           strings.TrimSpace(item.AssetID),
		AssetType:         strings.TrimSpace(item.AssetType),
		AssetName:         strings.TrimSpace(item.AssetName),
		Scope:             fallbackString(strings.TrimSpace(item.Scope), "system"),
		SourceKind:        fallbackString(strings.TrimSpace(item.SourceKind), "truth_dir_source"),
		Mode:              "ref",
		Priority:          priority,
		ReadOnly:          item.ReadOnly,
		CandidateWritable: false,
		AuthScope:         "system_default",
		Ref: &contextassets.Ref{
			RefType:         "compiled_asset",
			Target:          strings.TrimSpace(item.AssetID),
			Version:         strings.TrimSpace(compileResult.CompiledVersion),
			Checksum:        fallbackString(strings.TrimSpace(compileResult.CompiledChecksum), strings.TrimSpace(compileResult.SourceChecksum)),
			TruthDirVersion: fallbackString(strings.TrimSpace(compileResult.TruthDirVersion), strings.TrimSpace(item.TruthDirVersion)),
			DetailEndpoint:  fmt.Sprintf("/api/system-resources/%s/compile-result", strings.TrimSpace(item.AssetID)),
		},
		Resolution: contextassets.Resolution{
			PreferCompiled:      true,
			AllowDetailFetch:    allowDetailFetch,
			AllowInlineFallback: false,
			ResidentHint:        residentHint,
			BackgroundOnly:      false,
		},
		Metadata: contextassets.Metadata{
			SourceLabel: strings.TrimSpace(item.SourcePath),
			Tags:        tags,
		},
	}
}

func defaultSystemContextAssetPolicy(assetType string) (priority int, residentHint bool, allowDetailFetch bool) {
	switch strings.TrimSpace(assetType) {
	case "policy_rule":
		return 1000, true, true
	case "persona", "agent_profile":
		return 100, true, false
	case "user_profile", "memory_view":
		return 90, true, false
	default:
		return 50, false, false
	}
}

func fallbackString(value, fallback string) string {
	if strings.TrimSpace(value) != "" {
		return strings.TrimSpace(value)
	}
	return strings.TrimSpace(fallback)
}

// ListSystemResourceVersions returns one resource's persisted version snapshots.
// ListSystemResourceVersions 返回单条资源的持久化版本快照列表。
func (s *Service) ListSystemResourceVersions(ctx context.Context, assetID string) ([]controlplane.SystemResourceVersionSummary, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListSystemResourceVersions(ctx, assetID)
}

// GetSystemResourceVersion returns one resource version snapshot.
// GetSystemResourceVersion 返回单条资源版本快照。
func (s *Service) GetSystemResourceVersion(ctx context.Context, assetID, versionID string) (controlplane.SystemResourceVersionDetail, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceVersionDetail{}, nil
	}
	return s.ControlPlane.GetSystemResourceVersion(ctx, assetID, versionID)
}

// RollbackSystemResourceVersion restores one historical resource snapshot and re-runs the pipeline.
// RollbackSystemResourceVersion 会恢复一条历史资源快照并重新执行 pipeline。
func (s *Service) RollbackSystemResourceVersion(ctx context.Context, assetID, versionID string) (controlplane.SystemResourceMutationResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceMutationResult{}, nil
	}
	return s.ControlPlane.RollbackSystemResourceVersion(ctx, assetID, versionID)
}

// ListSystemResourceAudit returns the audit-visible mutation trail for one resource.
// ListSystemResourceAudit 返回单条资源对外可见的审计轨迹。
func (s *Service) ListSystemResourceAudit(ctx context.Context, assetID string) ([]controlplane.SystemResourceAuditEntry, error) {
	if s.ControlPlane == nil {
		return nil, nil
	}
	return s.ControlPlane.ListSystemResourceAudit(ctx, assetID)
}

// CreateSystemResource creates one system resource in the active truth dir.
// CreateSystemResource 会在 active truth dir 中创建一条 system resource。
func (s *Service) CreateSystemResource(ctx context.Context, input controlplane.SystemResourceCreateRequest) (controlplane.SystemResourceMutationResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceMutationResult{}, nil
	}
	return s.ControlPlane.CreateSystemResource(ctx, input)
}

// DeleteSystemResource deletes one system resource from the active truth dir.
// DeleteSystemResource 会从 active truth dir 删除一条 system resource。
func (s *Service) DeleteSystemResource(ctx context.Context, assetID string) error {
	if s.ControlPlane == nil {
		return nil
	}
	return s.ControlPlane.DeleteSystemResource(ctx, assetID)
}

// PatchSystemResourceMetadata patches one system resource metadata block.
// PatchSystemResourceMetadata 会更新单条 system resource 的元数据块。
func (s *Service) PatchSystemResourceMetadata(ctx context.Context, assetID string, patch controlplane.SystemResourceMetadataPatch) (controlplane.SystemResourceDetail, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceDetail{}, nil
	}
	return s.ControlPlane.PatchSystemResourceMetadata(ctx, assetID, patch)
}

// GetSystemResourceSource returns one editable source body.
// GetSystemResourceSource 返回一份可编辑的 source 内容。
func (s *Service) GetSystemResourceSource(ctx context.Context, assetID string) (controlplane.SystemResourceSource, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceSource{}, nil
	}
	return s.ControlPlane.GetSystemResourceSource(ctx, assetID)
}

// UpdateSystemResourceSource updates one source body and re-runs the default pipeline.
// UpdateSystemResourceSource 会更新一份 source 内容并重新执行默认 pipeline。
func (s *Service) UpdateSystemResourceSource(ctx context.Context, assetID string, input controlplane.SystemResourceSource) (controlplane.SystemResourceMutationResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceMutationResult{}, nil
	}
	return s.ControlPlane.PutSystemResourceSource(ctx, assetID, input)
}

// ParseSystemResource re-runs one parse pipeline.
// ParseSystemResource 会重新执行一次 parse pipeline。
func (s *Service) ParseSystemResource(ctx context.Context, assetID string) (controlplane.SystemResourceMutationResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceMutationResult{}, nil
	}
	return s.ControlPlane.ParseSystemResource(ctx, assetID)
}

// CompileSystemResource re-runs compile and activate for one system resource.
// CompileSystemResource 会重新执行单条 system resource 的 compile 与 activate。
func (s *Service) CompileSystemResource(ctx context.Context, assetID string) (controlplane.SystemResourceMutationResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceMutationResult{}, nil
	}
	return s.ControlPlane.CompileSystemResource(ctx, assetID)
}

// ActivateSystemResource marks one compiled system resource as active.
// ActivateSystemResource 会把一条已编译 system resource 标记为 active。
func (s *Service) ActivateSystemResource(ctx context.Context, assetID string) (controlplane.SystemResourceMutationResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceMutationResult{}, nil
	}
	return s.ControlPlane.ActivateSystemResource(ctx, assetID)
}

// GetSystemResourcePipeline returns the latest pipeline state for one system resource.
// GetSystemResourcePipeline 返回单条 system resource 最近一次 pipeline 状态。
func (s *Service) GetSystemResourcePipeline(ctx context.Context, assetID string) (controlplane.SystemResourcePipeline, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourcePipeline{}, nil
	}
	return s.ControlPlane.GetSystemResourcePipeline(ctx, assetID)
}

// GetSystemResourceParseResult returns the latest parse result for one system resource.
// GetSystemResourceParseResult 返回单条 system resource 最近一次 parse 结果。
func (s *Service) GetSystemResourceParseResult(ctx context.Context, assetID string) (controlplane.SystemResourceParseResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceParseResult{}, nil
	}
	return s.ControlPlane.GetSystemResourceParseResult(ctx, assetID)
}

// GetSystemResourceCompileResult returns the latest compile result for one system resource.
// GetSystemResourceCompileResult 返回单条 system resource 最近一次 compile 结果。
func (s *Service) GetSystemResourceCompileResult(ctx context.Context, assetID string) (controlplane.SystemResourceCompileResult, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceCompileResult{}, nil
	}
	return s.ControlPlane.GetSystemResourceCompileResult(ctx, assetID)
}

// BuildSystemResourceDebugPayload builds one ref-first debug payload for one endpoint.
// BuildSystemResourceDebugPayload 会为某个 endpoint 构建一份 ref-first 调试载荷。
func (s *Service) BuildSystemResourceDebugPayload(ctx context.Context, assetID string, endpoint string) (controlplane.SystemResourceDebugPayload, error) {
	if s.ControlPlane == nil {
		return controlplane.SystemResourceDebugPayload{}, nil
	}
	return s.ControlPlane.BuildSystemResourceDebugPayload(ctx, assetID, endpoint)
}

// DownloadSystemResource returns one source file payload for download.
// DownloadSystemResource 返回一份 source 文件下载内容。
func (s *Service) DownloadSystemResource(ctx context.Context, assetID string) ([]byte, string, error) {
	if s.ControlPlane == nil {
		return nil, "", nil
	}
	return s.ControlPlane.DownloadSystemResource(ctx, assetID)
}

// ExportSystemResources exports the current active truth-dir snapshot.
// ExportSystemResources 导出当前 active truth dir 快照。
func (s *Service) ExportSystemResources(ctx context.Context) ([]byte, controlplane.SystemResourceExport, error) {
	if s.ControlPlane == nil {
		return nil, controlplane.SystemResourceExport{}, nil
	}
	return s.ControlPlane.ExportSystemResources(ctx)
}

// ResolveContextAssets resolves one request context asset list into runtime-ready compiled payloads.
// ResolveContextAssets 会把一组请求上下文资产解析成运行时可直接消费的编译载荷。
func (s *Service) ResolveContextAssets(ctx context.Context, assets []contextassets.Asset) ([]contextassets.ResolvedAsset, error) {
	return s.resolveContextAssets(ctx, assets)
}

// MatchScene runs scene matching with the effective control-plane scene catalog.
// MatchScene 会基于有效控制面场景目录执行场景匹配。
func (s *Service) MatchScene(ctx context.Context, input runtimescene.MatchInput) runtimescene.MatchResult {
	catalog, err := effectiveSceneCatalog(ctx, s.ControlPlane)
	if err != nil {
		return runtimescene.Match(input)
	}
	return runtimescene.MatchWithCatalog(catalog, input)
}

// SceneDefinition returns one effective scene definition when present.
// SceneDefinition 会在存在时返回一条有效场景定义。
func (s *Service) SceneDefinition(ctx context.Context, id string) (runtimescene.Definition, bool) {
	catalog, err := effectiveSceneCatalog(ctx, s.ControlPlane)
	if err != nil {
		return runtimescene.Definition{}, false
	}
	return runtimescene.FindDefinition(catalog, id)
}

func effectiveSceneCatalog(ctx context.Context, manager *controlplane.Manager) ([]runtimescene.Definition, error) {
	if manager == nil {
		return runtimescene.BuiltinCatalog(), nil
	}
	items, err := manager.ListScenes(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]runtimescene.Definition, 0, len(items))
	for _, item := range items {
		result = append(result, runtimescene.Definition{
			ID:                 item.ID,
			Description:        item.Description,
			Keywords:           append([]string(nil), item.Keywords...),
			DefaultSkills:      append([]string(nil), item.DefaultSkills...),
			SuggestedQuestions: append([]string(nil), item.SuggestedQuestions...),
			Enabled:            item.Enabled,
			MatchScore:         item.MatchScore,
		})
	}
	return result, nil
}

func effectiveSkillRegistry(manager *controlplane.Manager, loader skills.Loader) (*skills.Registry, error) {
	registry, err := loader.Load(context.Background())
	if err != nil {
		return nil, err
	}
	if manager == nil {
		return registry, nil
	}
	defs, err := manager.ApplySkillOverrides(context.Background(), registry.List())
	if err != nil {
		return nil, err
	}
	return skills.NewRegistryFromDefinitions(defs), nil
}

func visibleToolDefinitions(skillDefs []skills.Definition) ([]tools.Definition, error) {
	referenced := make(map[string]struct{})
	for _, def := range skillDefs {
		for _, toolName := range def.ToolNames {
			toolName = strings.TrimSpace(toolName)
			if toolName != "" {
				referenced[toolName] = struct{}{}
			}
		}
	}
	if len(referenced) == 0 {
		return []tools.Definition{}, nil
	}

	defs, err := tools.DemoDefinitions()
	if err != nil {
		return nil, err
	}
	result := make([]tools.Definition, 0, len(referenced))
	for name := range referenced {
		if def, ok := defs[name]; ok {
			result = append(result, def)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}
