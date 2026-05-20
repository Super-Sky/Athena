package runtime

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	einoschema "github.com/cloudwego/eino/schema"
	"moss/internal/config"
	"moss/internal/customization"
	"moss/internal/memory"
	"moss/internal/model"
	modelparams "moss/internal/model/parameters"
	"moss/internal/observability"
	"moss/internal/policy"
	runtimetask "moss/internal/runtime/task"
	"moss/internal/session"
	"moss/internal/skills"
	"moss/internal/tools"
)

type stubTurnExecutor struct {
	called bool
}

func (s *stubTurnExecutor) Prepare(_ context.Context, _ RuntimeState, spec *ExecutionSpec, messages []adk.Message) (*PreparedExecution, error) {
	s.called = true
	return &PreparedExecution{
		Spec:     spec,
		Messages: messages,
	}, nil
}

type recordingModelProvider struct {
	calls []model.ChatConfig
	fail  map[string]error
}

func (p *recordingModelProvider) NewChatModel(_ context.Context, cfg model.ChatConfig) (einomodel.ToolCallingChatModel, error) {
	p.calls = append(p.calls, cfg)
	if p.fail != nil {
		if err := p.fail[cfg.ModelRecordID]; err != nil {
			return nil, err
		}
	}
	return &stubChatModel{}, nil
}

type stubChatModel struct{}

func (stubChatModel) Generate(context.Context, []*einoschema.Message, ...einomodel.Option) (*einoschema.Message, error) {
	return einoschema.AssistantMessage("ok", nil), nil
}

func (stubChatModel) Stream(context.Context, []*einoschema.Message, ...einomodel.Option) (*einoschema.StreamReader[*einoschema.Message], error) {
	return nil, nil
}

func (s *stubChatModel) WithTools([]*einoschema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	return s, nil
}

func newTestRuntimeService(tb testing.TB, p policy.CapabilityPolicy, executor TurnExecutor) *Service {
	tb.Helper()

	toolDefs, err := tools.DemoDefinitions()
	if err != nil {
		tb.Fatalf("DemoDefinitions() error = %v", err)
	}

	return NewService(
		config.Config{
			Runtime: config.RuntimeConfig{
				MaxConcurrentRequests: 2,
				MaxConcurrentTools:    2,
				RequestTimeoutSeconds: 30,
			},
		},
		p,
		memory.DefaultContextPolicy(),
		skills.BuiltinRegistry(),
		skills.NewAdapter(),
		toolDefs,
		executor,
		observability.NewNoopManager(),
	)
}

func TestBuildModelSpecPreservesExplicitSelectionAndFallback(t *testing.T) {
	selection := &model.Selection{
		Explicit: true,
		Primary: model.ChatConfig{
			ProviderID:       "provider-primary",
			ProviderName:     "Primary Provider",
			ProviderProtocol: "openai_compatible",
			ModelRecordID:    "model-primary",
			ProviderModelID:  "gpt-primary",
			ModelDisplayName: "Primary Model",
			Headers:          map[string]string{"Authorization": "secret", "X-Test": "value"},
		},
		Fallback: &model.ChatConfig{
			ProviderID:       "provider-fallback",
			ProviderName:     "Fallback Provider",
			ProviderProtocol: "anthropic",
			ModelRecordID:    "model-fallback",
			ProviderModelID:  "claude-fallback",
			ModelDisplayName: "Fallback Model",
		},
	}

	spec := buildModelSpec(selection)
	if !spec.ExplicitSelection {
		t.Fatalf("expected explicit selection")
	}
	if spec.Requested.ModelRecordID != "model-primary" {
		t.Fatalf("requested model record id = %q, want model-primary", spec.Requested.ModelRecordID)
	}
	if spec.Requested.ProviderModelID != "gpt-primary" {
		t.Fatalf("requested provider model id = %q, want gpt-primary", spec.Requested.ProviderModelID)
	}
	if spec.Executed.ModelRecordID != "model-primary" {
		t.Fatalf("executed model record id = %q, want model-primary", spec.Executed.ModelRecordID)
	}
	if spec.FallbackConfig == nil || spec.FallbackConfig.ModelRecordID != "model-fallback" {
		t.Fatalf("unexpected fallback config = %#v", spec.FallbackConfig)
	}
	if spec.Requested.Headers["Authorization"] != "***redacted***" {
		t.Fatalf("authorization header = %#v, want redacted", spec.Requested.Headers)
	}
	if spec.Requested.Headers["X-Test"] != "***redacted***" {
		t.Fatalf("x-test header = %#v, want redacted", spec.Requested.Headers)
	}
}

