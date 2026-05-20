// types.go defines the core runtime types such as execution spec, actions, waiting state, and model contracts.
// types.go 定义执行规格、动作、等待态和模型契约等核心 runtime 类型。
package runtime

import (
	"time"

	"github.com/cloudwego/eino/adk"
	"moss/internal/customization"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	runtimetask "moss/internal/runtime/task"
	"moss/internal/session"
)

// SupplementTarget identifies who should receive the follow-up action.
// SupplementTarget 表示后续动作主要发给谁。
type SupplementTarget string

const (
	SupplementTargetUser   SupplementTarget = "user"
	SupplementTargetClient SupplementTarget = "client"
	SupplementTargetAuto   SupplementTarget = "auto"
)

// ActionType names the control actions that can interrupt or redirect the normal answer flow.
// ActionType 描述会打断或重定向正常回答链路的控制动作。
type ActionType string

const (
	ActionTypeInformationRequest ActionType = "information_request"
	ActionTypePendingHuman       ActionType = "pending_human"
)

// TurnResultKind describes what a single runtime turn produced.
// TurnResultKind 描述单轮 runtime 执行的结果类别。
type TurnResultKind string

const (
	TurnResultFinal   TurnResultKind = "final"
	TurnResultAction  TurnResultKind = "action"
	TurnResultPartial TurnResultKind = "partial"
	TurnResultError   TurnResultKind = "error"
)

// RuntimeStage names the replaceable steps inside the runtime skeleton.
// RuntimeStage 表示 runtime 主骨架中的可替换阶段。
type RuntimeStage string

const (
	StageContextAssembly      RuntimeStage = "context_assembly"
	StageCapabilityResolution RuntimeStage = "capability_resolution"
	StageTurnExecution        RuntimeStage = "turn_execution"
	StageTurnProcessing       RuntimeStage = "turn_processing"
)

// SupplementOutcome defines whether a gap is resumed, closed, or handed off.
// SupplementOutcome 定义等待缺口是被恢复、关闭还是转交外部处理。
type SupplementOutcome string

const (
	SupplementOutcomeProvided           SupplementOutcome = "provided"
	SupplementOutcomeUnableToProvide    SupplementOutcome = "unable_to_provide"
	SupplementOutcomeTimeoutExpired     SupplementOutcome = "timeout_expired"
	SupplementOutcomeAbandonAndContinue SupplementOutcome = "abandon_and_continue"
	SupplementOutcomePendingHuman       SupplementOutcome = "pending_human"
)

// RequestStatus is the client-visible terminal or transitional status for one request.
// RequestStatus 表示单次请求对客户端可见的阶段或结束状态。
type RequestStatus string

const (
	RequestStatusCompleted                    RequestStatus = "completed"
	RequestStatusWaitingForInformation        RequestStatus = "waiting_for_information"
	RequestStatusTimedOut                     RequestStatus = "timed_out"
	RequestStatusMissingInformationUnresolved RequestStatus = "missing_information_unresolved"
	RequestStatusPendingHuman                 RequestStatus = "pending_human"
	RequestStatusPolicyRejected               RequestStatus = "policy_rejected"
	RequestStatusInvalidResumeToken           RequestStatus = "invalid_resume_token"
	RequestStatusInvalidModel                 RequestStatus = "invalid_model"
)

// OrchestrationState names the first-phase orchestration semantics visible to app/runtime.
// OrchestrationState 描述第一阶段对 app/runtime 可见的编排状态语义。
type OrchestrationState string

const (
	OrchestrationStateNormalized OrchestrationState = "normalized"
	OrchestrationStateGoverned   OrchestrationState = "governed"
	OrchestrationStateExecuting  OrchestrationState = "executing"
	OrchestrationStateWaiting    OrchestrationState = "waiting"
	OrchestrationStateResumed    OrchestrationState = "resumed"
	OrchestrationStateDegraded   OrchestrationState = "degraded"
	OrchestrationStateCompleted  OrchestrationState = "completed"
	OrchestrationStateAborted    OrchestrationState = "aborted"
)

// OrchestrationStatus captures the minimal current orchestration state for one request.
// OrchestrationStatus 描述单次请求当前最小编排状态。
type OrchestrationStatus struct {
	EntryState   OrchestrationState `json:"entry_state,omitempty"`
	CurrentState OrchestrationState `json:"current_state,omitempty"`
	Reason       string             `json:"reason,omitempty"`
}

