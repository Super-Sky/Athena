// types.go defines the file-backed control-plane contract for scenes, skills, and runtime tuning.
// types.go 定义场景、skill 和运行参数使用的文件化控制面契约。
package controlplane

// TruthDirInfo captures the current system-truth directory path and active version marker.
// TruthDirInfo 描述当前系统真相目录路径和活动版本标记。
type TruthDirInfo struct {
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

// AuthStatus captures the current control-plane authentication state.
// AuthStatus 描述当前控制面认证状态。
type AuthStatus struct {
	Authenticated     bool         `json:"authenticated"`
	LockState         string       `json:"lock_state,omitempty"`
	RemainingAttempts int          `json:"remaining_attempts,omitempty"`
	FailedAttempts    int          `json:"failed_attempts,omitempty"`
	SessionExpiresAt  string       `json:"session_expires_at,omitempty"`
	TruthDir          TruthDirInfo `json:"truth_dir,omitempty"`
}

// LoginRequest carries the single shared token used for control-plane login.
// LoginRequest 描述控制面登录使用的共享 token。
type LoginRequest struct {
	Token string `json:"token"`
}

// ToolConfig captures one effective tool definition exposed to the control plane.
// ToolConfig 描述控制面对外暴露的一条有效 tool 定义。
type ToolConfig struct {
	Name                 string `json:"name"`
	Description          string `json:"description,omitempty"`
	ToolScope            string `json:"tool_scope,omitempty"`
	RequiresConfirmation bool   `json:"requires_confirmation"`
	SideEffectLevel      string `json:"side_effect_level,omitempty"`
	InputSchemaSummary   string `json:"input_schema_summary,omitempty"`
	OutputSchemaSummary  string `json:"output_schema_summary,omitempty"`
	Enabled              bool   `json:"enabled"`
}

// SceneConfig captures one effective scene definition exposed to the control plane.
// SceneConfig 描述控制面对外暴露的一条有效场景定义。
type SceneConfig struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description,omitempty"`
	Keywords           []string `json:"keywords,omitempty"`
	DefaultSkills      []string `json:"default_skills,omitempty"`
	SuggestedQuestions []string `json:"suggested_questions,omitempty"`
	Enabled            bool     `json:"enabled"`
	MatchScore         int      `json:"match_score,omitempty"`
}

// SkillConfig captures one effective skill definition exposed to the control plane.
// SkillConfig 描述控制面对外暴露的一条有效 skill 定义。
type SkillConfig struct {
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Guidance    string   `json:"guidance,omitempty"`
	ToolNames   []string `json:"tool_names,omitempty"`
	Enabled     bool     `json:"enabled"`
}

// GovernanceConfig captures the safe governance toggles and budgets exposed by the control plane.
// GovernanceConfig 描述控制面对外暴露的安全治理开关与预算参数。
type GovernanceConfig struct {
	ChoiceRequiredEnabled     bool `json:"choice_required_enabled"`
	AutomationFallbackEnabled bool `json:"automation_fallback_enabled"`
	PlanningProgressEnabled   bool `json:"planning_progress_enabled"`
	FactQualityGateEnabled    bool `json:"fact_quality_gate_enabled"`
	ToolHintEmissionEnabled   bool `json:"tool_hint_emission_enabled"`
	KnowledgeRetrievalEnabled bool `json:"knowledge_retrieval_emission_enabled"`
	MaxPlanningSteps          int  `json:"max_planning_steps"`
	MaxToolHints              int  `json:"max_tool_hints"`
}

// RuntimeTuning remains as the compatibility alias for governance controls.
// RuntimeTuning 继续作为 governance 配置的兼容别名。
type RuntimeTuning = GovernanceConfig

// ConfigVersionSummary captures one persisted control-plane version snapshot.
// ConfigVersionSummary 描述一条持久化控制面版本快照摘要。
type ConfigVersionSummary struct {
	VersionID string `json:"version_id"`
	CreatedAt string `json:"created_at"`
	CreatedBy string `json:"created_by,omitempty"`
	Summary   string `json:"summary,omitempty"`
}

// ConfigVersionDetail captures one version snapshot together with the full document.
// ConfigVersionDetail 描述单个版本快照及其完整配置文档。
type ConfigVersionDetail struct {
	ConfigVersionSummary
	Document Document `json:"document"`
}

