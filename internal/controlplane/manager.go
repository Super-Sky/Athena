// manager.go builds effective control-plane views and applies persisted overrides.
// manager.go 负责构建控制面的有效视图并应用持久化 override。
package controlplane

import (
	"context"
	"fmt"
	"sort"
	"strings"

	runtimescene "moss/internal/runtime/scene"
	"moss/internal/skills"
	"moss/internal/tools"
)

// Manager resolves effective control-plane data from builtins plus persisted overrides.
// Manager 负责把内置定义与持久化 override 合成为有效控制面数据。
type Manager struct {
	store          Store
	truthDir       string
	activeStateDir string
	auth           authConfig
	authStore      AuthStateStore
}

// NewManager creates one control-plane manager backed by the provided store.
// NewManager 根据给定 store 创建一个控制面管理器。
func NewManager(store Store) *Manager {
	return &Manager{store: store}
}

// NewManagerWithTruthDir creates one control-plane manager and binds one active truth dir.
// NewManagerWithTruthDir 创建一个绑定 active truth dir 的控制面管理器。
func NewManagerWithTruthDir(store Store, truthDir string) *Manager {
	manager := NewManager(store)
	manager.SetTruthDir(truthDir)
	return manager
}

// NewManagerWithTruthAndStateDirs creates one control-plane manager with separate source truth and generated state roots.
// NewManagerWithTruthAndStateDirs 创建一个拆分 source truth 与生成状态目录的控制面管理器。
func NewManagerWithTruthAndStateDirs(store Store, truthDir string, activeStateDir string) *Manager {
	manager := NewManagerWithTruthDir(store, truthDir)
	manager.SetActiveStateDir(activeStateDir)
	return manager
}

// SetTruthDir configures the file-backed system-truth directory used by system resources.
// SetTruthDir 配置 system resources 使用的文件化真相目录。
func (m *Manager) SetTruthDir(path string) {
	if m == nil {
		return
	}
	m.truthDir = strings.TrimSpace(path)
	if m.authStore == nil {
		m.authStore = NewFileAuthStateStore(m.authStatePath())
	}
}

// SetActiveStateDir configures where generated system-resource state is stored.
// SetActiveStateDir 配置 system resources 生成状态的存储目录。
func (m *Manager) SetActiveStateDir(path string) {
	if m == nil {
		return
	}
	m.activeStateDir = strings.TrimSpace(path)
}

// SetAuthStateStore overrides the persistence used for control-plane auth sessions and locks.
// SetAuthStateStore 用于覆盖控制面认证会话与锁定状态的持久化实现。
func (m *Manager) SetAuthStateStore(store AuthStateStore) {
	if m == nil || store == nil {
		return
	}
	m.authStore = store
}

// LoadDocument returns the persisted override document with default runtime tuning merged in.
// LoadDocument 返回已持久化 override 文档，并合并默认运行开关。
func (m *Manager) LoadDocument(ctx context.Context) (Document, error) {
	if m == nil || m.store == nil {
		defaults := DefaultRuntimeTuning()
		return Document{Governance: defaults, Runtime: defaults}, nil
	}
	return m.store.Load(ctx)
}

// ListScenes returns the effective scene catalog exposed to the control plane.
// ListScenes 返回控制面对外暴露的有效场景目录。
func (m *Manager) ListScenes(ctx context.Context) ([]SceneConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return nil, err
	}
	overrides := make([]runtimescene.Definition, 0, len(doc.Scenes))
	for _, item := range doc.Scenes {
		overrides = append(overrides, runtimescene.Definition{
			ID:                 item.ID,
			Description:        strings.TrimSpace(item.Description),
			Keywords:           compactStrings(item.Keywords),
			DefaultSkills:      compactStrings(item.DefaultSkills),
			SuggestedQuestions: compactStrings(item.SuggestedQuestions),
			Enabled:            item.Enabled,
			MatchScore:         item.MatchScore,
		})
	}
	merged := runtimescene.MergeCatalog(runtimescene.BuiltinCatalog(), overrides)
	result := make([]SceneConfig, 0, len(merged))
	for _, item := range merged {
		result = append(result, SceneConfig{
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

// PutScene creates or replaces one persisted scene override.
// PutScene 会创建或替换一条持久化场景 override。
func (m *Manager) PutScene(ctx context.Context, id string, input SceneConfig) (SceneConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return SceneConfig{}, err
	}
	item := SceneConfig{
		ID:                 strings.TrimSpace(id),
		Description:        strings.TrimSpace(input.Description),
		Keywords:           compactStrings(input.Keywords),
		DefaultSkills:      compactStrings(input.DefaultSkills),
		SuggestedQuestions: compactStrings(input.SuggestedQuestions),
		Enabled:            input.Enabled,
		MatchScore:         input.MatchScore,
	}
	updated := false
	for idx := range doc.Scenes {
		if strings.TrimSpace(doc.Scenes[idx].ID) == item.ID {
			doc.Scenes[idx] = item
			updated = true
			break
		}
	}
	if !updated {
		doc.Scenes = append(doc.Scenes, item)
	}
	sort.Slice(doc.Scenes, func(i, j int) bool { return doc.Scenes[i].ID < doc.Scenes[j].ID })
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return SceneConfig{}, err
		}
		if err := m.recordVersion(ctx, doc, fmt.Sprintf("update scene %s", item.ID)); err != nil {
			return SceneConfig{}, err
		}
	}
	return item, nil
}