// Input is the normalized runtime input after app-level orchestration.
// Input 是经过 app 层编排后的标准 runtime 输入。
type Input struct {
	RequestID       string
	SessionID       string
	Query           string
	ModelSelection  *model.Selection
	Task            *runtimetask.RuntimeTask
	Orchestration   OrchestrationState
	Customization   customization.UserCustomization
	Supplement      *SupplementPayload
	TimeoutOverride time.Duration
	Pending         *session.PendingState
}

// SupplementPayload carries either actual supplemental data or an explicit outcome about the waiting gap.
// SupplementPayload 携带补充数据本身，或对等待缺口的明确处理结果。
type SupplementPayload struct {
	Data    map[string]string `json:"data,omitempty"`
	Outcome SupplementOutcome `json:"outcome,omitempty"`
	Resume  *ResumeContext    `json:"resume,omitempty"`
}

// ResumeContext binds a supplemental request to the exact gap that may be resumed.
// ResumeContext 用于把补数请求绑定到要恢复的具体等待缺口。
type ResumeContext struct {
	Stage       RuntimeStage `json:"stage,omitempty"`
	ResumeToken string       `json:"resume_token,omitempty"`
}

// RuntimeState holds per-request execution state shared across runtime steps.
// RuntimeState 保存单次请求在 runtime 各阶段共享的执行状态。
type RuntimeState struct {
	RequestID string
	SessionID string
	Turn      int
}

// PreservedContext is the runtime-visible alias for session continuity state.
// PreservedContext 是 runtime 可见的 continuity state 别名。
type PreservedContext = session.PreservedContext

// SkillSpec captures the resolved high-level capability guidance for the current turn.
// SkillSpec 保存当前轮解析后的高层能力语义与 guidance。
type SkillSpec struct {
	PrimarySkill    string   `json:"primary_skill,omitempty"`
	AuxiliarySkills []string `json:"auxiliary_skills,omitempty"`
	Guidance        string   `json:"guidance,omitempty"`
}

// ToolSpec captures the resolved atomic tools allowed in the current turn.
// ToolSpec 保存当前轮允许使用的原子 tools 及其来源关系。
type ToolSpec struct {
	AllowedTools []string            `json:"allowed_tools,omitempty"`
	Constraints  map[string][]string `json:"constraints,omitempty"`
	Sources      map[string]string   `json:"sources,omitempty"`
}

// CapabilityContract captures the first-phase declaration/governed/runtime-consumption relationship.
// CapabilityContract 表示第一阶段 declaration/governed/runtime-consumption 三层能力关系。
type CapabilityContract struct {
	Declarations       CapabilityDeclarations `json:"declarations,omitempty"`
	GovernedState      GovernedCapabilities   `json:"governed_state,omitempty"`
	RuntimeConsumption RuntimeConsumption     `json:"runtime_consumption,omitempty"`
}

// CapabilityDeclarations captures the declaration-side capability inputs before governance filtering.
// CapabilityDeclarations 描述治理过滤前的声明态能力输入。
type CapabilityDeclarations struct {
	RequestedSkills []string `json:"requested_skills,omitempty"`
	RequestedTools  []string `json:"requested_tools,omitempty"`
}

// GovernedCapabilities captures the capabilities that remain visible after policy/governance filtering.
// GovernedCapabilities 描述经过 policy/governance 过滤后仍可见的能力集合。
type GovernedCapabilities struct {
	Skills []string `json:"skills,omitempty"`
	Tools  []string `json:"tools,omitempty"`
}

// RuntimeConsumption captures the canonical runtime-consumption contract boundary.
// RuntimeConsumption 描述 canonical runtime-consumption contract 边界。
type RuntimeConsumption struct {
	ContractName string `json:"contract_name,omitempty"`
}

// InferenceSpec captures how the executor should shape a single reasoning turn.
// InferenceSpec 描述单轮推理的目标、输出模式与 schema 约束。
type InferenceSpec struct {
	Goal             string                    `json:"goal,omitempty"`
	OutputSchema     string                    `json:"output_schema,omitempty"`
	OutputMode       string                    `json:"output_mode,omitempty"`
	StructuredOutput *StructuredOutputContract `json:"structured_output,omitempty"`
}