// SystemResourceSummary captures one file-backed system resource visible in the control plane.
// SystemResourceSummary 描述控制面可见的一条文件化系统资源摘要。
type SystemResourceSummary struct {
	AssetID         string `json:"asset_id"`
	AssetType       string `json:"asset_type"`
	AssetName       string `json:"asset_name,omitempty"`
	Scope           string `json:"scope,omitempty"`
	SourceKind      string `json:"source_kind,omitempty"`
	Status          string `json:"status,omitempty"`
	TruthDirVersion string `json:"truth_dir_version,omitempty"`
	CompiledVersion string `json:"compiled_version,omitempty"`
	UpdatedAt       string `json:"updated_at,omitempty"`
	ReadOnly        bool   `json:"read_only,omitempty"`
}

// SystemResourceParseResult captures the latest parse result for one system resource.
// SystemResourceParseResult 描述一条系统资源最近一次解析结果。
type SystemResourceParseResult struct {
	AssetID    string         `json:"asset_id,omitempty"`
	Status     string         `json:"status,omitempty"`
	Summary    string         `json:"summary,omitempty"`
	Warnings   []string       `json:"warnings,omitempty"`
	Errors     []string       `json:"errors,omitempty"`
	Parsed     map[string]any `json:"parsed,omitempty"`
	SourceHash string         `json:"source_hash,omitempty"`
	UpdatedAt  string         `json:"updated_at,omitempty"`
}

// SystemResourceCompileResult captures the latest compiled payload for one system resource.
// SystemResourceCompileResult 描述一条系统资源最近一次编译结果。
type SystemResourceCompileResult struct {
	AssetID          string         `json:"asset_id,omitempty"`
	Status           string         `json:"status,omitempty"`
	Summary          string         `json:"summary,omitempty"`
	GuidanceText     string         `json:"guidance_text,omitempty"`
	SourceChecksum   string         `json:"source_checksum,omitempty"`
	CompiledChecksum string         `json:"compiled_checksum,omitempty"`
	CompiledVersion  string         `json:"compiled_version,omitempty"`
	TruthDirVersion  string         `json:"truth_dir_version,omitempty"`
	Payload          map[string]any `json:"payload,omitempty"`
	UpdatedAt        string         `json:"updated_at,omitempty"`
}

// SystemResourcePipeline captures the current pipeline stage for one system resource.
// SystemResourcePipeline 描述一条系统资源当前的处理流水线状态。
type SystemResourcePipeline struct {
	PipelineID      string   `json:"pipeline_id,omitempty"`
	AssetID         string   `json:"asset_id,omitempty"`
	Status          string   `json:"status,omitempty"`
	CurrentStep     string   `json:"current_step,omitempty"`
	ProgressPercent int      `json:"progress_percent,omitempty"`
	StartedAt       string   `json:"started_at,omitempty"`
	UpdatedAt       string   `json:"updated_at,omitempty"`
	Warnings        []string `json:"warnings,omitempty"`
	Errors          []string `json:"errors,omitempty"`
}

// SystemResourceDetail captures the editable detail view for one system resource.
// SystemResourceDetail 描述一条系统资源的可编辑详情视图。
type SystemResourceDetail struct {
	SystemResourceSummary
	SourcePath    string                       `json:"source_path,omitempty"`
	Metadata      map[string]any               `json:"metadata,omitempty"`
	ParseResult   *SystemResourceParseResult   `json:"parse_result,omitempty"`
	CompileResult *SystemResourceCompileResult `json:"compile_result,omitempty"`
	Pipeline      *SystemResourcePipeline      `json:"pipeline,omitempty"`
}

