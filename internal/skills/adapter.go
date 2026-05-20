// adapter.go adapts raw skill definitions and uploaded packages into runtime-facing skill views.
// adapter.go 负责把原始 skill 定义和上传 package 适配为面向 runtime 的 skill 视图。
package skills

import "sort"

// Skill is the transport-friendly declaration shape for an official skill.
// Skill 是对外更友好的官方 skill 声明结构。
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ToolNames   []string `json:"tool_names,omitempty"`
	Guidance    string   `json:"guidance,omitempty"`
}

// AdaptedSkill is the runtime-friendly view derived from a declaration-time skill.
// AdaptedSkill 是由声明态 skill 派生出的 runtime 友好视图。
type AdaptedSkill struct {
	Name        string
	Description string
	ToolNames   []string
	Guidance    string
}

// Adapter converts declaration-time skill definitions into runtime-friendly shapes.
// Adapter 负责把声明态 skill 定义转换为 runtime 更易消费的结构。
type Adapter interface {
	Adapt(Definition) AdaptedSkill
}

// DefaultAdapter is the scaffold's default skill adapter.
// DefaultAdapter 是当前脚手架默认使用的 skill adapter。
type DefaultAdapter struct{}

// NewAdapter creates the scaffold's default skill adapter.
// NewAdapter 创建当前脚手架默认 skill adapter。
func NewAdapter() Adapter {
	return DefaultAdapter{}
}

// Adapt normalizes one declaration-time skill into stable runtime ordering.
// Adapt 会把一条声明态 skill 规范化为稳定顺序的 runtime 结构。
func (DefaultAdapter) Adapt(def Definition) AdaptedSkill {
	toolNames := append([]string(nil), def.ToolNames...)
	sort.Strings(toolNames)
	return AdaptedSkill{
		Name:        def.Name,
		Description: def.Description,
		ToolNames:   toolNames,
		Guidance:    def.Guidance,
	}
}