// StructuredDecision is the first-phase minimal machine-readable decision container.
// StructuredDecision 表示第一阶段最小机器可解析 decision 容器。
type StructuredDecision struct {
	Verdict string         `json:"verdict,omitempty"`
	Reason  string         `json:"reason,omitempty"`
	Detail  map[string]any `json:"detail,omitempty"`
}

// StructuredOutputContract captures the requested and emitted structured-output contract for one turn.
// StructuredOutputContract 描述单轮请求和实际产出的结构化输出契约。
type StructuredOutputContract struct {
	ContractID     string              `json:"contract_id,omitempty"`
	Mode           string              `json:"mode,omitempty"`
	SchemaName     string              `json:"schema_name,omitempty"`
	SchemaVersion  string              `json:"schema_version,omitempty"`
	Requested      bool                `json:"requested,omitempty"`
	Emitted        bool                `json:"emitted,omitempty"`
	FallbackReason string              `json:"fallback_reason,omitempty"`
	Decision       *StructuredDecision `json:"decision,omitempty"`
	Detail         map[string]any      `json:"detail,omitempty"`
}

// GovernanceDecisionKind names the minimal execution-governance decision for one request.
// GovernanceDecisionKind 表示单次请求的最小执行治理决策。
type GovernanceDecisionKind string

const (
	GovernanceDecisionAllow        GovernanceDecisionKind = "allow"
	GovernanceDecisionAsk          GovernanceDecisionKind = "ask"
	GovernanceDecisionDegrade      GovernanceDecisionKind = "degrade"
	GovernanceDecisionDeny         GovernanceDecisionKind = "deny"
	GovernanceDecisionPendingHuman GovernanceDecisionKind = "pending_human"
)

// GovernanceDecision captures the first-phase governance outcome in a transport-safe form.
// GovernanceDecision 用于以 transport-safe 方式表达第一阶段治理结果。
type GovernanceDecision struct {
	Decision GovernanceDecisionKind `json:"decision,omitempty"`
	Reason   string                 `json:"reason,omitempty"`
	Detail   map[string]any         `json:"detail,omitempty"`
}

// ModelEndpoint captures one provider/model pair without exposing sensitive secrets.
// ModelEndpoint 描述一个具体 provider/model 端点，但不会暴露敏感密钥。
type ModelEndpoint struct {
	ProviderID       string            `json:"provider_id,omitempty"`
	ProviderName     string            `json:"provider_name,omitempty"`
	ProviderProtocol string            `json:"provider_protocol,omitempty"`
	ModelRecordID    string            `json:"model_record_id,omitempty"`
	ProviderModelID  string            `json:"provider_model_id,omitempty"`
	ModelDisplayName string            `json:"model_display_name,omitempty"`
	Headers          map[string]string `json:"headers,omitempty"`
}

// ModelExecution describes both the requested model selection and the model actually executed.
// ModelExecution 同时描述请求选中的模型与最终实际执行的模型。
type ModelExecution struct {
	Requested          ModelEndpoint                        `json:"requested"`
	Executed           ModelEndpoint                        `json:"executed"`
	ExplicitSelection  bool                                 `json:"explicit_selection,omitempty"`
	FallbackAvailable  bool                                 `json:"fallback_available,omitempty"`
	FallbackUsed       bool                                 `json:"fallback_used,omitempty"`
	FallbackReason     string                               `json:"fallback_reason,omitempty"`
	PrimaryConfig      *model.ChatConfig                    `json:"-"`
	FallbackConfig     *model.ChatConfig                    `json:"-"`
	ExecutedConfig     *model.ChatConfig                    `json:"-"`
	ResolvedParameters *modelparams.ResolvedModelParameters `json:"-"`
}

// ModelSpec captures the selected provider/model pair without exposing sensitive secrets.
// ModelSpec 描述本次选中的供应商与模型组合，但不会暴露敏感密钥。
type ModelSpec = ModelExecution

// WaitTimeoutPolicy carries the effective waiting timeout for a gap.
// WaitTimeoutPolicy 表示某个等待缺口实际采用的超时策略。
type WaitTimeoutPolicy struct {
	TimeoutAfter time.Duration `json:"timeout_after,omitempty"`
}