func TestServicePrepareResolvesModelParameters(t *testing.T) {
	service := newTestRuntimeService(t, policy.AllowAll(), &stubTurnExecutor{})
	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-model-policy"}, Input{
		RequestID: "req-model-policy",
		SessionID: "sess-model-policy",
		Query:     "generate workflow plan",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-1",
			TaskType:    runtimetask.InputKindWorkflowStepRequest,
			TaskSubtype: "default_workflow",
			Scene:       "workflow",
			UserGoal:    "generate workflow plan",
			OutputMode:  "workflow_plan",
			InputPayload: map[string]any{
				"model_policy_override": map[string]any{
					"reasoning_mode": "high",
				},
			},
		},
		ModelSelection: &model.Selection{
			Primary: model.ChatConfig{
				ProviderID:       "provider-primary",
				ProviderName:     "Primary Provider",
				ProviderProtocol: "openai_compatible",
				ModelRecordID:    "model-primary",
				ProviderModelID:  "gpt-primary",
				ModelDisplayName: "Primary Model",
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Spec.Model.ResolvedParameters == nil {
		t.Fatalf("expected resolved model parameters")
	}
	if prepared.Spec.Model.ResolvedParameters.PolicyName != "workflow_planning" {
		t.Fatalf("policy_name = %q, want workflow_planning", prepared.Spec.Model.ResolvedParameters.PolicyName)
	}
	if prepared.Spec.Model.PrimaryConfig == nil || prepared.Spec.Model.PrimaryConfig.ResolvedParameters == nil {
		t.Fatalf("expected primary config to carry resolved parameters")
	}
	if got := prepared.Spec.Metadata.Constraints["model_policy"]; got != "workflow_planning" {
		t.Fatalf("model_policy = %#v, want workflow_planning", got)
	}
}

func TestServicePrepareConsumesModelPolicyOverrideFromTaskInputPayload(t *testing.T) {
	service := newTestRuntimeService(t, policy.AllowAll(), &stubTurnExecutor{})
	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-model-override"}, Input{
		RequestID: "req-model-override",
		SessionID: "sess-model-override",
		Query:     "format this strictly",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-override",
			TaskType:    runtimetask.InputKindChat,
			TaskSubtype: "default",
			Scene:       "default",
			UserGoal:    "format this strictly",
			OutputMode:  "text",
			InputPayload: map[string]any{
				"model_policy_override": map[string]any{
					"output_mode": "strict_json",
				},
			},
		},
		ModelSelection: &model.Selection{
			Primary: model.ChatConfig{
				ProviderID:       "provider-primary",
				ProviderName:     "Primary Provider",
				ProviderProtocol: "openai_compatible",
				ModelRecordID:    "model-primary",
				ProviderModelID:  "gpt-primary",
				ModelDisplayName: "Primary Model",
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Spec.Model.ResolvedParameters == nil {
		t.Fatalf("expected resolved parameters")
	}
	if prepared.Spec.Model.ResolvedParameters.ResponseFormat != modelparams.ResponseFormatJSONSchema {
		t.Fatalf("response_format = %q, want json_schema", prepared.Spec.Model.ResolvedParameters.ResponseFormat)
	}
}

func TestServicePrepareAppliesSpecificToolChoiceToAllowedTools(t *testing.T) {
	service := newTestRuntimeService(t, policy.AllowAll(), &stubTurnExecutor{})
	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-specific-tool"}, Input{
		RequestID: "req-specific-tool",
		SessionID: "sess-specific-tool",
		Query:     "look up profile only",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-specific-tool",
			TaskType:    runtimetask.InputKindChat,
			TaskSubtype: "default",
			Scene:       "default",
			UserGoal:    "look up profile only",
			OutputMode:  "text",
			InputPayload: map[string]any{
				"model_policy_override": map[string]any{
					"tool_policy": "specific_tool",
					"tool_name":   "lookup_profile",
				},
			},
		},
		Customization: customization.UserCustomization{
			EnabledTools: []string{"lookup_profile", "lookup_orders"},
		},
		ModelSelection: &model.Selection{
			Primary: model.ChatConfig{
				ProviderID:       "provider-primary",
				ProviderName:     "Primary Provider",
				ProviderProtocol: "openai_compatible",
				ModelRecordID:    "model-primary",
				ProviderModelID:  "gpt-primary",
				ModelDisplayName: "Primary Model",
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if len(prepared.Spec.Tools.AllowedTools) != 1 || prepared.Spec.Tools.AllowedTools[0] != "lookup_profile" {
		t.Fatalf("allowed tools = %#v, want only lookup_profile", prepared.Spec.Tools.AllowedTools)
	}
	if prepared.Spec.Model.ResolvedParameters == nil || prepared.Spec.Model.ResolvedParameters.ToolChoice.ToolName != "lookup_profile" {
		t.Fatalf("resolved tool choice = %#v, want lookup_profile", prepared.Spec.Model.ResolvedParameters)
	}
}

func TestEinoTurnExecutorPrepareUsesRequestedModelWithoutFallback(t *testing.T) {
	provider := &recordingModelProvider{}
	executor := NewEinoTurnExecutor(config.Config{}, provider, map[string]tools.Definition{}, observability.NewNoopManager())
	spec := &ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "user_overview", Guidance: "helpful"},
		Model: buildModelSpec(&model.Selection{
			Primary: model.ChatConfig{
				ProviderID:       "provider-primary",
				ProviderName:     "Primary Provider",
				ProviderProtocol: "openai_compatible",
				ModelRecordID:    "model-primary",
				ProviderModelID:  "gpt-primary",
				ModelDisplayName: "Primary Model",
			},
		}),
	}

	prepared, err := executor.Prepare(context.Background(), RuntimeState{RequestID: "req-model-1", SessionID: "sess-model-1", Turn: 1}, spec, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil || prepared.Runner == nil {
		t.Fatalf("expected runner to be prepared")
	}
	finalMessage := runGraphNativeAgent(t, prepared.Runner, prepared.Messages)
	if finalMessage.Content != "ok" {
		t.Fatalf("prepared runner final content = %q, want ok", finalMessage.Content)
	}
	if len(provider.calls) != 1 {
		t.Fatalf("provider calls = %d, want 1", len(provider.calls))
	}
	if provider.calls[0].ModelRecordID != "model-primary" {
		t.Fatalf("provider primary model = %q, want model-primary", provider.calls[0].ModelRecordID)
	}
	if spec.Model.Executed.ModelRecordID != "model-primary" {
		t.Fatalf("executed model = %q, want model-primary", spec.Model.Executed.ModelRecordID)
	}
	if spec.Model.FallbackUsed {
		t.Fatalf("expected fallback_used=false")
	}
	if spec.Model.ExecutedConfig == nil || spec.Model.ExecutedConfig.ModelRecordID != "model-primary" {
		t.Fatalf("executed config = %#v, want model-primary", spec.Model.ExecutedConfig)
	}
}

func TestEinoTurnExecutorPrepareFallsBackAndPreservesRequestedModel(t *testing.T) {
	provider := &recordingModelProvider{
		fail: map[string]error{"model-primary": context.DeadlineExceeded},
	}
	executor := NewEinoTurnExecutor(config.Config{}, provider, map[string]tools.Definition{}, observability.NewNoopManager())
	spec := &ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "user_overview", Guidance: "helpful"},
		Model: buildModelSpec(&model.Selection{
			Primary: model.ChatConfig{
				ProviderID:       "provider-primary",
				ProviderName:     "Primary Provider",
				ProviderProtocol: "openai_compatible",
				ModelRecordID:    "model-primary",
				ProviderModelID:  "gpt-primary",
				ModelDisplayName: "Primary Model",
			},
			Fallback: &model.ChatConfig{
				ProviderID:       "provider-fallback",
				ProviderName:     "Fallback Provider",
				ProviderProtocol: "anthropic",
				ModelRecordID:    "model-fallback",
				ProviderModelID:  "claude-fallback",
				ModelDisplayName: "Fallback Model",
			},
		}),
	}

	prepared, err := executor.Prepare(context.Background(), RuntimeState{RequestID: "req-model-2", SessionID: "sess-model-2", Turn: 1}, spec, nil)
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil || prepared.Runner == nil {
		t.Fatalf("expected runner to be prepared")
	}
	if len(provider.calls) != 2 {
		t.Fatalf("provider calls = %d, want 2", len(provider.calls))
	}
	if provider.calls[0].ModelRecordID != "model-primary" || provider.calls[1].ModelRecordID != "model-fallback" {
		t.Fatalf("provider calls = %#v, want primary then fallback", provider.calls)
	}
	if spec.Model.Requested.ModelRecordID != "model-primary" {
		t.Fatalf("requested model = %q, want model-primary", spec.Model.Requested.ModelRecordID)
	}
	if spec.Model.Executed.ModelRecordID != "model-fallback" {
		t.Fatalf("executed model = %q, want model-fallback", spec.Model.Executed.ModelRecordID)
	}
	if !spec.Model.FallbackUsed {
		t.Fatalf("expected fallback_used=true")
	}
	if spec.Model.FallbackReason != "primary_model_unavailable" {
		t.Fatalf("fallback reason = %q, want primary_model_unavailable", spec.Model.FallbackReason)
	}
	if spec.Model.Executed.Headers["fallback"] != "***redacted***" {
		t.Fatalf("executed headers = %#v, want redacted fallback marker", spec.Model.Executed.Headers)
	}
	if spec.Model.ExecutedConfig == nil || spec.Model.ExecutedConfig.ModelRecordID != "model-fallback" {
		t.Fatalf("executed config = %#v, want model-fallback", spec.Model.ExecutedConfig)
	}
}

