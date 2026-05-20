// eino_graph.go defines the Eino Graph foundation around Athena's turn execution boundary.
// eino_graph.go 定义围绕 Athena 单轮执行边界的 Eino Graph 基础骨架。
package runtime

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/compose"
	runtimetask "moss/internal/runtime/task"
)

const (
	// RuntimeGraphNodeContextAssembly marks the graph node that receives already assembled runtime messages.
	// RuntimeGraphNodeContextAssembly 表示接收已组装 runtime messages 的 graph 节点。
	RuntimeGraphNodeContextAssembly = "context_assembly"
	RuntimeGraphNodeCapability      = "capability_resolution"
	RuntimeGraphNodeGovernance      = "governance_decision"
	RuntimeGraphNodeTurnExecution   = "turn_execution"
	RuntimeGraphNodeSchema          = "schema_validation"
	RuntimeGraphNodeProjection      = "projection_candidate"
	RuntimeGraphNodePersistence     = "persistence_projection"
)

// EinoGraphTurnExecutorOptions carries optional graph-side persistence dependencies.
// EinoGraphTurnExecutorOptions 携带 graph 侧可选持久化依赖。
type EinoGraphTurnExecutorOptions struct {
	Store RuntimePersistenceStore
	Now   func() time.Time
}

// RuntimeGraphStepSnapshot records the safe status of one graph node.
// RuntimeGraphStepSnapshot 记录一个 graph 节点的安全状态。
type RuntimeGraphStepSnapshot struct {
	Name   string
	Status string
	Reason string
}

// RuntimeGraphCallbackEvent records safe Eino callback metadata for one graph invocation.
// RuntimeGraphCallbackEvent 记录一次 graph 调用中的安全 Eino callback 元数据。
type RuntimeGraphCallbackEvent struct {
	Node      string
	Component string
	Timing    string
}

// RuntimeGraphFrame is the request-local state passed between Eino Graph nodes.
// RuntimeGraphFrame 是在 Eino Graph 节点之间传递的请求级运行态。
type RuntimeGraphFrame struct {
	State          RuntimeState
	Spec           *ExecutionSpec
	Messages       []adk.Message
	Prepared       *PreparedExecution
	Projection     *ProjectionCandidate
	RecordSet      *MinimalPersistenceRecordSet
	Steps          []RuntimeGraphStepSnapshot
	CallbackEvents []RuntimeGraphCallbackEvent
	Metadata       map[string]any
}

// EinoGraphFoundation owns the compiled runtime graph and its persistence projection dependencies.
// EinoGraphFoundation 持有已编译的 runtime graph 及其持久化投影依赖。
type EinoGraphFoundation struct {
	Executor TurnExecutor
	Store    RuntimePersistenceStore
	Now      func() time.Time

	compileMu sync.Mutex
	compiled  compose.Runnable[*RuntimeGraphFrame, *RuntimeGraphFrame]
}

// EinoGraphTurnExecutor routes one runtime turn through the Eino Graph foundation before returning PreparedExecution.
// EinoGraphTurnExecutor 会先让单轮 runtime 执行经过 Eino Graph 骨架，再返回 PreparedExecution。
type EinoGraphTurnExecutor struct {
	graph *EinoGraphFoundation
}

// NewEinoGraphTurnExecutor wraps an existing turn executor with Athena's Eino Graph foundation.
// NewEinoGraphTurnExecutor 用 Athena 的 Eino Graph 基础骨架包装已有单轮执行器。
func NewEinoGraphTurnExecutor(base TurnExecutor, opts EinoGraphTurnExecutorOptions) TurnExecutor {
	return EinoGraphTurnExecutor{
		graph: &EinoGraphFoundation{
			Executor: base,
			Store:    opts.Store,
			Now:      opts.Now,
		},
	}
}

// Prepare implements TurnExecutor by invoking the graph and returning the prepared execution output.
// Prepare 通过调用 graph 实现 TurnExecutor，并返回准备好的执行输出。
func (e EinoGraphTurnExecutor) Prepare(ctx context.Context, state RuntimeState, spec *ExecutionSpec, messages []adk.Message) (*PreparedExecution, error) {
	if e.graph == nil {
		return nil, fmt.Errorf("eino runtime graph foundation is required")
	}
	frame, err := e.graph.Run(ctx, &RuntimeGraphFrame{
		State:    state,
		Spec:     spec,
		Messages: messages,
		Metadata: map[string]any{},
	})
	if err != nil {
		return nil, err
	}
	if frame == nil || frame.Prepared == nil {
		return nil, fmt.Errorf("eino runtime graph completed without prepared execution")
	}
	if frame.RecordSet != nil {
		frame.Prepared.RuntimeRecords = frame.RecordSet
		frame.Prepared.TerminalProjector = &RuntimeTerminalProjector{
			Store:     e.graph.Store,
			Now:       e.graph.Now,
			RecordSet: frame.RecordSet,
			Callbacks: frame.Prepared.CallbackRecorder,
			Metadata: map[string]any{
				"graph_steps":     graphStepNames(frame.Steps),
				"callback_events": graphCallbackEventSummaries(frame.CallbackEvents),
			},
		}
	}
	return frame.Prepared, nil
}

