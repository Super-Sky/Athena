package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/cloudwego/eino/adk"
	einomodel "github.com/cloudwego/eino/components/model"
	einotool "github.com/cloudwego/eino/components/tool"
	einoschema "github.com/cloudwego/eino/schema"
)

func TestEinoGraphNativeAgentRunsToolsNodeLoopWithState(t *testing.T) {
	ctx := context.Background()
	model := &graphNativeRecordingModel{
		generate: func(call int, messages []*einoschema.Message) (*einoschema.Message, error) {
			switch call {
			case 1:
				return einoschema.AssistantMessage("", []einoschema.ToolCall{
					{
						ID:   "call_lookup",
						Type: "function",
						Function: einoschema.FunctionCall{
							Name:      "lookup",
							Arguments: `{"query":"athena"}`,
						},
					},
				}), nil
			case 2:
				if !hasMessageRole(messages, einoschema.System) {
					t.Fatalf("second model input is missing system instruction: %#v", messageRoles(messages))
				}
				if !hasMessageRole(messages, einoschema.User) {
					t.Fatalf("second model input is missing original user message: %#v", messageRoles(messages))
				}
				if !hasAssistantToolCall(messages, "call_lookup") {
					t.Fatalf("second model input is missing assistant tool call: %#v", messageRoles(messages))
				}
				toolMessage := findToolMessage(messages, "call_lookup")
				if toolMessage == nil {
					t.Fatalf("second model input is missing tool result: %#v", messageRoles(messages))
				}
				if toolMessage.ToolName != "lookup" {
					t.Fatalf("tool message tool name = %q, want lookup", toolMessage.ToolName)
				}
				if !strings.Contains(toolMessage.Content, `"answer":"ok"`) {
					t.Fatalf("tool message content = %q, want lookup result", toolMessage.Content)
				}
				message := einoschema.AssistantMessage("tool result observed", nil)
				message.ResponseMeta = &einoschema.ResponseMeta{
					Usage: &einoschema.TokenUsage{
						PromptTokens:     7,
						CompletionTokens: 3,
						TotalTokens:      10,
					},
				}
				return message, nil
			default:
				return nil, fmt.Errorf("unexpected generate call %d", call)
			}
		},
	}
	lookupTool := &graphNativeLookupTool{}
	recorder := NewRuntimeCallbackRecorder(RuntimeCallbackRecorderConfig{
		ProviderName: "Test Provider",
		ModelName:    "test-model",
	})
	agent, err := NewEinoGraphNativeAgent(ctx, EinoGraphNativeAgentConfig{
		Name:        "test-agent",
		Instruction: "Use tools when needed.",
		Model:       model,
		Tools:       []einotool.BaseTool{lookupTool},
		Callbacks:   recorder.Handler(),
	})
	if err != nil {
		t.Fatalf("NewEinoGraphNativeAgent() error = %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	finalMessage := runGraphNativeAgent(t, runner, []*einoschema.Message{einoschema.UserMessage("lookup athena")})

	if finalMessage.Content != "tool result observed" {
		t.Fatalf("final message content = %q, want tool result observed", finalMessage.Content)
	}
	if model.withToolsCalls != 1 {
		t.Fatalf("WithTools calls = %d, want 1", model.withToolsCalls)
	}
	if model.generateCalls != 2 {
		t.Fatalf("Generate calls = %d, want 2", model.generateCalls)
	}
	if lookupTool.calls != 1 {
		t.Fatalf("lookup tool calls = %d, want 1", lookupTool.calls)
	}
	callbackEvents := recorder.Events()
	if !hasRuntimeCallback(callbackEvents, runtimeCallbackComponentModel) || !hasRuntimeCallback(callbackEvents, runtimeCallbackComponentTool) {
		t.Fatalf("callback events = %#v, want model and tool callbacks", callbackEvents)
	}
	if !hasModelTokenUsage(callbackEvents, 10) {
		t.Fatalf("callback events = %#v, want model token usage", callbackEvents)
	}
}

func TestEinoGraphNativeAgentRunsModelOnlyPath(t *testing.T) {
	ctx := context.Background()
	model := &graphNativeRecordingModel{
		generate: func(call int, messages []*einoschema.Message) (*einoschema.Message, error) {
			if call != 1 {
				return nil, fmt.Errorf("unexpected generate call %d", call)
			}
			if len(messages) != 2 {
				t.Fatalf("model input length = %d, want system + user", len(messages))
			}
			if messages[0].Role != einoschema.System || messages[0].Content != "Be concise." {
				t.Fatalf("first message = %#v, want system instruction", messages[0])
			}
			if messages[1].Role != einoschema.User || messages[1].Content != "hello" {
				t.Fatalf("second message = %#v, want user message", messages[1])
			}
			return einoschema.AssistantMessage("ok", nil), nil
		},
	}
	agent, err := NewEinoGraphNativeAgent(ctx, EinoGraphNativeAgentConfig{
		Name:        "test-agent",
		Instruction: "Be concise.",
		Model:       model,
	})
	if err != nil {
		t.Fatalf("NewEinoGraphNativeAgent() error = %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	finalMessage := runGraphNativeAgent(t, runner, []*einoschema.Message{einoschema.UserMessage("hello")})

	if finalMessage.Content != "ok" {
		t.Fatalf("final message content = %q, want ok", finalMessage.Content)
	}
	if model.withToolsCalls != 0 {
		t.Fatalf("WithTools calls = %d, want 0", model.withToolsCalls)
	}
}

func TestEinoGraphNativeAgentCheckpointInterruptResume(t *testing.T) {
	ctx := context.Background()
	checkpointID := "graph-native-checkpoint-test"
	store := NewMemoryRuntimeGraphCheckpointStore()
	approvalTool := &graphNativeApprovalTool{}
	model := &graphNativeRecordingModel{
		generate: func(call int, messages []*einoschema.Message) (*einoschema.Message, error) {
			switch call {
			case 1:
				return einoschema.AssistantMessage("", []einoschema.ToolCall{
					{
						ID:   "call_approval",
						Type: "function",
						Function: einoschema.FunctionCall{
							Name:      "approval",
							Arguments: `{"action":"continue"}`,
						},
					},
				}), nil
			case 2:
				toolMessage := findToolMessage(messages, "call_approval")
				if toolMessage == nil || !strings.Contains(toolMessage.Content, `"approved":true`) {
					t.Fatalf("resume model input missing approved tool result: %#v", messages)
				}
				return einoschema.AssistantMessage("resumed after approval", nil), nil
			default:
				return nil, fmt.Errorf("unexpected generate call %d", call)
			}
		},
	}
	agent, err := NewEinoGraphNativeAgent(ctx, EinoGraphNativeAgentConfig{
		Name:            "checkpoint-agent",
		Instruction:     "Use approval when required.",
		Model:           model,
		Tools:           []einotool.BaseTool{approvalTool},
		CheckpointStore: store,
		CheckpointID:    checkpointID,
	})
	if err != nil {
		t.Fatalf("NewEinoGraphNativeAgent() error = %v", err)
	}

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	interrupt := runGraphNativeAgentUntilInterrupt(t, runner, []*einoschema.Message{einoschema.UserMessage("needs approval")})
	if interrupt.Action == nil || interrupt.Action.Interrupted == nil || len(interrupt.Action.Interrupted.InterruptContexts) == 0 {
		t.Fatalf("interrupt event = %#v, want interrupt contexts", interrupt)
	}
	if !approvalTool.interrupted || approvalTool.resumed {
		t.Fatalf("approval tool interrupted=%v resumed=%v, want first interrupt only", approvalTool.interrupted, approvalTool.resumed)
	}
	if snapshot, ok := store.Snapshot(checkpointID); !ok || snapshot.PayloadSize == 0 || snapshot.PayloadSHA256 == "" {
		t.Fatalf("checkpoint snapshot = %#v, %v; want stored checkpoint metadata", snapshot, ok)
	}

	finalMessage := runGraphNativeIteratorToFinal(t, agent.Resume(context.Background(), &adk.ResumeInfo{}))
	if finalMessage.Content != "resumed after approval" {
		t.Fatalf("final message content = %q, want resumed after approval", finalMessage.Content)
	}
	if !approvalTool.resumed {
		t.Fatal("approval tool did not observe checkpoint resume state")
	}
	if model.generateCalls != 2 {
		t.Fatalf("Generate calls = %d, want first call plus resume final call", model.generateCalls)
	}
}

func TestMemoryRuntimeGraphCheckpointStoreRejectsCredentialLikePayload(t *testing.T) {
	store := NewMemoryRuntimeGraphCheckpointStore()
	err := store.Set(context.Background(), "checkpoint-credential", []byte("Authorization: Bearer sk-test"))
	if !errors.Is(err, ErrRuntimeCheckpointRejected) {
		t.Fatalf("Set() error = %v, want ErrRuntimeCheckpointRejected", err)
	}
	if _, ok := store.Snapshot("checkpoint-credential"); ok {
		t.Fatal("credential-like checkpoint payload was stored")
	}
}

type graphNativeRecordingModel struct {
	generateCalls  int
	withToolsCalls int
	tools          []*einoschema.ToolInfo
	generate       func(call int, messages []*einoschema.Message) (*einoschema.Message, error)
}

func (m *graphNativeRecordingModel) Generate(_ context.Context, messages []*einoschema.Message, _ ...einomodel.Option) (*einoschema.Message, error) {
	m.generateCalls++
	if m.generate == nil {
		return einoschema.AssistantMessage("ok", nil), nil
	}
	return m.generate(m.generateCalls, messages)
}

func (m *graphNativeRecordingModel) Stream(context.Context, []*einoschema.Message, ...einomodel.Option) (*einoschema.StreamReader[*einoschema.Message], error) {
	return nil, fmt.Errorf("stream is not used by graph-native agent tests")
}

func (m *graphNativeRecordingModel) WithTools(tools []*einoschema.ToolInfo) (einomodel.ToolCallingChatModel, error) {
	m.withToolsCalls++
	m.tools = append([]*einoschema.ToolInfo(nil), tools...)
	return m, nil
}

type graphNativeLookupTool struct {
	calls int
}

func (t *graphNativeLookupTool) Info(context.Context) (*einoschema.ToolInfo, error) {
	return &einoschema.ToolInfo{
		Name: "lookup",
		Desc: "Lookup test information.",
		ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"query": {
				Type: "string",
				Desc: "Lookup query.",
			},
		}),
	}, nil
}

func (t *graphNativeLookupTool) InvokableRun(_ context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	t.calls++
	if !strings.Contains(argumentsInJSON, "athena") {
		return "", fmt.Errorf("arguments = %q, want athena query", argumentsInJSON)
	}
	return `{"answer":"ok"}`, nil
}

type graphNativeApprovalTool struct {
	interrupted bool
	resumed     bool
}

func (t *graphNativeApprovalTool) Info(context.Context) (*einoschema.ToolInfo, error) {
	return &einoschema.ToolInfo{
		Name: "approval",
		Desc: "Request approval before continuing.",
		ParamsOneOf: einoschema.NewParamsOneOfByParams(map[string]*einoschema.ParameterInfo{
			"action": {
				Type: "string",
				Desc: "Action to approve.",
			},
		}),
	}, nil
}

func (t *graphNativeApprovalTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...einotool.Option) (string, error) {
	wasInterrupted, _, _ := einotool.GetInterruptState[string](ctx)
	if !wasInterrupted {
		t.interrupted = true
		return "", einotool.StatefulInterrupt(ctx, "approval required", argumentsInJSON)
	}
	t.resumed = true
	return `{"approved":true}`, nil
}

func runGraphNativeAgent(tb testing.TB, runner *adk.Runner, messages []*einoschema.Message) *einoschema.Message {
	tb.Helper()

	iter := runner.Run(context.Background(), messages)
	return runGraphNativeIteratorToFinal(tb, iter)
}

func runGraphNativeIteratorToFinal(tb testing.TB, iter *adk.AsyncIterator[*adk.AgentEvent]) *einoschema.Message {
	tb.Helper()

	var finalMessage *einoschema.Message
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			tb.Fatalf("runner event error = %v", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		if event.Output.MessageOutput.Message != nil {
			finalMessage = event.Output.MessageOutput.Message
		}
	}
	if finalMessage == nil {
		tb.Fatalf("runner produced no final message")
	}
	return finalMessage
}

func runGraphNativeAgentUntilInterrupt(tb testing.TB, runner *adk.Runner, messages []*einoschema.Message) *adk.AgentEvent {
	tb.Helper()

	iter := runner.Run(context.Background(), messages)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event == nil {
			continue
		}
		if event.Err != nil {
			tb.Fatalf("runner event error = %v", event.Err)
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			return event
		}
	}
	tb.Fatalf("runner produced no interrupt event")
	return nil
}

func hasMessageRole(messages []*einoschema.Message, role einoschema.RoleType) bool {
	for _, msg := range messages {
		if msg != nil && msg.Role == role {
			return true
		}
	}
	return false
}

func hasAssistantToolCall(messages []*einoschema.Message, callID string) bool {
	for _, msg := range messages {
		if msg == nil || msg.Role != einoschema.Assistant {
			continue
		}
		for _, toolCall := range msg.ToolCalls {
			if toolCall.ID == callID {
				return true
			}
		}
	}
	return false
}

func findToolMessage(messages []*einoschema.Message, callID string) *einoschema.Message {
	for _, msg := range messages {
		if msg != nil && msg.Role == einoschema.Tool && msg.ToolCallID == callID {
			return msg
		}
	}
	return nil
}

func messageRoles(messages []*einoschema.Message) []einoschema.RoleType {
	roles := make([]einoschema.RoleType, 0, len(messages))
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		roles = append(roles, msg.Role)
	}
	return roles
}

func hasRuntimeCallback(events []RuntimeComponentCallbackEvent, component string) bool {
	for _, event := range events {
		if event.Component == component && event.Status == "success" {
			return true
		}
	}
	return false
}

func hasModelTokenUsage(events []RuntimeComponentCallbackEvent, totalTokens int) bool {
	for _, event := range events {
		if event.Component == runtimeCallbackComponentModel && event.TotalTokens == totalTokens {
			return true
		}
	}
	return false
}
