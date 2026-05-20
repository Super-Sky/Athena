// model_parameters.go resolves Athena-internal model parameter policies for app-layer model invocations.
// model_parameters.go 负责为 app 层模型调用解析 Athena 内部模型参数策略。
package app

import (
	"strings"

	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
)

func resolveAppModelConfig(selection model.ChatConfig, context modelparams.ModelPolicyContext) (model.ChatConfig, error) {
	configCopy := selection
	resolved, err := modelparams.ResolveModelParameters(context)
	if err != nil {
		return model.ChatConfig{}, err
	}
	configCopy.ResolvedParameters = &resolved
	return configCopy, nil
}

func startupGreetingPolicyContext() modelparams.ModelPolicyContext {
	return modelparams.ModelPolicyContext{
		TaskType:     "system_probe",
		LoopStage:    modelparams.LoopStageInitialAnswer,
		AllowedTools: nil,
		ControlledOverride: modelparams.ControlledOverride{
			OutputMode:    modelparams.OutputModeIntentText,
			ReasoningMode: modelparams.ReasoningModeIntentLow,
			ToolPolicy:    modelparams.ToolPolicyIntentNone,
		},
	}
}

func providerProbePolicyContext() modelparams.ModelPolicyContext {
	return modelparams.ModelPolicyContext{
		TaskType:     "provider_probe",
		LoopStage:    modelparams.LoopStageInitialAnswer,
		AllowedTools: nil,
		ControlledOverride: modelparams.ControlledOverride{
			OutputMode:    modelparams.OutputModeIntentText,
			ReasoningMode: modelparams.ReasoningModeIntentLow,
			ToolPolicy:    modelparams.ToolPolicyIntentNone,
		},
	}
}

func runtimeScenarioPolicyContext(req RuntimeScenarioRequest) modelparams.ModelPolicyContext {
	return modelparams.ModelPolicyContext{
		TaskType:                 strings.TrimSpace(req.TaskType),
		Scene:                    strings.TrimSpace(req.TaskSubtype),
		DesiredOutputMode:        "structured",
		LoopStage:                modelparams.LoopStageExecutionGovernance,
		StructuredOutputRequired: true,
		AllowedTools:             nil,
		ControlledOverride: modelparams.ControlledOverride{
			OutputMode:    modelparams.OutputModeIntentStrictJSON,
			ReasoningMode: modelparams.ReasoningModeIntentHigh,
			ToolPolicy:    modelparams.ToolPolicyIntentNone,
		},
	}
}