// SystemResourceVersionSummary captures one persisted system-resource snapshot summary.
// SystemResourceVersionSummary 描述一条 system resource 持久化快照摘要。
type SystemResourceVersionSummary struct {
	VersionID        string `json:"version_id"`
	AssetID          string `json:"asset_id"`
	Action           string `json:"action,omitempty"`
	Summary          string `json:"summary,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	TruthDirVersion  string `json:"truth_dir_version,omitempty"`
	CompiledVersion  string `json:"compiled_version,omitempty"`
	SourceChecksum   string `json:"source_checksum,omitempty"`
	CompiledChecksum string `json:"compiled_checksum,omitempty"`
	RolledBackFrom   string `json:"rolled_back_from,omitempty"`
}

// SystemResourceVersionDetail captures one persisted system-resource snapshot together with the resource state.
// SystemResourceVersionDetail 描述一条 system resource 持久化快照及对应资源状态。
type SystemResourceVersionDetail struct {
	SystemResourceVersionSummary
	Resource      SystemResourceDetail `json:"resource"`
	SourceContent string               `json:"source_content,omitempty"`
}

// SystemResourceAuditEntry captures one user-visible audit entry for a system resource mutation.
// SystemResourceAuditEntry 描述一条 system resource 变更的用户可见审计记录。
type SystemResourceAuditEntry struct {
	EventID          string         `json:"event_id"`
	AssetID          string         `json:"asset_id"`
	Action           string         `json:"action,omitempty"`
	Summary          string         `json:"summary,omitempty"`
	CreatedAt        string         `json:"created_at,omitempty"`
	TruthDirVersion  string         `json:"truth_dir_version,omitempty"`
	CompiledVersion  string         `json:"compiled_version,omitempty"`
	SourceChecksum   string         `json:"source_checksum,omitempty"`
	CompiledChecksum string         `json:"compiled_checksum,omitempty"`
	RolledBackFrom   string         `json:"rolled_back_from,omitempty"`
	Detail           map[string]any `json:"detail,omitempty"`
}

// SystemResourceSource captures the editable raw source content for one system resource.
// SystemResourceSource 描述一条系统资源的原始内容视图。
type SystemResourceSource struct {
	AssetID       string `json:"asset_id,omitempty"`
	SourceContent string `json:"source_content,omitempty"`
	Message       string `json:"message,omitempty"`
	UpdatedAt     string `json:"updated_at,omitempty"`
}

// SystemResourceMetadataPatch captures editable metadata fields for one system resource.
// SystemResourceMetadataPatch 描述一条系统资源允许修改的元数据字段。
type SystemResourceMetadataPatch struct {
	AssetType  string         `json:"asset_type,omitempty"`
	AssetName  string         `json:"asset_name,omitempty"`
	Scope      string         `json:"scope,omitempty"`
	SourceKind string         `json:"source_kind,omitempty"`
	ReadOnly   *bool          `json:"read_only,omitempty"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// SystemResourceCreateInput captures creation payload for one file-backed system resource.
// SystemResourceCreateInput 描述创建一条文件化系统资源的请求体。
type SystemResourceCreateInput struct {
	AssetID       string         `json:"asset_id"`
	AssetType     string         `json:"asset_type,omitempty"`
	AssetName     string         `json:"asset_name,omitempty"`
	Scope         string         `json:"scope,omitempty"`
	SourceKind    string         `json:"source_kind,omitempty"`
	ReadOnly      bool           `json:"read_only,omitempty"`
	SourceContent string         `json:"source_content,omitempty"`
	Message       string         `json:"message,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// SystemResourceCreateRequest keeps backward-compatible naming for transport and tests.
// SystemResourceCreateRequest 保持 transport 和测试使用的兼容命名。
type SystemResourceCreateRequest = SystemResourceCreateInput

// SystemResourceMutationResult captures the accepted mutation plus current pipeline status.
// SystemResourceMutationResult 描述变更已接收以及当前 pipeline 状态。
type SystemResourceMutationResult struct {
	AssetID  string                 `json:"asset_id,omitempty"`
	Accepted bool                   `json:"accepted"`
	Pipeline SystemResourcePipeline `json:"pipeline"`
}

// SystemResourceDebugPayload captures one endpoint-specific debug payload generated from one resource.
// SystemResourceDebugPayload 描述从一条资源生成的某个接口专用调试载荷。
type SystemResourceDebugPayload struct {
	Endpoint string         `json:"endpoint"`
	Payload  map[string]any `json:"payload"`
}

// ToolGovernanceRule captures one compiled policy rule for a runtime tool request.
// ToolGovernanceRule 描述一条编译后的 runtime tool 治理规则。
type ToolGovernanceRule struct {
	RuleID         string         `json:"rule_id,omitempty"`
	MatchTool      string         `json:"match_tool,omitempty"`
	MatchScope     string         `json:"match_scope,omitempty"`
	MatchOperation string         `json:"match_operation,omitempty"`
	MatchRisk      string         `json:"match_risk,omitempty"`
	Decision       string         `json:"decision"`
	Reason         string         `json:"reason,omitempty"`
	RedactFields   []string       `json:"redact_fields,omitempty"`
	SandboxRef     string         `json:"sandbox_ref,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
}

// ToolGovernancePolicy captures the effective compiled policy used before tool execution.
// ToolGovernancePolicy 描述 tool 执行前使用的有效编译治理策略。
type ToolGovernancePolicy struct {
	PolicyID        string               `json:"policy_id,omitempty"`
	AssetID         string               `json:"asset_id,omitempty"`
	Name            string               `json:"name,omitempty"`
	DefaultDecision string               `json:"default_decision,omitempty"`
	DecisionModel   string               `json:"decision_model,omitempty"`
	Rules           []ToolGovernanceRule `json:"rules,omitempty"`
	CompiledVersion string               `json:"compiled_version,omitempty"`
	TruthDirVersion string               `json:"truth_dir_version,omitempty"`
	SourceChecksum  string               `json:"source_checksum,omitempty"`
	UpdatedAt       string               `json:"updated_at,omitempty"`
	Metadata        map[string]any       `json:"metadata,omitempty"`
}

// ToolGovernanceDecisionRequest captures one runtime tool request to classify.
// ToolGovernanceDecisionRequest 描述一次待判定的 runtime tool 请求。
type ToolGovernanceDecisionRequest struct {
	ToolName  string         `json:"tool_name"`
	ToolScope string         `json:"tool_scope,omitempty"`
	Operation string         `json:"operation,omitempty"`
	RiskLevel string         `json:"risk_level,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// ToolGovernanceDecision captures the persisted decision for one tool request.
// ToolGovernanceDecision 描述一次 tool 请求对应的持久化治理判定。
type ToolGovernanceDecision struct {
	DecisionID      string         `json:"decision_id"`
	Decision        string         `json:"decision"`
	Reason          string         `json:"reason,omitempty"`
	MatchedRuleID   string         `json:"matched_rule_id,omitempty"`
	PolicyAssetID   string         `json:"policy_asset_id,omitempty"`
	PolicyVersion   string         `json:"policy_version,omitempty"`
	ToolName        string         `json:"tool_name,omitempty"`
	ToolScope       string         `json:"tool_scope,omitempty"`
	Operation       string         `json:"operation,omitempty"`
	RiskLevel       string         `json:"risk_level,omitempty"`
	RedactFields    []string       `json:"redact_fields,omitempty"`
	SandboxRef      string         `json:"sandbox_ref,omitempty"`
	EvaluatedAt     string         `json:"evaluated_at,omitempty"`
	TruthDirVersion string         `json:"truth_dir_version,omitempty"`
	Metadata        map[string]any `json:"metadata,omitempty"`
}

// SystemResourceExportInfo captures metadata about one exported truth-dir snapshot.
// SystemResourceExportInfo 描述一次真相目录导出快照的元信息。
type SystemResourceExportInfo struct {
	TruthDirVersion string `json:"truth_dir_version,omitempty"`
	ExportFile      string `json:"export_file,omitempty"`
	AssetCount      int    `json:"asset_count,omitempty"`
}

// SystemResourceExport keeps backward-compatible naming for exported truth-dir metadata.
// SystemResourceExport 保持导出真相目录元数据的兼容命名。
type SystemResourceExport = SystemResourceExportInfo

// Document stores all persisted control-plane overrides in one file.
// Document 保存控制面所有持久化 override。
type Document struct {
	Scenes     []SceneConfig    `json:"scenes,omitempty"`
	Skills     []SkillConfig    `json:"skills,omitempty"`
	Tools      []ToolConfig     `json:"tools,omitempty"`
	Governance GovernanceConfig `json:"governance"`
	Runtime    RuntimeTuning    `json:"runtime"`
}

// BootstrapPayload is the front-end bootstrap payload for the control-plane web app.
// BootstrapPayload 是控制面 web 前端使用的启动载荷。
type BootstrapPayload struct {
	Scenes          []SceneConfig           `json:"scenes"`
	Skills          []SkillConfig           `json:"skills"`
	Tools           []ToolConfig            `json:"tools"`
	SystemResources []SystemResourceSummary `json:"system_resources"`
	Governance      GovernanceConfig        `json:"governance"`
	Runtime         RuntimeTuning           `json:"runtime"`
	ConfigVersions  []ConfigVersionSummary  `json:"config_versions"`
	SwaggerSpecURL  string                  `json:"swagger_spec_url"`
}

// DefaultRuntimeTuning returns the baseline safe runtime toggles.
// DefaultRuntimeTuning 返回基础安全运行开关。
func DefaultRuntimeTuning() GovernanceConfig {
	return GovernanceConfig{
		ChoiceRequiredEnabled:     true,
		AutomationFallbackEnabled: true,
		PlanningProgressEnabled:   true,
		FactQualityGateEnabled:    true,
		ToolHintEmissionEnabled:   true,
		KnowledgeRetrievalEnabled: true,
		MaxPlanningSteps:          6,
		MaxToolHints:              4,
	}
}