// Run executes the Eino Graph runtime foundation for one prepared frame.
// Run 针对一个 frame 执行 Eino Graph runtime 基础骨架。
func (g *EinoGraphFoundation) Run(ctx context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	if frame == nil {
		return nil, fmt.Errorf("runtime graph frame is required")
	}
	runnable, err := g.compile(ctx)
	if err != nil {
		return nil, err
	}
	recorder := newRuntimeGraphCallbackRecorder(frame)
	out, err := runnable.Invoke(ctx, frame, compose.WithCallbacks(recorder.handler()))
	if err != nil {
		return nil, err
	}
	out.CallbackEvents = recorder.events()
	return out, nil
}

func (g *EinoGraphFoundation) compile(ctx context.Context) (compose.Runnable[*RuntimeGraphFrame, *RuntimeGraphFrame], error) {
	g.compileMu.Lock()
	defer g.compileMu.Unlock()
	if g.compiled != nil {
		return g.compiled, nil
	}
	graph := compose.NewGraph[*RuntimeGraphFrame, *RuntimeGraphFrame]()
	nodes := []struct {
		name string
		fn   func(context.Context, *RuntimeGraphFrame) (*RuntimeGraphFrame, error)
	}{
		{RuntimeGraphNodeContextAssembly, g.contextAssemblyNode},
		{RuntimeGraphNodeCapability, g.capabilityNode},
		{RuntimeGraphNodeGovernance, g.governanceNode},
		{RuntimeGraphNodeTurnExecution, g.turnExecutionNode},
		{RuntimeGraphNodeSchema, g.schemaValidationNode},
		{RuntimeGraphNodeProjection, g.projectionCandidateNode},
		{RuntimeGraphNodePersistence, g.persistenceProjectionNode},
	}
	for _, node := range nodes {
		if err := graph.AddLambdaNode(node.name, compose.InvokableLambda(node.fn)); err != nil {
			return nil, err
		}
	}
	edges := []struct {
		from string
		to   string
	}{
		{compose.START, RuntimeGraphNodeContextAssembly},
		{RuntimeGraphNodeContextAssembly, RuntimeGraphNodeCapability},
		{RuntimeGraphNodeCapability, RuntimeGraphNodeGovernance},
		{RuntimeGraphNodeGovernance, RuntimeGraphNodeTurnExecution},
		{RuntimeGraphNodeTurnExecution, RuntimeGraphNodeSchema},
		{RuntimeGraphNodeSchema, RuntimeGraphNodeProjection},
		{RuntimeGraphNodeProjection, RuntimeGraphNodePersistence},
		{RuntimeGraphNodePersistence, compose.END},
	}
	for _, edge := range edges {
		if err := graph.AddEdge(edge.from, edge.to); err != nil {
			return nil, err
		}
	}
	runnable, err := graph.Compile(ctx, compose.WithGraphName("athena_runtime_graph"), compose.WithNodeTriggerMode(compose.AllPredecessor))
	if err != nil {
		return nil, err
	}
	g.compiled = runnable
	return runnable, nil
}

func (g EinoGraphFoundation) contextAssemblyNode(_ context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	frame.ensureMetadata()
	frame.Metadata["message_count"] = len(frame.Messages)
	frame.markStep(RuntimeGraphNodeContextAssembly, "success", "messages_received_from_runtime_context_assembler")
	return frame, nil
}

func (g EinoGraphFoundation) capabilityNode(_ context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	if frame.Spec == nil {
		return nil, fmt.Errorf("runtime graph execution spec is required")
	}
	frame.ensureMetadata()
	frame.Metadata["primary_skill"] = frame.Spec.Skill.PrimarySkill
	frame.Metadata["allowed_tool_count"] = len(frame.Spec.Tools.AllowedTools)
	frame.markStep(RuntimeGraphNodeCapability, "success", "execution_spec_received_from_capability_resolver")
	return frame, nil
}

