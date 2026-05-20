// types.go defines the canonical model-parameter policy input and resolved-output contracts.
// types.go 定义标准模型参数策略输入以及最终解析结果契约。
package parameters

// LoopStage captures the runtime loop stage used for model-parameter resolution.
// LoopStage 描述模型参数解析时使用的 runtime loop 阶段。
type LoopStage string

const (
	// LoopStageInitialAnswer marks the first direct-answer stage.
	// LoopStageInitialAnswer 表示首轮直接回答阶段。
	LoopStageInitialAnswer LoopStage = "initial_answer"

	// LoopStageSceneMatch marks the scene-matching stage.
	// LoopStageSceneMatch 表示场景判定阶段。
	LoopStageSceneMatch LoopStage = "scene_match"

	// LoopStageStructuredResult marks the structured-result stage.
	// LoopStageStructuredResult 表示结构化结果阶段。
	LoopStageStructuredResult LoopStage = "structured_result"

	// LoopStageWorkflowPlan marks the workflow planning stage.
	// LoopStageWorkflowPlan 表示工作流规划阶段。
	LoopStageWorkflowPlan LoopStage = "workflow_plan"

	// LoopStageWorkflowStep marks the workflow step callback stage.
	// LoopStageWorkflowStep 表示工作流步骤回调阶段。
	LoopStageWorkflowStep LoopStage = "workflow_step"

	// LoopStageExecutionGovernance marks the execution-governance stage.
	// LoopStageExecutionGovernance 表示执行治理阶段。
	LoopStageExecutionGovernance LoopStage = "execution_governance"

	// LoopStageArtifactWrite marks the delivery-writing stage.
	// LoopStageArtifactWrite 表示交付物写入阶段。
	LoopStageArtifactWrite LoopStage = "artifact_write"

	// LoopStageNextQuestions marks the next-question generation stage.
	// LoopStageNextQuestions 表示追问生成阶段。
	LoopStageNextQuestions LoopStage = "next_questions"

	// LoopStageRetryFormatting marks the retry-formatting stage.
	// LoopStageRetryFormatting 表示重试格式化阶段。
	LoopStageRetryFormatting LoopStage = "retry_formatting"
)

// StepRiskLevel captures the bounded risk level used for step-level parameter overrides.
// StepRiskLevel 描述步骤级参数覆盖使用的受限风险等级。
type StepRiskLevel string

const (
	// StepRiskLevelLow marks one low-risk step.
	// StepRiskLevelLow 表示低风险步骤。
	StepRiskLevelLow StepRiskLevel = "low"

	// StepRiskLevelMedium marks one medium-risk step.
	// StepRiskLevelMedium 表示中风险步骤。
	StepRiskLevelMedium StepRiskLevel = "medium"

	// StepRiskLevelHigh marks one high-risk step.
	// StepRiskLevelHigh 表示高风险步骤。
	StepRiskLevelHigh StepRiskLevel = "high"
)

// OutputModeIntent captures the controlled output-mode override intent accepted from callers.
// OutputModeIntent 描述调用方可传入的受控输出模式意图。
type OutputModeIntent string

const (
	// OutputModeIntentText requests plain-text output behavior.
	// OutputModeIntentText 表示文本输出意图。
	OutputModeIntentText OutputModeIntent = "text"

	// OutputModeIntentStructured requests structured output behavior.
	// OutputModeIntentStructured 表示结构化输出意图。
	OutputModeIntentStructured OutputModeIntent = "structured"

	// OutputModeIntentStrictJSON requests strict JSON output behavior.
	// OutputModeIntentStrictJSON 表示严格 JSON 输出意图。
	OutputModeIntentStrictJSON OutputModeIntent = "strict_json"
)

// ReasoningModeIntent captures the controlled reasoning-depth override intent.
// ReasoningModeIntent 描述受控推理深度意图。
type ReasoningModeIntent string

const (
	// ReasoningModeIntentLow requests low reasoning depth.
	// ReasoningModeIntentLow 表示低推理深度。
	ReasoningModeIntentLow ReasoningModeIntent = "low"

	// ReasoningModeIntentMedium requests medium reasoning depth.
	// ReasoningModeIntentMedium 表示中等推理深度。
	ReasoningModeIntentMedium ReasoningModeIntent = "medium"

	// ReasoningModeIntentHigh requests high reasoning depth.
	// ReasoningModeIntentHigh 表示高推理深度。
	ReasoningModeIntentHigh ReasoningModeIntent = "high"
)

// ToolPolicyIntent captures the controlled tool-usage intent accepted from callers.
// ToolPolicyIntent 描述调用方可传入的受控工具使用意图。
type ToolPolicyIntent string

const (
	// ToolPolicyIntentNone disables tool calls.
	// ToolPolicyIntentNone 表示禁用工具调用。
	ToolPolicyIntentNone ToolPolicyIntent = "none"

	// ToolPolicyIntentAuto lets the model decide whether to use tools.
	// ToolPolicyIntentAuto 表示由模型自行决定是否使用工具。
	ToolPolicyIntentAuto ToolPolicyIntent = "auto"

	// ToolPolicyIntentRequired forces the model to use tools.
	// ToolPolicyIntentRequired 表示强制模型使用工具。
	ToolPolicyIntentRequired ToolPolicyIntent = "required"

	// ToolPolicyIntentSpecificTool forces one specific tool call.
	// ToolPolicyIntentSpecificTool 表示强制调用一个指定工具。
	ToolPolicyIntentSpecificTool ToolPolicyIntent = "specific_tool"
)

// ResponseFormat captures the canonical response-format value consumed by Athena runtime.
// ResponseFormat 描述 Athena runtime 消费的标准响应格式值。
type ResponseFormat string

