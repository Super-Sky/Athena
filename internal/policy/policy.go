// policy.go defines capability-policy primitives used to constrain skills, tools, and wait behavior.
// policy.go 定义用于约束 skills、tools 和等待行为的 capability policy 基础结构。
package policy

import (
	"time"

	"moss/internal/customization"
)

// CapabilityPolicy captures repository-level capability allowlists and waiting-related defaults.
// CapabilityPolicy 描述仓库级能力白名单和等待相关默认策略。
type CapabilityPolicy struct {
	AllowedSkills               map[string]bool
	AllowedTools                map[string]bool
	AllowSupplementRequests     bool
	AllowDegradeWithoutResponse bool
	DefaultWaitTimeout          time.Duration
	MaxWaitTimeout              time.Duration
}

// AllowAll returns the permissive baseline policy used by the scaffold.
// AllowAll 返回脚手架默认使用的放行基线策略。
func AllowAll() CapabilityPolicy {
	return CapabilityPolicy{
		AllowedSkills:               map[string]bool{},
		AllowedTools:                map[string]bool{},
		AllowSupplementRequests:     true,
		AllowDegradeWithoutResponse: true,
		DefaultWaitTimeout:          30 * time.Minute,
		MaxWaitTimeout:              24 * time.Hour,
	}
}

// FilterSkills keeps only the skill names allowed by the current policy.
// FilterSkills 只保留当前策略允许的 skill 名称。
func (p CapabilityPolicy) FilterSkills(names []string) []string {
	if len(p.AllowedSkills) == 0 {
		return append([]string(nil), names...)
	}

	result := make([]string, 0, len(names))
	for _, name := range names {
		if p.AllowedSkills[name] {
			result = append(result, name)
		}
	}
	return result
}

// FilterTools keeps only the tool names allowed by the current policy.
// FilterTools 只保留当前策略允许的 tool 名称。
func (p CapabilityPolicy) FilterTools(names []string) []string {
	if len(p.AllowedTools) == 0 {
		return append([]string(nil), names...)
	}

	result := make([]string, 0, len(names))
	for _, name := range names {
		if p.AllowedTools[name] {
			result = append(result, name)
		}
	}
	return result
}

// ApplyCustomization applies capability filtering onto one request-level customization payload.
// ApplyCustomization 会把 capability 过滤规则应用到单次请求的 customization 载荷上。
func ApplyCustomization(c customization.UserCustomization, p CapabilityPolicy) customization.UserCustomization {
	c.EnabledSkills = p.FilterSkills(c.EnabledSkills)
	c.EnabledTools = p.FilterTools(c.EnabledTools)
	return c
}
