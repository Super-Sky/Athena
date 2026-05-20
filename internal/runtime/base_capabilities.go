// base_capabilities.go defines Athena-owned base capability contracts for artifact writing,
// read-only resource reads, structured parsing, local preprocessing, fact-quality gating,
// and bounded runtime-state queries.
// base_capabilities.go 定义 Athena 自有的基础能力契约，用于交付物写入、
// 只读资源读取、结构化解析、本地预处理、事实质量门禁以及有边界的运行时状态查询。
package runtime

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"gopkg.in/yaml.v3"
	"moss/internal/session"
	"moss/internal/workflow"
)

const (
	// DesiredOutputModeArtifactWrite marks one explicit delivery-writing request.
	// DesiredOutputModeArtifactWrite 表示一次显式交付物写入请求。
	DesiredOutputModeArtifactWrite = "artifact_write"

	// DesiredOutputModeReadOnlyResourceRead marks one explicit read-only resource read request.
	// DesiredOutputModeReadOnlyResourceRead 表示一次显式只读资源读取请求。
	DesiredOutputModeReadOnlyResourceRead = "read_only_resource_read"

	// DesiredOutputModeStructuredDataParse marks one explicit structured parse request.
	// DesiredOutputModeStructuredDataParse 表示一次显式结构化解析请求。
	DesiredOutputModeStructuredDataParse = "structured_data_parse"

	// DesiredOutputModeLocalDataTransform marks one explicit local data transform request.
	// DesiredOutputModeLocalDataTransform 表示一次显式本地数据转换请求。
	DesiredOutputModeLocalDataTransform = "local_data_transform"

	// DesiredOutputModeFactQualityGate marks one explicit fact-quality gate request.
	// DesiredOutputModeFactQualityGate 表示一次显式事实质量门禁请求。
	DesiredOutputModeFactQualityGate = "fact_quality_gate"

	// DesiredOutputModeQueryRuntimeState marks one explicit runtime-state query request.
	// DesiredOutputModeQueryRuntimeState 表示一次显式运行时状态查询请求。
	DesiredOutputModeQueryRuntimeState = "query_runtime_state"
)

var artifactWhitelistDirectories = map[string]struct{}{
	"deliverables": {},
	"reports":      {},
	"scripts":      {},
	"exports":      {},
}

// ArtifactOwnerType captures the canonical owner layer used for shared-root artifact placement.
// ArtifactOwnerType 描述共享目录交付物放置时使用的标准归属层级。
type ArtifactOwnerType string

const (
	// ArtifactOwnerTypeUser places the artifact under a user-owned subtree.
	// ArtifactOwnerTypeUser 表示交付物归属到用户目录层。
	ArtifactOwnerTypeUser ArtifactOwnerType = "user"

	// ArtifactOwnerTypeWorkspace places the artifact under a workspace-owned subtree.
	// ArtifactOwnerTypeWorkspace 表示交付物归属到工作区目录层。
	ArtifactOwnerTypeWorkspace ArtifactOwnerType = "workspace"

	// ArtifactOwnerTypeApp places the artifact under an app-owned subtree.
	// ArtifactOwnerTypeApp 表示交付物归属到应用实例目录层。
	ArtifactOwnerTypeApp ArtifactOwnerType = "app"

	// ArtifactOwnerTypeIntegration places the artifact under an integration-owned subtree.
	// ArtifactOwnerTypeIntegration 表示交付物归属到集成实例目录层。
	ArtifactOwnerTypeIntegration ArtifactOwnerType = "integration"
)

// ArtifactWriteRequest captures the Athena-side delivery-writing request contract.
// ArtifactWriteRequest 描述 Athena 侧交付物写入请求契约。
type ArtifactWriteRequest struct {
	ArtifactID           string            `json:"artifact_id,omitempty"`
	ArtifactOwnerKey     string            `json:"artifact_owner_key,omitempty"`
	ArtifactOwnerType    ArtifactOwnerType `json:"artifact_owner_type,omitempty"`
	Kind                 string            `json:"kind,omitempty"`
	Title                string            `json:"title,omitempty"`
	Filename             string            `json:"filename,omitempty"`
	Language             string            `json:"language,omitempty"`
	RequiresConfirmation bool              `json:"requires_confirmation,omitempty"`
	SafeToAutoWrite      bool              `json:"safe_to_auto_write,omitempty"`
	RelativePath         string            `json:"relative_path,omitempty"`
	RequestID            string            `json:"request_id,omitempty"`
	SessionID            string            `json:"session_id,omitempty"`
}

// ArtifactWriteResult captures the file-path, provenance, and write-status metadata returned after one artifact write.
// ArtifactWriteResult 描述一次交付物写入完成后返回的路径、依据与写入状态元信息。
type ArtifactWriteResult struct {
	ArtifactID            string   `json:"artifact_id,omitempty"`
	RelativePath          string   `json:"relative_path,omitempty"`
	Filename              string   `json:"filename,omitempty"`
	Kind                  string   `json:"kind,omitempty"`
	Title                 string   `json:"title,omitempty"`
	Summary               string   `json:"summary,omitempty"`
	Description           string   `json:"description,omitempty"`
	GenerationReason      string   `json:"generation_reason,omitempty"`
	SourceRefs            []string `json:"source_refs,omitempty"`
	SourceSummary         string   `json:"source_summary,omitempty"`
	InputSources          []string `json:"input_sources,omitempty"`
	EvidenceRefs          []string `json:"evidence_refs,omitempty"`
	Assumptions           []string `json:"assumptions,omitempty"`
	RelatedTaskType       string   `json:"related_task_type,omitempty"`
	RelatedScene          string   `json:"related_scene,omitempty"`
	RelatedWorkflowStepID string   `json:"related_workflow_step_id,omitempty"`
	Completeness          string   `json:"completeness,omitempty"`
	RequiresConfirmation  bool     `json:"requires_confirmation,omitempty"`
	SafeToAutoWrite       bool     `json:"safe_to_auto_write,omitempty"`
	Checksum              string   `json:"checksum,omitempty"`
	SizeBytes             int      `json:"size_bytes,omitempty"`
	WriteStatus           string   `json:"write_status,omitempty"`
	Overwritten           bool     `json:"overwritten,omitempty"`
}

// ArtifactWriteInput carries runtime-only inputs needed to materialize one artifact on disk.
// ArtifactWriteInput 描述把一个交付物真正写入磁盘时所需的 runtime 内部输入。
type ArtifactWriteInput struct {
	SharedRootDir         string
	Request               ArtifactWriteRequest
	Content               string
	GenerationReason      string
	SourceRefs            []string
	SourceSummary         string
	InputSources          []string
	EvidenceRefs          []string
	Assumptions           []string
	RelatedTaskType       string
	RelatedScene          string
	RelatedWorkflowStepID string
	Summary               string
	Description           string
	Completeness          string
}

// ResourceKind captures the canonical read-only resource category Athena can consume.
// ResourceKind 描述 Athena 可消费的标准只读资源类别。
type ResourceKind string

const (
	// ResourceKindInjectedDocument marks one injected document or snippet.
	// ResourceKindInjectedDocument 表示一份注入文档或文档片段。
	ResourceKindInjectedDocument ResourceKind = "injected_document"

	// ResourceKindConfigFragment marks one injected config fragment.
	// ResourceKindConfigFragment 表示一段注入配置片段。
	ResourceKindConfigFragment ResourceKind = "config_fragment"

	// ResourceKindTaskContextFile marks one task-local context file projection.
	// ResourceKindTaskContextFile 表示一次任务上下文文件投影。
	ResourceKindTaskContextFile ResourceKind = "task_context_file"

	// ResourceKindKnowledgeView marks one knowledge execution-surface projection.
	// ResourceKindKnowledgeView 表示一条知识执行面投影。
	ResourceKindKnowledgeView ResourceKind = "knowledge_view"

	// ResourceKindRuntimeCache marks one Athena-owned runtime cache projection.
	// ResourceKindRuntimeCache 表示一条 Athena 自有运行时缓存投影。
	ResourceKindRuntimeCache ResourceKind = "runtime_cache"
)