const (
	// ResponseFormatText keeps free-form text as the target output format.
	// ResponseFormatText 表示以自由文本作为目标输出格式。
	ResponseFormatText ResponseFormat = "text"

	// ResponseFormatJSONObject keeps JSON object output as the target output format.
	// ResponseFormatJSONObject 表示以 JSON object 作为目标输出格式。
	ResponseFormatJSONObject ResponseFormat = "json_object"

	// ResponseFormatJSONSchema keeps JSON schema-constrained output as the target output format.
	// ResponseFormatJSONSchema 表示以 JSON schema 约束输出作为目标格式。
	ResponseFormatJSONSchema ResponseFormat = "json_schema"
)

// ToolChoiceKind captures the canonical tool-choice value consumed by Athena runtime.
// ToolChoiceKind 描述 Athena runtime 消费的标准工具选择值。
type ToolChoiceKind string

const (
	// ToolChoiceNone disables tool calls.
	// ToolChoiceNone 表示禁用工具调用。
	ToolChoiceNone ToolChoiceKind = "none"

	// ToolChoiceAuto lets the model decide whether to call a tool.
	// ToolChoiceAuto 表示由模型自行决定是否调用工具。
	ToolChoiceAuto ToolChoiceKind = "auto"

	// ToolChoiceRequired requires at least one tool call.
	// ToolChoiceRequired 表示至少要求一次工具调用。
	ToolChoiceRequired ToolChoiceKind = "required"

	// ToolChoiceSpecificTool requires one specific tool call.
	// ToolChoiceSpecificTool 表示要求调用一个指定工具。
	ToolChoiceSpecificTool ToolChoiceKind = "specific_tool"
)

// ReasoningEffort captures the canonical reasoning-effort value consumed by Athena runtime.
// ReasoningEffort 描述 Athena runtime 消费的标准推理力度值。
type ReasoningEffort string

const (
	// ReasoningEffortLow keeps the reasoning budget low.
	// ReasoningEffortLow 表示低推理预算。
	ReasoningEffortLow ReasoningEffort = "low"

	// ReasoningEffortMedium keeps the reasoning budget medium.
	// ReasoningEffortMedium 表示中等推理预算。
	ReasoningEffortMedium ReasoningEffort = "medium"

	// ReasoningEffortHigh keeps the reasoning budget high.
	// ReasoningEffortHigh 表示高推理预算。
	ReasoningEffortHigh ReasoningEffort = "high"
)

// ToolChoice captures the canonical tool-choice object consumed by Athena runtime.
// ToolChoice 描述 Athena runtime 消费的标准工具选择对象。
type ToolChoice struct {
	Kind     ToolChoiceKind `json:"kind,omitempty"`
	ToolName string         `json:"tool_name,omitempty"`
}

// ControlledOverride captures the bounded override intent accepted from callers.
// ControlledOverride 描述调用方可传入的受控覆盖意图。
type ControlledOverride struct {
	OutputMode    OutputModeIntent    `json:"output_mode,omitempty"`
	ReasoningMode ReasoningModeIntent `json:"reasoning_mode,omitempty"`
	ToolPolicy    ToolPolicyIntent    `json:"tool_policy,omitempty"`
	ToolName      string              `json:"tool_name,omitempty"`
}

// ModelPolicyContext captures the complete strategy input needed to resolve final model parameters.
// ModelPolicyContext 描述解析最终模型参数所需的完整策略输入。
type ModelPolicyContext struct {
	TaskType                 string             `json:"task_type,omitempty"`
	Scene                    string             `json:"scene,omitempty"`
	DesiredOutputMode        string             `json:"desired_output_mode,omitempty"`
	LoopStage                LoopStage          `json:"loop_stage,omitempty"`
	StepType                 string             `json:"step_type,omitempty"`
	StepRiskLevel            StepRiskLevel      `json:"step_risk_level,omitempty"`
	HasToolCall              bool               `json:"has_tool_call,omitempty"`
	IsRetry                  bool               `json:"is_retry,omitempty"`
	StructuredOutputRequired bool               `json:"structured_output_required,omitempty"`
	AllowedTools             []string           `json:"allowed_tools,omitempty"`
	ControlledOverride       ControlledOverride `json:"controlled_override,omitempty"`
}

// ResolvedModelParameters captures the final runtime-consumable model parameter object.
// ResolvedModelParameters 描述最终供 runtime 消费的模型参数对象。
type ResolvedModelParameters struct {
	PolicyName      string          `json:"policy_name,omitempty"`
	PolicyVersion   string          `json:"policy_version,omitempty"`
	Temperature     float64         `json:"temperature,omitempty"`
	TopP            float64         `json:"top_p,omitempty"`
	MaxOutputTokens int             `json:"max_output_tokens,omitempty"`
	Seed            int64           `json:"seed,omitempty"`
	ResponseFormat  ResponseFormat  `json:"response_format,omitempty"`
	ToolChoice      ToolChoice      `json:"tool_choice,omitempty"`
	ReasoningEffort ReasoningEffort `json:"reasoning_effort,omitempty"`
	Stop            []string        `json:"stop,omitempty"`
}

// ParameterTemplate captures one named parameter template or override patch.
// ParameterTemplate 描述一个具名参数模板或覆盖补丁。
type ParameterTemplate struct {
	Name            string
	Temperature     *float64
	TopP            *float64
	MaxOutputTokens *int
	Seed            *int64
	ResponseFormat  *ResponseFormat
	ToolChoice      *ToolChoice
	ReasoningEffort *ReasoningEffort
	Stop            []string
}