// ApplySkillOverrides overlays persisted control-plane changes onto one base skill list.
// ApplySkillOverrides 会把持久化控制面变更叠加到基础 skill 列表上。
func (m *Manager) ApplySkillOverrides(ctx context.Context, defs []skills.Definition) ([]skills.Definition, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return nil, err
	}
	result := make([]skills.Definition, 0, len(defs))
	index := make(map[string]SkillConfig, len(doc.Skills))
	for _, item := range doc.Skills {
		index[strings.TrimSpace(item.Name)] = item
	}
	for _, def := range defs {
		override, ok := index[strings.TrimSpace(def.Name)]
		if ok {
			if !override.Enabled {
				continue
			}
			if strings.TrimSpace(override.Description) != "" {
				def.Description = strings.TrimSpace(override.Description)
			}
			if strings.TrimSpace(override.Guidance) != "" {
				def.Guidance = strings.TrimSpace(override.Guidance)
			}
			if len(override.ToolNames) > 0 {
				def.ToolNames = compactStrings(override.ToolNames)
			}
		}
		result = append(result, def)
	}
	return result, nil
}

// ListSkills returns the effective skill list exposed to the control plane.
// ListSkills 返回控制面对外暴露的有效 skill 列表。
func (m *Manager) ListSkills(ctx context.Context, defs []skills.Definition) ([]SkillConfig, error) {
	effective, err := m.ApplySkillOverrides(ctx, defs)
	if err != nil {
		return nil, err
	}
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return nil, err
	}
	disabled := make(map[string]SkillConfig)
	for _, item := range doc.Skills {
		if !item.Enabled {
			disabled[strings.TrimSpace(item.Name)] = item
		}
	}
	result := make([]SkillConfig, 0, len(effective)+len(disabled))
	for _, def := range effective {
		result = append(result, SkillConfig{
			Name:        def.Name,
			Description: def.Description,
			Guidance:    def.Guidance,
			ToolNames:   append([]string(nil), def.ToolNames...),
			Enabled:     true,
		})
		delete(disabled, strings.TrimSpace(def.Name))
	}
	for _, item := range disabled {
		result = append(result, SkillConfig{
			Name:        item.Name,
			Description: item.Description,
			Guidance:    item.Guidance,
			ToolNames:   append([]string(nil), item.ToolNames...),
			Enabled:     false,
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// PutSkill creates or replaces one persisted skill override.
// PutSkill 会创建或替换一条持久化 skill override。
func (m *Manager) PutSkill(ctx context.Context, name string, input SkillConfig) (SkillConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return SkillConfig{}, err
	}
	item := SkillConfig{
		Name:        strings.TrimSpace(name),
		Description: strings.TrimSpace(input.Description),
		Guidance:    strings.TrimSpace(input.Guidance),
		ToolNames:   compactStrings(input.ToolNames),
		Enabled:     input.Enabled,
	}
	updated := false
	for idx := range doc.Skills {
		if strings.TrimSpace(doc.Skills[idx].Name) == item.Name {
			doc.Skills[idx] = item
			updated = true
			break
		}
	}
	if !updated {
		doc.Skills = append(doc.Skills, item)
	}
	sort.Slice(doc.Skills, func(i, j int) bool { return doc.Skills[i].Name < doc.Skills[j].Name })
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return SkillConfig{}, err
		}
		if err := m.recordVersion(ctx, doc, fmt.Sprintf("update skill %s", item.Name)); err != nil {
			return SkillConfig{}, err
		}
	}
	return item, nil
}