func (g EinoGraphFoundation) governanceNode(_ context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	frame.ensureMetadata()
	decision := ""
	if frame.Spec != nil && frame.Spec.Metadata.Governance != nil {
		decision = string(frame.Spec.Metadata.Governance.Decision)
	}
	frame.Metadata["governance_decision"] = decision
	frame.markStep(RuntimeGraphNodeGovernance, "success", defaultString(decision, "governance_metadata_absent"))
	return frame, nil
}

func (g EinoGraphFoundation) turnExecutionNode(ctx context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	if g.Executor == nil {
		return nil, fmt.Errorf("runtime graph executor is required")
	}
	prepared, err := g.Executor.Prepare(ctx, frame.State, frame.Spec, frame.Messages)
	if err != nil {
		frame.markStep(RuntimeGraphNodeTurnExecution, "failed", err.Error())
		return nil, err
	}
	frame.Prepared = prepared
	frame.markStep(RuntimeGraphNodeTurnExecution, "success", "base_turn_executor_prepared")
	return frame, nil
}

func (g EinoGraphFoundation) schemaValidationNode(_ context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	if frame.Prepared == nil {
		return nil, fmt.Errorf("runtime graph prepared execution is required before schema validation")
	}
	frame.ensureMetadata()
	frame.Metadata["structured_output_requested"] = frame.Spec != nil && frame.Spec.Inference.StructuredOutput != nil && frame.Spec.Inference.StructuredOutput.Requested
	frame.markStep(RuntimeGraphNodeSchema, "success", "prepared_execution_contract_validated")
	return frame, nil
}

func (g EinoGraphFoundation) projectionCandidateNode(_ context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	frame.Projection = &ProjectionCandidate{
		CandidateKind: "prepared_execution",
		Status:        "ready",
		Summary:       "runtime graph prepared a candidate execution output",
		RedactedPayload: map[string]any{
			"message_count": len(frame.Messages),
			"has_runner":    frame.Prepared != nil && frame.Prepared.Runner != nil,
		},
		Metadata: map[string]any{
			"projection_source": "eino_runtime_graph",
		},
	}
	frame.markStep(RuntimeGraphNodeProjection, "success", "minimal_candidate_output_projected")
	return frame, nil
}

func (g EinoGraphFoundation) persistenceProjectionNode(ctx context.Context, frame *RuntimeGraphFrame) (*RuntimeGraphFrame, error) {
	if g.Store == nil {
		frame.markStep(RuntimeGraphNodePersistence, "skipped", "runtime_persistence_store_unconfigured")
		return frame, nil
	}
	writer := PersistenceWriter{Store: g.Store, Now: g.Now}
	recordSet, err := writer.WriteMinimalRun(ctx, MinimalPersistenceInput{
		Task:             runtimeTaskFromGraphFrame(frame),
		IdempotencyScope: runtimeGraphIdempotencyScope(frame),
		IdempotencyKey:   strings.TrimSpace(frame.State.RequestID),
		Metadata: map[string]any{
			"graph_writer":    "eino_runtime_graph",
			"writer":          "eino_runtime_graph",
			"graph_steps":     graphStepNames(frame.Steps),
			"callback_events": graphCallbackEventSummaries(frame.CallbackEvents),
		},
	})
	if err != nil {
		frame.markStep(RuntimeGraphNodePersistence, "failed", err.Error())
		return nil, err
	}
	frame.RecordSet = &recordSet
	contractID := runtimeContractIDFromGraphFrame(frame)
	if hookStore, ok := g.Store.(HookBindingStore); ok && strings.TrimSpace(contractID) != "" {
		count, err := (RuntimeHookBridge{
			Store:      g.Store,
			HookStore:  hookStore,
			Now:        g.Now,
			RecordSet:  frame.RecordSet,
			ContractID: contractID,
		}).ProjectHookPoint(ctx, HookPointBeforeRun)
		if err != nil {
			frame.markStep(RuntimeGraphNodePersistence, "failed", err.Error())
			return nil, err
		}
		if count > 0 {
			frame.Metadata["runtime_hook_count"] = count
			frame.Metadata["runtime_contract_id"] = contractID
		}
	}
	if frame.Prepared != nil {
		frame.Prepared.RuntimeRecords = frame.RecordSet
		frame.Prepared.TerminalProjector = &RuntimeTerminalProjector{
			Store:     g.Store,
			Now:       g.Now,
			RecordSet: frame.RecordSet,
			Callbacks: frame.Prepared.CallbackRecorder,
			Metadata: map[string]any{
				"graph_steps":     graphStepNames(frame.Steps),
				"callback_events": graphCallbackEventSummaries(frame.CallbackEvents),
			},
		}
	}
	frame.markStep(RuntimeGraphNodePersistence, "success", "minimal_runtime_persistence_projected")
	return frame, nil
}

