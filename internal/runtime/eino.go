// eino.go adapts the repository runtime contract onto the default Eino-backed turn executor.
// eino.go 负责把仓库 runtime 契约适配到默认基于 Eino 的单轮执行器上。
package runtime

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	einomessage "github.com/cloudwego/eino/schema"
	"moss/internal/config"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	"moss/internal/observability"
	"moss/internal/tools"
)

// EinoTurnExecutor prepares one runnable Eino execution from runtime state and execution spec.
// EinoTurnExecutor 负责根据 runtime 状态和执行规格准备一轮可运行的 Eino 执行。
type EinoTurnExecutor struct {
	Config          config.Config
	ModelProvider   model.Provider
	ToolDefs        map[string]tools.Definition
	ToolProvider    func(context.Context) map[string]tools.Definition
	Observability   *observability.Manager
	CheckpointStore RuntimeGraphCheckpointByteStore
}

// NewEinoTurnExecutorWithCatalog creates an Eino executor backed by a dynamic tool catalog.
// NewEinoTurnExecutorWithCatalog 创建由动态工具目录支持的 Eino 执行器。
func NewEinoTurnExecutorWithCatalog(cfg config.Config, provider model.Provider, catalog *tools.Catalog, obs *observability.Manager, checkpointStore RuntimeGraphCheckpointByteStore) TurnExecutor {
	return EinoTurnExecutor{
		Config:          cfg,
		ModelProvider:   provider,
		ToolProvider:    func(context.Context) map[string]tools.Definition { return catalog.Snapshot() },
		Observability:   obs,
		CheckpointStore: checkpointStore,
	}
}

// NewEinoTurnExecutor creates the default Eino-backed turn executor used by the runtime service.
// NewEinoTurnExecutor 创建 runtime service 默认使用的基于 Eino 的单轮执行器。
func NewEinoTurnExecutor(cfg config.Config, provider model.Provider, toolDefs map[string]tools.Definition, obs *observability.Manager) TurnExecutor {
	return EinoTurnExecutor{
		Config:        cfg,
		ModelProvider: provider,
		ToolDefs:      toolDefs,
		Observability: obs,
	}
}

// NewEinoTurnExecutorWithCheckpointStore creates an Eino executor with private checkpoint persistence.
// NewEinoTurnExecutorWithCheckpointStore 创建带私有 checkpoint 持久化的 Eino 执行器。
func NewEinoTurnExecutorWithCheckpointStore(cfg config.Config, provider model.Provider, toolDefs map[string]tools.Definition, obs *observability.Manager, checkpointStore RuntimeGraphCheckpointByteStore) TurnExecutor {
	return EinoTurnExecutor{
		Config:          cfg,
		ModelProvider:   provider,
		ToolDefs:        toolDefs,
		Observability:   obs,
		CheckpointStore: checkpointStore,
	}
}