// ResourceScope captures the bounded source scope of one read-only resource.
// ResourceScope 描述一条只读资源的受控来源范围。
type ResourceScope string

const (
	// ResourceScopeSession marks session-scoped injected resources.
	// ResourceScopeSession 表示 session 级注入资源。
	ResourceScopeSession ResourceScope = "session"

	// ResourceScopeTask marks task-scoped injected resources.
	// ResourceScopeTask 表示 task 级注入资源。
	ResourceScopeTask ResourceScope = "task"

	// ResourceScopeKnowledge marks knowledge execution-surface resources.
	// ResourceScopeKnowledge 表示知识执行面资源。
	ResourceScopeKnowledge ResourceScope = "knowledge"

	// ResourceScopeRuntime marks Athena-owned runtime cache resources.
	// ResourceScopeRuntime 表示 Athena 自有 runtime cache 资源。
	ResourceScopeRuntime ResourceScope = "runtime"
)

// ResourceProjection captures the payload shape requested by one read-only resource read.
// ResourceProjection 描述一次只读资源读取请求希望拿到的载荷形状。
type ResourceProjection string

const (
	// ResourceProjectionFull requests the full content field.
	// ResourceProjectionFull 表示返回完整 content 字段。
	ResourceProjectionFull ResourceProjection = "full"

	// ResourceProjectionSummary requests the summary field.
	// ResourceProjectionSummary 表示返回 summary 字段。
	ResourceProjectionSummary ResourceProjection = "summary"

	// ResourceProjectionMetadata requests metadata only.
	// ResourceProjectionMetadata 表示只返回 metadata 字段。
	ResourceProjectionMetadata ResourceProjection = "metadata"
)

// ReadOnlyResourceReadRequest captures the Athena-side read-only resource request contract.
// ReadOnlyResourceReadRequest 描述 Athena 侧只读资源读取请求契约。
type ReadOnlyResourceReadRequest struct {
	ResourceID    string             `json:"resource_id,omitempty"`
	ResourceKind  ResourceKind       `json:"resource_kind,omitempty"`
	ResourceScope ResourceScope      `json:"resource_scope,omitempty"`
	Projection    ResourceProjection `json:"projection,omitempty"`
	RequestID     string             `json:"request_id,omitempty"`
	SessionID     string             `json:"session_id,omitempty"`
}

// ReadOnlyResourceReadResult captures the bounded response returned for one read-only resource read.
// ReadOnlyResourceReadResult 描述一次只读资源读取返回的有边界结果。
type ReadOnlyResourceReadResult struct {
	ResourceID    string         `json:"resource_id,omitempty"`
	ResourceKind  ResourceKind   `json:"resource_kind,omitempty"`
	ResourceScope ResourceScope  `json:"resource_scope,omitempty"`
	Content       string         `json:"content,omitempty"`
	Summary       string         `json:"summary,omitempty"`
	Metadata      map[string]any `json:"metadata,omitempty"`
}

// StructuredDataParseRequest captures the structured-parse request contract.
// StructuredDataParseRequest 描述结构化解析请求契约。
type StructuredDataParseRequest struct {
	Format    string `json:"format,omitempty"`
	Content   string `json:"content,omitempty"`
	Delimiter string `json:"delimiter,omitempty"`
	HasHeader bool   `json:"has_header,omitempty"`
}

// StructuredDataParseResult captures the normalized parse result returned to transport callers.
// StructuredDataParseResult 描述返回给传输层调用方的归一化解析结果。
type StructuredDataParseResult struct {
	Format   string   `json:"format,omitempty"`
	Parsed   any      `json:"parsed,omitempty"`
	RowCount int      `json:"row_count,omitempty"`
	Keys     []string `json:"keys,omitempty"`
	Summary  string   `json:"summary,omitempty"`
}

// LocalDataTransformRequest captures one explicit local data preprocessing request.
// LocalDataTransformRequest 描述一次显式本地数据预处理请求。
type LocalDataTransformRequest struct {
	Operation     string            `json:"operation,omitempty"`
	SourceKeys    []string          `json:"source_keys,omitempty"`
	FieldPaths    []string          `json:"field_paths,omitempty"`
	OutputAliases map[string]string `json:"output_aliases,omitempty"`
	RequestID     string            `json:"request_id,omitempty"`
	SessionID     string            `json:"session_id,omitempty"`
}

// LocalDataTransformResult captures one normalized local preprocessing result.
// LocalDataTransformResult 描述一次归一化的本地预处理结果。
type LocalDataTransformResult struct {
	Operation    string   `json:"operation,omitempty"`
	SourceKeys   []string `json:"source_keys,omitempty"`
	Data         any      `json:"data,omitempty"`
	MissingPaths []string `json:"missing_paths,omitempty"`
	Summary      string   `json:"summary,omitempty"`
}

// FactQuestionScope captures the semantic scope of one fact-sensitive question.
// FactQuestionScope 描述一次事实敏感问题的语义范围。
type FactQuestionScope string

const (
	// FactQuestionScopeHistoricalContext means the question is mainly about historical context.
	// FactQuestionScopeHistoricalContext 表示问题主要关注历史上下文。
	FactQuestionScopeHistoricalContext FactQuestionScope = "historical_context"

	// FactQuestionScopeCurrentState means the question asks for current state or latest status.
	// FactQuestionScopeCurrentState 表示问题要求当前状态或最新状态。
	FactQuestionScopeCurrentState FactQuestionScope = "current_state"

	// FactQuestionScopeTrend means the question asks for trend or time-series changes.
	// FactQuestionScopeTrend 表示问题要求趋势或时间序列变化。
	FactQuestionScopeTrend FactQuestionScope = "trend"

	// FactQuestionScopeMixed means the question mixes historical and current-state semantics.
	// FactQuestionScopeMixed 表示问题同时混合历史和当前状态语义。
	FactQuestionScopeMixed FactQuestionScope = "mixed"
)

// FactSourceMode captures which source family the current answer depends on.
// FactSourceMode 描述当前回答依赖的事实来源模式。
type FactSourceMode string

const (
	// FactSourceModeSessionOnly means only session context was available.
	// FactSourceModeSessionOnly 表示只有 session 上下文参与判断。
	FactSourceModeSessionOnly FactSourceMode = "session_only"

	// FactSourceModeFreshLookupOnly means the answer is based only on fresh lookup or fresh snapshot.
	// FactSourceModeFreshLookupOnly 表示回答只基于 fresh lookup 或最新快照。
	FactSourceModeFreshLookupOnly FactSourceMode = "fresh_lookup_only"

	// FactSourceModeSessionPlusFreshLookup means session context and fresh data were both used.
	// FactSourceModeSessionPlusFreshLookup 表示同时使用了 session 上下文和 fresh data。
	FactSourceModeSessionPlusFreshLookup FactSourceMode = "session_plus_fresh_lookup"
)

// FactAnswerMode captures whether Athena should answer definitively, provisionally, or clarify first.
// FactAnswerMode 描述 Athena 应直接下结论、给初步判断还是先反问。
type FactAnswerMode string

const (
	// FactAnswerModeDefinitive means evidence is sufficient for a definitive answer.
	// FactAnswerModeDefinitive 表示证据已足够给出确定性结论。
	FactAnswerModeDefinitive FactAnswerMode = "definitive"

	// FactAnswerModeProvisional means the answer should be marked as provisional.
	// FactAnswerModeProvisional 表示回答只能作为初步判断。
	FactAnswerModeProvisional FactAnswerMode = "provisional"

	// FactAnswerModeClarification means Athena should ask for clarification or fresh data first.
	// FactAnswerModeClarification 表示 Athena 应先反问或要求 fresh data。
	FactAnswerModeClarification FactAnswerMode = "clarification"
)

