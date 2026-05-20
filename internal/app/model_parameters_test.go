// model_parameters_test.go verifies app-layer model invocations consume AGENT-5 resolved parameter policies.
// model_parameters_test.go 验证 app 层模型调用会消费 AGENT-5 解析后的参数策略。
package app

import (
	"context"
	"testing"

	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"moss/internal/config"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	"moss/internal/observability"
	"moss/internal/session"
)

type recordingAppModelProvider struct {
	calls []model.ChatConfig
}

func (p *recordingAppModelProvider) NewChatModel(_ context.Context, cfg model.ChatConfig) (einomodel.ToolCallingChatModel, error) {
	p.calls = append(p.calls, cfg)
	return &stubAppChatModel{}, nil
}

type stubAppChatModel struct{}

func (stubAppChatModel) Generate(context.Context, []*einoschema.Message, ...einomodel.Option) (*einoschema.Message, error) {
	return einoschema.AssistantMessage(`{"decision":"allow","reason":"ok","user_visible_copy":"ok"}`, nil), nil
}

func (stubAppChatModel) Stream(context.Context, []*einoschema.Message, ...einomodel.Option) (*einoschema.StreamReader[*einoschema.Message], error) {
	return nil, nil
}

func (s *stubAppChatModel) WithTools([]*einoschema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return s, nil
}

func TestTestProviderModelUsesPolicyResolvedParameters(t *testing.T) {
	ctx := context.Background()
	modelStore := model.NewMemoryStore("test-key")
	providerDef, err := modelStore.CreateProvider(ctx, model.ProviderUpsertInput{
		Name:                  "probe-provider",
		BaseURL:               "https://example.com/v1",
		Protocol:              model.ProtocolOpenAICompatible,
		APIKey:                "sk-test",
		RequestTimeoutSeconds: 30,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	modelRecord, err := modelStore.CreateProviderModel(ctx, providerDef.ID, model.ProviderModelUpsertInput{
		ModelID:     "gpt-test",
		DisplayName: "GPT Test",
		Enabled:     true,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel() error = %v", err)
	}

	provider := &recordingAppModelProvider{}
	svc := NewServiceWithDependencies(config.Config{}, observability.NewNoopManager(), session.NewMemoryStore(), modelStore)
	svc.ModelProvider = provider

	if _, err := svc.TestProviderModel(ctx, modelRecord.ID); err != nil {
		t.Fatalf("TestProviderModel() error = %v", err)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(provider.calls))
	}
	if provider.calls[0].ResolvedParameters == nil {
		t.Fatalf("expected resolved parameters on provider probe config")
	}
	if provider.calls[0].ResolvedParameters.ToolChoice.Kind != modelparams.ToolChoiceNone {
		t.Fatalf("tool_choice = %#v, want none", provider.calls[0].ResolvedParameters.ToolChoice)
	}
}

func TestRuntimeScenarioPolicyContextUsesStrictStructuredGovernance(t *testing.T) {
	context := runtimeScenarioPolicyContext(RuntimeScenarioRequest{
		TaskType:    "runtime_event_analysis",
		TaskSubtype: "openclaw_before_tool_call",
	})
	if context.LoopStage != modelparams.LoopStageExecutionGovernance {
		t.Fatalf("loop_stage = %q, want execution_governance", context.LoopStage)
	}
	if context.ControlledOverride.OutputMode != modelparams.OutputModeIntentStrictJSON {
		t.Fatalf("output_mode = %q, want strict_json", context.ControlledOverride.OutputMode)
	}
	if context.ControlledOverride.ToolPolicy != modelparams.ToolPolicyIntentNone {
		t.Fatalf("tool_policy = %q, want none", context.ControlledOverride.ToolPolicy)
	}
}