// ApplyToolOverrides overlays persisted control-plane changes onto one base tool list.
// ApplyToolOverrides 会把持久化控制面变更叠加到基础 tool 列表上。
func (m *Manager) ApplyToolOverrides(ctx context.Context, defs []tools.Definition) ([]ToolConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return nil, err
	}
	index := make(map[string]ToolConfig, len(doc.Tools))
	for _, item := range doc.Tools {
		index[strings.TrimSpace(item.Name)] = item
	}
	result := make([]ToolConfig, 0, len(defs))
	for _, def := range defs {
		item := ToolConfig{
			Name:                 def.Name,
			Description:          def.Description,
			ToolScope:            def.ToolScope,
			RequiresConfirmation: def.RequiresConfirmation,
			SideEffectLevel:      def.SideEffectLevel,
			InputSchemaSummary:   def.InputSchemaSummary,
			OutputSchemaSummary:  def.OutputSchemaSummary,
			Enabled:              true,
		}
		if override, ok := index[strings.TrimSpace(def.Name)]; ok {
			if !override.Enabled {
				continue
			}
			if strings.TrimSpace(override.Description) != "" {
				item.Description = strings.TrimSpace(override.Description)
			}
			if strings.TrimSpace(override.ToolScope) != "" {
				item.ToolScope = strings.TrimSpace(override.ToolScope)
			}
			if strings.TrimSpace(override.SideEffectLevel) != "" {
				item.SideEffectLevel = strings.TrimSpace(override.SideEffectLevel)
			}
			if strings.TrimSpace(override.InputSchemaSummary) != "" {
				item.InputSchemaSummary = strings.TrimSpace(override.InputSchemaSummary)
			}
			if strings.TrimSpace(override.OutputSchemaSummary) != "" {
				item.OutputSchemaSummary = strings.TrimSpace(override.OutputSchemaSummary)
			}
			item.RequiresConfirmation = override.RequiresConfirmation
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

// ListTools returns the effective tool registry exposed to the control plane.
// ListTools 返回控制面对外暴露的有效 tool registry。
func (m *Manager) ListTools(ctx context.Context, defs []tools.Definition) ([]ToolConfig, error) {
	return m.ApplyToolOverrides(ctx, defs)
}

// PutTool creates or replaces one persisted tool override.
// PutTool 会创建或替换一条持久化 tool override。
func (m *Manager) PutTool(ctx context.Context, name string, input ToolConfig) (ToolConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return ToolConfig{}, err
	}
	item := ToolConfig{
		Name:                 strings.TrimSpace(name),
		Description:          strings.TrimSpace(input.Description),
		ToolScope:            strings.TrimSpace(input.ToolScope),
		RequiresConfirmation: input.RequiresConfirmation,
		SideEffectLevel:      strings.TrimSpace(input.SideEffectLevel),
		InputSchemaSummary:   strings.TrimSpace(input.InputSchemaSummary),
		OutputSchemaSummary:  strings.TrimSpace(input.OutputSchemaSummary),
		Enabled:              input.Enabled,
	}
	updated := false
	for idx := range doc.Tools {
		if strings.TrimSpace(doc.Tools[idx].Name) == item.Name {
			doc.Tools[idx] = item
			updated = true
			break
		}
	}
	if !updated {
		doc.Tools = append(doc.Tools, item)
	}
	sort.Slice(doc.Tools, func(i, j int) bool { return doc.Tools[i].Name < doc.Tools[j].Name })
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return ToolConfig{}, err
		}
		if err := m.recordVersion(ctx, doc, fmt.Sprintf("update tool %s", item.Name)); err != nil {
			return ToolConfig{}, err
		}
	}
	return item, nil
}

// Runtime returns the effective runtime tuning.
// Runtime 返回有效运行开关。
func (m *Manager) Runtime(ctx context.Context) (RuntimeTuning, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return GovernanceConfig{}, err
	}
	return doc.Governance, nil
}

// PutRuntime replaces the persisted runtime tuning.
// PutRuntime 会替换持久化运行开关。
func (m *Manager) PutRuntime(ctx context.Context, input RuntimeTuning) (RuntimeTuning, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return GovernanceConfig{}, err
	}
	doc.Governance = mergeRuntimeTuning(input)
	doc.Runtime = doc.Governance
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return GovernanceConfig{}, err
		}
		if err := m.recordVersion(ctx, doc, "update runtime compatibility tuning"); err != nil {
			return GovernanceConfig{}, err
		}
	}
	return doc.Governance, nil
}

// Governance returns the effective governance configuration.
// Governance 返回有效治理配置。
func (m *Manager) Governance(ctx context.Context) (GovernanceConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return GovernanceConfig{}, err
	}
	return doc.Governance, nil
}

// PutGovernance replaces the persisted governance configuration.
// PutGovernance 会替换持久化治理配置。
func (m *Manager) PutGovernance(ctx context.Context, input GovernanceConfig) (GovernanceConfig, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return GovernanceConfig{}, err
	}
	doc.Governance = mergeRuntimeTuning(input)
	doc.Runtime = doc.Governance
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return GovernanceConfig{}, err
		}
		if err := m.recordVersion(ctx, doc, "update governance"); err != nil {
			return GovernanceConfig{}, err
		}
	}
	return doc.Governance, nil
}