// Prepare resolves one runnable model/tool execution for the current runtime turn.
// Prepare 负责为当前 runtime 单轮解析出一份可运行的模型和工具执行对象。
func (e EinoTurnExecutor) Prepare(ctx context.Context, state RuntimeState, spec *ExecutionSpec, messages []adk.Message) (*PreparedExecution, error) {
	startedAt := time.Now()
	if spec.Model.PrimaryConfig == nil {
		return nil, fmt.Errorf("execution spec is missing primary model configuration")
	}
	controls := ResolveAgentRuntimeControlSettings(spec)
	chatModel, primaryAttempts, err := e.newChatModelWithRetry(ctx, *spec.Model.PrimaryConfig, controls.ModelRetryMaxAttempts)
	fallbackAttempts := 0
	if err != nil {
		if !controls.ModelFailoverEnabled || spec.Model.FallbackConfig == nil || spec.Model.ExplicitSelection {
			return nil, err
		}
		chatModel, fallbackAttempts, err = e.newChatModelWithRetry(ctx, *spec.Model.FallbackConfig, controls.ModelRetryMaxAttempts)
		if err != nil {
			return nil, err
		}
		spec.Model.Executed = ModelEndpoint{
			ProviderID:       spec.Model.FallbackConfig.ProviderID,
			ProviderName:     spec.Model.FallbackConfig.ProviderName,
			ProviderProtocol: spec.Model.FallbackConfig.ProviderProtocol,
			ModelRecordID:    spec.Model.FallbackConfig.ModelRecordID,
			ProviderModelID:  spec.Model.FallbackConfig.ProviderModelID,
			ModelDisplayName: spec.Model.FallbackConfig.ModelDisplayName,
			Headers:          map[string]string{"fallback": "***redacted***"},
		}
		spec.Model.ExecutedConfig = spec.Model.FallbackConfig
		spec.Model.FallbackUsed = true
		spec.Model.FallbackReason = "primary_model_unavailable"
		if controls.ModelRetryMaxAttempts > 0 {
			spec.Model.FallbackReason = "primary_model_unavailable_after_retry"
		}
	} else {
		spec.Model.Executed = spec.Model.Requested
		spec.Model.ExecutedConfig = spec.Model.PrimaryConfig
	}
	if spec.Metadata.Constraints == nil {
		spec.Metadata.Constraints = map[string]any{}
	}
	spec.Metadata.Constraints["runtime_model_prepare_primary_attempts"] = primaryAttempts
	spec.Metadata.Constraints["runtime_model_prepare_fallback_attempts"] = fallbackAttempts

	toolDefinitions := e.ToolDefs
	if e.ToolProvider != nil {
		toolDefinitions = e.ToolProvider(ctx)
	}
	selectedTools := make([]tool.BaseTool, 0, len(spec.Tools.AllowedTools))
	actualToolRefs := make([]RunRevisionRef, 0, len(spec.Tools.AllowedTools))
	declarationsByName := make(map[string]ToolDefinition, len(spec.Tools.Declarations))
	for _, declaration := range spec.Tools.Declarations {
		declarationsByName[strings.TrimSpace(declaration.Function.Name)] = declaration
	}
	for _, toolName := range spec.Tools.AllowedTools {
		def, ok := toolDefinitions[toolName]
		if !ok {
			continue
		}
		info, err := def.BaseTool.Info(ctx)
		if err != nil {
			return nil, fmt.Errorf("resolve tool %q schema: %w", toolName, err)
		}
		if declaration, declared := declarationsByName[toolName]; declared {
			info, err = graphNativeToolInfoFromDefinition(declaration)
			if err != nil {
				return nil, fmt.Errorf("resolve declared tool %q schema: %w", toolName, err)
			}
		}
		actualToolRefs = append(actualToolRefs, actualRunToolRevisionRef(toolName, spec.Tools.Sources[toolName], def, info))
		selectedTools = append(selectedTools, def.BaseTool)
	}
	spec.Tools.RevisionRefs = actualToolRefs

	instruction := spec.Skill.Guidance
	if instruction == "" {
		instruction = "You are a helpful assistant."
	}
	if spec.Model.ResolvedParameters != nil {
		switch spec.Model.ResolvedParameters.ToolChoice.Kind {
		case modelparams.ToolChoiceNone:
			instruction += "\nDo not call any tools in this turn."
		case modelparams.ToolChoiceRequired:
			if len(selectedTools) == 0 {
				return nil, fmt.Errorf("tool_choice=required but no allowed tools are available")
			}
			instruction += "\nYou must call at least one allowed tool before returning the final answer."
		case modelparams.ToolChoiceSpecificTool:
			toolName := strings.TrimSpace(spec.Model.ResolvedParameters.ToolChoice.ToolName)
			if toolName == "" {
				return nil, fmt.Errorf("tool_choice=specific_tool requires tool_name")
			}
			if len(selectedTools) == 0 {
				return nil, fmt.Errorf("tool_choice=specific_tool but the specified tool is not available")
			}
			instruction += fmt.Sprintf("\nYou must call the tool %q before returning the final answer.", toolName)
		}
	}
	if spec.Inference.OutputSchema != "" {
		instruction += fmt.Sprintf("\nReturn content that satisfies this output schema requirement: %s", spec.Inference.OutputSchema)
	}
	if spec.Inference.Goal != "" {
		instruction += fmt.Sprintf("\nCurrent goal: %s", spec.Inference.Goal)
	}
	if len(spec.Inference.SuccessCriteria) > 0 {
		instruction += "\nDo not return a final answer until every success criterion in the execution guidance is satisfied."
	}
	promptRef := NewRunRevisionRef("prompt", "assembled_runtime_prompt", RunManifestSchemaVersion, "eino_turn_executor", instruction)
	spec.Metadata.ManifestRefs.Prompt = &promptRef

	callbackRecorder := NewRuntimeCallbackRecorder(RuntimeCallbackRecorderConfig{
		ProviderName: spec.Model.Executed.ProviderName,
		ModelName:    spec.Model.Executed.ProviderModelID,
	})
	toolTranscript := NewToolCallTranscript()
	checkpointRef := RuntimeGraphCheckpointRefForTurn(state)
	agent, err := NewEinoGraphNativeAgent(ctx, EinoGraphNativeAgentConfig{
		Name:             "AthenaRuntimeAgent",
		Description:      fmt.Sprintf("Runtime turn agent for skill=%s", spec.Skill.PrimarySkill),
		Instruction:      instruction,
		Model:            chatModel,
		Tools:            selectedTools,
		ToolDeclarations: append([]ToolDefinition(nil), spec.Tools.Declarations...),
		ToolChoice:       resolvedToolChoice(spec.Model.ResolvedParameters),
		ToolTranscript:   toolTranscript,
		Callbacks:        callbackRecorder.Handler(),
		CheckpointStore:  e.CheckpointStore,
		CheckpointID:     checkpointRef.CheckpointID,
		Budget:           spec.Inference.Budget,
	})
	if err != nil {
		return nil, err
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{
		EnableStreaming: true,
		Agent:           agent,
	})

	e.Observability.Trace(ctx, "runtime.turn_executor.prepare", map[string]string{
		"request_id":   state.RequestID,
		"session_id":   state.SessionID,
		"skill":        spec.Skill.PrimarySkill,
		"model_policy": resolvedPolicyName(spec.Model.ResolvedParameters),
		"persona_id":   stringValue(spec.Metadata.Constraints["persona_id"]),
	})
	e.Observability.Emit(ctx, observability.Event{
		Name:      "runtime.turn_executor.prepared",
		RequestID: state.RequestID,
		SessionID: state.SessionID,
		Turn:      state.Turn,
		Stage:     string(StageTurnExecution),
		Detail: map[string]any{
			"primary_skill":             spec.Skill.PrimarySkill,
			"tool_count":                len(selectedTools),
			"requested_provider_name":   spec.Model.Requested.ProviderName,
			"requested_model_id":        spec.Model.Requested.ProviderModelID,
			"requested_model_record_id": spec.Model.Requested.ModelRecordID,
			"executed_provider_name":    spec.Model.Executed.ProviderName,
			"executed_model_id":         spec.Model.Executed.ProviderModelID,
			"executed_model_record_id":  spec.Model.Executed.ModelRecordID,
			"fallback_used":             spec.Model.FallbackUsed,
			"fallback_reason":           spec.Model.FallbackReason,
			"explicit_model_selection":  spec.Model.ExplicitSelection,
			"fallback_available":        spec.Model.FallbackAvailable,
			"model_policy":              resolvedPolicyName(spec.Model.ResolvedParameters),
			"model_policy_version":      resolvedPolicyVersion(spec.Model.ResolvedParameters),
			"model_temperature":         resolvedPolicyTemperature(spec.Model.ResolvedParameters),
			"model_top_p":               resolvedPolicyTopP(spec.Model.ResolvedParameters),
			"model_max_output_tokens":   resolvedPolicyMaxOutputTokens(spec.Model.ResolvedParameters),
			"model_tool_choice":         resolvedPolicyToolChoice(spec.Model.ResolvedParameters),
			"model_reasoning_effort":    resolvedPolicyReasoningEffort(spec.Model.ResolvedParameters),
			"runtime_cancel_enabled":    controls.CancelEnabled,
			"runtime_cancel_mode":       controls.CancelMode,
			"runtime_model_retry_max":   controls.ModelRetryMaxAttempts,
			"runtime_failover_enabled":  controls.ModelFailoverEnabled,
			"runtime_after_agent":       controls.AfterAgentEnabled,
			"prepare_primary_attempts":  primaryAttempts,
			"prepare_fallback_attempts": fallbackAttempts,
			"persona_id":                stringValue(spec.Metadata.Constraints["persona_id"]),
			"persona_name":              stringValue(spec.Metadata.Constraints["persona_name"]),
		},
	})
	e.Observability.Observe("runtime_turn_prepare_ms", float64(time.Since(startedAt).Milliseconds()), map[string]string{
		"skill":           spec.Skill.PrimarySkill,
		"requested_model": spec.Model.Requested.ProviderModelID,
		"executed_model":  spec.Model.Executed.ProviderModelID,
	})
	e.Observability.Inc("runtime_turn_prepare_total", map[string]string{
		"skill":           spec.Skill.PrimarySkill,
		"requested_model": spec.Model.Requested.ProviderModelID,
		"executed_model":  spec.Model.Executed.ProviderModelID,
	})

	return &PreparedExecution{
		Runner:           runner,
		Messages:         normalizeMessages(messages),
		CallbackRecorder: callbackRecorder,
		ToolTranscript:   toolTranscript,
		CheckpointRef:    &checkpointRef,
	}, nil
}

