// resolver.go resolves the final model-parameter object from one policy context.
// resolver.go 负责根据策略上下文解析最终模型参数对象。
package parameters

import (
	"fmt"
	"sort"
	"strings"
)

// ResolveModelParameters is the single resolution entry that merges the internal template chain and controlled override.
// ResolveModelParameters 是唯一的模型参数解析入口，会合并内部模板链和受控 override。
func ResolveModelParameters(context ModelPolicyContext) (ResolvedModelParameters, error) {
	catalog := DefaultTemplateCatalog()
	return resolveWithCatalog(catalog, context)
}

func resolveWithCatalog(catalog TemplateCatalog, context ModelPolicyContext) (ResolvedModelParameters, error) {
	resolved := ResolvedModelParameters{
		PolicyName:    catalog.GlobalDefault.Name,
		PolicyVersion: catalog.Version,
	}
	applyTemplate(&resolved, catalog.GlobalDefault, false)

	if template, ok := catalog.ByTaskType[strings.TrimSpace(context.TaskType)]; ok {
		resolved.PolicyName = template.Name
		applyTemplate(&resolved, template, false)
	}
	if template, ok := catalog.ByScene[strings.TrimSpace(context.Scene)]; ok {
		applyTemplate(&resolved, template, false)
	}
	if template, ok := catalog.ByDesiredOutputMode[strings.TrimSpace(context.DesiredOutputMode)]; ok {
		resolved.PolicyName = template.Name
		applyTemplate(&resolved, template, false)
	}
	if template, ok := catalog.ByLoopStage[context.LoopStage]; ok {
		resolved.PolicyName = template.Name
		applyTemplate(&resolved, template, false)
	}
	if template, ok := catalog.ByStepType[strings.TrimSpace(context.StepType)]; ok {
		applyTemplate(&resolved, template, false)
	}
	if template, ok := catalog.ByRiskLevel[context.StepRiskLevel]; ok {
		applyTemplate(&resolved, template, false)
	}
	if context.IsRetry {
		applyTemplate(&resolved, catalog.RetryOverride, false)
	}
	if err := applyControlledOverride(&resolved, context.ControlledOverride, normalizedAllowedTools(context.AllowedTools)); err != nil {
		return ResolvedModelParameters{}, err
	}
	return resolved, nil
}

// ParseControlledOverride validates one caller-provided override payload and rejects raw-parameter passthrough.
// ParseControlledOverride 会校验调用方传入的 override 载荷，并拒绝原始参数直传。
func ParseControlledOverride(raw map[string]any) (ControlledOverride, error) {
	if len(raw) == 0 {
		return ControlledOverride{}, nil
	}
	allowedKeys := map[string]struct{}{
		"output_mode":    {},
		"reasoning_mode": {},
		"tool_policy":    {},
		"tool_name":      {},
	}
	for key := range raw {
		if _, ok := allowedKeys[key]; !ok {
			return ControlledOverride{}, fmt.Errorf("controlled override only accepts intent fields")
		}
	}

	override := ControlledOverride{
		OutputMode:    OutputModeIntent(strings.TrimSpace(stringValue(raw["output_mode"]))),
		ReasoningMode: ReasoningModeIntent(strings.TrimSpace(stringValue(raw["reasoning_mode"]))),
		ToolPolicy:    ToolPolicyIntent(strings.TrimSpace(stringValue(raw["tool_policy"]))),
		ToolName:      strings.TrimSpace(stringValue(raw["tool_name"])),
	}
	if err := validateControlledOverride(override); err != nil {
		return ControlledOverride{}, err
	}
	return override, nil
}

