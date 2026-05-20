// resolver_test.go verifies parameter template matching, override validation, and fail-closed behavior.
// resolver_test.go 验证参数模板命中、override 校验以及 fail-closed 行为。
package parameters

import "testing"

func TestResolveModelParametersSelectsTaskTypeTemplate(t *testing.T) {
	resolved, err := ResolveModelParameters(ModelPolicyContext{
		TaskType: "workflow_step_request",
	})
	if err != nil {
		t.Fatalf("ResolveModelParameters() error = %v", err)
	}
	if resolved.PolicyName != "workflow_planning" {
		t.Fatalf("policy_name = %q, want workflow_planning", resolved.PolicyName)
	}
	if resolved.ResponseFormat != ResponseFormatJSONSchema {
		t.Fatalf("response_format = %q, want json_schema", resolved.ResponseFormat)
	}
	if resolved.ReasoningEffort != ReasoningEffortHigh {
		t.Fatalf("reasoning_effort = %q, want high", resolved.ReasoningEffort)
	}
}

func TestResolveModelParametersLoopStageOverrides(t *testing.T) {
	resolved, err := ResolveModelParameters(ModelPolicyContext{
		TaskType:  "chat",
		LoopStage: LoopStageNextQuestions,
	})
	if err != nil {
		t.Fatalf("ResolveModelParameters() error = %v", err)
	}
	if resolved.PolicyName != "next_questions" {
		t.Fatalf("policy_name = %q, want next_questions", resolved.PolicyName)
	}
	if resolved.Temperature != 0.35 {
		t.Fatalf("temperature = %v, want 0.35", resolved.Temperature)
	}
	if resolved.MaxOutputTokens != 200 {
		t.Fatalf("max_output_tokens = %d, want 200", resolved.MaxOutputTokens)
	}
}

func TestResolveModelParametersRetryFormattingIsMoreConservative(t *testing.T) {
	resolved, err := ResolveModelParameters(ModelPolicyContext{
		TaskType:                 "chat",
		LoopStage:                LoopStageStructuredResult,
		IsRetry:                  true,
		StructuredOutputRequired: true,
	})
	if err != nil {
		t.Fatalf("ResolveModelParameters() error = %v", err)
	}
	if resolved.Temperature != 0.0 {
		t.Fatalf("temperature = %v, want 0.0", resolved.Temperature)
	}
	if resolved.TopP != 1.0 {
		t.Fatalf("top_p = %v, want 1.0", resolved.TopP)
	}
	if resolved.ReasoningEffort != ReasoningEffortLow {
		t.Fatalf("reasoning_effort = %q, want low", resolved.ReasoningEffort)
	}
}

func TestResolveModelParametersFailsClosedWhenSpecificToolMissingName(t *testing.T) {
	_, err := ResolveModelParameters(ModelPolicyContext{
		ControlledOverride: ControlledOverride{
			ToolPolicy: ToolPolicyIntentSpecificTool,
		},
		AllowedTools: []string{"knowledge_search"},
	})
	if err == nil {
		t.Fatalf("expected tool_name validation error")
	}
}

func TestResolveModelParametersRejectsToolOutsideAllowedTools(t *testing.T) {
	_, err := ResolveModelParameters(ModelPolicyContext{
		ControlledOverride: ControlledOverride{
			ToolPolicy: ToolPolicyIntentSpecificTool,
			ToolName:   "run_shell",
		},
		AllowedTools: []string{"knowledge_search"},
	})
	if err == nil {
		t.Fatalf("expected allowed_tools validation error")
	}
}

func TestParseControlledOverrideRejectsRawParameterPassthrough(t *testing.T) {
	_, err := ParseControlledOverride(map[string]any{
		"output_mode": "structured",
		"temperature": 0.7,
	})
	if err == nil {
		t.Fatalf("expected raw parameter passthrough rejection")
	}
}

func TestParseControlledOverrideAcceptsIntentOnly(t *testing.T) {
	override, err := ParseControlledOverride(map[string]any{
		"output_mode":    "strict_json",
		"reasoning_mode": "high",
		"tool_policy":    "specific_tool",
		"tool_name":      "knowledge_search",
	})
	if err != nil {
		t.Fatalf("ParseControlledOverride() error = %v", err)
	}
	if override.OutputMode != OutputModeIntentStrictJSON {
		t.Fatalf("output_mode = %q, want strict_json", override.OutputMode)
	}
	if override.ReasoningMode != ReasoningModeIntentHigh {
		t.Fatalf("reasoning_mode = %q, want high", override.ReasoningMode)
	}
	if override.ToolPolicy != ToolPolicyIntentSpecificTool || override.ToolName != "knowledge_search" {
		t.Fatalf("tool override = %#v, want specific_tool/knowledge_search", override)
	}
}
