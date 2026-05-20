// types.go defines runtime asset bundle and visible skill metadata structures.
// types.go 定义 runtime 资产 bundle 与可见 skill 元数据结构。
package runtimeassets

// SkillSource names the stable source category for one registered runtime skill.
// SkillSource 表示一条已注册 runtime skill 的稳定来源类别。
type SkillSource string

const (
	SkillSourceBuiltin        SkillSource = "builtin"
	SkillSourceProductManaged SkillSource = "product_managed"
	SkillSourceClientManaged  SkillSource = "client_managed"
)

// SkillExecutionTarget names where one suggested business action should actually execute.
// SkillExecutionTarget 表示一条业务动作建议应当在哪一侧真正执行。
type SkillExecutionTarget string

const (
	SkillExecutionTargetAthena SkillExecutionTarget = "athena"
	SkillExecutionTargetClient SkillExecutionTarget = "client"
)

// TaskAssetBundle captures the whitelisted runtime assets for one task subtype.
// TaskAssetBundle 描述某个任务子类允许加载的 runtime 资产白名单。
type TaskAssetBundle struct {
	ID                   string   `json:"id"`
	TaskType             string   `json:"task_type"`
	TaskSubtype          string   `json:"task_subtype"`
	RequestedOutputModes []string `json:"requested_output_modes"`
	SystemPrompt         string   `json:"system_prompt"`
	AuditSummaryHint     string   `json:"audit_summary_hint,omitempty"`
}

// SkillMetadata captures the visible registry metadata for one runtime skill.
// SkillMetadata 描述一条 runtime skill 的对外可见 registry 元数据。
type SkillMetadata struct {
	ID                  string               `json:"id"`
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	Source              SkillSource          `json:"source"`
	Product             string               `json:"product,omitempty"`
	Owner               string               `json:"owner,omitempty"`
	Version             string               `json:"version,omitempty"`
	Status              string               `json:"status,omitempty"`
	ExecutionTarget     SkillExecutionTarget `json:"execution_target"`
	AllowedTaskTypes    []string             `json:"allowed_task_types,omitempty"`
	AllowedTaskSubtypes []string             `json:"allowed_task_subtypes,omitempty"`
	AllowedOutputModes  []string             `json:"allowed_output_modes,omitempty"`
	UserVisible         bool                 `json:"user_visible"`
}

// SkillFilter narrows visible runtime skills by source and task context.
// SkillFilter 用于按来源和任务上下文收窄可见 runtime skills。
type SkillFilter struct {
	Source              SkillSource
	TaskType            string
	TaskSubtype         string
	RequestedOutputMode string
}