func applyControlledOverride(resolved *ResolvedModelParameters, override ControlledOverride, allowedTools map[string]struct{}) error {
	if resolved == nil {
		return nil
	}
	if err := validateControlledOverride(override); err != nil {
		return err
	}

	switch override.OutputMode {
	case "":
	case OutputModeIntentText:
		resolved.ResponseFormat = ResponseFormatText
	case OutputModeIntentStructured:
		resolved.ResponseFormat = ResponseFormatJSONObject
	case OutputModeIntentStrictJSON:
		resolved.ResponseFormat = ResponseFormatJSONSchema
	}

	switch override.ReasoningMode {
	case "":
	case ReasoningModeIntentLow:
		resolved.ReasoningEffort = ReasoningEffortLow
	case ReasoningModeIntentMedium:
		resolved.ReasoningEffort = ReasoningEffortMedium
	case ReasoningModeIntentHigh:
		resolved.ReasoningEffort = ReasoningEffortHigh
	}

	switch override.ToolPolicy {
	case "":
	case ToolPolicyIntentNone:
		resolved.ToolChoice = ToolChoice{Kind: ToolChoiceNone}
	case ToolPolicyIntentAuto:
		resolved.ToolChoice = ToolChoice{Kind: ToolChoiceAuto}
	case ToolPolicyIntentRequired:
		resolved.ToolChoice = ToolChoice{Kind: ToolChoiceRequired}
	case ToolPolicyIntentSpecificTool:
		if _, ok := allowedTools[override.ToolName]; !ok {
			return fmt.Errorf("tool_name %q is not part of allowed_tools", override.ToolName)
		}
		resolved.ToolChoice = ToolChoice{Kind: ToolChoiceSpecificTool, ToolName: override.ToolName}
	}

	return nil
}

func validateControlledOverride(override ControlledOverride) error {
	switch override.OutputMode {
	case "", OutputModeIntentText, OutputModeIntentStructured, OutputModeIntentStrictJSON:
	default:
		return fmt.Errorf("output_mode must be one of text, structured, strict_json")
	}
	switch override.ReasoningMode {
	case "", ReasoningModeIntentLow, ReasoningModeIntentMedium, ReasoningModeIntentHigh:
	default:
		return fmt.Errorf("reasoning_mode must be one of low, medium, high")
	}
	switch override.ToolPolicy {
	case "", ToolPolicyIntentNone, ToolPolicyIntentAuto, ToolPolicyIntentRequired, ToolPolicyIntentSpecificTool:
	default:
		return fmt.Errorf("tool_policy must be one of none, auto, required, specific_tool")
	}
	if override.ToolPolicy == ToolPolicyIntentSpecificTool && strings.TrimSpace(override.ToolName) == "" {
		return fmt.Errorf("tool_name is required when tool_policy=specific_tool")
	}
	if override.ToolPolicy != ToolPolicyIntentSpecificTool && strings.TrimSpace(override.ToolName) != "" {
		return fmt.Errorf("tool_name is only allowed when tool_policy=specific_tool")
	}
	return nil
}

func applyTemplate(resolved *ResolvedModelParameters, template ParameterTemplate, forcePolicyName bool) {
	if resolved == nil {
		return
	}
	if forcePolicyName && strings.TrimSpace(template.Name) != "" {
		resolved.PolicyName = template.Name
	}
	if template.Temperature != nil {
		resolved.Temperature = *template.Temperature
	}
	if template.TopP != nil {
		resolved.TopP = *template.TopP
	}
	if template.MaxOutputTokens != nil {
		resolved.MaxOutputTokens = *template.MaxOutputTokens
	}
	if template.Seed != nil {
		resolved.Seed = *template.Seed
	}
	if template.ResponseFormat != nil {
		resolved.ResponseFormat = *template.ResponseFormat
	}
	if template.ToolChoice != nil {
		resolved.ToolChoice = *template.ToolChoice
	}
	if template.ReasoningEffort != nil {
		resolved.ReasoningEffort = *template.ReasoningEffort
	}
	if len(template.Stop) > 0 {
		resolved.Stop = append([]string(nil), template.Stop...)
	}
}

func normalizedAllowedTools(allowedTools []string) map[string]struct{} {
	if len(allowedTools) == 0 {
		return map[string]struct{}{}
	}
	cleaned := make([]string, 0, len(allowedTools))
	for _, tool := range allowedTools {
		if trimmed := strings.TrimSpace(tool); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	sort.Strings(cleaned)
	result := make(map[string]struct{}, len(cleaned))
	for _, tool := range cleaned {
		result[tool] = struct{}{}
	}
	return result
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}
