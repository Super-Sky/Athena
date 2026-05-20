// persistence.go defines the core runtime persistence contracts for task-run records.
// persistence.go 定义 task-run 记录的核心 runtime 持久化契约。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	// TaskRunStatusCreated marks a run that has been allocated but not started.
	// TaskRunStatusCreated 表示 run 已创建但尚未开始执行。
	TaskRunStatusCreated   = "created"
	TaskRunStatusRunning   = "running"
	TaskRunStatusWaiting   = "waiting"
	TaskRunStatusResumed   = "resumed"
	TaskRunStatusCompleted = "completed"
	TaskRunStatusFailed    = "failed"
	TaskRunStatusCancelled = "cancelled"

	// TaskStepStatusPending marks a step that has been planned but not started.
	// TaskStepStatusPending 表示 step 已规划但尚未开始执行。
	TaskStepStatusPending = "pending"
	TaskStepStatusRunning = "running"
	TaskStepStatusSuccess = "success"
	TaskStepStatusFailed  = "failed"
	TaskStepStatusSkipped = "skipped"

	// LifecycleSubjectRun names lifecycle events attached to a task run.
	// LifecycleSubjectRun 表示生命周期事件主体是 task run。
	LifecycleSubjectRun  = "run"
	LifecycleSubjectStep = "step"

	// RuntimeContractStatusDraft marks a contract that can be edited but not selected by default execution.
	// RuntimeContractStatusDraft 表示 contract 仍可编辑，默认执行不会直接选择。
	RuntimeContractStatusDraft      = "draft"
	RuntimeContractStatusActive     = "active"
	RuntimeContractStatusDeprecated = "deprecated"
	RuntimeContractStatusArchived   = "archived"

	// Projection schema versions for known candidate kinds.
	// 已知候选输出类型对应的 projection schema 版本。
	ProjectionSchemaVersionMinimalOutput      = "runtime_projection.minimal_output.v1"
	ProjectionSchemaVersionPreparedExecution  = "runtime_projection.prepared_execution.v1"
	ProjectionSchemaVersionTerminalOutput     = "runtime_projection.terminal_output.v1"
	ProjectionSchemaVersionValidationMCP      = "runtime_projection.validation_mcp_result.v1"
	ProjectionSchemaVersionExternalSandboxRef = "runtime_projection.external_sandbox_ref.v1"
	ProjectionSchemaVersionAssistantMessage   = "runtime_projection.assistant_message.v1"
)

// ErrInvalidRuntimePersistenceInput marks one rejected runtime persistence write.
// ErrInvalidRuntimePersistenceInput 表示一次 runtime 持久化写入输入不合法。
var ErrInvalidRuntimePersistenceInput = errors.New("invalid runtime persistence input")

// TaskRun is the persisted core runtime aggregate root for one execution attempt.
// TaskRun 表示一次执行尝试的核心 runtime 持久化聚合根。
type TaskRun struct {
	ID               string
	TaskID           string
	TaskType         string
	TaskSubtype      string
	InputKind        string
	Scene            string
	WorkspaceID      string
	AppInstanceID    string
	Status           string
	IdempotencyScope string
	IdempotencyKey   string
	RetentionPolicy  string
	Metadata         map[string]any
	CreatedAt        time.Time
	UpdatedAt        time.Time
	StartedAt        *time.Time
	CompletedAt      *time.Time
}

// TaskStep is one persisted step under a task run.
// TaskStep 表示 task run 下的一条持久化步骤记录。
type TaskStep struct {
	ID          string
	RunID       string
	Sequence    int
	StepType    string
	Name        string
	Status      string
	Metadata    map[string]any
	CreatedAt   time.Time
	UpdatedAt   time.Time
	StartedAt   *time.Time
	CompletedAt *time.Time
}

// RuntimeTrace stores a safe runtime trace summary and redacted payload.
// RuntimeTrace 保存安全的 runtime trace 摘要和脱敏 payload。
type RuntimeTrace struct {
	ID              string
	RunID           string
	StepID          string
	TraceType       string
	Summary         string
	SafeLabels      map[string]string
	RedactedPayload map[string]any
	Metadata        map[string]any
	CreatedAt       time.Time
}

// Usage records generic resource consumption for a run or step.
// Usage 记录 run 或 step 的通用资源消耗。
type Usage struct {
	ID           string
	RunID        string
	StepID       string
	ResourceType string
	Provider     string
	ResourceName string
	Unit         string
	Amount       float64
	Cost         *float64
	Currency     string
	Metadata     map[string]any
	CreatedAt    time.Time
}