// ListVersions returns the persisted configuration version summaries.
// ListVersions 返回持久化配置版本摘要列表。
func (m *Manager) ListVersions(ctx context.Context) ([]ConfigVersionSummary, error) {
	if m == nil || m.store == nil {
		return nil, nil
	}
	return m.store.ListVersions(ctx)
}

// GetVersion returns one persisted configuration version detail.
// GetVersion 返回单个持久化配置版本详情。
func (m *Manager) GetVersion(ctx context.Context, versionID string) (ConfigVersionDetail, error) {
	if m == nil || m.store == nil {
		return ConfigVersionDetail{}, fmt.Errorf("control plane store is not configured")
	}
	return m.store.LoadVersion(ctx, versionID)
}

// RollbackVersion restores one historical configuration version and records a new current snapshot.
// RollbackVersion 会恢复一个历史配置版本，并记录一条新的当前快照。
func (m *Manager) RollbackVersion(ctx context.Context, versionID string) (ConfigVersionDetail, error) {
	if m == nil || m.store == nil {
		return ConfigVersionDetail{}, fmt.Errorf("control plane store is not configured")
	}
	detail, err := m.store.LoadVersion(ctx, versionID)
	if err != nil {
		return ConfigVersionDetail{}, err
	}
	doc := mergeDocument(detail.Document)
	if err := m.store.Save(ctx, doc); err != nil {
		return ConfigVersionDetail{}, err
	}
	current := ConfigVersionDetail{
		ConfigVersionSummary: ConfigVersionSummary{
			VersionID: newVersionID(),
			Summary:   fmt.Sprintf("rollback to %s", strings.TrimSpace(versionID)),
		},
		Document: doc,
	}
	if err := m.store.SaveVersion(ctx, current); err != nil {
		return ConfigVersionDetail{}, err
	}
	return current, nil
}

// Bootstrap builds the combined bootstrap payload for the web console.
// Bootstrap 会为 web 控制台构建组合后的启动载荷。
func (m *Manager) Bootstrap(ctx context.Context, skillDefs []skills.Definition, toolDefs []tools.Definition, swaggerSpecURL string) (*BootstrapPayload, error) {
	scenes, err := m.ListScenes(ctx)
	if err != nil {
		return nil, err
	}
	skillsView, err := m.ListSkills(ctx, skillDefs)
	if err != nil {
		return nil, err
	}
	toolsView, err := m.ListTools(ctx, toolDefs)
	if err != nil {
		return nil, err
	}
	governanceView, err := m.Governance(ctx)
	if err != nil {
		return nil, err
	}
	versions, err := m.ListVersions(ctx)
	if err != nil {
		return nil, err
	}
	systemResources, err := m.ListSystemResources(ctx)
	if err != nil {
		return nil, err
	}
	return &BootstrapPayload{
		Scenes:          ensureSceneConfigs(scenes),
		Skills:          ensureSkillConfigs(skillsView),
		Tools:           ensureToolConfigs(toolsView),
		SystemResources: ensureSystemResourceSummaries(systemResources),
		Governance:      governanceView,
		Runtime:         governanceView,
		ConfigVersions:  ensureConfigVersionSummaries(versions),
		SwaggerSpecURL:  strings.TrimSpace(swaggerSpecURL),
	}, nil
}

func ensureSceneConfigs(items []SceneConfig) []SceneConfig {
	if items == nil {
		return []SceneConfig{}
	}
	return items
}

func ensureSkillConfigs(items []SkillConfig) []SkillConfig {
	if items == nil {
		return []SkillConfig{}
	}
	return items
}

func ensureToolConfigs(items []ToolConfig) []ToolConfig {
	if items == nil {
		return []ToolConfig{}
	}
	return items
}

func ensureSystemResourceSummaries(items []SystemResourceSummary) []SystemResourceSummary {
	if items == nil {
		return []SystemResourceSummary{}
	}
	return items
}

func ensureConfigVersionSummaries(items []ConfigVersionSummary) []ConfigVersionSummary {
	if items == nil {
		return []ConfigVersionSummary{}
	}
	return items
}

func (m *Manager) recordVersion(ctx context.Context, doc Document, summary string) error {
	if m == nil || m.store == nil {
		return nil
	}
	return m.store.SaveVersion(ctx, ConfigVersionDetail{
		ConfigVersionSummary: ConfigVersionSummary{
			VersionID: newVersionID(),
			Summary:   strings.TrimSpace(summary),
		},
		Document: doc,
	})
}

func compactStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}
