// eino_native_agent.go implements Athena's graph-native Eino turn agent.
// eino_native_agent.go 实现 Athena 的 Eino graph-native 单轮 agent。
package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	einomodel "github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	einoschema "github.com/cloudwego/eino/schema"
	runtimetools "moss/internal/tools"
)

const (
	runtimeGraphNativeModelNode = "graph_native_chat_model"
	runtimeGraphNativeToolsNode = "graph_native_tools_node"
)

// runtimeGraphNativeState stores the per-run ReAct message history inside Eino Graph state.
// runtimeGraphNativeState 在 Eino Graph state 内保存每次运行的 ReAct 消息历史。
type runtimeGraphNativeState struct {
	Messages []*einoschema.Message
}

// EinoGraphNativeAgentConfig contains the graph-native model/tool execution dependencies.
// EinoGraphNativeAgentConfig 保存 graph-native model/tool 执行依赖。
type EinoGraphNativeAgentConfig struct {
	Name            string
	Description     string
	Instruction     string
	Model           einomodel.ToolCallingChatModel
	Tools           []tool.BaseTool
	Callbacks       callbacks.Handler
	CheckpointStore RuntimeGraphCheckpointByteStore
	CheckpointID    string
}

// EinoGraphNativeAgent executes one turn through Eino Graph ChatModel and ToolsNode components.
// EinoGraphNativeAgent 通过 Eino Graph ChatModel 和 ToolsNode 组件执行单轮请求。
type EinoGraphNativeAgent struct {
	name         string
	description  string
	instruction  string
	runnable     compose.Runnable[[]*einoschema.Message, *einoschema.Message]
	callbacks    callbacks.Handler
	checkpointID string
	mu           sync.Mutex
	initialInput []*einoschema.Message
}

func init() {
	einoschema.RegisterName[*runtimeGraphNativeState]("athena_runtime_graph_native_state_v1")
}

// NewEinoGraphNativeAgent compiles a graph-native ReAct-style turn agent.
// NewEinoGraphNativeAgent 编译一个 graph-native ReAct 风格的单轮 agent。
func NewEinoGraphNativeAgent(ctx context.Context, cfg EinoGraphNativeAgentConfig) (*EinoGraphNativeAgent, error) {
	if cfg.Model == nil {
		return nil, fmt.Errorf("graph-native chat model is required")
	}
	checkpointID := ""
	if cfg.CheckpointStore != nil {
		checkpointID = strings.TrimSpace(cfg.CheckpointID)
	}
	if cfg.CheckpointStore != nil && checkpointID == "" {
		return nil, fmt.Errorf("graph-native checkpoint id is required when checkpoint store is configured")
	}
	modelNode := cfg.Model
	toolsNode, err := newRuntimeGraphNativeToolsNode(ctx, cfg.Tools)
	if err != nil {
		return nil, err
	}
	if toolsNode != nil {
		modelNode, err = bindGraphNativeTools(ctx, cfg.Model, cfg.Tools)
		if err != nil {
			return nil, err
		}
	}

	graph := compose.NewGraph[[]*einoschema.Message, *einoschema.Message](
		compose.WithGenLocalState(func(context.Context) *runtimeGraphNativeState {
			return &runtimeGraphNativeState{}
		}),
	)
	if err := graph.AddChatModelNode(
		runtimeGraphNativeModelNode,
		modelNode,
		compose.WithStatePreHandler(runtimeGraphNativeModelPreHandler),
		compose.WithStatePostHandler(runtimeGraphNativeModelPostHandler),
	); err != nil {
		return nil, err
	}
	if err := graph.AddEdge(compose.START, runtimeGraphNativeModelNode); err != nil {
		return nil, err
	}
	if toolsNode == nil {
		if err := graph.AddEdge(runtimeGraphNativeModelNode, compose.END); err != nil {
			return nil, err
		}
	} else {
		if err := graph.AddToolsNode(
			runtimeGraphNativeToolsNode,
			toolsNode,
			compose.WithStatePreHandler(runtimeGraphNativeToolsPreHandler),
		); err != nil {
			return nil, err
		}
		if err := graph.AddBranch(runtimeGraphNativeModelNode, compose.NewGraphBranch(
			func(_ context.Context, msg *einoschema.Message) (string, error) {
				if msg != nil && len(msg.ToolCalls) > 0 {
					return runtimeGraphNativeToolsNode, nil
				}
				return compose.END, nil
			},
			map[string]bool{runtimeGraphNativeToolsNode: true, compose.END: true},
		)); err != nil {
			return nil, err
		}
		if err := graph.AddEdge(runtimeGraphNativeToolsNode, runtimeGraphNativeModelNode); err != nil {
			return nil, err
		}
	}
	compileOptions := []compose.GraphCompileOption{
		compose.WithGraphName("athena_graph_native_turn"),
		compose.WithMaxRunSteps(12),
	}
	if cfg.CheckpointStore != nil {
		compileOptions = append(compileOptions, compose.WithCheckPointStore(cfg.CheckpointStore))
	}
	runnable, err := graph.Compile(ctx, compileOptions...)
	if err != nil {
		return nil, err
	}
	return &EinoGraphNativeAgent{
		name:         defaultString(cfg.Name, "AthenaGraphNativeRuntimeAgent"),
		description:  cfg.Description,
		instruction:  cfg.Instruction,
		runnable:     runnable,
		callbacks:    cfg.Callbacks,
		checkpointID: checkpointID,
	}, nil
}