// FactQualityGateRequest captures one explicit fact-quality evaluation request.
// FactQualityGateRequest 描述一次显式事实质量评估请求。
type FactQualityGateRequest struct {
	QuestionScope          FactQuestionScope `json:"question_scope,omitempty"`
	SessionContextUsed     bool              `json:"session_context_used,omitempty"`
	FreshLookupPerformed   bool              `json:"fresh_lookup_performed,omitempty"`
	LatestSnapshotProvided bool              `json:"latest_snapshot_provided,omitempty"`
	ConflictingSignals     bool              `json:"conflicting_signals,omitempty"`
	EvidenceRefs           []string          `json:"evidence_refs,omitempty"`
	MissingData            []string          `json:"missing_data,omitempty"`
	ClarificationHint      string            `json:"clarification_hint,omitempty"`
	RequestID              string            `json:"request_id,omitempty"`
	SessionID              string            `json:"session_id,omitempty"`
}

// FactQualityGateResult captures one normalized fact-freshness and evidence-gate result.
// FactQualityGateResult 描述一次归一化的事实新鲜度与证据门禁结果。
type FactQualityGateResult struct {
	QuestionScope        FactQuestionScope `json:"question_scope,omitempty"`
	SourceMode           FactSourceMode    `json:"source_mode,omitempty"`
	FreshnessLevel       string            `json:"freshness_level,omitempty"`
	AuthorityLevel       string            `json:"authority_level,omitempty"`
	Completeness         string            `json:"completeness,omitempty"`
	Consistency          string            `json:"consistency,omitempty"`
	FreshLookupPerformed bool              `json:"fresh_lookup_performed,omitempty"`
	EvidenceGatePassed   bool              `json:"evidence_gate_passed,omitempty"`
	AnswerMode           FactAnswerMode    `json:"answer_mode,omitempty"`
	NeedsClarification   bool              `json:"needs_clarification,omitempty"`
	MissingData          []string          `json:"missing_data,omitempty"`
	ClarifyingQuestion   string            `json:"clarifying_question,omitempty"`
	EvidenceRefs         []string          `json:"evidence_refs,omitempty"`
	Summary              string            `json:"summary,omitempty"`
}

// RuntimeStateQueryRequest captures the explicit bounded runtime-state query request.
// RuntimeStateQueryRequest 描述显式且有边界的运行时状态查询请求。
type RuntimeStateQueryRequest struct {
	SessionID     string   `json:"session_id,omitempty"`
	WorkflowRunID string   `json:"workflow_run_id,omitempty"`
	Include       []string `json:"include,omitempty"`
	RequestID     string   `json:"request_id,omitempty"`
}

// RuntimeSessionSnapshot captures the bounded session portion returned by runtime-state queries.
// RuntimeSessionSnapshot 描述运行时状态查询返回的受限 session 片段。
type RuntimeSessionSnapshot struct {
	SessionID          string `json:"session_id,omitempty"`
	Status             string `json:"status,omitempty"`
	MessageCount       int    `json:"message_count,omitempty"`
	DeferredQueueCount int    `json:"deferred_queue_count,omitempty"`
	ContextAssetCount  int    `json:"context_asset_count,omitempty"`
	ContextBindingCount int   `json:"context_binding_count,omitempty"`
	CompiledRefCount   int    `json:"compiled_ref_count,omitempty"`
	Pending            bool   `json:"pending,omitempty"`
	PendingStage       string `json:"pending_stage,omitempty"`
}

// GovernanceSummary captures the bounded execution-governance summary returned to runtime-state callers.
// GovernanceSummary 描述返回给 runtime-state 查询调用方的受限执行治理摘要。
type GovernanceSummary struct {
	Decision         string `json:"decision,omitempty"`
	RiskLevel        string `json:"risk_level,omitempty"`
	ExecutionStatus  string `json:"execution_status,omitempty"`
	HasIntent        bool   `json:"has_intent,omitempty"`
	HasExecution     bool   `json:"has_execution,omitempty"`
	RequiresApproval bool   `json:"requires_approval,omitempty"`
}

// RuntimeStateQueryResult captures the bounded runtime-state query response contract.
// RuntimeStateQueryResult 描述有边界的运行时状态查询响应契约。
type RuntimeStateQueryResult struct {
	SessionSnapshot   *RuntimeSessionSnapshot `json:"session_snapshot,omitempty"`
	WorkflowPlan      *workflow.Plan          `json:"workflow_plan,omitempty"`
	StructuredResult  map[string]any          `json:"structured_result,omitempty"`
	GovernanceSummary *GovernanceSummary      `json:"governance_summary,omitempty"`
	LastTurnSummary   string                  `json:"last_turn_summary,omitempty"`
}

// ResolveArtifactWriteRequest reads one explicit artifact write request from the current payload.
// ResolveArtifactWriteRequest 会从当前载荷中读取一次显式交付物写入请求。
func ResolveArtifactWriteRequest(inputPayload map[string]any, desiredOutputMode string) *ArtifactWriteRequest {
	if !isExplicitArtifactWriteMode(desiredOutputMode) && extractArtifactWriteRequestMap(inputPayload) == nil {
		return nil
	}
	raw := extractArtifactWriteRequestMap(inputPayload)
	if raw == nil {
		return &ArtifactWriteRequest{}
	}
	return &ArtifactWriteRequest{
		ArtifactID:           stringValue(raw["artifact_id"]),
		ArtifactOwnerKey:     stringValue(raw["artifact_owner_key"]),
		ArtifactOwnerType:    ArtifactOwnerType(stringValue(raw["artifact_owner_type"])),
		Kind:                 stringValue(raw["kind"]),
		Title:                stringValue(raw["title"]),
		Filename:             stringValue(raw["filename"]),
		Language:             stringValue(raw["language"]),
		RequiresConfirmation: boolValue(raw["requires_confirmation"]),
		SafeToAutoWrite:      !rawValueExists(raw, "safe_to_auto_write") || boolValue(raw["safe_to_auto_write"]),
		RelativePath:         stringValue(raw["relative_path"]),
		RequestID:            stringValue(raw["request_id"]),
		SessionID:            stringValue(raw["session_id"]),
	}
}

// WriteArtifact validates one artifact write request and materializes the final file inside the shared root.
// WriteArtifact 会校验交付物写入请求，并把最终文件写入共享根目录。
func WriteArtifact(input ArtifactWriteInput) (*ArtifactWriteResult, error) {
	root := strings.TrimSpace(input.SharedRootDir)
	if root == "" {
		return nil, fmt.Errorf("shared root dir is required")
	}
	if !utf8.ValidString(input.Content) {
		return nil, fmt.Errorf("artifact content must be valid UTF-8 text")
	}
	if strings.ContainsRune(input.Content, '\x00') {
		return nil, fmt.Errorf("artifact content must not contain NUL bytes")
	}

	req := input.Request
	if err := validateArtifactOwner(req.ArtifactOwnerType, req.ArtifactOwnerKey); err != nil {
		return nil, err
	}

	relativePath, filename, err := resolveArtifactRelativePath(req)
	if err != nil {
		return nil, err
	}
	targetPath, err := materializeArtifactPath(root, relativePath)
	if err != nil {
		return nil, err
	}
	finalRelativePath, finalPath, err := versionedArtifactPath(root, relativePath, targetPath)
	if err != nil {
		return nil, err
	}
	if err := writeUTF8TextAtomically(finalPath, input.Content); err != nil {
		return nil, err
	}

	payload := []byte(input.Content)
	checksumBytes := sha256.Sum256(payload)
	result := &ArtifactWriteResult{
		ArtifactID:            defaultStringValue(strings.TrimSpace(req.ArtifactID), buildArtifactID(req, finalRelativePath)),
		RelativePath:          finalRelativePath,
		Filename:              filename,
		Kind:                  normalizeArtifactKind(req.Kind, req.Language, filename),
		Title:                 defaultStringValue(strings.TrimSpace(req.Title), filename),
		Summary:               defaultArtifactSummary(input, filename),
		Description:           defaultArtifactDescription(input, filename),
		GenerationReason:      strings.TrimSpace(input.GenerationReason),
		SourceRefs:            append([]string(nil), input.SourceRefs...),
		SourceSummary:         strings.TrimSpace(input.SourceSummary),
		InputSources:          append([]string(nil), input.InputSources...),
		EvidenceRefs:          append([]string(nil), input.EvidenceRefs...),
		Assumptions:           append([]string(nil), input.Assumptions...),
		RelatedTaskType:       strings.TrimSpace(input.RelatedTaskType),
		RelatedScene:          strings.TrimSpace(input.RelatedScene),
		RelatedWorkflowStepID: strings.TrimSpace(input.RelatedWorkflowStepID),
		Completeness:          defaultStringValue(strings.TrimSpace(input.Completeness), "complete"),
		RequiresConfirmation:  req.RequiresConfirmation,
		SafeToAutoWrite:       req.SafeToAutoWrite,
		Checksum:              hex.EncodeToString(checksumBytes[:]),
		SizeBytes:             len(payload),
		WriteStatus:           "written",
		Overwritten:           false,
	}
	return result, nil
}