func TestEinoTurnExecutorPrepareFailsClosedWhenRequiredToolHasNoAllowedTools(t *testing.T) {
	provider := &recordingModelProvider{}
	executor := NewEinoTurnExecutor(config.Config{}, provider, map[string]tools.Definition{}, observability.NewNoopManager())
	spec := &ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "user_overview", Guidance: "helpful"},
		Model: ModelSpec{
			Requested: ModelEndpoint{
				ProviderModelID: "gpt-primary",
			},
			Executed: ModelEndpoint{
				ProviderModelID: "gpt-primary",
			},
			PrimaryConfig: &model.ChatConfig{
				ProviderID:       "provider-primary",
				ProviderName:     "Primary Provider",
				ProviderProtocol: "openai_compatible",
				ModelRecordID:    "model-primary",
				ProviderModelID:  "gpt-primary",
				ModelDisplayName: "Primary Model",
			},
			ResolvedParameters: &modelparams.ResolvedModelParameters{
				PolicyName:    "required_tool",
				PolicyVersion: "v1",
				ToolChoice: modelparams.ToolChoice{
					Kind: modelparams.ToolChoiceRequired,
				},
			},
		},
		Tools: ToolSpec{
			AllowedTools: nil,
			Sources:      map[string]string{},
		},
	}

	_, err := executor.Prepare(context.Background(), RuntimeState{RequestID: "req-required-tool", SessionID: "sess-required-tool", Turn: 1}, spec, nil)
	if err == nil {
		t.Fatalf("expected fail-closed error when tool_choice=required has no allowed tools")
	}
}

func TestServicePrepareAllowsDefaultChatWithoutSubjectData(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 15 * time.Minute
	p.MaxWaitTimeout = 2 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-1"}, Input{
		RequestID: "req-1",
		SessionID: "sess-1",
		Query:     "show user order summary",
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial != nil {
		t.Fatalf("expected no initial waiting action for default chat, got %#v", prepared.Initial)
	}
	if !executor.called {
		t.Fatalf("expected turn executor to be called")
	}
	if required, _ := prepared.Spec.Metadata.Constraints["subject_context_required"].(bool); required {
		t.Fatalf("subject_context_required = true, want false for default chat")
	}
}