// Name returns the ADK-visible agent name.
// Name 返回 ADK 可见的 agent 名称。
func (a *EinoGraphNativeAgent) Name(context.Context) string {
	if a == nil {
		return ""
	}
	return a.name
}

// Description returns the ADK-visible agent description.
// Description 返回 ADK 可见的 agent 描述。
func (a *EinoGraphNativeAgent) Description(context.Context) string {
	if a == nil {
		return ""
	}
	return a.description
}

// Run executes the compiled Eino graph and emits the final message as an ADK event.
// Run 执行已编译的 Eino graph，并把最终消息作为 ADK event 发出。
func (a *EinoGraphNativeAgent) Run(ctx context.Context, input *adk.AgentInput, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	messages := graphNativeInputMessages(input, a.instruction)
	a.storeInitialInput(messages)
	return a.invoke(ctx, messages)
}

// Resume continues an interrupted graph-native Eino run from its checkpoint.
// Resume 从 checkpoint 继续一个被中断的 graph-native Eino run。
func (a *EinoGraphNativeAgent) Resume(ctx context.Context, _ *adk.ResumeInfo, _ ...adk.AgentRunOption) *adk.AsyncIterator[*adk.AgentEvent] {
	if a != nil && strings.TrimSpace(a.name) != "" {
		ctx = adk.AppendAddressSegment(ctx, adk.AddressSegmentAgent, a.name)
	}
	return a.invoke(ctx, a.loadInitialInput())
}

func (a *EinoGraphNativeAgent) invoke(ctx context.Context, messages []*einoschema.Message) *adk.AsyncIterator[*adk.AgentEvent] {
	iter, generator := adk.NewAsyncIteratorPair[*adk.AgentEvent]()
	go func() {
		defer generator.Close()
		if a == nil || a.runnable == nil {
			generator.Send(&adk.AgentEvent{Err: fmt.Errorf("graph-native agent is not configured")})
			return
		}
		output, err := a.runnable.Invoke(ctx, messages, a.invokeOptions()...)
		if err != nil {
			if interrupt, ok := compose.ExtractInterruptInfo(err); ok {
				generator.Send(&adk.AgentEvent{
					AgentName: a.name,
					Action: &adk.AgentAction{
						Interrupted: &adk.InterruptInfo{
							Data:              interrupt.State,
							InterruptContexts: interrupt.InterruptContexts,
						},
					},
				})
				return
			}
			generator.Send(&adk.AgentEvent{AgentName: a.name, Err: err})
			return
		}
		if output == nil {
			output = einoschema.AssistantMessage("", nil)
		}
		generator.Send(&adk.AgentEvent{
			AgentName: a.name,
			Output: &adk.AgentOutput{
				MessageOutput: &adk.MessageVariant{
					Message:  output,
					Role:     output.Role,
					ToolName: output.ToolName,
				},
			},
		})
	}()
	return iter
}