// TaskRunLifecycleEvent stores run and step state transitions.
// TaskRunLifecycleEvent 保存 run 与 step 的状态转换事件。
type TaskRunLifecycleEvent struct {
	ID          string
	RunID       string
	StepID      string
	EventType   string
	SubjectType string
	SubjectID   string
	FromStatus  string
	ToStatus    string
	Reason      string
	Metadata    map[string]any
	OccurredAt  time.Time
}

// ProjectionCandidate stores the minimal candidate-output projection for a run or step.
// ProjectionCandidate 保存 run 或 step 的最小候选输出投影。
type ProjectionCandidate struct {
	ID                    string
	RunID                 string
	StepID                string
	CandidateKind         string
	Status                string
	Summary               string
	SchemaVersion         string
	RedactedPayload       map[string]any
	SemanticPayload       map[string]any
	ArtifactRefs          map[string]any
	UIHints               map[string]any
	MaterializationTarget map[string]any
	Metadata              map[string]any
	CreatedAt             time.Time
}

// RuntimeContract is Athena's stable runtime execution contract envelope.
// RuntimeContract 是 Athena 稳定的 runtime 执行契约包络。
type RuntimeContract struct {
	ID                   string
	Name                 string
	Version              string
	Status               string
	TaskType             string
	InputSchema          map[string]any
	ExecutionProfile     map[string]any
	ExitPolicy           map[string]any
	CapabilityProfile    map[string]any
	GovernancePolicyRefs map[string]any
	HookBindings         map[string]any
	ProjectionPolicy     map[string]any
	SystemTruthRefs      map[string]any
	IdempotencyScope     string
	IdempotencyKey       string
	Metadata             map[string]any
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

// RuntimePersistenceStore is the core storage boundary for runtime task-run persistence.
// RuntimePersistenceStore 是 runtime task-run 核心持久化的存储边界。
type RuntimePersistenceStore interface {
	AutoMigrate(context.Context) error
	CreateTaskRun(context.Context, TaskRun) (TaskRun, error)
	GetTaskRun(context.Context, string) (TaskRun, bool, error)
	ListTaskRuns(context.Context, TaskRunListFilter) ([]TaskRun, error)
	CreateTaskStep(context.Context, TaskStep) (TaskStep, error)
	GetTaskStep(context.Context, string) (TaskStep, bool, error)
	ListTaskSteps(context.Context, string) ([]TaskStep, error)
	CreateLifecycleEvent(context.Context, TaskRunLifecycleEvent) (TaskRunLifecycleEvent, error)
	GetLifecycleEvent(context.Context, string) (TaskRunLifecycleEvent, bool, error)
	ListLifecycleEventsByRun(context.Context, string) ([]TaskRunLifecycleEvent, error)
	ListLifecycleEventsBySubject(context.Context, string, string) ([]TaskRunLifecycleEvent, error)
	CreateRuntimeTrace(context.Context, RuntimeTrace) (RuntimeTrace, error)
	GetRuntimeTrace(context.Context, string) (RuntimeTrace, bool, error)
	ListRuntimeTraces(context.Context, RuntimeTraceListFilter) ([]RuntimeTrace, error)
	CreateUsage(context.Context, Usage) (Usage, error)
	GetUsage(context.Context, string) (Usage, bool, error)
	ListUsage(context.Context, UsageListFilter) ([]Usage, error)
	CreateProjectionCandidate(context.Context, ProjectionCandidate) (ProjectionCandidate, error)
	GetProjectionCandidate(context.Context, string) (ProjectionCandidate, bool, error)
	ListProjectionCandidates(context.Context, ProjectionCandidateListFilter) ([]ProjectionCandidate, error)
}

// RuntimeContractStore is the storage boundary for v2 runtime contract records.
// RuntimeContractStore 是 v2 runtime contract 记录的存储边界。
type RuntimeContractStore interface {
	AutoMigrate(context.Context) error
	CreateRuntimeContract(context.Context, RuntimeContract) (RuntimeContract, error)
	PutRuntimeContract(context.Context, RuntimeContract) (RuntimeContract, error)
	GetRuntimeContract(context.Context, string) (RuntimeContract, bool, error)
	ListRuntimeContracts(context.Context, RuntimeContractListFilter) ([]RuntimeContract, error)
}

// RuntimePersistenceTransactor executes a set of persistence writes atomically.
// RuntimePersistenceTransactor 会原子执行一组持久化写入。
type RuntimePersistenceTransactor interface {
	WithTransaction(context.Context, func(RuntimePersistenceStore) error) error
}

// TaskRunListFilter narrows task-run listing without adding business semantics.
// TaskRunListFilter 用通用字段筛选 task-run 列表，不引入业务语义。
type TaskRunListFilter struct {
	WorkspaceID string
	Status      string
	Limit       int
}

// RuntimeTraceListFilter narrows trace listing by run and optional step.
// RuntimeTraceListFilter 按 run 和可选 step 筛选 trace 列表。
type RuntimeTraceListFilter struct {
	RunID  string
	StepID string
	Limit  int
}

// UsageListFilter narrows generic usage listing by run and optional step.
// UsageListFilter 按 run 和可选 step 筛选通用 usage 列表。
type UsageListFilter struct {
	RunID  string
	StepID string
	Limit  int
}

// ProjectionCandidateListFilter narrows candidate-output projection listing by run and optional step.
// ProjectionCandidateListFilter 按 run 和可选 step 筛选候选输出投影列表。
type ProjectionCandidateListFilter struct {
	RunID  string
	StepID string
	Limit  int
}

// RuntimeContractListFilter narrows contract listings by stable queryable fields.
// RuntimeContractListFilter 按稳定可查询字段筛选 contract 列表。
type RuntimeContractListFilter struct {
	TaskType string
	Status   string
	Limit    int
}

// ValidateRuntimeContract verifies one runtime contract payload before persistence.
// ValidateRuntimeContract 在持久化前校验 runtime contract payload。
func ValidateRuntimeContract(input RuntimeContract) error {
	return validateRuntimeContract(input)
}

func validateRuntimeContract(input RuntimeContract) error {
	if strings.TrimSpace(input.Name) == "" {
		return invalidRuntimePersistenceInput("runtime contract name is required")
	}
	if strings.TrimSpace(input.Version) == "" {
		return invalidRuntimePersistenceInput("runtime contract version is required")
	}
	if strings.TrimSpace(input.TaskType) == "" {
		return invalidRuntimePersistenceInput("runtime contract task_type is required")
	}
	if !validRuntimeContractStatus(defaultString(input.Status, RuntimeContractStatusDraft)) {
		return invalidRuntimePersistenceInput("unsupported runtime contract status %q", input.Status)
	}
	for name, value := range map[string]any{
		"input_schema":           input.InputSchema,
		"execution_profile":      input.ExecutionProfile,
		"exit_policy":            input.ExitPolicy,
		"capability_profile":     input.CapabilityProfile,
		"governance_policy_refs": input.GovernancePolicyRefs,
		"hook_bindings":          input.HookBindings,
		"projection_policy":      input.ProjectionPolicy,
		"system_truth_refs":      input.SystemTruthRefs,
		"metadata":               input.Metadata,
	} {
		if containsCredentialLikeRuntimeContractValue(value) {
			return invalidRuntimePersistenceInput("runtime contract %s contains credential-like plaintext", name)
		}
	}
	return nil
}

func validateTaskRun(input TaskRun) error {
	if strings.TrimSpace(input.TaskID) == "" {
		return invalidRuntimePersistenceInput("task run task_id is required")
	}
	if !validTaskRunStatus(defaultString(input.Status, TaskRunStatusCreated)) {
		return invalidRuntimePersistenceInput("unsupported task run status %q", input.Status)
	}
	return nil
}

func validateTaskStep(input TaskStep) error {
	if strings.TrimSpace(input.RunID) == "" {
		return invalidRuntimePersistenceInput("task step run_id is required")
	}
	if input.Sequence <= 0 {
		return invalidRuntimePersistenceInput("task step sequence must be positive")
	}
	if !validTaskStepStatus(defaultString(input.Status, TaskStepStatusPending)) {
		return invalidRuntimePersistenceInput("unsupported task step status %q", input.Status)
	}
	return nil
}

func validateLifecycleEvent(input TaskRunLifecycleEvent) error {
	if strings.TrimSpace(input.RunID) == "" {
		return invalidRuntimePersistenceInput("lifecycle event run_id is required")
	}
	if strings.TrimSpace(input.EventType) == "" || strings.TrimSpace(input.SubjectType) == "" || strings.TrimSpace(input.SubjectID) == "" {
		return invalidRuntimePersistenceInput("lifecycle event event_type, subject_type, and subject_id are required")
	}
	switch strings.TrimSpace(input.SubjectType) {
	case LifecycleSubjectRun:
		if !validTaskRunStatus(input.ToStatus) {
			return invalidRuntimePersistenceInput("unsupported lifecycle run to_status %q", input.ToStatus)
		}
	case LifecycleSubjectStep:
		if strings.TrimSpace(input.StepID) == "" {
			return invalidRuntimePersistenceInput("step lifecycle event step_id is required")
		}
		if !validTaskStepStatus(input.ToStatus) {
			return invalidRuntimePersistenceInput("unsupported lifecycle step to_status %q", input.ToStatus)
		}
	default:
		return invalidRuntimePersistenceInput("unsupported lifecycle subject_type %q", input.SubjectType)
	}
	return nil
}

func validateRuntimeTrace(input RuntimeTrace) error {
	if strings.TrimSpace(input.RunID) == "" {
		return invalidRuntimePersistenceInput("runtime trace run_id is required")
	}
	if strings.TrimSpace(input.Summary) == "" {
		return invalidRuntimePersistenceInput("runtime trace summary is required")
	}
	return nil
}

func validateUsage(input Usage) error {
	if strings.TrimSpace(input.RunID) == "" {
		return invalidRuntimePersistenceInput("usage run_id is required")
	}
	if strings.TrimSpace(input.ResourceType) == "" || strings.TrimSpace(input.Unit) == "" {
		return invalidRuntimePersistenceInput("usage resource_type and unit are required")
	}
	if input.Amount < 0 {
		return invalidRuntimePersistenceInput("usage amount must not be negative")
	}
	return nil
}

func validateProjectionCandidate(input ProjectionCandidate) error {
	if strings.TrimSpace(input.RunID) == "" {
		return invalidRuntimePersistenceInput("projection candidate run_id is required")
	}
	if strings.TrimSpace(input.CandidateKind) == "" {
		return invalidRuntimePersistenceInput("projection candidate candidate_kind is required")
	}
	for name, value := range map[string]any{
		"redacted_payload":       input.RedactedPayload,
		"semantic_payload":       input.SemanticPayload,
		"artifact_refs":          input.ArtifactRefs,
		"ui_hints":               input.UIHints,
		"materialization_target": input.MaterializationTarget,
		"metadata":               input.Metadata,
	} {
		if containsCredentialLikeRuntimeContractValue(value) {
			return invalidRuntimePersistenceInput("projection candidate %s contains credential-like plaintext", name)
		}
	}
	return nil
}

func normalizeProjectionCandidate(input ProjectionCandidate) ProjectionCandidate {
	normalized := input
	normalized.CandidateKind = strings.TrimSpace(normalized.CandidateKind)
	normalized.SchemaVersion = strings.TrimSpace(normalized.SchemaVersion)
	if normalized.SchemaVersion == "" {
		normalized.SchemaVersion = defaultProjectionSchemaVersion(normalized.CandidateKind)
	}
	return normalized
}

func defaultProjectionSchemaVersion(candidateKind string) string {
	switch strings.TrimSpace(candidateKind) {
	case "minimal_output":
		return ProjectionSchemaVersionMinimalOutput
	case "prepared_execution":
		return ProjectionSchemaVersionPreparedExecution
	case "terminal_output":
		return ProjectionSchemaVersionTerminalOutput
	case "validation_mcp_result":
		return ProjectionSchemaVersionValidationMCP
	case "external_sandbox_ref":
		return ProjectionSchemaVersionExternalSandboxRef
	case "assistant_message":
		return ProjectionSchemaVersionAssistantMessage
	default:
		return ""
	}
}

func validTaskRunStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case TaskRunStatusCreated, TaskRunStatusRunning, TaskRunStatusWaiting, TaskRunStatusResumed, TaskRunStatusCompleted, TaskRunStatusFailed, TaskRunStatusCancelled:
		return true
	default:
		return false
	}
}

func validTaskStepStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case TaskStepStatusPending, TaskStepStatusRunning, TaskStepStatusSuccess, TaskStepStatusFailed, TaskStepStatusSkipped:
		return true
	default:
		return false
	}
}

func validRuntimeContractStatus(status string) bool {
	switch strings.TrimSpace(status) {
	case RuntimeContractStatusDraft, RuntimeContractStatusActive, RuntimeContractStatusDeprecated, RuntimeContractStatusArchived:
		return true
	default:
		return false
	}
}

func invalidRuntimePersistenceInput(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidRuntimePersistenceInput, fmt.Sprintf(format, args...))
}

func containsCredentialLikeRuntimeContractValue(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return looksCredentialLikeRuntimeContractString(typed)
	case map[string]any:
		for key, child := range typed {
			if looksCredentialLikeRuntimeContractString(key) || containsCredentialLikeRuntimeContractValue(child) {
				return true
			}
		}
	case map[string]string:
		for key, child := range typed {
			if looksCredentialLikeRuntimeContractString(key) || looksCredentialLikeRuntimeContractString(child) {
				return true
			}
		}
	case []any:
		for _, child := range typed {
			if containsCredentialLikeRuntimeContractValue(child) {
				return true
			}
		}
	case []string:
		for _, child := range typed {
			if looksCredentialLikeRuntimeContractString(child) {
				return true
			}
		}
	}
	return false
}

func looksCredentialLikeRuntimeContractString(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	if lower == "" {
		return false
	}
	markers := []string{
		"authorization",
		"bearer ",
		"api_key",
		"api-key",
		"x-api-key",
		"access_token",
		"refresh_token",
		"password",
		"secret",
		"credential",
		"sk-",
		"akia",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
