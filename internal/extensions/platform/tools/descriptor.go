// descriptor.go defines platform tool descriptors and invocation hints consumed by Athena extensions.
// descriptor.go 定义 Athena 扩展层消费的 platform tool 描述符与调用提示。
package tools

import (
	"strings"

	platformautomation "moss/internal/extensions/platform/automation"
)

// SideEffectLevel captures the side-effect severity of one platform tool.
// SideEffectLevel 描述一个 platform tool 的副作用等级。
type SideEffectLevel string

const (
	// SideEffectLevelReadOnly means the tool only reads or queries state.
	// SideEffectLevelReadOnly 表示 tool 只读取或查询状态。
	SideEffectLevelReadOnly SideEffectLevel = "read_only"

	// SideEffectLevelStateful means the tool mutates state or creates a platform resource.
	// SideEffectLevelStateful 表示 tool 会修改状态或创建平台资源。
	SideEffectLevelStateful SideEffectLevel = "stateful"
)

// FieldDescriptor captures one structured field expected by a platform tool.
// FieldDescriptor 描述一个 platform tool 期望的结构化字段。
type FieldDescriptor struct {
	Name     string `json:"name,omitempty"`
	Type     string `json:"type,omitempty"`
	Required bool   `json:"required,omitempty"`
}

// ToolDescriptor captures one platform-provided tool contract that Athena can reason about.
// ToolDescriptor 描述一条 Athena 可推理的 platform tool contract。
type ToolDescriptor struct {
	Name                 string            `json:"name,omitempty"`
	Description          string            `json:"description,omitempty"`
	InputSchema          []FieldDescriptor `json:"input_schema,omitempty"`
	RequiresConfirmation bool              `json:"requires_confirmation,omitempty"`
	SideEffectLevel      SideEffectLevel   `json:"side_effect_level,omitempty"`
}

// ToolInvocationHint captures the tool Athena recommends platform to invoke after confirmation.
// ToolInvocationHint 描述 Athena 建议 platform 在确认后调用的一条 tool 提示。
type ToolInvocationHint struct {
	ToolName      string         `json:"tool_name,omitempty"`
	WhenToInvoke  string         `json:"when_to_invoke,omitempty"`
	Arguments     map[string]any `json:"arguments,omitempty"`
	RequiresUserConfirmation bool `json:"requires_user_confirmation,omitempty"`
}

// DefaultDescriptors returns the minimal platform tool descriptors Athena currently expects.
// DefaultDescriptors 返回 Athena 当前预期的最小 platform tool 描述集合。
func DefaultDescriptors() []ToolDescriptor {
	return []ToolDescriptor{
		{
			Name:        "get_platform_context_catalog",
			Description: "Load the platform context catalog that lists available summary and detail resources.",
			InputSchema: nil,
			RequiresConfirmation: false,
			SideEffectLevel:      SideEffectLevelReadOnly,
		},
		{
			Name:        "get_platform_context_detail",
			Description: "Load one detailed platform context resource by type.",
			InputSchema: []FieldDescriptor{
				{Name: "type", Type: "string", Required: true},
			},
			RequiresConfirmation: false,
			SideEffectLevel:      SideEffectLevelReadOnly,
		},
		{
			Name:        "save_automation_draft",
			Description: "Persist one draft plan for later review in platform.",
			InputSchema: []FieldDescriptor{
				{Name: "draft", Type: "object", Required: true},
			},
			RequiresConfirmation: false,
			SideEffectLevel:      SideEffectLevelStateful,
		},
		{
			Name:        "create_automation_from_payload",
			Description: "Create one confirmed automation directly from Athena's structured payload.",
			InputSchema: []FieldDescriptor{
				{Name: "payload", Type: "object", Required: true},
				{Name: "confirmation_source", Type: "string", Required: true},
			},
			RequiresConfirmation: true,
			SideEffectLevel:      SideEffectLevelStateful,
		},
		{
			Name:        "get_automation_draft",
			Description: "Load one saved automation draft by id.",
			InputSchema: []FieldDescriptor{
				{Name: "draft_id", Type: "string", Required: true},
			},
			RequiresConfirmation: false,
			SideEffectLevel:      SideEffectLevelReadOnly,
		},
		{
			Name:        "get_automation",
			Description: "Load one automation resource by id.",
			InputSchema: []FieldDescriptor{
				{Name: "automation_id", Type: "string", Required: true},
			},
			RequiresConfirmation: false,
			SideEffectLevel:      SideEffectLevelReadOnly,
		},
	}
}

// BuildAutomationHints returns tool invocation hints for confirmed automation creation flows.
// BuildAutomationHints 返回确认后自动化创建流程需要的 tool 调用提示。
func BuildAutomationHints(payload *platformautomation.CreatePayload) []ToolInvocationHint {
	if payload == nil {
		return nil
	}
	return []ToolInvocationHint{
		{
			ToolName:                 "create_automation_from_payload",
			WhenToInvoke:             "after_user_confirmation",
			Arguments:                map[string]any{"payload": payload, "confirmation_source": "user_confirmed"},
			RequiresUserConfirmation: true,
		},
	}
}

// BuildContextDetailHints returns read-only tool hints for loading platform context details on demand.
// BuildContextDetailHints 返回按需读取 platform 上下文 detail 的只读 tool 提示。
func BuildContextDetailHints(types []string) []ToolInvocationHint {
	hints := make([]ToolInvocationHint, 0, len(types))
	for _, typ := range compactStrings(types) {
		hints = append(hints, ToolInvocationHint{
			ToolName:                 "get_platform_context_detail",
			WhenToInvoke:             "before_final_answer_when_summary_is_insufficient",
			Arguments:                map[string]any{"type": typ},
			RequiresUserConfirmation: false,
		})
	}
	return hints
}

func compactToolNames(descriptors []ToolDescriptor) []string {
	result := make([]string, 0, len(descriptors))
	for _, descriptor := range descriptors {
		name := strings.TrimSpace(descriptor.Name)
		if name != "" {
			result = append(result, name)
		}
	}
	return result
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