// ResolveReadOnlyResourceRequest reads one explicit read-only resource request from the current payload.
// ResolveReadOnlyResourceRequest 会从当前载荷中读取一次显式只读资源请求。
func ResolveReadOnlyResourceRequest(inputPayload map[string]any, desiredOutputMode string) *ReadOnlyResourceReadRequest {
	if !isExplicitReadOnlyResourceMode(desiredOutputMode) && extractMap(inputPayload, "read_only_resource_request") == nil {
		return nil
	}
	raw := extractMap(inputPayload, "read_only_resource_request")
	if raw == nil {
		return &ReadOnlyResourceReadRequest{Projection: ResourceProjectionSummary}
	}
	projection := ResourceProjection(stringValue(raw["projection"]))
	if projection == "" {
		projection = ResourceProjectionSummary
	}
	return &ReadOnlyResourceReadRequest{
		ResourceID:    stringValue(raw["resource_id"]),
		ResourceKind:  ResourceKind(stringValue(raw["resource_kind"])),
		ResourceScope: ResourceScope(stringValue(raw["resource_scope"])),
		Projection:    projection,
		RequestID:     stringValue(raw["request_id"]),
		SessionID:     stringValue(raw["session_id"]),
	}
}

// ReadOnlyResourceRead resolves one bounded resource read from injected payload or local runtime state.
// ReadOnlyResourceRead 会从注入载荷或本地运行时状态中解析一次有边界的资源读取。
func ReadOnlyResourceRead(request ReadOnlyResourceReadRequest, inputPayload, globalContext, appContext map[string]any) *ReadOnlyResourceReadResult {
	resource := resolveResourceSource(request, inputPayload, globalContext, appContext)
	if len(resource) == 0 {
		return nil
	}
	result := &ReadOnlyResourceReadResult{
		ResourceID:    defaultStringValue(request.ResourceID, stringValue(resource["resource_id"])),
		ResourceKind:  request.ResourceKind,
		ResourceScope: request.ResourceScope,
	}
	switch request.Projection {
	case ResourceProjectionFull:
		result.Content = resolveResourceContent(resource)
	case ResourceProjectionMetadata:
		result.Metadata = mapValue(resource["metadata"])
	default:
		result.Summary = defaultStringValue(stringValue(resource["summary"]), summarizeText(resolveResourceContent(resource), 160))
	}
	if result.ResourceKind == "" {
		result.ResourceKind = ResourceKind(stringValue(resource["resource_kind"]))
	}
	if result.ResourceScope == "" {
		result.ResourceScope = ResourceScope(stringValue(resource["resource_scope"]))
	}
	return result
}

// ResolveStructuredDataParseRequest reads one explicit structured-parse request from the current payload.
// ResolveStructuredDataParseRequest 会从当前载荷中读取一次显式结构化解析请求。
func ResolveStructuredDataParseRequest(inputPayload map[string]any, desiredOutputMode string) *StructuredDataParseRequest {
	if !isExplicitStructuredParseMode(desiredOutputMode) && extractMap(inputPayload, "structured_data_request") == nil {
		return nil
	}
	raw := extractMap(inputPayload, "structured_data_request")
	if raw == nil {
		return &StructuredDataParseRequest{}
	}
	return &StructuredDataParseRequest{
		Format:    stringValue(raw["format"]),
		Content:   stringValue(raw["content"]),
		Delimiter: stringValue(raw["delimiter"]),
		HasHeader: !rawValueExists(raw, "has_header") || boolValue(raw["has_header"]),
	}
}

// ParseStructuredData parses one bounded structured text payload into a normalized transport-safe result.
// ParseStructuredData 会把一份有边界的结构化文本载荷解析为归一化的 transport-safe 结果。
func ParseStructuredData(request StructuredDataParseRequest) (*StructuredDataParseResult, error) {
	content := strings.TrimSpace(request.Content)
	if content == "" {
		return nil, nil
	}
	format := defaultStringValue(strings.TrimSpace(strings.ToLower(request.Format)), detectStructuredFormat(content))
	switch format {
	case "json":
		var parsed any
		if err := json.Unmarshal([]byte(content), &parsed); err != nil {
			return nil, err
		}
		return &StructuredDataParseResult{
			Format:  format,
			Parsed:  parsed,
			Keys:    extractTopLevelKeys(parsed),
			Summary: buildStructuredSummary(format, parsed),
		}, nil
	case "yaml", "yml":
		var parsed any
		if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
			return nil, err
		}
		normalized := normalizeYAMLValue(parsed)
		return &StructuredDataParseResult{
			Format:  "yaml",
			Parsed:  normalized,
			Keys:    extractTopLevelKeys(normalized),
			Summary: buildStructuredSummary("yaml", normalized),
		}, nil
	case "csv":
		reader := csv.NewReader(strings.NewReader(content))
		if strings.TrimSpace(request.Delimiter) != "" {
			runes := []rune(request.Delimiter)
			if len(runes) == 1 {
				reader.Comma = runes[0]
			}
		}
		rows, err := reader.ReadAll()
		if err != nil {
			return nil, err
		}
		parsed, rowCount, keys := normalizeCSVRows(rows, request.HasHeader)
		return &StructuredDataParseResult{
			Format:   format,
			Parsed:   parsed,
			RowCount: rowCount,
			Keys:     keys,
			Summary:  fmt.Sprintf("parsed %d csv rows", rowCount),
		}, nil
	case "frontmatter":
		parsed, err := parseFrontMatter(content)
		if err != nil {
			return nil, err
		}
		return &StructuredDataParseResult{
			Format:  format,
			Parsed:  parsed,
			Keys:    extractTopLevelKeys(parsed),
			Summary: buildStructuredSummary(format, parsed),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported structured data format %q", format)
	}
}

// ResolveLocalDataTransformRequest reads one explicit local preprocessing request from the current payload.
// ResolveLocalDataTransformRequest 会从当前载荷中读取一次显式本地预处理请求。
func ResolveLocalDataTransformRequest(inputPayload map[string]any, desiredOutputMode string) *LocalDataTransformRequest {
	if !isExplicitLocalDataTransformMode(desiredOutputMode) && extractMap(inputPayload, "local_data_transform_request") == nil {
		return nil
	}
	raw := extractMap(inputPayload, "local_data_transform_request")
	if raw == nil {
		return &LocalDataTransformRequest{Operation: "merge_objects"}
	}
	return &LocalDataTransformRequest{
		Operation:     defaultStringValue(stringValue(raw["operation"]), "merge_objects"),
		SourceKeys:    stringListValue(raw["source_keys"]),
		FieldPaths:    stringListValue(raw["field_paths"]),
		OutputAliases: stringMapValue(raw["output_aliases"]),
		RequestID:     stringValue(raw["request_id"]),
		SessionID:     stringValue(raw["session_id"]),
	}
}

