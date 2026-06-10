// contract_foundation.go defines v2 runtime contract foundation records.
// contract_foundation.go 定义 v2 runtime contract foundation 记录。
package runtime

import (
	"context"
	"strings"
	"time"
)

const (
	// TaskTypeStatusDraft marks a registered task type that is not execution-ready yet.
	// TaskTypeStatusDraft 表示 registered task type 仍未准备好执行。
	TaskTypeStatusDraft      = "draft"
	TaskTypeStatusActive     = "active"
	TaskTypeStatusDeprecated = "deprecated"

	// HookPointBeforeRun runs before one runtime task run starts.
	// HookPointBeforeRun 表示 hook 在 runtime task run 开始前执行。
	HookPointBeforeRun        = "before_run"
	HookPointBeforeStep       = "before_step"
	HookPointAfterStep        = "after_step"
	HookPointAfterRun         = "after_run"
	HookPointOnError          = "on_error"
	HookPointBeforeProjection = "before_projection"
	HookPointAfterProjection  = "after_projection"

	// HookBindingKindEinoMiddleware maps one hook to an Eino ADK middleware surface.
	// HookBindingKindEinoMiddleware 表示 hook 映射到 Eino ADK middleware 承载面。
	HookBindingKindEinoMiddleware = "eino_middleware"
	HookBindingKindEinoCallback   = "eino_callback"
	HookBindingKindGraphNode      = "graph_node"
	HookBindingKindPolicyRef      = "policy_ref"

	// HookFailurePolicyFailClosed blocks execution when the hook fails.
	// HookFailurePolicyFailClosed 表示 hook 失败时阻断执行。
	HookFailurePolicyFailClosed = "fail_closed"
	HookFailurePolicyRecordOnly = "record_only"

	// SystemTruthSourceStatusImported marks an accepted system truth source record.
	// SystemTruthSourceStatusImported 表示 system truth source 记录已被接收。
	SystemTruthSourceStatusImported = "imported"

	// SystemTruthDraftStatusDraft marks an editable system truth draft.
	// SystemTruthDraftStatusDraft 表示 system truth draft 仍处于可编辑状态。
	SystemTruthDraftStatusDraft    = "draft"
	SystemTruthDraftStatusCompiled = "compiled"
	SystemTruthDraftStatusRejected = "rejected"

	// SystemTruthCompileStatusSucceeded marks a successful typed compile.
	// SystemTruthCompileStatusSucceeded 表示 typed compile 已成功。
	SystemTruthCompileStatusSucceeded = "succeeded"
	SystemTruthCompileStatusFailed    = "failed"
)

