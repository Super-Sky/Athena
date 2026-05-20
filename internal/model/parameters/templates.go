// templates.go defines the internal parameter templates used by the model policy center.
// templates.go 定义模型参数策略中心使用的内部参数模板。
package parameters

// TemplateCatalog captures the internal parameter templates and per-dimension override tables.
// TemplateCatalog 描述内部参数模板以及按维度分类的覆盖表。
type TemplateCatalog struct {
	Version             string
	GlobalDefault       ParameterTemplate
	ByTaskType          map[string]ParameterTemplate
	ByScene             map[string]ParameterTemplate
	ByDesiredOutputMode map[string]ParameterTemplate
	ByLoopStage         map[LoopStage]ParameterTemplate
	ByStepType          map[string]ParameterTemplate
	ByRiskLevel         map[StepRiskLevel]ParameterTemplate
	RetryOverride       ParameterTemplate
}

// DefaultTemplateCatalog returns the first-phase internal model parameter templates.
// DefaultTemplateCatalog 返回第一阶段内部模型参数模板集合。
func DefaultTemplateCatalog() TemplateCatalog {
	return TemplateCatalog{
		Version: "v1",
		GlobalDefault: ParameterTemplate{
			Name:            "global_default",
			Temperature:     floatPtr(0.2),
			TopP:            floatPtr(0.9),
			MaxOutputTokens: intPtr(800),
			Seed:            int64Ptr(20260418),
			ResponseFormat:  responseFormatPtr(ResponseFormatText),
			ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceAuto}),
			ReasoningEffort: reasoningEffortPtr(ReasoningEffortMedium),
		},
		ByTaskType: map[string]ParameterTemplate{
			"chat": {
				Name:            "default_chat",
				Temperature:     floatPtr(0.25),
				TopP:            floatPtr(0.92),
				MaxOutputTokens: intPtr(900),
				ResponseFormat:  responseFormatPtr(ResponseFormatText),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceAuto}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortMedium),
			},
			"workflow_step_request": {
				Name:            "workflow_planning",
				Temperature:     floatPtr(0.15),
				TopP:            floatPtr(0.85),
				MaxOutputTokens: intPtr(1200),
				ResponseFormat:  responseFormatPtr(ResponseFormatJSONSchema),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceAuto}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortHigh),
			},
		},
		ByDesiredOutputMode: map[string]ParameterTemplate{
			"artifact_write": {
				Name:            "artifact_write",
				Temperature:     floatPtr(0.1),
				TopP:            floatPtr(0.82),
				MaxOutputTokens: intPtr(1400),
				ResponseFormat:  responseFormatPtr(ResponseFormatText),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceNone}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortMedium),
			},
		},
		ByLoopStage: map[LoopStage]ParameterTemplate{
			LoopStageStructuredResult: {
				Name:            "stable_structured",
				Temperature:     floatPtr(0.05),
				TopP:            floatPtr(0.8),
				MaxOutputTokens: intPtr(1000),
				ResponseFormat:  responseFormatPtr(ResponseFormatJSONSchema),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceNone}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortMedium),
			},
			LoopStageWorkflowPlan: {
				Name:            "workflow_planning",
				Temperature:     floatPtr(0.12),
				TopP:            floatPtr(0.84),
				MaxOutputTokens: intPtr(1400),
				ResponseFormat:  responseFormatPtr(ResponseFormatJSONSchema),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceAuto}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortHigh),
			},
			LoopStageExecutionGovernance: {
				Name:            "execution_governance",
				Temperature:     floatPtr(0.02),
				TopP:            floatPtr(0.7),
				MaxOutputTokens: intPtr(700),
				ResponseFormat:  responseFormatPtr(ResponseFormatJSONSchema),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceNone}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortHigh),
			},
			LoopStageNextQuestions: {
				Name:            "next_questions",
				Temperature:     floatPtr(0.35),
				TopP:            floatPtr(0.95),
				MaxOutputTokens: intPtr(200),
				ResponseFormat:  responseFormatPtr(ResponseFormatText),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceNone}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortLow),
			},
			LoopStageRetryFormatting: {
				Name:            "retry_formatting",
				Temperature:     floatPtr(0.0),
				TopP:            floatPtr(1.0),
				MaxOutputTokens: intPtr(700),
				ResponseFormat:  responseFormatPtr(ResponseFormatJSONSchema),
				ToolChoice:      toolChoicePtr(ToolChoice{Kind: ToolChoiceNone}),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortLow),
			},
		},
		ByRiskLevel: map[StepRiskLevel]ParameterTemplate{
			StepRiskLevelHigh: {
				Name:            "high_risk_step",
				Temperature:     floatPtr(0.01),
				TopP:            floatPtr(0.65),
				ReasoningEffort: reasoningEffortPtr(ReasoningEffortHigh),
			},
		},
		RetryOverride: ParameterTemplate{
			Name:            "retry_override",
			Temperature:     floatPtr(0.0),
			TopP:            floatPtr(1.0),
			ReasoningEffort: reasoningEffortPtr(ReasoningEffortLow),
		},
	}
}

func floatPtr(value float64) *float64 {
	return &value
}

func intPtr(value int) *int {
	return &value
}

func int64Ptr(value int64) *int64 {
	return &value
}

func responseFormatPtr(value ResponseFormat) *ResponseFormat {
	return &value
}

func reasoningEffortPtr(value ReasoningEffort) *ReasoningEffort {
	return &value
}

func toolChoicePtr(value ToolChoice) *ToolChoice {
	return &value
}