// TransformLocalData executes one bounded local preprocessing request against injected local data sources.
// TransformLocalData 会针对注入的本地数据源执行一次有边界的本地预处理请求。
func TransformLocalData(request LocalDataTransformRequest, inputPayload, globalContext, appContext map[string]any) (*LocalDataTransformResult, error) {
	sources := collectLocalDataSources(inputPayload, globalContext, appContext)
	if len(sources) == 0 {
		return nil, nil
	}
	sourceKeys := append([]string(nil), request.SourceKeys...)
	if len(sourceKeys) == 0 {
		for key := range sources {
			sourceKeys = append(sourceKeys, key)
		}
		sort.Strings(sourceKeys)
	}
	switch strings.TrimSpace(request.Operation) {
	case "", "merge_objects":
		merged := map[string]any{}
		used := make([]string, 0, len(sourceKeys))
		for _, key := range sourceKeys {
			source := mapValue(sources[key])
			if len(source) == 0 {
				continue
			}
			for field, value := range source {
				merged[field] = value
			}
			used = append(used, key)
		}
		if len(merged) == 0 {
			return nil, nil
		}
		return &LocalDataTransformResult{
			Operation:  "merge_objects",
			SourceKeys: used,
			Data:       merged,
			Summary:    fmt.Sprintf("merged %d local data sources", len(used)),
		}, nil
	case "project_fields":
		if len(sourceKeys) == 0 {
			return nil, fmt.Errorf("project_fields requires at least one source")
		}
		source := mapValue(sources[sourceKeys[0]])
		if len(source) == 0 {
			return nil, nil
		}
		projected := map[string]any{}
		missing := []string{}
		for _, fieldPath := range request.FieldPaths {
			fieldPath = strings.TrimSpace(fieldPath)
			if fieldPath == "" {
				continue
			}
			value, ok := resolvePathValue(source, fieldPath)
			if !ok {
				missing = append(missing, fieldPath)
				continue
			}
			key := defaultStringValue(request.OutputAliases[fieldPath], lastPathSegment(fieldPath))
			projected[key] = value
		}
		return &LocalDataTransformResult{
			Operation:    "project_fields",
			SourceKeys:   []string{sourceKeys[0]},
			Data:         projected,
			MissingPaths: missing,
			Summary:      fmt.Sprintf("projected %d fields from %s", len(projected), sourceKeys[0]),
		}, nil
	default:
		return nil, fmt.Errorf("unsupported local data transform operation %q", request.Operation)
	}
}

// ResolveFactQualityGateRequest reads one explicit fact-quality gate request from the current payload.
// ResolveFactQualityGateRequest 会从当前载荷中读取一次显式事实质量门禁请求。
func ResolveFactQualityGateRequest(inputPayload map[string]any, desiredOutputMode, query string) *FactQualityGateRequest {
	if !isExplicitFactQualityMode(desiredOutputMode) && extractMap(inputPayload, "fact_quality_request") == nil {
		return nil
	}
	raw := extractMap(inputPayload, "fact_quality_request")
	scope := inferFactQuestionScope(query)
	if raw == nil {
		return &FactQualityGateRequest{
			QuestionScope:      scope,
			SessionContextUsed: true,
			MissingData:        defaultMissingData(scope, query),
		}
	}
	if parsedScope := FactQuestionScope(stringValue(raw["question_scope"])); parsedScope != "" {
		scope = parsedScope
	}
	return &FactQualityGateRequest{
		QuestionScope:          scope,
		SessionContextUsed:     !rawValueExists(raw, "session_context_used") || boolValue(raw["session_context_used"]),
		FreshLookupPerformed:   boolValue(raw["fresh_lookup_performed"]),
		LatestSnapshotProvided: boolValue(raw["latest_snapshot_provided"]),
		ConflictingSignals:     boolValue(raw["conflicting_signals"]),
		EvidenceRefs:           stringListValue(raw["evidence_refs"]),
		MissingData:            defaultStringList(stringListValue(raw["missing_data"]), defaultMissingData(scope, query)),
		ClarificationHint:      stringValue(raw["clarification_hint"]),
		RequestID:              stringValue(raw["request_id"]),
		SessionID:              stringValue(raw["session_id"]),
	}
}

// EvaluateFactQualityGate builds one normalized fact-quality result for platform consumption.
// EvaluateFactQualityGate 会构建一份标准化事实质量结果供 platform 消费。
func EvaluateFactQualityGate(request FactQualityGateRequest) *FactQualityGateResult {
	scope := request.QuestionScope
	if scope == "" {
		scope = FactQuestionScopeHistoricalContext
	}
	sourceMode := resolveFactSourceMode(request)
	freshnessLevel, authorityLevel := resolveFactAuthority(request)
	completeness := "sufficient"
	if len(request.MissingData) > 0 {
		completeness = "partial"
	}
	consistency := "consistent"
	if request.ConflictingSignals {
		consistency = "conflicting"
	}
	evidenceGatePassed := evaluateEvidenceGate(scope, request)
	answerMode := resolveFactAnswerMode(evidenceGatePassed, request.MissingData)
	clarifyingQuestion := ""
	if answerMode == FactAnswerModeClarification {
		clarifyingQuestion = defaultStringValue(strings.TrimSpace(request.ClarificationHint), buildClarifyingQuestion(scope, request.MissingData))
	}
	return &FactQualityGateResult{
		QuestionScope:        scope,
		SourceMode:           sourceMode,
		FreshnessLevel:       freshnessLevel,
		AuthorityLevel:       authorityLevel,
		Completeness:         completeness,
		Consistency:          consistency,
		FreshLookupPerformed: request.FreshLookupPerformed,
		EvidenceGatePassed:   evidenceGatePassed,
		AnswerMode:           answerMode,
		NeedsClarification:   answerMode == FactAnswerModeClarification,
		MissingData:          append([]string(nil), request.MissingData...),
		ClarifyingQuestion:   clarifyingQuestion,
		EvidenceRefs:         append([]string(nil), request.EvidenceRefs...),
		Summary:              buildFactQualitySummary(sourceMode, answerMode, len(request.MissingData)),
	}
}

// ResolveRuntimeStateQueryRequest reads one explicit bounded runtime-state query request from the current payload.
// ResolveRuntimeStateQueryRequest 会从当前载荷中读取一次显式且有边界的运行时状态查询请求。
func ResolveRuntimeStateQueryRequest(inputPayload map[string]any, desiredOutputMode string) *RuntimeStateQueryRequest {
	if !isExplicitRuntimeStateMode(desiredOutputMode) && extractMap(inputPayload, "runtime_state_request") == nil {
		return nil
	}
	raw := extractMap(inputPayload, "runtime_state_request")
	if raw == nil {
		return &RuntimeStateQueryRequest{}
	}
	return &RuntimeStateQueryRequest{
		SessionID:     stringValue(raw["session_id"]),
		WorkflowRunID: stringValue(raw["workflow_run_id"]),
		Include:       stringListValue(raw["include"]),
		RequestID:     stringValue(raw["request_id"]),
	}
}

