// context.go parses the user-level persona context injected by platform and turns it into runtime-safe guidance data.
// context.go 负责解析 platform 注入的用户级 persona context，并把它收敛成 runtime 可安全消费的 guidance 数据。
package persona

import "strings"

const maxStyleRules = 6

// Context captures the minimal user-level persona shape Athena should consume from global context.
// Context 描述 Athena 应从全局上下文消费的最小用户级 persona 结构。
type Context struct {
	ID            string   `json:"id,omitempty"`
	Name          string   `json:"name,omitempty"`
	Description   string   `json:"description,omitempty"`
	StyleRules    []string `json:"style_rules,omitempty"`
	ExampleDialog string   `json:"example_dialog,omitempty"`
}

// BuildContext extracts one persona context from `global_context.persona_context`.
// BuildContext 负责从 `global_context.persona_context` 中提取 persona 上下文。
func BuildContext(globalContext map[string]any) *Context {
	if len(globalContext) == 0 {
		return nil
	}
	raw, ok := globalContext["persona_context"]
	if !ok {
		return nil
	}
	personaMap, ok := raw.(map[string]any)
	if !ok {
		return nil
	}
	ctx := &Context{
		ID:            stringValue(personaMap["id"]),
		Name:          stringValue(personaMap["name"]),
		Description:   stringValue(personaMap["description"]),
		StyleRules:    stringSlice(personaMap["style_rules"]),
		ExampleDialog: stringValue(personaMap["example_dialog"]),
	}
	if ctx.ID == "" && ctx.Name == "" && ctx.Description == "" && len(ctx.StyleRules) == 0 && ctx.ExampleDialog == "" {
		return nil
	}
	return ctx
}

// GuidanceLines returns deterministic guidance that constrains persona to expression style only.
// GuidanceLines 返回稳定 guidance 文本，并明确 persona 只影响表达风格。
func (c *Context) GuidanceLines() []string {
	if c == nil {
		return nil
	}
	lines := make([]string, 0, 6+len(c.StyleRules))
	if c.ID != "" {
		lines = append(lines, "Current user persona id: "+c.ID)
	}
	if c.Name != "" {
		lines = append(lines, "Current user persona name: "+c.Name)
	}
	if c.Description != "" {
		lines = append(lines, "Current user persona description: "+c.Description)
	}
	if len(c.StyleRules) > 0 {
		lines = append(lines, "Current user persona style rules:")
		for _, rule := range c.StyleRules {
			rule = strings.TrimSpace(rule)
			if rule != "" {
				lines = append(lines, "- "+rule)
			}
		}
	}
	if c.ExampleDialog != "" {
		lines = append(lines, "Current user persona example dialog: "+c.ExampleDialog)
	}
	lines = append(lines, "User persona affects wording, tone, length, evidence presentation, and recommendation style only.")
	lines = append(lines, "User persona must not change facts, evidence thresholds, risk ratings, safety boundaries, or execution decisions.")
	return lines
}

func stringValue(value any) string {
	if text, ok := value.(string); ok {
		return strings.TrimSpace(text)
	}
	return ""
}

func stringSlice(value any) []string {
	switch items := value.(type) {
	case []string:
		return compactStrings(items)
	case []any:
		result := make([]string, 0, len(items))
		for _, item := range items {
			if text, ok := item.(string); ok {
				result = append(result, text)
			}
		}
		return compactStrings(result)
	default:
		return nil
	}
}

func compactStrings(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if item != "" {
			result = append(result, item)
		}
		if len(result) == maxStyleRules {
			break
		}
	}
	return result
}