func actualRunToolRevisionRef(toolName, source string, definition tools.Definition, info *einomessage.ToolInfo) RunRevisionRef {
	var parameters any
	if info != nil && info.ParamsOneOf != nil {
		if schema, err := info.ParamsOneOf.ToJSONSchema(); err == nil {
			parameters = schema
		}
	}
	payload := map[string]any{
		"implementation_type": fmt.Sprintf("%T", definition.BaseTool),
		"definition": map[string]any{
			"name": definition.Name, "description": definition.Description,
			"required_inputs": append([]string(nil), definition.RequiredInputs...),
			"tool_scope":      definition.ToolScope, "requires_confirmation": definition.RequiresConfirmation,
			"side_effect_level": definition.SideEffectLevel,
		},
	}
	actualName := strings.TrimSpace(toolName)
	if info != nil {
		actualName = firstNonEmptyRunManifestValue(info.Name, actualName)
		payload["model_binding"] = map[string]any{
			"name": info.Name, "description": info.Desc, "extra": info.Extra, "parameters": parameters,
		}
	}
	return NewRunRevisionRef("tool", actualName, "", source, payload)
}

func (e EinoTurnExecutor) newChatModelWithRetry(ctx context.Context, cfg model.ChatConfig, maxRetry int) (einomodel.ToolCallingChatModel, int, error) {
	totalAttempts := clampRetryAttempts(maxRetry) + 1
	var lastErr error
	for idx := 0; idx < totalAttempts; idx++ {
		chatModel, err := e.ModelProvider.NewChatModel(ctx, cfg)
		if err == nil {
			return chatModel, idx + 1, nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, idx + 1, ctx.Err()
		}
	}
	return nil, totalAttempts, lastErr
}