// QueryRuntimeState builds one bounded runtime-state response from the current session, result, and workflow context.
// QueryRuntimeState 会根据当前 session、结果和 workflow 上下文构建一次有边界的运行时状态响应。
func QueryRuntimeState(request RuntimeStateQueryRequest, currentSession *session.Session, currentResult map[string]any, workflowPlan *workflow.Plan, executionIntent *ExecutionIntent, executionResult *ExecutionResult, lastTurnSummary string) *RuntimeStateQueryResult {
	includeSet := normalizedIncludeSet(request.Include)
	result := &RuntimeStateQueryResult{}

	if includeSet["session_snapshot"] {
		result.SessionSnapshot = buildRuntimeSessionSnapshot(currentSession)
	}
	if includeSet["workflow_plan"] && workflowPlan != nil {
		copied := *workflowPlan
		copied.Steps = append([]workflow.Step(nil), workflowPlan.Steps...)
		result.WorkflowPlan = &copied
	}
	if includeSet["structured_result"] && len(currentResult) > 0 {
		result.StructuredResult = cloneStructuredResultForState(currentResult)
	}
	if includeSet["governance_summary"] {
		result.GovernanceSummary = buildGovernanceSummary(executionIntent, executionResult)
	}
	if includeSet["last_turn_summary"] {
		result.LastTurnSummary = strings.TrimSpace(lastTurnSummary)
	}
	if result.SessionSnapshot == nil && result.WorkflowPlan == nil && len(result.StructuredResult) == 0 && result.GovernanceSummary == nil && result.LastTurnSummary == "" {
		return nil
	}
	return result
}

func isExplicitArtifactWriteMode(mode string) bool {
	return strings.TrimSpace(mode) == DesiredOutputModeArtifactWrite
}

func isExplicitReadOnlyResourceMode(mode string) bool {
	return strings.TrimSpace(mode) == DesiredOutputModeReadOnlyResourceRead
}

func isExplicitStructuredParseMode(mode string) bool {
	return strings.TrimSpace(mode) == DesiredOutputModeStructuredDataParse
}

func isExplicitLocalDataTransformMode(mode string) bool {
	return strings.TrimSpace(mode) == DesiredOutputModeLocalDataTransform
}

func isExplicitFactQualityMode(mode string) bool {
	return strings.TrimSpace(mode) == DesiredOutputModeFactQualityGate
}

func isExplicitRuntimeStateMode(mode string) bool {
	return strings.TrimSpace(mode) == DesiredOutputModeQueryRuntimeState
}

func extractArtifactWriteRequestMap(inputPayload map[string]any) map[string]any {
	return extractMap(inputPayload, "artifact_request")
}

func extractMap(inputPayload map[string]any, key string) map[string]any {
	if len(inputPayload) == 0 {
		return nil
	}
	nested, ok := inputPayload[key].(map[string]any)
	if !ok || len(nested) == 0 {
		return nil
	}
	return nested
}

func validateArtifactOwner(ownerType ArtifactOwnerType, ownerKey string) error {
	if strings.TrimSpace(ownerKey) == "" {
		return fmt.Errorf("artifact_owner_key is required")
	}
	switch ownerType {
	case ArtifactOwnerTypeUser, ArtifactOwnerTypeWorkspace, ArtifactOwnerTypeApp, ArtifactOwnerTypeIntegration:
		return nil
	default:
		return fmt.Errorf("artifact_owner_type must be one of user, workspace, app, integration")
	}
}

func resolveArtifactRelativePath(request ArtifactWriteRequest) (string, string, error) {
	filename := defaultArtifactFilename(request)
	if strings.TrimSpace(request.RelativePath) == "" {
		relativePath := path.Join(ownerDirectoryPrefix(request.ArtifactOwnerType, request.ArtifactOwnerKey), artifactKindDirectory(request.Kind, request.Language, filename), filename)
		return relativePath, filename, nil
	}
	cleaned, err := validateRelativeArtifactPath(strings.TrimSpace(request.RelativePath))
	if err != nil {
		return "", "", err
	}
	expectedPrefix := ownerDirectoryPrefix(request.ArtifactOwnerType, request.ArtifactOwnerKey) + "/"
	if !strings.HasPrefix(cleaned, expectedPrefix) {
		return "", "", fmt.Errorf("relative_path must stay under %s", expectedPrefix)
	}
	parts := strings.Split(cleaned, "/")
	if len(parts) < 4 {
		return "", "", fmt.Errorf("relative_path must include owner prefix, whitelist directory, and filename")
	}
	if _, ok := artifactWhitelistDirectories[parts[2]]; !ok {
		return "", "", fmt.Errorf("relative_path must use one of the whitelist directories")
	}
	return cleaned, path.Base(cleaned), nil
}

func validateRelativeArtifactPath(raw string) (string, error) {
	trimmed := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	switch {
	case trimmed == "":
		return "", fmt.Errorf("relative_path must not be empty")
	case strings.HasPrefix(trimmed, "/"):
		return "", fmt.Errorf("absolute paths are not allowed")
	case strings.HasPrefix(trimmed, "~"):
		return "", fmt.Errorf("home-directory paths are not allowed")
	case filepath.VolumeName(trimmed) != "":
		return "", fmt.Errorf("volume-prefixed paths are not allowed")
	case strings.Contains(trimmed, ".."):
		return "", fmt.Errorf("parent-directory traversal is not allowed")
	}
	cleaned := path.Clean(trimmed)
	if cleaned == "." || strings.HasPrefix(cleaned, "../") {
		return "", fmt.Errorf("relative_path escapes the shared root")
	}
	return cleaned, nil
}

func materializeArtifactPath(sharedRootDir, relativePath string) (string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(sharedRootDir))
	if err != nil {
		return "", err
	}
	target := filepath.Join(rootAbs, filepath.FromSlash(relativePath))
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return "", err
	}
	if !strings.HasPrefix(targetAbs, rootAbs+string(os.PathSeparator)) && targetAbs != rootAbs {
		return "", fmt.Errorf("relative_path escapes the configured shared root")
	}
	return targetAbs, nil
}

func versionedArtifactPath(sharedRootDir, relativePath, absoluteTarget string) (string, string, error) {
	if _, err := os.Stat(absoluteTarget); os.IsNotExist(err) {
		return relativePath, absoluteTarget, nil
	} else if err != nil {
		return "", "", err
	}
	ext := path.Ext(relativePath)
	base := strings.TrimSuffix(relativePath, ext)
	for version := 2; version < 1000; version++ {
		candidateRelative := fmt.Sprintf("%s.v%d%s", base, version, ext)
		candidateAbsolute, err := materializeArtifactPath(sharedRootDir, candidateRelative)
		if err != nil {
			return "", "", err
		}
		if _, err := os.Stat(candidateAbsolute); os.IsNotExist(err) {
			return candidateRelative, candidateAbsolute, nil
		} else if err != nil {
			return "", "", err
		}
	}
	return "", "", fmt.Errorf("unable to allocate a versioned artifact path")
}

func writeUTF8TextAtomically(targetPath, content string) error {
	parent := filepath.Dir(targetPath)
	if err := os.MkdirAll(parent, 0o755); err != nil {
		return err
	}
	tempFile, err := os.CreateTemp(parent, ".athena-artifact-*")
	if err != nil {
		return err
	}
	tempPath := tempFile.Name()
	defer os.Remove(tempPath)
	if _, err := tempFile.WriteString(content); err != nil {
		tempFile.Close()
		return err
	}
	if err := tempFile.Close(); err != nil {
		return err
	}
	return os.Rename(tempPath, targetPath)
}

func defaultArtifactFilename(request ArtifactWriteRequest) string {
	if trimmed := strings.TrimSpace(request.Filename); trimmed != "" {
		return sanitizeFilename(trimmed)
	}
	title := sanitizeFilename(strings.ToLower(strings.ReplaceAll(strings.TrimSpace(request.Title), " ", "-")))
	if title == "" {
		title = normalizeArtifactKind(request.Kind, request.Language, "")
	}
	ext := artifactFileExtension(request.Kind, request.Language)
	if !strings.HasSuffix(title, ext) {
		title += ext
	}
	return title
}

func defaultArtifactSummary(input ArtifactWriteInput, filename string) string {
	if trimmed := strings.TrimSpace(input.Summary); trimmed != "" {
		return trimmed
	}
	return summarizeText(input.Content, 160)
}

func defaultArtifactDescription(input ArtifactWriteInput, filename string) string {
	if trimmed := strings.TrimSpace(input.Description); trimmed != "" {
		return trimmed
	}
	return fmt.Sprintf("delivery artifact written to %s", filename)
}