func (a *EinoGraphNativeAgent) invokeOptions() []compose.Option {
	options := []compose.Option{}
	if a != nil && strings.TrimSpace(a.checkpointID) != "" {
		options = append(options, compose.WithCheckPointID(strings.TrimSpace(a.checkpointID)))
	}
	if a != nil && a.callbacks != nil {
		options = append(options, compose.WithCallbacks(a.callbacks))
	}
	return options
}

func (a *EinoGraphNativeAgent) storeInitialInput(messages []*einoschema.Message) {
	if a == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	a.initialInput = appendMessageCopies(nil, messages...)
}

func (a *EinoGraphNativeAgent) loadInitialInput() []*einoschema.Message {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return appendMessageCopies(nil, a.initialInput...)
}

func newRuntimeGraphNativeToolsNode(ctx context.Context, selectedTools []tool.BaseTool) (*compose.ToolsNode, error) {
	if len(selectedTools) == 0 {
		return nil, nil
	}
	return compose.NewToolNode(ctx, &compose.ToolsNodeConfig{
		Tools:               selectedTools,
		ExecuteSequentially: false,
		ToolCallMiddlewares: []compose.ToolMiddleware{
			toolsTraceMiddleware(),
		},
	})
}

func bindGraphNativeTools(ctx context.Context, modelNode einomodel.ToolCallingChatModel, selectedTools []tool.BaseTool) (einomodel.ToolCallingChatModel, error) {
	toolInfos := make([]*einoschema.ToolInfo, 0, len(selectedTools))
	for _, selectedTool := range selectedTools {
		if selectedTool == nil {
			continue
		}
		info, err := selectedTool.Info(ctx)
		if err != nil {
			return nil, err
		}
		if info != nil {
			toolInfos = append(toolInfos, info)
		}
	}
	if len(toolInfos) == 0 {
		return modelNode, nil
	}
	return modelNode.WithTools(toolInfos)
}

func graphNativeInputMessages(input *adk.AgentInput, instruction string) []*einoschema.Message {
	messages := []adk.Message(nil)
	if input != nil {
		messages = input.Messages
	}
	normalized := normalizeMessages(messages)
	if instruction == "" {
		return normalized
	}
	withInstruction := make([]*einoschema.Message, 0, len(normalized)+1)
	withInstruction = append(withInstruction, einoschema.SystemMessage(instruction))
	withInstruction = append(withInstruction, normalized...)
	return withInstruction
}

func runtimeGraphNativeModelPreHandler(_ context.Context, in []*einoschema.Message, state *runtimeGraphNativeState) ([]*einoschema.Message, error) {
	if state == nil {
		return in, nil
	}
	state.Messages = appendMessageCopies(state.Messages, in...)
	return appendMessageCopies(nil, state.Messages...), nil
}

func runtimeGraphNativeModelPostHandler(_ context.Context, out *einoschema.Message, state *runtimeGraphNativeState) (*einoschema.Message, error) {
	if state == nil || out == nil {
		return out, nil
	}
	state.Messages = appendMessageCopies(state.Messages, out)
	return out, nil
}

func runtimeGraphNativeToolsPreHandler(_ context.Context, in *einoschema.Message, state *runtimeGraphNativeState) (*einoschema.Message, error) {
	if in != nil || state == nil {
		return in, nil
	}
	for i := len(state.Messages) - 1; i >= 0; i-- {
		msg := state.Messages[i]
		if msg != nil && msg.Role == einoschema.Assistant && len(msg.ToolCalls) > 0 {
			return appendMessageCopies(nil, msg)[0], nil
		}
	}
	return in, nil
}

func appendMessageCopies(dst []*einoschema.Message, messages ...*einoschema.Message) []*einoschema.Message {
	for _, msg := range messages {
		if msg == nil {
			continue
		}
		copied := *msg
		if len(msg.ToolCalls) > 0 {
			copied.ToolCalls = append([]einoschema.ToolCall(nil), msg.ToolCalls...)
		}
		if len(msg.Extra) > 0 {
			copied.Extra = make(map[string]any, len(msg.Extra))
			for key, value := range msg.Extra {
				copied.Extra[key] = value
			}
		}
		dst = append(dst, &copied)
	}
	return dst
}

func toolsTraceMiddleware() compose.ToolMiddleware {
	return runtimetools.NewToolTraceMiddleware().WrapToolCall
}