// ProcessingSpec captures downstream handling preferences after one turn is resolved.
// ProcessingSpec 描述单轮结果在后续处理阶段的偏好与等待策略。
type ProcessingSpec struct {
	PreferDirectAnswer bool              `json:"prefer_direct_answer,omitempty"`
	PreferActionFirst  bool              `json:"prefer_action_first,omitempty"`
	GatherInfo         bool              `json:"gather_info,omitempty"`
	WaitPolicy         WaitTimeoutPolicy `json:"wait_policy,omitempty"`
}

// ExecutionMetadata stores resolver reasons and machine-readable constraints for observability.
// ExecutionMetadata 保存 resolver 理由和可观测约束信息。
type ExecutionMetadata struct {
	ResolverReason   string               `json:"resolver_reason,omitempty"`
	Governance       *GovernanceDecision  `json:"governance,omitempty"`
	Capability       *CapabilityContract  `json:"capability,omitempty"`
	Orchestration    *OrchestrationStatus `json:"orchestration,omitempty"`
	PreservedContext *PreservedContext    `json:"preserved_context,omitempty"`
	Constraints      map[string]any       `json:"constraints,omitempty"`
}

// ExecutionSpec is the full runtime contract for one turn.
// ExecutionSpec 是当前一轮执行的完整 runtime 规格。
type ExecutionSpec struct {
	Skill      SkillSpec         `json:"skill"`
	Tools      ToolSpec          `json:"tools"`
	Model      ModelSpec         `json:"model"`
	Inference  InferenceSpec     `json:"inference"`
	Processing ProcessingSpec    `json:"processing"`
	Metadata   ExecutionMetadata `json:"metadata"`
}

// MissingInformationItem explains one missing field that blocks the current gap.
// MissingInformationItem 描述当前缺口中某个阻塞字段的缺失原因。
type MissingInformationItem struct {
	Field    string `json:"field"`
	Reason   string `json:"reason"`
	Impact   string `json:"impact"`
	Required bool   `json:"required"`
}

// InformationRequestAction asks the caller to provide the missing data required by the current gap.
// InformationRequestAction 要求调用方补充关闭当前缺口所需的数据。
type InformationRequestAction struct {
	Missing         []MissingInformationItem `json:"missing,omitempty"`
	AllowDegrade    bool                     `json:"allow_degrade"`
	SuggestedAction string                   `json:"suggested_action,omitempty"`
	Target          SupplementTarget         `json:"target,omitempty"`
	WaitPolicy      WaitTimeoutPolicy        `json:"wait_policy,omitempty"`
}

// ActionSchemaField describes one machine-readable input field required by a client action.
// ActionSchemaField 描述客户端动作里一个可机器解析的输入字段约束。
type ActionSchemaField struct {
	Type     string   `json:"type,omitempty"`
	Required bool     `json:"required,omitempty"`
	Enum     []string `json:"enum,omitempty"`
}

// ActionSchema describes the structured input contract the client should follow.
// ActionSchema 描述客户端应遵守的结构化输入契约。
type ActionSchema struct {
	Input map[string]ActionSchemaField `json:"input,omitempty"`
}

// ActionExpectedResult describes how the caller should respond after handling one action.
// ActionExpectedResult 描述调用方在处理一个动作后应该怎样回传结果。
type ActionExpectedResult struct {
	ResumeTokenRequired bool                `json:"resume_token_required,omitempty"`
	AllowedOutcomes     []SupplementOutcome `json:"allowed_outcomes,omitempty"`
}

// PendingHumanAction marks that the automatic flow stops here and expects external follow-up.
// PendingHumanAction 表示自动链路在此停止，后续需由客户端或外部系统接管。
type PendingHumanAction struct {
	Reason          string            `json:"reason,omitempty"`
	SuggestedAction string            `json:"suggested_action,omitempty"`
	Target          SupplementTarget  `json:"target,omitempty"`
	Context         map[string]string `json:"context,omitempty"`
}

// GapClosedAction reports that the active waiting gap has been closed and its token is no longer valid.
// GapClosedAction 表示当前等待缺口已关闭，对应 token 已失效。
type GapClosedAction struct {
	ResumeToken  string `json:"resume_token,omitempty"`
	CloseReason  string `json:"close_reason,omitempty"`
	NextStep     string `json:"next_step,omitempty"`
	TokenInvalid bool   `json:"token_invalid,omitempty"`
}