func buildArtifactID(request ArtifactWriteRequest, relativePath string) string {
	if strings.TrimSpace(request.ArtifactID) != "" {
		return strings.TrimSpace(request.ArtifactID)
	}
	return "artifact-" + strings.ReplaceAll(relativePath, "/", "-")
}

func ownerDirectoryPrefix(ownerType ArtifactOwnerType, ownerKey string) string {
	switch ownerType {
	case ArtifactOwnerTypeWorkspace:
		return path.Join("workspaces", ownerKey)
	case ArtifactOwnerTypeApp:
		return path.Join("apps", ownerKey)
	case ArtifactOwnerTypeIntegration:
		return path.Join("integrations", ownerKey)
	default:
		return path.Join("users", ownerKey)
	}
}

func artifactKindDirectory(kind, language, filename string) string {
	normalized := normalizeArtifactKind(kind, language, filename)
	switch normalized {
	case "markdown", "txt", "report":
		return "reports"
	case "python", "shell", "sql", "javascript", "typescript":
		return "scripts"
	case "json", "yaml", "csv", "xml", "toml", "ini", "config":
		return "exports"
	default:
		return "deliverables"
	}
}

func normalizeArtifactKind(kind, language, filename string) string {
	normalized := strings.TrimSpace(strings.ToLower(kind))
	if normalized != "" {
		return normalized
	}
	switch strings.TrimSpace(strings.ToLower(language)) {
	case "python", "shell", "sql", "javascript", "typescript":
		return strings.TrimSpace(strings.ToLower(language))
	}
	switch strings.TrimSpace(strings.ToLower(path.Ext(filename))) {
	case ".md":
		return "markdown"
	case ".txt":
		return "txt"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".csv":
		return "csv"
	case ".py":
		return "python"
	case ".sh":
		return "shell"
	case ".sql":
		return "sql"
	case ".xml":
		return "xml"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".toml":
		return "toml"
	case ".ini", ".conf":
		return "config"
	default:
		return "deliverable"
	}
}

func artifactFileExtension(kind, language string) string {
	switch normalizeArtifactKind(kind, language, "") {
	case "markdown":
		return ".md"
	case "txt":
		return ".txt"
	case "json":
		return ".json"
	case "yaml":
		return ".yaml"
	case "csv":
		return ".csv"
	case "python":
		return ".py"
	case "shell":
		return ".sh"
	case "sql":
		return ".sql"
	case "xml":
		return ".xml"
	case "javascript":
		return ".js"
	case "typescript":
		return ".ts"
	case "toml":
		return ".toml"
	case "config":
		return ".conf"
	default:
		return ".txt"
	}
}

func sanitizeFilename(raw string) string {
	replacer := strings.NewReplacer("/", "-", "\\", "-", "..", "-", "~", "-", ":", "-")
	cleaned := strings.TrimSpace(replacer.Replace(raw))
	if cleaned == "" {
		return "artifact.txt"
	}
	return cleaned
}

func resolveResourceSource(request ReadOnlyResourceReadRequest, inputPayload, globalContext, appContext map[string]any) map[string]any {
	if resources := extractMap(inputPayload, "read_only_resources"); len(resources) > 0 {
		if nested, ok := resources[request.ResourceID].(map[string]any); ok {
			return nested
		}
	}
	switch request.ResourceKind {
	case ResourceKindKnowledgeView:
		if resource := extractMap(inputPayload, "knowledge_view"); len(resource) > 0 {
			return resource
		}
	case ResourceKindRuntimeCache:
		if resource := extractMap(inputPayload, "runtime_cache"); len(resource) > 0 {
			return resource
		}
		if resource := extractMap(globalContext, "runtime_cache"); len(resource) > 0 {
			return resource
		}
	case ResourceKindConfigFragment:
		if resource := extractMap(appContext, request.ResourceID); len(resource) > 0 {
			return map[string]any{"metadata": resource}
		}
		if resource := extractMap(globalContext, request.ResourceID); len(resource) > 0 {
			return map[string]any{"metadata": resource}
		}
	}
	return nil
}

func resolveResourceContent(resource map[string]any) string {
	if resource == nil {
		return ""
	}
	if content := stringValue(resource["content"]); content != "" {
		return content
	}
	if payload := resource["payload"]; payload != nil {
		if encoded, err := json.Marshal(payload); err == nil {
			return string(encoded)
		}
	}
	if metadata := mapValue(resource["metadata"]); len(metadata) > 0 {
		if encoded, err := json.Marshal(metadata); err == nil {
			return string(encoded)
		}
	}
	return ""
}

func detectStructuredFormat(content string) string {
	trimmed := strings.TrimSpace(content)
	switch {
	case strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "["):
		return "json"
	case strings.HasPrefix(trimmed, "---\n") || strings.HasPrefix(trimmed, "---\r\n"):
		return "frontmatter"
	case strings.Contains(trimmed, ",") && strings.Contains(trimmed, "\n"):
		return "csv"
	default:
		return "yaml"
	}
}

func normalizeCSVRows(rows [][]string, hasHeader bool) (any, int, []string) {
	if len(rows) == 0 {
		return []map[string]string{}, 0, nil
	}
	if !hasHeader || len(rows[0]) == 0 {
		return rows, len(rows), nil
	}
	headers := rows[0]
	parsed := make([]map[string]string, 0, len(rows)-1)
	for _, row := range rows[1:] {
		entry := make(map[string]string, len(headers))
		for idx, header := range headers {
			if idx < len(row) {
				entry[header] = row[idx]
			}
		}
		parsed = append(parsed, entry)
	}
	return parsed, len(parsed), append([]string(nil), headers...)
}

func parseFrontMatter(content string) (map[string]any, error) {
	normalized := strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(normalized, "---\n") {
		return nil, fmt.Errorf("frontmatter payload must start with ---")
	}
	parts := strings.SplitN(normalized, "\n---\n", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("frontmatter payload is missing closing ---")
	}
	var metadata map[string]any
	if err := yaml.Unmarshal([]byte(strings.TrimPrefix(parts[0], "---\n")), &metadata); err != nil {
		return nil, err
	}
	return map[string]any{
		"frontmatter": metadata,
		"body":        parts[1],
	}, nil
}

func normalizeYAMLValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = normalizeYAMLValue(item)
		}
		return result
	case map[any]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[fmt.Sprintf("%v", key)] = normalizeYAMLValue(item)
		}
		return result
	case []any:
		result := make([]any, 0, len(typed))
		for _, item := range typed {
			result = append(result, normalizeYAMLValue(item))
		}
		return result
	default:
		return typed
	}
}

func buildStructuredSummary(format string, parsed any) string {
	keys := extractTopLevelKeys(parsed)
	if len(keys) == 0 {
		return fmt.Sprintf("parsed %s payload", format)
	}
	return fmt.Sprintf("parsed %s payload with keys: %s", format, strings.Join(keys, ", "))
}