// TaskTypeRegistration is one registered runtime task type and its default contract link.
// TaskTypeRegistration 表示一个已注册 runtime task type 及其默认 contract 关联。
type TaskTypeRegistration struct {
	ID                string
	TypeKey           string
	DisplayName       string
	Description       string
	Status            string
	InputSchema       map[string]any
	ValidatorRefs     map[string]any
	DefaultContractID string
	Compatibility     map[string]any
	Metadata          map[string]any
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

// HookBinding describes one allowlisted runtime lifecycle extension binding.
// HookBinding 描述一个 allowlisted runtime 生命周期扩展绑定。
type HookBinding struct {
	ID            string
	ContractID    string
	HookPoint     string
	BindingKind   string
	BindingRef    string
	OrderIndex    int
	Enabled       bool
	FailurePolicy string
	Config        map[string]any
	Metadata      map[string]any
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// SystemTruthSource records the original source accepted into the system truth lifecycle.
// SystemTruthSource 记录进入 system truth lifecycle 的原始来源。
type SystemTruthSource struct {
	ID          string
	AssetID     string
	SourceKind  string
	SourceRef   string
	Status      string
	Content     map[string]any
	ContentHash string
	Metadata    map[string]any
	CreatedAt   time.Time
}

// SystemTruthDraft records one editable system truth draft.
// SystemTruthDraft 记录一份可编辑 system truth draft。
type SystemTruthDraft struct {
	ID           string
	SourceID     string
	AssetID      string
	Status       string
	Author       string
	Reason       string
	BaseActiveID string
	Content      map[string]any
	DiffSummary  string
	Metadata     map[string]any
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// SystemTruthCompileResult records diagnostics and compiled payload for one draft.
// SystemTruthCompileResult 记录一份 draft 的 diagnostics 与 compiled payload。
type SystemTruthCompileResult struct {
	ID              string
	DraftID         string
	AssetID         string
	Status          string
	Summary         string
	Diagnostics     map[string]any
	CompiledPayload map[string]any
	ContentHash     string
	Metadata        map[string]any
	CreatedAt       time.Time
}

// SystemTruthActiveVersion records one audited active pointer change.
// SystemTruthActiveVersion 记录一次可审计的 active pointer 变更。
type SystemTruthActiveVersion struct {
	ID              string
	AssetID         string
	CompileResultID string
	DraftID         string
	ActivatedBy     string
	Reason          string
	RollbackFromID  string
	Metadata        map[string]any
	ActivatedAt     time.Time
}

// TaskTypeRegistryStore is the storage boundary for registered task types.
// TaskTypeRegistryStore 是 registered task type 的存储边界。
type TaskTypeRegistryStore interface {
	AutoMigrate(context.Context) error
	CreateTaskTypeRegistration(context.Context, TaskTypeRegistration) (TaskTypeRegistration, error)
	PutTaskTypeRegistration(context.Context, TaskTypeRegistration) (TaskTypeRegistration, error)
	GetTaskTypeRegistration(context.Context, string) (TaskTypeRegistration, bool, error)
	GetTaskTypeRegistrationByKey(context.Context, string) (TaskTypeRegistration, bool, error)
	ListTaskTypeRegistrations(context.Context, TaskTypeRegistrationListFilter) ([]TaskTypeRegistration, error)
}

// HookBindingStore is the storage boundary for runtime hook bindings.
// HookBindingStore 是 runtime hook binding 的存储边界。
type HookBindingStore interface {
	AutoMigrate(context.Context) error
	CreateHookBinding(context.Context, HookBinding) (HookBinding, error)
	PutHookBinding(context.Context, HookBinding) (HookBinding, error)
	GetHookBinding(context.Context, string) (HookBinding, bool, error)
	ListHookBindings(context.Context, HookBindingListFilter) ([]HookBinding, error)
}

// SystemTruthLifecycleStore is the storage boundary for system truth lifecycle records.
// SystemTruthLifecycleStore 是 system truth lifecycle 记录的存储边界。
type SystemTruthLifecycleStore interface {
	AutoMigrate(context.Context) error
	CreateSystemTruthSource(context.Context, SystemTruthSource) (SystemTruthSource, error)
	GetSystemTruthSource(context.Context, string) (SystemTruthSource, bool, error)
	ListSystemTruthSources(context.Context, SystemTruthSourceListFilter) ([]SystemTruthSource, error)
	CreateSystemTruthDraft(context.Context, SystemTruthDraft) (SystemTruthDraft, error)
	GetSystemTruthDraft(context.Context, string) (SystemTruthDraft, bool, error)
	ListSystemTruthDrafts(context.Context, SystemTruthDraftListFilter) ([]SystemTruthDraft, error)
	CreateSystemTruthCompileResult(context.Context, SystemTruthCompileResult) (SystemTruthCompileResult, error)
	GetSystemTruthCompileResult(context.Context, string) (SystemTruthCompileResult, bool, error)
	ListSystemTruthCompileResults(context.Context, SystemTruthCompileResultListFilter) ([]SystemTruthCompileResult, error)
	ActivateSystemTruthVersion(context.Context, SystemTruthActiveVersion) (SystemTruthActiveVersion, error)
	GetSystemTruthActiveVersion(context.Context, string) (SystemTruthActiveVersion, bool, error)
	GetActiveSystemTruthVersion(context.Context, string) (SystemTruthActiveVersion, bool, error)
	ListSystemTruthActiveVersions(context.Context, string) ([]SystemTruthActiveVersion, error)
}

// TaskTypeRegistrationListFilter narrows registered task type listing.
// TaskTypeRegistrationListFilter 筛选 registered task type 列表。
type TaskTypeRegistrationListFilter struct {
	Status string
	Limit  int
}

// HookBindingListFilter narrows hook binding listing.
// HookBindingListFilter 筛选 hook binding 列表。
type HookBindingListFilter struct {
	ContractID string
	HookPoint  string
	Enabled    *bool
	Limit      int
}

// SystemTruthSourceListFilter narrows system truth source listing.
// SystemTruthSourceListFilter 筛选 system truth source 列表。
type SystemTruthSourceListFilter struct {
	AssetID string
	Status  string
	Limit   int
}

// SystemTruthDraftListFilter narrows system truth draft listing.
// SystemTruthDraftListFilter 筛选 system truth draft 列表。
type SystemTruthDraftListFilter struct {
	SourceID string
	AssetID  string
	Status   string
	Limit    int
}

// SystemTruthCompileResultListFilter narrows system truth compile result listing.
// SystemTruthCompileResultListFilter 筛选 system truth compile result 列表。
type SystemTruthCompileResultListFilter struct {
	DraftID string
	AssetID string
	Status  string
	Limit   int
}

// ValidateTaskTypeRegistration verifies one task type registration payload before persistence.
// ValidateTaskTypeRegistration 在持久化前校验 task type registration payload。
func ValidateTaskTypeRegistration(input TaskTypeRegistration) error {
	return validateTaskTypeRegistration(input)
}

func validateTaskTypeRegistration(input TaskTypeRegistration) error {
	if strings.TrimSpace(input.TypeKey) == "" {
		return invalidRuntimePersistenceInput("task type type_key is required")
	}
	status := defaultString(input.Status, TaskTypeStatusDraft)
	if !validTaskTypeStatus(status) {
		return invalidRuntimePersistenceInput("unsupported task type status %q", input.Status)
	}
	if status == TaskTypeStatusActive {
		if strings.TrimSpace(input.DefaultContractID) == "" {
			return invalidRuntimePersistenceInput("active task type default_contract_id is required")
		}
		if len(input.InputSchema) == 0 {
			return invalidRuntimePersistenceInput("active task type input_schema is required")
		}
		if !hasTaskTypeValidators(input.ValidatorRefs) {
			return invalidRuntimePersistenceInput("active task type validator_refs.validators is required")
		}
	}
	for name, value := range map[string]any{
		"input_schema":   input.InputSchema,
		"validator_refs": input.ValidatorRefs,
		"compatibility":  input.Compatibility,
		"metadata":       input.Metadata,
	} {
		if containsCredentialLikeRuntimeContractValue(value) {
			return invalidRuntimePersistenceInput("task type %s contains credential-like plaintext", name)
		}
	}
	return nil
}

func hasTaskTypeValidators(values map[string]any) bool {
	if values == nil {
		return false
	}
	raw, ok := values["validators"]
	if !ok {
		return false
	}
	switch typed := raw.(type) {
	case []any:
		for _, item := range typed {
			text, ok := item.(string)
			if ok && strings.TrimSpace(text) != "" {
				return true
			}
		}
	case []string:
		for _, item := range typed {
			if strings.TrimSpace(item) != "" {
				return true
			}
		}
	}
	return false
}

// ValidateHookBinding verifies one hook binding payload before persistence.
// ValidateHookBinding 在持久化前校验 hook binding payload。
func ValidateHookBinding(input HookBinding) error {
	return validateHookBinding(input)
}

func validateHookBinding(input HookBinding) error {
	if strings.TrimSpace(input.ContractID) == "" {
		return invalidRuntimePersistenceInput("hook binding contract_id is required")
	}
	if !validHookPoint(input.HookPoint) {
		return invalidRuntimePersistenceInput("unsupported hook point %q", input.HookPoint)
	}
	if !validHookBindingKind(input.BindingKind) {
		return invalidRuntimePersistenceInput("unsupported hook binding kind %q", input.BindingKind)
	}
	if strings.TrimSpace(input.BindingRef) == "" {
		return invalidRuntimePersistenceInput("hook binding binding_ref is required")
	}
	if !isAllowlistedHookBinding(input.BindingRef) {
		return invalidRuntimePersistenceInput("hook binding %q is not allowlisted", input.BindingRef)
	}
	if !validHookFailurePolicy(defaultString(input.FailurePolicy, HookFailurePolicyFailClosed)) {
		return invalidRuntimePersistenceInput("unsupported hook failure_policy %q", input.FailurePolicy)
	}
	for name, value := range map[string]any{"config": input.Config, "metadata": input.Metadata} {
		if containsCredentialLikeRuntimeContractValue(value) {
			return invalidRuntimePersistenceInput("hook binding %s contains credential-like plaintext", name)
		}
	}
	return nil
}

// ValidateSystemTruthSource verifies one system truth source before persistence.
// ValidateSystemTruthSource 在持久化前校验 system truth source。
func ValidateSystemTruthSource(input SystemTruthSource) error {
	return validateSystemTruthSource(input)
}

func validateSystemTruthSource(input SystemTruthSource) error {
	if strings.TrimSpace(input.AssetID) == "" {
		return invalidRuntimePersistenceInput("system truth source asset_id is required")
	}
	if strings.TrimSpace(input.SourceKind) == "" {
		return invalidRuntimePersistenceInput("system truth source source_kind is required")
	}
	if !validSystemTruthSourceStatus(defaultString(input.Status, SystemTruthSourceStatusImported)) {
		return invalidRuntimePersistenceInput("unsupported system truth source status %q", input.Status)
	}
	return validateSystemTruthSafePayload("system truth source", input.Content, input.Metadata)
}

// ValidateSystemTruthDraft verifies one editable system truth draft before persistence.
// ValidateSystemTruthDraft 在持久化前校验可编辑的 system truth draft。
func ValidateSystemTruthDraft(input SystemTruthDraft) error {
	return validateSystemTruthDraft(input)
}

func validateSystemTruthDraft(input SystemTruthDraft) error {
	if strings.TrimSpace(input.SourceID) == "" || strings.TrimSpace(input.AssetID) == "" {
		return invalidRuntimePersistenceInput("system truth draft source_id and asset_id are required")
	}
	if !validSystemTruthDraftStatus(defaultString(input.Status, SystemTruthDraftStatusDraft)) {
		return invalidRuntimePersistenceInput("unsupported system truth draft status %q", input.Status)
	}
	return validateSystemTruthSafePayload("system truth draft", input.Content, input.Metadata)
}

// ValidateSystemTruthCompileResult verifies one system truth compile result before persistence.
// ValidateSystemTruthCompileResult 在持久化前校验 system truth compile result。
func ValidateSystemTruthCompileResult(input SystemTruthCompileResult) error {
	return validateSystemTruthCompileResult(input)
}

func validateSystemTruthCompileResult(input SystemTruthCompileResult) error {
	if strings.TrimSpace(input.DraftID) == "" || strings.TrimSpace(input.AssetID) == "" {
		return invalidRuntimePersistenceInput("system truth compile draft_id and asset_id are required")
	}
	if !validSystemTruthCompileStatus(input.Status) {
		return invalidRuntimePersistenceInput("unsupported system truth compile status %q", input.Status)
	}
	for name, value := range map[string]any{"compiled_payload": input.CompiledPayload, "diagnostics": input.Diagnostics, "metadata": input.Metadata} {
		if containsCredentialLikeRuntimeContractValue(value) {
			return invalidRuntimePersistenceInput("system truth compile %s contains credential-like plaintext", name)
		}
	}
	return nil
}

// ValidateSystemTruthActiveVersion verifies one system truth active pointer change.
// ValidateSystemTruthActiveVersion 校验一次 system truth active pointer 变更。
func ValidateSystemTruthActiveVersion(input SystemTruthActiveVersion) error {
	return validateSystemTruthActiveVersion(input)
}

func validateSystemTruthActiveVersion(input SystemTruthActiveVersion) error {
	if strings.TrimSpace(input.AssetID) == "" || strings.TrimSpace(input.CompileResultID) == "" || strings.TrimSpace(input.DraftID) == "" {
		return invalidRuntimePersistenceInput("system truth active asset_id, compile_result_id, and draft_id are required")
	}
	return validateSystemTruthSafePayload("system truth active", nil, input.Metadata)
}

func validateSystemTruthSafePayload(label string, payload map[string]any, metadata map[string]any) error {
	for name, value := range map[string]any{"payload": payload, "metadata": metadata} {
		if containsCredentialLikeRuntimeContractValue(value) {
			return invalidRuntimePersistenceInput("%s %s contains credential-like plaintext", label, name)
		}
	}
	return nil
}

func validTaskTypeStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case TaskTypeStatusDraft, TaskTypeStatusActive, TaskTypeStatusDeprecated:
		return true
	default:
		return false
	}
}

func validHookPoint(value string) bool {
	switch strings.TrimSpace(value) {
	case HookPointBeforeRun, HookPointBeforeStep, HookPointAfterStep, HookPointAfterRun, HookPointOnError, HookPointBeforeProjection, HookPointAfterProjection:
		return true
	default:
		return false
	}
}

func validHookBindingKind(value string) bool {
	switch strings.TrimSpace(value) {
	case HookBindingKindEinoMiddleware, HookBindingKindEinoCallback, HookBindingKindGraphNode, HookBindingKindPolicyRef:
		return true
	default:
		return false
	}
}

func validHookFailurePolicy(value string) bool {
	switch strings.TrimSpace(value) {
	case HookFailurePolicyFailClosed, HookFailurePolicyRecordOnly:
		return true
	default:
		return false
	}
}

func validSystemTruthSourceStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case SystemTruthSourceStatusImported:
		return true
	default:
		return false
	}
}

func validSystemTruthDraftStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case SystemTruthDraftStatusDraft, SystemTruthDraftStatusCompiled, SystemTruthDraftStatusRejected:
		return true
	default:
		return false
	}
}

func validSystemTruthCompileStatus(value string) bool {
	switch strings.TrimSpace(value) {
	case SystemTruthCompileStatusSucceeded, SystemTruthCompileStatusFailed:
		return true
	default:
		return false
	}
}

func isAllowlistedHookBinding(bindingRef string) bool {
	switch strings.TrimSpace(bindingRef) {
	case "runtime_contract_guard", "runtime_trace_recorder", "system_truth_guard", "projection_boundary_guard":
		return true
	default:
		return false
	}
}