// Action is the transport-safe envelope for runtime control actions.
// Action 是 runtime 控制动作对外传输的统一封装。
type Action struct {
	Type               ActionType                `json:"type"`
	Code               string                    `json:"code,omitempty"`
	Message            string                    `json:"message,omitempty"`
	Target             SupplementTarget          `json:"target,omitempty"`
	Schema             *ActionSchema             `json:"schema,omitempty"`
	Payload            map[string]any            `json:"payload,omitempty"`
	TimeoutPolicy      *WaitTimeoutPolicy        `json:"timeout_policy,omitempty"`
	ExpectedResult     *ActionExpectedResult     `json:"expected_result,omitempty"`
	InformationRequest *InformationRequestAction `json:"information_request,omitempty"`
	PendingHuman       *PendingHumanAction       `json:"pending_human,omitempty"`
}

// Notification describes one state change that the client should observe but not execute.
// Notification 描述一个客户端应感知但无需执行的状态变化。
type Notification struct {
	Code        string         `json:"code,omitempty"`
	Message     string         `json:"message,omitempty"`
	SessionID   string         `json:"session_id,omitempty"`
	ResumeToken string         `json:"resume_token,omitempty"`
	Detail      map[string]any `json:"detail,omitempty"`
	NextStep    string         `json:"next_step,omitempty"`
	ClientHint  string         `json:"client_hint,omitempty"`
}

// ProtocolError describes one machine-readable protocol or state error returned to the client.
// ProtocolError 描述一个返回给客户端的可机器解析协议或状态错误。
type ProtocolError struct {
	Code         string         `json:"code,omitempty"`
	Reason       string         `json:"reason,omitempty"`
	Retryable    bool           `json:"retryable,omitempty"`
	ClientAction string         `json:"client_action,omitempty"`
	Detail       map[string]any `json:"detail,omitempty"`
}

// WaitState is the transport-facing snapshot of the active waiting gap.
// WaitState 是当前活跃等待缺口对外暴露的状态快照。
type WaitState struct {
	Stage        RuntimeStage  `json:"stage"`
	StartedAt    time.Time     `json:"started_at"`
	TimeoutAt    time.Time     `json:"timeout_at"`
	TimeoutAfter time.Duration `json:"timeout_after"`
	ResumeToken  string        `json:"resume_token,omitempty"`
}

// TimeoutOutcome describes what the loop controller chooses after a wait policy is evaluated.
// TimeoutOutcome 描述 loop controller 在评估超时后选择的后续动作。
type TimeoutOutcome string

const (
	TimeoutOutcomeFinish  TimeoutOutcome = "finish"
	TimeoutOutcomeDegrade TimeoutOutcome = "degrade"
	TimeoutOutcomePending TimeoutOutcome = "pending"
)

// TurnResult is the normalized result exchanged between runtime steps.
// TurnResult 是 runtime 各阶段之间传递的标准化结果。
type TurnResult struct {
	Kind          TurnResultKind     `json:"kind"`
	Action        *Action            `json:"action,omitempty"`
	Content       string             `json:"content,omitempty"`
	Error         string             `json:"error,omitempty"`
	WaitState     *WaitState         `json:"wait_state,omitempty"`
	Orchestration OrchestrationState `json:"orchestration,omitempty"`
}

// PreparedExecution holds everything the transport layer needs before the runner starts.
// PreparedExecution 保存 transport 层在启动 runner 前需要的全部结果。
type PreparedExecution struct {
	Spec              *ExecutionSpec
	Runner            *adk.Runner
	Messages          []adk.Message
	Initial           *TurnResult
	InitialError      *ProtocolError
	TimeoutWait       *WaitState
	InitialStatus     RequestStatus
	Orchestration     OrchestrationState
	StructuredOutput  *StructuredOutputContract
	Governance        *GovernanceDecision
	RuntimeRecords    *MinimalPersistenceRecordSet
	TerminalProjector *RuntimeTerminalProjector
	CallbackRecorder  *RuntimeCallbackRecorder
	CheckpointRef     *RuntimeGraphCheckpointRef
}