func normalizeMessages(messages []adk.Message) []adk.Message {
	result := make([]adk.Message, 0, len(messages))
	for _, msg := range messages {
		result = append(result, msg)
	}
	if len(result) == 0 {
		result = append(result, einomessage.UserMessage("hello"))
	}
	return result
}

func resolvedPolicyName(parameters *modelparams.ResolvedModelParameters) string {
	if parameters == nil {
		return ""
	}
	return parameters.PolicyName
}

func resolvedPolicyVersion(parameters *modelparams.ResolvedModelParameters) string {
	if parameters == nil {
		return ""
	}
	return parameters.PolicyVersion
}

func resolvedPolicyTemperature(parameters *modelparams.ResolvedModelParameters) float64 {
	if parameters == nil {
		return 0
	}
	return parameters.Temperature
}

func resolvedPolicyTopP(parameters *modelparams.ResolvedModelParameters) float64 {
	if parameters == nil {
		return 0
	}
	return parameters.TopP
}

func resolvedPolicyMaxOutputTokens(parameters *modelparams.ResolvedModelParameters) int {
	if parameters == nil {
		return 0
	}
	return parameters.MaxOutputTokens
}

func resolvedPolicyToolChoice(parameters *modelparams.ResolvedModelParameters) string {
	if parameters == nil {
		return ""
	}
	return string(parameters.ToolChoice.Kind)
}

func resolvedToolChoice(parameters *modelparams.ResolvedModelParameters) modelparams.ToolChoice {
	if parameters == nil {
		return modelparams.ToolChoice{Kind: modelparams.ToolChoiceAuto}
	}
	return parameters.ToolChoice
}

func resolvedPolicyReasoningEffort(parameters *modelparams.ResolvedModelParameters) string {
	if parameters == nil {
		return ""
	}
	return string(parameters.ReasoningEffort)
}