func extractTopLevelKeys(parsed any) []string {
	mapped, ok := parsed.(map[string]any)
	if !ok || len(mapped) == 0 {
		return nil
	}
	keys := make([]string, 0, len(mapped))
	for key := range mapped {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func normalizedIncludeSet(include []string) map[string]bool {
	result := map[string]bool{
		"session_snapshot":   false,
		"workflow_plan":      false,
		"structured_result":  false,
		"governance_summary": false,
		"last_turn_summary":  false,
	}
	if len(include) == 0 {
		for key := range result {
			result[key] = true
		}
		return result
	}
	for _, item := range include {
		if _, ok := result[item]; ok {
			result[item] = true
		}
	}
	return result
}

func buildRuntimeSessionSnapshot(currentSession *session.Session) *RuntimeSessionSnapshot {
	if currentSession == nil {
		return nil
	}
	snapshot := &RuntimeSessionSnapshot{
		SessionID:          currentSession.ID,
		Status:             currentSession.Status(),
		MessageCount:       len(currentSession.Messages),
		DeferredQueueCount: len(currentSession.DeferredQueue),
		ContextAssetCount:  len(currentSession.ContextAssets),
		ContextBindingCount: len(currentSession.ContextAssetBindings),
		CompiledRefCount:   len(currentSession.CompiledAssetRefs),
		Pending:            currentSession.Pending != nil,
	}
	if currentSession.Pending != nil {
		snapshot.PendingStage = currentSession.Pending.Stage
	}
	return snapshot
}

func cloneStructuredResultForState(currentResult map[string]any) map[string]any {
	cloned := make(map[string]any, len(currentResult))
	for key, value := range currentResult {
		if key == "runtime_state" {
			continue
		}
		cloned[key] = value
	}
	return cloned
}

func buildGovernanceSummary(executionIntent *ExecutionIntent, executionResult *ExecutionResult) *GovernanceSummary {
	if executionIntent == nil && executionResult == nil {
		return nil
	}
	summary := &GovernanceSummary{
		HasIntent:    executionIntent != nil,
		HasExecution: executionResult != nil,
	}
	if executionIntent != nil {
		summary.RiskLevel = string(executionIntent.RiskLevel)
		if executionIntent.DenyReason != "" {
			summary.Decision = "deny"
		} else if executionIntent.RequiresConfirmation {
			summary.Decision = "confirm"
			summary.RequiresApproval = true
		} else if executionIntent.Allowed {
			summary.Decision = "allow"
		}
	}
	if executionResult != nil {
		summary.ExecutionStatus = executionResult.Status
		if summary.Decision == "" {
			summary.Decision = "observed_execution_result"
		}
	}
	return summary
}

func summarizeText(text string, limit int) string {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return string(runes[:limit]) + "..."
}

func rawValueExists(values map[string]any, key string) bool {
	if len(values) == 0 {
		return false
	}
	_, ok := values[key]
	return ok
}

func stringMapValue(value any) map[string]string {
	mapped, ok := value.(map[string]any)
	if !ok || len(mapped) == 0 {
		return nil
	}
	result := make(map[string]string, len(mapped))
	for key, item := range mapped {
		if text := strings.TrimSpace(stringValue(item)); text != "" {
			result[strings.TrimSpace(key)] = text
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func collectLocalDataSources(inputPayload, globalContext, appContext map[string]any) map[string]any {
	combined := map[string]any{}
	for _, values := range []map[string]any{globalContext, appContext, inputPayload} {
		nested := extractMap(values, "local_data_sources")
		for key, value := range nested {
			combined[key] = value
		}
	}
	if len(combined) == 0 {
		return nil
	}
	return combined
}

func resolvePathValue(root map[string]any, fieldPath string) (any, bool) {
	current := any(root)
	for _, segment := range strings.Split(fieldPath, ".") {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return nil, false
		}
		mapped, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		next, ok := mapped[segment]
		if !ok {
			return nil, false
		}
		current = next
	}
	return current, true
}

func lastPathSegment(fieldPath string) string {
	parts := strings.Split(strings.TrimSpace(fieldPath), ".")
	if len(parts) == 0 {
		return ""
	}
	return strings.TrimSpace(parts[len(parts)-1])
}

func inferFactQuestionScope(query string) FactQuestionScope {
	normalized := strings.ToLower(strings.TrimSpace(query))
	switch {
	case normalized == "":
		return FactQuestionScopeHistoricalContext
	case strings.Contains(normalized, "趋势") || strings.Contains(normalized, "trend"):
		return FactQuestionScopeTrend
	case strings.Contains(normalized, "当前") || strings.Contains(normalized, "现在") || strings.Contains(normalized, "latest") || strings.Contains(normalized, "current") || strings.Contains(normalized, "status"):
		return FactQuestionScopeCurrentState
	default:
		return FactQuestionScopeHistoricalContext
	}
}

func defaultMissingData(scope FactQuestionScope, query string) []string {
	normalized := strings.ToLower(strings.TrimSpace(query))
	switch scope {
	case FactQuestionScopeTrend:
		return []string{"recent_time_series"}
	case FactQuestionScopeCurrentState, FactQuestionScopeMixed:
		switch {
		case strings.Contains(normalized, "order") || strings.Contains(normalized, "订单"):
			return []string{"latest_orders"}
		case strings.Contains(normalized, "review") || strings.Contains(normalized, "审核"):
			return []string{"review_status"}
		case strings.Contains(normalized, "risk") || strings.Contains(normalized, "风险") || strings.Contains(normalized, "security") || strings.Contains(normalized, "安全"):
			return []string{"risk_flags", "review_status", "latest_orders"}
		default:
			return []string{"latest_snapshot"}
		}
	default:
		return nil
	}
}

func defaultStringList(value []string, fallback []string) []string {
	if len(value) > 0 {
		return value
	}
	return append([]string(nil), fallback...)
}

func resolveFactSourceMode(request FactQualityGateRequest) FactSourceMode {
	hasFresh := request.FreshLookupPerformed || request.LatestSnapshotProvided
	switch {
	case hasFresh && request.SessionContextUsed:
		return FactSourceModeSessionPlusFreshLookup
	case hasFresh:
		return FactSourceModeFreshLookupOnly
	default:
		return FactSourceModeSessionOnly
	}
}

func resolveFactAuthority(request FactQualityGateRequest) (string, string) {
	switch {
	case request.FreshLookupPerformed:
		return "current", "fresh_lookup"
	case request.LatestSnapshotProvided:
		return "current", "platform_snapshot"
	case request.SessionContextUsed:
		return "historical", "session"
	default:
		return "historical", "session"
	}
}

func evaluateEvidenceGate(scope FactQuestionScope, request FactQualityGateRequest) bool {
	hasFreshTruth := request.FreshLookupPerformed || request.LatestSnapshotProvided
	hasEvidence := len(request.EvidenceRefs) > 0
	hasMissing := len(request.MissingData) > 0
	if request.ConflictingSignals {
		return false
	}
	switch scope {
	case FactQuestionScopeCurrentState, FactQuestionScopeTrend, FactQuestionScopeMixed:
		return !hasMissing && (hasFreshTruth || hasEvidence)
	default:
		if hasMissing {
			return false
		}
		return true
	}
}

func resolveFactAnswerMode(evidenceGatePassed bool, missingData []string) FactAnswerMode {
	switch {
	case evidenceGatePassed:
		return FactAnswerModeDefinitive
	case len(missingData) > 0:
		return FactAnswerModeClarification
	default:
		return FactAnswerModeProvisional
	}
}

func buildClarifyingQuestion(scope FactQuestionScope, missingData []string) string {
	if len(missingData) == 0 {
		return "当前证据还不够充分。你希望我先基于历史上下文给出初步判断，还是先获取最新数据？"
	}
	switch scope {
	case FactQuestionScopeTrend:
		return fmt.Sprintf("当前缺少趋势判断所需的新数据。若要给出趋势结论，我需要先获取：%s。你希望我先基于已有上下文做初步判断，还是先查询最新状态？", strings.Join(missingData, "、"))
	default:
		return fmt.Sprintf("当前缺少最新事实数据。若要给出准确结论，我需要先获取：%s。你希望我先基于历史上下文做初步判断，还是先查询最新状态？", strings.Join(missingData, "、"))
	}
}

func buildFactQualitySummary(sourceMode FactSourceMode, answerMode FactAnswerMode, missingCount int) string {
	switch answerMode {
	case FactAnswerModeDefinitive:
		return fmt.Sprintf("fact quality passed with source_mode=%s", sourceMode)
	case FactAnswerModeProvisional:
		return fmt.Sprintf("fact quality is provisional with source_mode=%s", sourceMode)
	default:
		return fmt.Sprintf("fact quality requires clarification with %d missing data points", missingCount)
	}
}

func defaultArtifactTimestamp() string {
	return time.Now().UTC().Format(time.RFC3339)
}