func TestServicePrepareReturnsInformationRequestWhenExplicitSubjectToolNeedsUserID(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 15 * time.Minute
	p.MaxWaitTimeout = 2 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-1"}, Input{
		RequestID: "req-1",
		SessionID: "sess-1",
		Query:     "show user order summary",
		Customization: customization.UserCustomization{
			EnabledTools: []string{"lookup_profile"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial == nil || prepared.Initial.Action == nil {
		t.Fatalf("expected initial information request action, got %#v", prepared.Initial)
	}
	if missing := prepared.Initial.Action.InformationRequest.Missing; len(missing) != 1 || missing[0].Field != "user_id" {
		t.Fatalf("missing fields = %#v, want user_id", missing)
	}
	if required, _ := prepared.Spec.Metadata.Constraints["subject_context_required"].(bool); !required {
		t.Fatalf("subject_context_required = false, want true")
	}
	if got := prepared.Spec.Metadata.Constraints["subject_context_reason"]; got != "explicit_capability_requires_subject_context" {
		t.Fatalf("subject_context_reason = %#v", got)
	}
	if executor.called {
		t.Fatalf("turn executor should not be called when waiting for information")
	}
}

func TestServicePrepareUsesSupplementAndTimeoutOverride(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 30 * time.Minute
	p.MaxWaitTimeout = 2 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-2"}, Input{
		RequestID: "req-2",
		SessionID: "sess-2",
		Query:     "show user order summary",
		Supplement: &SupplementPayload{
			Data: map[string]string{"user_id": "u1001"},
		},
		TimeoutOverride: 5 * time.Minute,
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if prepared.Initial != nil {
		t.Fatalf("expected no initial action, got %#v", prepared.Initial)
	}
	if !executor.called {
		t.Fatalf("expected turn executor to be called")
	}
	if got := prepared.Spec.Processing.WaitPolicy.TimeoutAfter; got != 5*time.Minute {
		t.Fatalf("wait policy timeout = %s, want %s", got, 5*time.Minute)
	}
	if got := prepared.Spec.Skill.PrimarySkill; got != "user_overview" {
		t.Fatalf("primary skill = %q, want user_overview", got)
	}
	if got := prepared.Spec.Metadata.Constraints["task_kind"]; got != runtimetask.InputKindChat {
		t.Fatalf("task_kind = %#v, want %q", got, runtimetask.InputKindChat)
	}
	if got := prepared.Spec.Metadata.Constraints["task_type"]; got != runtimetask.InputKindChat {
		t.Fatalf("task_type = %#v, want %q", got, runtimetask.InputKindChat)
	}
	if got := prepared.Spec.Metadata.Constraints["task_subtype"]; got != "default" {
		t.Fatalf("task_subtype = %#v, want default", got)
	}
	if got := prepared.Spec.Metadata.Constraints["scene"]; got != "default" {
		t.Fatalf("scene = %#v, want default", got)
	}
	if len(prepared.Spec.Tools.AllowedTools) != 0 {
		t.Fatalf("allowed tools len = %d, want 0 for context-only user_overview", len(prepared.Spec.Tools.AllowedTools))
	}
	if prepared.Spec.Metadata.Capability == nil {
		t.Fatalf("expected capability contract metadata")
	}
	if got := prepared.Spec.Metadata.Capability.RuntimeConsumption.ContractName; got != "ExecutionSpec" {
		t.Fatalf("runtime contract name = %q, want ExecutionSpec", got)
	}
	if len(prepared.Spec.Metadata.Capability.GovernedState.Skills) == 0 {
		t.Fatalf("expected governed skills in capability contract")
	}
	if prepared.Spec.Skill.Guidance == "" {
		t.Fatalf("expected synthesized guidance")
	}
	if len(prepared.Messages) == 0 {
		t.Fatalf("expected assembled messages")
	}
	if prepared.StructuredOutput == nil || !prepared.StructuredOutput.Requested {
		t.Fatalf("expected structured output contract")
	}
	if prepared.StructuredOutput.Emitted {
		t.Fatalf("expected first pass to keep text fallback, got %#v", prepared.StructuredOutput)
	}
	if prepared.StructuredOutput.ContractID != "structured-output.v1" {
		t.Fatalf("contract_id = %q, want structured-output.v1", prepared.StructuredOutput.ContractID)
	}
	if prepared.StructuredOutput.SchemaName != "structured_output" {
		t.Fatalf("schema_name = %q, want structured_output", prepared.StructuredOutput.SchemaName)
	}
	if prepared.StructuredOutput.FallbackReason != "text_stream_only" {
		t.Fatalf("fallback_reason = %q, want text_stream_only", prepared.StructuredOutput.FallbackReason)
	}
	if prepared.Governance == nil || prepared.Governance.Decision != GovernanceDecisionAllow {
		t.Fatalf("governance = %#v, want allow", prepared.Governance)
	}
}

func TestServicePrepareRecordsStructuredOutputAudit(t *testing.T) {
	p := policy.AllowAll()
	executor := &stubTurnExecutor{}
	manager := observability.NewDefaultManager()
	service := NewService(
		config.Config{Runtime: config.RuntimeConfig{MaxConcurrentRequests: 2, MaxConcurrentTools: 2, RequestTimeoutSeconds: 30}},
		p,
		memory.DefaultContextPolicy(),
		skills.BuiltinRegistry(),
		skills.NewAdapter(),
		mustDemoToolDefinitions(t),
		executor,
		manager,
	)

	_, err := service.Prepare(context.Background(), &session.Session{ID: "sess-audit"}, Input{
		RequestID: "req-audit",
		SessionID: "sess-audit",
		Query:     "show user profile",
		Supplement: &SupplementPayload{
			Data: map[string]string{"user_id": "u1001"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	audits := manager.SnapshotAudits()
	foundPrepared := false
	foundEmission := false
	for _, record := range audits {
		if record.Action == "execution_spec_prepared" {
			if _, ok := record.Detail["structured_output"]; !ok {
				t.Fatalf("expected structured_output detail in execution_spec_prepared audit: %#v", record.Detail)
			}
			foundPrepared = true
		}
		if record.Action == "structured_output_emission_planned" {
			foundEmission = true
		}
	}
	if !foundPrepared || !foundEmission {
		t.Fatalf("expected structured output audits, got %#v", audits)
	}
}

func TestServicePrepareCarriesEffectiveContextAssetViews(t *testing.T) {
	p := policy.AllowAll()
	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-effective"}, Input{
		RequestID: "req-effective",
		SessionID: "sess-effective",
		Query:     "请按当前 persona 和规则解释输出边界",
		Task: &runtimetask.RuntimeTask{
			UserGoal: "请按当前 persona 和规则解释输出边界",
			TaskType: "chat",
			Scene:    "default",
			GlobalContext: map[string]any{
				"context_assets": []any{
					map[string]any{
						"asset_id":   "persona.security",
						"asset_type": "persona",
						"priority":   100,
						"content": map[string]any{
							"summary":      "直接、严格、证据优先",
							"bottom_lines": []string{"不虚构"},
						},
					},
					map[string]any{
						"asset_id":   "policy_rule.core.boundary",
						"asset_type": "policy_rule",
						"priority":   90,
						"content": map[string]any{
							"title":          "Boundary Rule",
							"guidance_lines": []string{"先核对事实"},
							"hard_gates":     []string{"禁止跨租户泄漏"},
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	constraints := prepared.Spec.Metadata.Constraints
	if constraints == nil {
		t.Fatalf("expected metadata constraints")
	}
	persona, _ := constraints["effective_persona"].(map[string]any)
	if got, _ := persona["summary"].(string); strings.TrimSpace(got) != "直接、严格、证据优先" {
		t.Fatalf("effective_persona.summary = %q, want 直接、严格、证据优先", got)
	}
	rules, _ := constraints["effective_policy_rules"].([]map[string]any)
	if len(rules) == 0 {
		if raw, ok := constraints["effective_policy_rules"].([]any); ok {
			for _, item := range raw {
				if typed, ok := item.(map[string]any); ok {
					rules = append(rules, typed)
				}
			}
		}
	}
	if len(rules) == 0 {
		t.Fatalf("effective_policy_rules = %#v, want non-empty", constraints["effective_policy_rules"])
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Effective persona is active") {
		t.Fatalf("guidance = %q, want effective view guidance", prepared.Spec.Skill.Guidance)
	}
}

func runtimeContainsString(items []string, target string) bool {
	for _, item := range items {
		if strings.TrimSpace(item) == strings.TrimSpace(target) {
			return true
		}
	}
	return false
}

func mustDemoToolDefinitions(tb testing.TB) map[string]tools.Definition {
	tb.Helper()
	toolDefs, err := tools.DemoDefinitions()
	if err != nil {
		tb.Fatalf("DemoDefinitions() error = %v", err)
	}
	return toolDefs
}

func TestCapabilityContractPreservesRequestedAndGovernedLayers(t *testing.T) {
	p := policy.AllowAll()
	p.AllowedSkills = map[string]bool{"user_overview": true}
	p.AllowedTools = map[string]bool{"lookup_profile": true}

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-contract"}, Input{
		RequestID: "req-contract",
		SessionID: "sess-contract",
		Query:     "show user profile",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview", "nonexistent"},
		},
		Supplement: &SupplementPayload{
			Data: map[string]string{"user_id": "u1001"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	contract := prepared.Spec.Metadata.Capability
	if contract == nil {
		t.Fatalf("expected capability contract metadata")
	}
	if len(contract.Declarations.RequestedSkills) != 2 {
		t.Fatalf("requested skills = %#v, want 2 items", contract.Declarations.RequestedSkills)
	}
	if len(contract.GovernedState.Skills) != 1 || contract.GovernedState.Skills[0] != "user_overview" {
		t.Fatalf("governed skills = %#v, want [user_overview]", contract.GovernedState.Skills)
	}
	if len(contract.GovernedState.Tools) != 0 {
		t.Fatalf("governed tools = %#v, want no implicit demo tools", contract.GovernedState.Tools)
	}
}

func TestCapabilityContractExcludesRegistryMissingButPolicyAllowedSkills(t *testing.T) {
	p := policy.AllowAll()
	p.AllowedSkills = map[string]bool{
		"user_overview": true,
		"ghost_skill":   true,
	}
	p.AllowedTools = map[string]bool{"lookup_profile": true}

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-contract-missing"}, Input{
		RequestID: "req-contract-missing",
		SessionID: "sess-contract-missing",
		Query:     "show user profile",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview", "ghost_skill"},
		},
		Supplement: &SupplementPayload{
			Data: map[string]string{"user_id": "u1001"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	contract := prepared.Spec.Metadata.Capability
	if contract == nil {
		t.Fatalf("expected capability contract metadata")
	}
	if len(contract.Declarations.RequestedSkills) != 2 {
		t.Fatalf("requested skills = %#v, want 2 items", contract.Declarations.RequestedSkills)
	}
	if len(contract.GovernedState.Skills) != 1 || contract.GovernedState.Skills[0] != "user_overview" {
		t.Fatalf("governed skills = %#v, want only registry-resolvable skills", contract.GovernedState.Skills)
	}
}

func TestServicePreparePrefersNormalizedTaskGoal(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-task-1"}, Input{
		RequestID: "req-task-1",
		SessionID: "sess-task-1",
		Query:     "legacy raw query",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-1",
			TaskType:    runtimetask.InputKindInspectionTask,
			TaskSubtype: "manual_inspection",
			InputKind:   runtimetask.InputKindInspectionTask,
			Scene:       "inspection",
			WorkspaceID: "ws-1",
			UserGoal:    "analyze the current user's risk posture",
			KnownFacts: map[string]string{
				"user_id": "u1001",
			},
			OutputMode: runtimetask.DefaultOutputModeText,
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial != nil {
		t.Fatalf("expected no initial action, got %#v", prepared.Initial)
	}
	if prepared.Spec.Inference.Goal != "analyze the current user's risk posture" {
		t.Fatalf("Inference.Goal = %q", prepared.Spec.Inference.Goal)
	}
	if len(prepared.Messages) == 0 {
		t.Fatalf("expected assembled messages")
	}
	lastMessage := prepared.Messages[len(prepared.Messages)-1]
	if lastMessage.Content != "analyze the current user's risk posture" {
		t.Fatalf("final message content = %q", lastMessage.Content)
	}
	if got := prepared.Spec.Metadata.Constraints["task_id"]; got != "task-1" {
		t.Fatalf("task_id = %#v, want task-1", got)
	}
	if got := prepared.Spec.Metadata.Constraints["task_type"]; got != "inspection_task" {
		t.Fatalf("task_type = %#v, want inspection_task", got)
	}
	if got := prepared.Spec.Metadata.Constraints["task_subtype"]; got != "manual_inspection" {
		t.Fatalf("task_subtype = %#v, want manual_inspection", got)
	}
	if got := prepared.Spec.Metadata.Constraints["scene"]; got != "inspection" {
		t.Fatalf("scene = %#v, want inspection", got)
	}
	if got := prepared.Spec.Metadata.Constraints["workspace_id"]; got != "ws-1" {
		t.Fatalf("workspace_id = %#v, want ws-1", got)
	}
	if executor.called != true {
		t.Fatalf("expected turn executor to be called")
	}
}

func TestServicePrepareUsesAppSkillBundleAndPersona(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-app-1"}, Input{
		RequestID: "req-app-1",
		SessionID: "sess-app-1",
		Query:     "show user profile",
		Task: &runtimetask.RuntimeTask{
			TaskID:        "task-app-1",
			TaskType:      runtimetask.InputKindChat,
			TaskSubtype:   "app_dialogue",
			InputKind:     runtimetask.InputKindChat,
			Scene:         "application_dialogue",
			AppInstanceID: "app-inst-1",
			UserLanguage:  "en-US",
			UserGoal:      "show user profile",
			AppContext: map[string]any{
				"persona":         "security expert",
				"skills":          []any{"user_overview"},
				"guide_questions": []any{"Q1", "Q2"},
			},
			GlobalContext: map[string]any{
				"user_context": map[string]any{"display_name": "User One"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Spec.Skill.PrimarySkill != "user_overview" {
		t.Fatalf("PrimarySkill = %q, want user_overview", prepared.Spec.Skill.PrimarySkill)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Current app persona: security expert") {
		t.Fatalf("Guidance = %q, want persona", prepared.Spec.Skill.Guidance)
	}
	if got := prepared.Spec.Metadata.Constraints["guide_questions"]; got == nil {
		t.Fatalf("guide_questions should be present in constraints")
	}
}

func TestServicePrepareUsesBuiltinSecurityReviewSkill(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-cso-1"}, Input{
		RequestID: "req-cso-1",
		SessionID: "sess-cso-1",
		Query:     "请做一次 CSO 安全审计并分析供应链风险",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-cso-1",
			TaskType:    runtimetask.InputKindChat,
			TaskSubtype: "security_review",
			InputKind:   runtimetask.InputKindChat,
			Scene:       "security_review",
			UserGoal:    "请做一次 CSO 安全审计并分析供应链风险",
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Spec.Skill.PrimarySkill != "cso_review" {
		t.Fatalf("PrimarySkill = %q, want cso_review", prepared.Spec.Skill.PrimarySkill)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "CSO-style security review") {
		t.Fatalf("Guidance = %q, want security review guidance", prepared.Spec.Skill.Guidance)
	}
}

func TestServicePrepareInfersSecurityReviewSkillFromQuery(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-cso-2"}, Input{
		RequestID: "req-cso-2",
		SessionID: "sess-cso-2",
		Query:     "请做一次 CSO 安全审计并分析供应链风险",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-cso-2",
			TaskType:    runtimetask.InputKindChat,
			TaskSubtype: "default",
			InputKind:   runtimetask.InputKindChat,
			Scene:       "",
			UserGoal:    "请做一次 CSO 安全审计并分析供应链风险",
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Spec.Skill.PrimarySkill != "cso_review" {
		t.Fatalf("PrimarySkill = %q, want cso_review", prepared.Spec.Skill.PrimarySkill)
	}
}

func TestServicePrepareUsesGlobalPersonaContextWithoutReplacingAppPersona(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-persona-1"}, Input{
		RequestID: "req-persona-1",
		SessionID: "sess-persona-1",
		Query:     "analyze the current risk posture",
		Task: &runtimetask.RuntimeTask{
			TaskID:       "task-persona-1",
			TaskType:     runtimetask.InputKindChat,
			TaskSubtype:  "default",
			InputKind:    runtimetask.InputKindChat,
			Scene:        "default",
			UserLanguage: "zh-CN",
			UserGoal:     "analyze the current risk posture",
			AppContext:   map[string]any{"persona": "security expert"},
			GlobalContext: map[string]any{"persona_context": map[string]any{
				"id":             "persona_serious",
				"name":           "严肃",
				"description":    "用词严谨，重数据和证据",
				"style_rules":    []any{"优先给出依据和边界", "不要夸张渲染风险"},
				"example_dialog": "用户:这个风险严重吗？\n墨思:依据当前证据，这个问题属于高风险。",
			}},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Current app persona: security expert") {
		t.Fatalf("Guidance = %q, want app persona", prepared.Spec.Skill.Guidance)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Current user persona id: persona_serious") {
		t.Fatalf("Guidance = %q, want persona id", prepared.Spec.Skill.Guidance)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Current user persona description: 用词严谨，重数据和证据") {
		t.Fatalf("Guidance = %q, want persona description", prepared.Spec.Skill.Guidance)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "must not change facts") {
		t.Fatalf("Guidance = %q, want fact boundary", prepared.Spec.Skill.Guidance)
	}
	if got := prepared.Spec.Metadata.Constraints["persona_id"]; got != "persona_serious" {
		t.Fatalf("persona_id = %#v, want persona_serious", got)
	}
	if got := prepared.Spec.Metadata.Constraints["persona_name"]; got != "严肃" {
		t.Fatalf("persona_name = %#v, want 严肃", got)
	}
}

func TestServicePrepareUsesPlatformContextSummariesInGuidance(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-platform-context-1"}, Input{
		RequestID: "req-platform-context-1",
		SessionID: "sess-platform-context-1",
		Query:     "结合我的身份和长期记忆，介绍一下你目前如何理解我。",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-platform-context-1",
			TaskType:    runtimetask.InputKindChat,
			TaskSubtype: "default",
			InputKind:   runtimetask.InputKindChat,
			Scene:       "default",
			UserGoal:    "结合我的身份和长期记忆，介绍一下你目前如何理解我。",
			GlobalContext: map[string]any{
				"platform_context_catalog": map[string]any{
					"identity": map[string]any{"available": true},
					"memory":   map[string]any{"available": true},
					"persona":  map[string]any{"available": true},
				},
				"identity_summary": map[string]any{"summary": "安全负责人，关注供应链治理"},
				"memory_summary":   map[string]any{"summary": "偏好结构化回答和长期连续性"},
				"persona_summary":  map[string]any{"summary": "用词严谨，重依据"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Relevant platform identity summary: 安全负责人，关注供应链治理") {
		t.Fatalf("Guidance = %q, want identity summary", prepared.Spec.Skill.Guidance)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Relevant platform memory summary: 偏好结构化回答和长期连续性") {
		t.Fatalf("Guidance = %q, want memory summary", prepared.Spec.Skill.Guidance)
	}
	usedContexts, ok := prepared.Spec.Metadata.Constraints["used_contexts"].([]string)
	if !ok || len(usedContexts) < 2 {
		t.Fatalf("used_contexts = %#v", prepared.Spec.Metadata.Constraints["used_contexts"])
	}
}

func TestServicePreparePrefersPlatformContextDetailsInGuidance(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-platform-detail-1"}, Input{
		RequestID: "req-platform-detail-1",
		SessionID: "sess-platform-detail-1",
		Query:     "结合我的知识详情，给我一个近期供应链安全关注点总结。",
		Task: &runtimetask.RuntimeTask{
			TaskID:      "task-platform-detail-1",
			TaskType:    runtimetask.InputKindChat,
			TaskSubtype: "security_review",
			InputKind:   runtimetask.InputKindChat,
			Scene:       "security_review",
			UserGoal:    "结合我的知识详情，给我一个近期供应链安全关注点总结。",
			GlobalContext: map[string]any{
				"platform_context_catalog": map[string]any{
					"knowledge": map[string]any{"available": true},
				},
				"knowledge_summary": map[string]any{"summary": "供应链"},
				"knowledge_detail":  map[string]any{"summary": "近期重点关注供应链安全和依赖风险"},
			},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !strings.Contains(prepared.Spec.Skill.Guidance, "Relevant platform knowledge detail: 近期重点关注供应链安全和依赖风险") {
		t.Fatalf("Guidance = %q, want knowledge detail", prepared.Spec.Skill.Guidance)
	}
}

func TestServicePrepareSkipsChatSupplementWaitForInspectionTask(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-inspect-1"}, Input{
		RequestID: "req-inspect-1",
		SessionID: "sess-inspect-1",
		Task: &runtimetask.RuntimeTask{
			TaskID:                "task-inspect-1",
			TaskType:              runtimetask.InputKindInspectionTask,
			TaskSubtype:           "manual_inspection",
			InputKind:             runtimetask.InputKindInspectionTask,
			Scene:                 "inspection",
			WorkspaceID:           "ws-1",
			MainSessionID:         "sess-inspect-1",
			IntegrationInstanceID: "integration-1",
			TriggerType:           "manual_inspection",
			UserGoal:              "inspect current workspace risk",
			OutputMode:            runtimetask.DefaultOutputModeText,
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial != nil {
		t.Fatalf("expected no initial waiting action for inspection task, got %#v", prepared.Initial)
	}
	if !executor.called {
		t.Fatalf("expected turn executor to be called")
	}
}

func TestServicePrepareCapsTimeoutOverrideAtPolicyMax(t *testing.T) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-3"}, Input{
		RequestID:       "req-3",
		SessionID:       "sess-3",
		Query:           "show user risk flags",
		Customization:   customization.UserCustomization{EnabledSkills: []string{"user_overview"}},
		TimeoutOverride: 3 * time.Hour,
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}

	if prepared.Initial == nil || prepared.Initial.WaitState == nil {
		t.Fatalf("expected waiting result")
	}
	if got := prepared.Initial.WaitState.TimeoutAfter; got != 10*time.Minute {
		t.Fatalf("timeout_after = %s, want %s", got, 10*time.Minute)
	}
}

func TestServicePrepareDegradesAfterSupplementTimeout(t *testing.T) {
	p := policy.AllowAll()
	p.AllowDegradeWithoutResponse = true
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{
		ID: "sess-4",
		Pending: &session.PendingState{
			Stage:       string(StageCapabilityResolution),
			Status:      string(RequestStatusWaitingForInformation),
			ResumeToken: "resume-1",
			TimeoutAt:   time.Now().Add(-time.Minute),
		},
	}, Input{
		RequestID: "req-4",
		SessionID: "sess-4",
		Query:     "show user profile",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
		Pending: &session.PendingState{
			Stage:       string(StageCapabilityResolution),
			Status:      string(RequestStatusWaitingForInformation),
			ResumeToken: "resume-1",
			TimeoutAt:   time.Now().Add(-time.Minute),
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial != nil {
		t.Fatalf("expected degraded execution to continue without initial action, got %#v", prepared.Initial)
	}
	if !executor.called {
		t.Fatalf("expected turn executor to run after degrade")
	}
	if got := prepared.Spec.Metadata.Constraints["degraded"]; got != true {
		t.Fatalf("expected degraded=true constraint, got %#v", got)
	}
	if got := prepared.Spec.Metadata.Constraints["degrade_reason"]; got != string(SupplementOutcomeTimeoutExpired) {
		t.Fatalf("degrade_reason = %#v, want %q", got, SupplementOutcomeTimeoutExpired)
	}
	if prepared.Governance == nil || prepared.Governance.Decision != GovernanceDecisionDegrade {
		t.Fatalf("governance = %#v, want degrade", prepared.Governance)
	}
	if prepared.Spec.Metadata.Orchestration == nil || prepared.Spec.Metadata.Orchestration.CurrentState != OrchestrationStateDegraded {
		t.Fatalf("orchestration = %#v, want degraded", prepared.Spec.Metadata.Orchestration)
	}
}

func TestServicePrepareReturnsTimedOutWhenDegradeDisabled(t *testing.T) {
	p := policy.AllowAll()
	p.AllowDegradeWithoutResponse = false
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	pending := &session.PendingState{
		Stage:       string(StageCapabilityResolution),
		Status:      string(RequestStatusWaitingForInformation),
		ResumeToken: "resume-2",
		TimeoutAt:   time.Now().Add(-time.Minute),
	}
	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-5", Pending: pending}, Input{
		RequestID: "req-5",
		SessionID: "sess-5",
		Query:     "show user orders",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
		Pending: pending,
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial == nil || prepared.Initial.Error == "" {
		t.Fatalf("expected initial timeout error, got %#v", prepared.Initial)
	}
	if got := prepared.InitialStatus; got != RequestStatusTimedOut {
		t.Fatalf("initial status = %q, want %q", got, RequestStatusTimedOut)
	}
	if prepared.InitialError == nil {
		t.Fatalf("expected structured initial error")
	}
	if prepared.InitialError.Code != string(RequestStatusTimedOut) || prepared.InitialError.Reason != "timeout_closed" {
		t.Fatalf("unexpected initial error = %#v", prepared.InitialError)
	}
	if prepared.Governance == nil || prepared.Governance.Decision != GovernanceDecisionDeny {
		t.Fatalf("governance = %#v, want deny", prepared.Governance)
	}
	if prepared.Spec == nil || prepared.Spec.Metadata.Orchestration == nil || prepared.Spec.Metadata.Orchestration.CurrentState != OrchestrationStateAborted {
		t.Fatalf("orchestration = %#v, want aborted", prepared.Spec)
	}
	if executor.called {
		t.Fatalf("turn executor should not run when timeout finishes the request")
	}
}

func TestServicePrepareAbandonAndContinueRunsMainline(t *testing.T) {
	p := policy.AllowAll()
	p.AllowDegradeWithoutResponse = false
	p.DefaultWaitTimeout = 10 * time.Minute
	p.MaxWaitTimeout = 1 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	pending := &session.PendingState{
		Stage:       string(StageCapabilityResolution),
		Status:      string(RequestStatusWaitingForInformation),
		ResumeToken: "resume-3",
		TimeoutAt:   time.Now().Add(time.Minute),
	}
	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-6", Pending: pending}, Input{
		RequestID: "req-6",
		SessionID: "sess-6",
		Query:     "show user profile",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
		Pending: pending,
		Supplement: &SupplementPayload{
			Outcome: SupplementOutcomeAbandonAndContinue,
			Resume:  &ResumeContext{ResumeToken: "resume-3"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial != nil {
		t.Fatalf("expected degraded execution to continue, got %#v", prepared.Initial)
	}
	if !executor.called {
		t.Fatalf("expected executor to run after explicit abandon-and-continue")
	}
	if got := prepared.Spec.Metadata.Constraints["degrade_reason"]; got != string(SupplementOutcomeAbandonAndContinue) {
		t.Fatalf("degrade_reason = %#v, want %q", got, SupplementOutcomeAbandonAndContinue)
	}
	if prepared.Governance == nil || prepared.Governance.Decision != GovernanceDecisionDegrade {
		t.Fatalf("governance = %#v, want degrade", prepared.Governance)
	}
	if prepared.Spec.Metadata.Orchestration == nil || prepared.Spec.Metadata.Orchestration.CurrentState != OrchestrationStateDegraded {
		t.Fatalf("orchestration = %#v, want degraded", prepared.Spec.Metadata.Orchestration)
	}
}

func TestServicePrepareReturnsPendingHumanAction(t *testing.T) {
	p := policy.AllowAll()
	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	pending := &session.PendingState{
		Stage:       string(StageCapabilityResolution),
		Status:      string(RequestStatusWaitingForInformation),
		ResumeToken: "resume-4",
		TimeoutAt:   time.Now().Add(time.Minute),
	}
	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-7", Pending: pending}, Input{
		RequestID: "req-7",
		SessionID: "sess-7",
		Query:     "show user profile",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
		Pending: pending,
		Supplement: &SupplementPayload{
			Outcome: SupplementOutcomePendingHuman,
			Resume:  &ResumeContext{ResumeToken: "resume-4"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial == nil || prepared.Initial.Action == nil {
		t.Fatalf("expected pending_human action, got %#v", prepared.Initial)
	}
	if got := prepared.Initial.Action.Type; got != ActionTypePendingHuman {
		t.Fatalf("action type = %q, want %q", got, ActionTypePendingHuman)
	}
	if prepared.Initial.Action.PendingHuman == nil {
		t.Fatalf("expected pending human payload")
	}
	if got := prepared.InitialStatus; got != RequestStatusPendingHuman {
		t.Fatalf("initial status = %q, want %q", got, RequestStatusPendingHuman)
	}
	if prepared.Governance == nil || prepared.Governance.Decision != GovernanceDecisionPendingHuman {
		t.Fatalf("governance = %#v, want pending_human", prepared.Governance)
	}
	if prepared.Spec == nil || prepared.Spec.Metadata.Orchestration == nil || prepared.Spec.Metadata.Orchestration.CurrentState != OrchestrationStateAborted {
		t.Fatalf("orchestration = %#v, want aborted", prepared.Spec)
	}
	if executor.called {
		t.Fatalf("executor should not run when pending_human is requested")
	}
}

func TestServicePrepareConsumesPreservedContextOnResumeWhenQueryEmpty(t *testing.T) {
	p := policy.AllowAll()
	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	pending := &session.PendingState{
		Stage:       string(StageCapabilityResolution),
		Status:      string(RequestStatusWaitingForInformation),
		ResumeToken: "resume-ctx",
		TimeoutAt:   time.Now().Add(time.Minute),
		Preserved: &session.PreservedContext{
			Goal:           "show user profile",
			LastUserIntent: "show user profile",
			Facts: map[string]string{
				"case_id": "case-ctx-1",
			},
		},
	}

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-ctx", Pending: pending}, Input{
		RequestID: "req-ctx",
		SessionID: "sess-ctx",
		Pending:   pending,
		Supplement: &SupplementPayload{
			Outcome: SupplementOutcomeProvided,
			Data:    map[string]string{"user_id": "u1001"},
			Resume:  &ResumeContext{ResumeToken: "resume-ctx"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if !executor.called {
		t.Fatalf("expected executor to run")
	}
	if prepared.Spec.Inference.Goal != "show user profile" {
		t.Fatalf("inference goal = %q, want preserved goal", prepared.Spec.Inference.Goal)
	}
	if prepared.Spec.Metadata.PreservedContext == nil {
		t.Fatalf("expected preserved context metadata")
	}
	if prepared.Spec.Metadata.PreservedContext.Facts["case_id"] != "case-ctx-1" {
		t.Fatalf("preserved facts = %#v, want case_id", prepared.Spec.Metadata.PreservedContext.Facts)
	}
	if prepared.Spec.Metadata.Orchestration == nil || prepared.Spec.Metadata.Orchestration.CurrentState != OrchestrationStateResumed {
		t.Fatalf("orchestration = %#v, want resumed", prepared.Spec.Metadata.Orchestration)
	}
	if len(prepared.Messages) == 0 || !strings.Contains(prepared.Messages[0].Content, "Preserved continuity context") {
		t.Fatalf("expected preserved continuity message, got %#v", prepared.Messages)
	}
}

func TestServicePrepareReturnsPolicyRejectedWhenSupplementDisabled(t *testing.T) {
	p := policy.AllowAll()
	p.AllowSupplementRequests = false
	p.AllowDegradeWithoutResponse = false

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(t, p, executor)

	prepared, err := service.Prepare(context.Background(), &session.Session{ID: "sess-policy"}, Input{
		RequestID: "req-policy",
		SessionID: "sess-policy",
		Query:     "show user profile",
		Customization: customization.UserCustomization{
			EnabledSkills: []string{"user_overview"},
		},
	})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared.Initial == nil || prepared.Initial.Kind != TurnResultError {
		t.Fatalf("expected initial error, got %#v", prepared.Initial)
	}
	if prepared.InitialStatus != RequestStatusPolicyRejected {
		t.Fatalf("initial status = %q, want %q", prepared.InitialStatus, RequestStatusPolicyRejected)
	}
	if prepared.InitialError == nil {
		t.Fatalf("expected structured initial error")
	}
	if prepared.InitialError.Code != string(RequestStatusPolicyRejected) || prepared.InitialError.Reason != "supplement_not_allowed" {
		t.Fatalf("unexpected initial error = %#v", prepared.InitialError)
	}
	if prepared.InitialError.ClientAction != "stop_and_surface_error" {
		t.Fatalf("client action = %q, want stop_and_surface_error", prepared.InitialError.ClientAction)
	}
	if prepared.Governance == nil || prepared.Governance.Decision != GovernanceDecisionDeny {
		t.Fatalf("governance = %#v, want deny", prepared.Governance)
	}
}

func BenchmarkServicePrepareInformationRequest(b *testing.B) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 30 * time.Minute
	p.MaxWaitTimeout = 2 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(b, p, executor)
	sess := &session.Session{ID: "bench-sess-1"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Prepare(context.Background(), sess, Input{
			RequestID: "bench-req-1",
			SessionID: "bench-sess-1",
			Query:     "show user order summary",
		})
		if err != nil {
			b.Fatalf("Prepare() error = %v", err)
		}
	}
}

func BenchmarkServicePrepareResolvedExecution(b *testing.B) {
	p := policy.AllowAll()
	p.DefaultWaitTimeout = 30 * time.Minute
	p.MaxWaitTimeout = 2 * time.Hour

	executor := &stubTurnExecutor{}
	service := newTestRuntimeService(b, p, executor)
	sess := &session.Session{ID: "bench-sess-2"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := service.Prepare(context.Background(), sess, Input{
			RequestID: "bench-req-2",
			SessionID: "bench-sess-2",
			Query:     "show user order summary",
			Supplement: &SupplementPayload{
				Data: map[string]string{"user_id": "u1001"},
			},
			TimeoutOverride: 5 * time.Minute,
		})
		if err != nil {
			b.Fatalf("Prepare() error = %v", err)
		}
	}
}