func runtimeContractIDFromGraphFrame(frame *RuntimeGraphFrame) string {
	if frame == nil || frame.Spec == nil {
		return ""
	}
	if value, ok := frame.Spec.Metadata.Constraints["runtime_contract_id"]; ok {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	if value, ok := frame.Spec.Metadata.Constraints["contract_id"]; ok {
		return strings.TrimSpace(fmt.Sprint(value))
	}
	return ""
}

func (f *RuntimeGraphFrame) ensureMetadata() {
	if f.Metadata == nil {
		f.Metadata = map[string]any{}
	}
}

func (f *RuntimeGraphFrame) markStep(name string, status string, reason string) {
	f.Steps = append(f.Steps, RuntimeGraphStepSnapshot{
		Name:   name,
		Status: status,
		Reason: reason,
	})
}

type runtimeGraphCallbackRecorder struct {
	mu        sync.Mutex
	frame     *RuntimeGraphFrame
	eventList []RuntimeGraphCallbackEvent
}

func newRuntimeGraphCallbackRecorder(frame *RuntimeGraphFrame) *runtimeGraphCallbackRecorder {
	return &runtimeGraphCallbackRecorder{frame: frame}
}

func (r *runtimeGraphCallbackRecorder) handler() callbacks.Handler {
	return callbacks.NewHandlerBuilder().
		OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackInput) context.Context {
			r.record(info, "start")
			return ctx
		}).
		OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, _ callbacks.CallbackOutput) context.Context {
			r.record(info, "end")
			return ctx
		}).
		OnErrorFn(func(ctx context.Context, info *callbacks.RunInfo, err error) context.Context {
			reason := "error"
			if err != nil {
				reason = "error:" + err.Error()
			}
			r.record(info, reason)
			return ctx
		}).
		Build()
}

func (r *runtimeGraphCallbackRecorder) record(info *callbacks.RunInfo, timing string) {
	event := RuntimeGraphCallbackEvent{Timing: timing}
	if info != nil {
		event.Node = info.Name
		event.Component = fmt.Sprint(info.Component)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.eventList = append(r.eventList, event)
	if r.frame != nil {
		r.frame.CallbackEvents = append(r.frame.CallbackEvents, event)
	}
}

func (r *runtimeGraphCallbackRecorder) events() []RuntimeGraphCallbackEvent {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]RuntimeGraphCallbackEvent(nil), r.eventList...)
}

func runtimeTaskFromGraphFrame(frame *RuntimeGraphFrame) *runtimetask.RuntimeTask {
	constraints := map[string]any{}
	if frame != nil && frame.Spec != nil && frame.Spec.Metadata.Constraints != nil {
		constraints = frame.Spec.Metadata.Constraints
	}
	taskID := stringFromConstraint(constraints, "task_id")
	if taskID == "" && frame != nil {
		taskID = frame.State.RequestID
	}
	return &runtimetask.RuntimeTask{
		TaskID:        defaultString(taskID, "runtime-graph-task"),
		TaskType:      defaultString(stringFromConstraint(constraints, "task_type"), "runtime_task"),
		TaskSubtype:   stringFromConstraint(constraints, "task_subtype"),
		InputKind:     defaultString(stringFromConstraint(constraints, "task_kind"), "runtime_task"),
		Scene:         stringFromConstraint(constraints, "scene"),
		WorkspaceID:   stringFromConstraint(constraints, "workspace_id"),
		AppInstanceID: stringFromConstraint(constraints, "app_instance_id"),
		OutputMode:    stringFromConstraint(constraints, "desired_output_mode"),
		UserGoal:      graphGoal(frame),
	}
}

func graphGoal(frame *RuntimeGraphFrame) string {
	if frame == nil || frame.Spec == nil {
		return ""
	}
	return frame.Spec.Inference.Goal
}

func runtimeGraphIdempotencyScope(frame *RuntimeGraphFrame) string {
	task := runtimeTaskFromGraphFrame(frame)
	return "eino_graph:" + defaultIdempotencyScope(task)
}

func graphStepNames(steps []RuntimeGraphStepSnapshot) []string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.Name)
	}
	return names
}

func graphCallbackEventSummaries(events []RuntimeGraphCallbackEvent) []map[string]string {
	output := make([]map[string]string, 0, len(events))
	for _, event := range events {
		output = append(output, map[string]string{
			"node":      event.Node,
			"component": event.Component,
			"timing":    event.Timing,
		})
	}
	return output
}

func stringFromConstraint(constraints map[string]any, key string) string {
	value, ok := constraints[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
