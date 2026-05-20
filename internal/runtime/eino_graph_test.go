package runtime

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/cloudwego/eino/adk"
	einomessage "github.com/cloudwego/eino/schema"
)

type graphStubTurnExecutor struct {
	called int
}

func (s *graphStubTurnExecutor) Prepare(_ context.Context, _ RuntimeState, spec *ExecutionSpec, messages []adk.Message) (*PreparedExecution, error) {
	s.called++
	return &PreparedExecution{
		Spec:     spec,
		Messages: messages,
	}, nil
}

func TestEinoGraphFoundationRunsNodeBoundariesAndCallbacks(t *testing.T) {
	executor := &graphStubTurnExecutor{}
	graph := EinoGraphFoundation{Executor: executor}
	frame, err := graph.Run(context.Background(), &RuntimeGraphFrame{
		State: RuntimeState{
			RequestID: "req-graph",
			SessionID: "sess-graph",
			Turn:      2,
		},
		Spec:     graphTestSpec(),
		Messages: []adk.Message{einomessage.UserMessage("hello")},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if executor.called != 1 {
		t.Fatalf("executor called %d times, want 1", executor.called)
	}
	if frame.Prepared == nil {
		t.Fatal("Run() did not produce PreparedExecution")
	}
	wantSteps := []string{
		RuntimeGraphNodeContextAssembly,
		RuntimeGraphNodeCapability,
		RuntimeGraphNodeGovernance,
		RuntimeGraphNodeTurnExecution,
		RuntimeGraphNodeSchema,
		RuntimeGraphNodeProjection,
		RuntimeGraphNodePersistence,
	}
	if got := graphStepNames(frame.Steps); strings.Join(got, ",") != strings.Join(wantSteps, ",") {
		t.Fatalf("steps = %v, want %v", got, wantSteps)
	}
	if frame.Projection == nil || frame.Projection.CandidateKind != "prepared_execution" {
		t.Fatalf("projection = %#v, want prepared_execution candidate", frame.Projection)
	}
	if len(frame.CallbackEvents) == 0 {
		t.Fatal("Run() did not collect Eino callback events")
	}
}

func TestEinoGraphFoundationProjectsMinimalPersistence(t *testing.T) {
	store := newGraphMemoryRuntimeStore()
	contract, err := store.CreateRuntimeContract(context.Background(), RuntimeContract{
		ID:       "contract-graph",
		Name:     "Graph Contract",
		Version:  "v1",
		Status:   RuntimeContractStatusActive,
		TaskType: "runtime_task",
	})
	if err != nil {
		t.Fatalf("CreateRuntimeContract() error = %v", err)
	}
	if _, err := store.CreateHookBinding(context.Background(), HookBinding{
		ContractID:    contract.ID,
		HookPoint:     HookPointBeforeRun,
		BindingKind:   HookBindingKindEinoMiddleware,
		BindingRef:    "runtime_contract_guard",
		OrderIndex:    1,
		Enabled:       true,
		FailurePolicy: HookFailurePolicyFailClosed,
	}); err != nil {
		t.Fatalf("CreateHookBinding() error = %v", err)
	}
	graph := EinoGraphFoundation{
		Executor: &graphStubTurnExecutor{},
		Store:    store,
		Now: func() time.Time {
			return time.Date(2026, 5, 6, 8, 0, 0, 0, time.UTC)
		},
	}
	frame, err := graph.Run(context.Background(), &RuntimeGraphFrame{
		State: RuntimeState{
			RequestID: "req-persist",
			SessionID: "sess-persist",
			Turn:      1,
		},
		Spec:     graphTestSpec(),
		Messages: []adk.Message{einomessage.UserMessage("persist")},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if frame.RecordSet == nil {
		t.Fatal("Run() did not write runtime persistence record set")
	}
	if frame.RecordSet.Run.TaskID != "task-graph" {
		t.Fatalf("run task_id = %q, want task-graph", frame.RecordSet.Run.TaskID)
	}
	if _, ok := frame.RecordSet.Run.Metadata["callback_events"]; !ok {
		t.Fatalf("run metadata = %#v, want callback_events projection", frame.RecordSet.Run.Metadata)
	}
	if len(frame.RecordSet.Events) < 2 {
		t.Fatalf("events = %d, want run and step lifecycle events", len(frame.RecordSet.Events))
	}
	if frame.RecordSet.Trace.RedactedPayload["credential_raw"] != "[redacted]" {
		t.Fatalf("trace redacted payload = %#v, want credential redaction", frame.RecordSet.Trace.RedactedPayload)
	}
	if frame.RecordSet.Usage.ResourceType == "" || frame.RecordSet.Usage.Unit == "" {
		t.Fatalf("usage = %#v, want generic resource usage", frame.RecordSet.Usage)
	}
	if frame.RecordSet.Projection.CandidateKind == "" {
		t.Fatalf("projection = %#v, want minimal candidate output", frame.RecordSet.Projection)
	}
	traces, err := store.ListRuntimeTraces(context.Background(), RuntimeTraceListFilter{RunID: frame.RecordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListRuntimeTraces() error = %v", err)
	}
	if !containsTraceType(traces, "runtime_hook_binding") {
		t.Fatalf("traces = %#v, want runtime hook bridge trace", traces)
	}
	usages, err := store.ListUsage(context.Background(), UsageListFilter{RunID: frame.RecordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	if !containsUsageResource(usages, "runtime_hook") {
		t.Fatalf("usages = %#v, want runtime_hook usage", usages)
	}
	runs, err := store.ListTaskRuns(context.Background(), TaskRunListFilter{WorkspaceID: "workspace-graph"})
	if err != nil {
		t.Fatalf("ListTaskRuns() error = %v", err)
	}
	if len(runs) != 1 {
		t.Fatalf("ListTaskRuns() = %d, want 1", len(runs))
	}
}

func TestEinoGraphTurnExecutorWrapsBaseExecutor(t *testing.T) {
	store := newGraphMemoryRuntimeStore()
	executor := &graphStubTurnExecutor{}
	wrapped := NewEinoGraphTurnExecutor(executor, EinoGraphTurnExecutorOptions{Store: store})
	prepared, err := wrapped.Prepare(context.Background(), RuntimeState{RequestID: "req-wrap", SessionID: "sess-wrap", Turn: 1}, graphTestSpec(), []adk.Message{einomessage.UserMessage("wrap")})
	if err != nil {
		t.Fatalf("Prepare() error = %v", err)
	}
	if prepared == nil || len(prepared.Messages) != 1 {
		t.Fatalf("Prepare() = %#v, want prepared execution with messages", prepared)
	}
	if executor.called != 1 {
		t.Fatalf("base executor called %d times, want 1", executor.called)
	}
	if len(store.runs) != 1 || len(store.steps) != 1 || len(store.traces) != 1 || len(store.usages) != 1 || len(store.projections) != 1 {
		t.Fatalf("store counts runs=%d steps=%d traces=%d usages=%d projections=%d, want all one", len(store.runs), len(store.steps), len(store.traces), len(store.usages), len(store.projections))
	}
}

func TestEinoGraphTerminalProjectorPersistsSafeOutcome(t *testing.T) {
	store := newGraphMemoryRuntimeStore()
	graph := EinoGraphFoundation{
		Executor: &graphStubTurnExecutor{},
		Store:    store,
		Now: func() time.Time {
			return time.Date(2026, 5, 6, 9, 0, 0, 0, time.UTC)
		},
	}
	frame, err := graph.Run(context.Background(), &RuntimeGraphFrame{
		State:    RuntimeState{RequestID: "req-terminal", SessionID: "sess-terminal", Turn: 1},
		Spec:     graphTestSpec(),
		Messages: []adk.Message{einomessage.UserMessage("terminal")},
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if frame.Prepared == nil || frame.Prepared.TerminalProjector == nil {
		t.Fatalf("prepared execution missing terminal projector: %#v", frame.Prepared)
	}
	recorder := NewRuntimeCallbackRecorder(RuntimeCallbackRecorderConfig{ProviderName: "Test Provider", ModelName: "test-model"})
	recorder.record(RuntimeComponentCallbackEvent{
		Component:        runtimeCallbackComponentModel,
		Node:             "graph_native_chat_model",
		Provider:         "Test Provider",
		ResourceName:     "test-model",
		Status:           "success",
		DurationMS:       12,
		InputCount:       2,
		OutputRuneCount:  6,
		PromptTokens:     11,
		CompletionTokens: 5,
		TotalTokens:      16,
		Metadata:         map[string]any{"safe_label": "model_callback"},
		ObservedAt:       time.Date(2026, 5, 6, 9, 0, 1, 0, time.UTC),
	})
	recorder.record(RuntimeComponentCallbackEvent{
		Component:       runtimeCallbackComponentTool,
		Node:            "lookup",
		Provider:        "eino",
		ResourceName:    "lookup",
		Status:          "success",
		DurationMS:      7,
		InputRuneCount:  18,
		OutputRuneCount: 15,
		Metadata:        map[string]any{"safe_label": "tool_callback", "argument_keys": []string{"query"}},
		ObservedAt:      time.Date(2026, 5, 6, 9, 0, 2, 0, time.UTC),
	})
	frame.Prepared.CallbackRecorder = recorder
	frame.Prepared.TerminalProjector.Callbacks = recorder
	err = frame.Prepared.ProjectTerminalOutcome(context.Background(), RuntimeTerminalOutcome{
		Status:          RuntimeTerminalStatusCompleted,
		Content:         "answer that mentions api_key=secret and Authorization: Bearer sk-test",
		ToolSideEffects: true,
		Metadata:        map[string]any{"respond_stage": "test"},
	})
	if err != nil {
		t.Fatalf("ProjectTerminalOutcome() error = %v", err)
	}
	traces, err := store.ListRuntimeTraces(context.Background(), RuntimeTraceListFilter{RunID: frame.RecordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListRuntimeTraces() error = %v", err)
	}
	var terminalTrace RuntimeTrace
	for _, trace := range traces {
		if trace.TraceType == "terminal_output_summary" {
			terminalTrace = trace
			break
		}
	}
	if terminalTrace.ID == "" {
		t.Fatalf("terminal trace not found in %#v", traces)
	}
	if !containsTraceType(traces, "callback_summary") {
		t.Fatalf("traces = %#v, want callback summary trace", traces)
	}
	if !containsTraceType(traces, "eino_model_callback") || !containsTraceType(traces, "eino_tool_callback") {
		t.Fatalf("traces = %#v, want fine-grained model and tool callback traces", traces)
	}
	if strings.Contains(fmt.Sprint(terminalTrace.RedactedPayload), "sk-test") || strings.Contains(fmt.Sprint(terminalTrace.RedactedPayload), "secret") {
		t.Fatalf("terminal trace leaked raw credential-like content: %#v", terminalTrace.RedactedPayload)
	}
	usages, err := store.ListUsage(context.Background(), UsageListFilter{RunID: frame.RecordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListUsage() error = %v", err)
	}
	if !containsUsageResource(usages, "runtime_output") || !containsUsageResource(usages, "eino_callback") {
		t.Fatalf("usages = %#v, want terminal runtime_output and eino_callback usage", usages)
	}
	if !containsUsageResource(usages, "model") || !containsUsageResource(usages, "tool") {
		t.Fatalf("usages = %#v, want fine-grained model and tool usage", usages)
	}
	projections, err := store.ListProjectionCandidates(context.Background(), ProjectionCandidateListFilter{RunID: frame.RecordSet.Run.ID})
	if err != nil {
		t.Fatalf("ListProjectionCandidates() error = %v", err)
	}
	if len(projections) != 2 || !containsProjectionKind(projections, "terminal_output") {
		t.Fatalf("projections = %#v, want terminal output projection", projections)
	}
	events, err := store.ListLifecycleEventsByRun(context.Background(), frame.RecordSet.Run.ID)
	if err != nil {
		t.Fatalf("ListLifecycleEventsByRun() error = %v", err)
	}
	if !containsLifecycleEvent(events, "run_terminal_observed") || !containsLifecycleEvent(events, "step_terminal_observed") {
		t.Fatalf("events = %#v, want run and step terminal lifecycle events", events)
	}
}

func TestRuntimeGraphCheckpointRefFromWait(t *testing.T) {
	wait := &WaitState{
		Stage:       StageTurnExecution,
		ResumeToken: "resume-token",
	}
	ref := RuntimeGraphCheckpointRefFromWait(
		RuntimeState{RequestID: "req-checkpoint", SessionID: "sess-checkpoint", Turn: 1},
		&MinimalPersistenceRecordSet{Run: TaskRun{ID: "run-checkpoint"}},
		wait,
	)
	if ref.CheckpointID != "athena_runtime_graph:sess-checkpoint:req-checkpoint:run-checkpoint:turn_execution:resume-token" {
		t.Fatalf("checkpoint_id = %q", ref.CheckpointID)
	}
	if ref.Stage != StageTurnExecution || ref.ResumeToken != "resume-token" || ref.RunID != "run-checkpoint" {
		t.Fatalf("checkpoint ref = %#v, want wait/resume mapping fields", ref)
	}
}

func graphTestSpec() *ExecutionSpec {
	return &ExecutionSpec{
		Skill: SkillSpec{PrimarySkill: "general_chat"},
		Tools: ToolSpec{AllowedTools: []string{"demo_search"}},
		Inference: InferenceSpec{
			Goal: "test graph runtime",
			StructuredOutput: &StructuredOutputContract{
				ContractID: "structured-output.v1",
				Requested:  true,
			},
		},
		Metadata: ExecutionMetadata{
			Governance: &GovernanceDecision{Decision: GovernanceDecisionAllow, Reason: "test"},
			Constraints: map[string]any{
				"task_id":             "task-graph",
				"task_type":           "runtime_task",
				"task_kind":           "chat",
				"workspace_id":        "workspace-graph",
				"app_instance_id":     "app-graph",
				"scene":               "general",
				"desired_output_mode": "text",
				"runtime_contract_id": "contract-graph",
			},
		},
	}
}

func containsLifecycleEvent(events []TaskRunLifecycleEvent, eventType string) bool {
	for _, event := range events {
		if event.EventType == eventType {
			return true
		}
	}
	return false
}

func containsUsageResource(usages []Usage, resourceType string) bool {
	for _, usage := range usages {
		if usage.ResourceType == resourceType && usage.Amount > 0 {
			return true
		}
	}
	return false
}

func containsProjectionKind(projections []ProjectionCandidate, candidateKind string) bool {
	for _, projection := range projections {
		if projection.CandidateKind == candidateKind {
			return true
		}
	}
	return false
}

type graphMemoryRuntimeStore struct {
	runs          map[string]TaskRun
	steps         map[string]TaskStep
	events        map[string]TaskRunLifecycleEvent
	traces        map[string]RuntimeTrace
	usages        map[string]Usage
	projections   map[string]ProjectionCandidate
	contracts     map[string]RuntimeContract
	hookBindings  map[string]HookBinding
	taskTypeIndex map[string]TaskTypeRegistration
}

func newGraphMemoryRuntimeStore() *graphMemoryRuntimeStore {
	return &graphMemoryRuntimeStore{
		runs:          map[string]TaskRun{},
		steps:         map[string]TaskStep{},
		events:        map[string]TaskRunLifecycleEvent{},
		traces:        map[string]RuntimeTrace{},
		usages:        map[string]Usage{},
		projections:   map[string]ProjectionCandidate{},
		contracts:     map[string]RuntimeContract{},
		hookBindings:  map[string]HookBinding{},
		taskTypeIndex: map[string]TaskTypeRegistration{},
	}
}

func (s *graphMemoryRuntimeStore) AutoMigrate(context.Context) error { return nil }

func (s *graphMemoryRuntimeStore) CreateTaskRun(_ context.Context, run TaskRun) (TaskRun, error) {
	if err := validateTaskRun(run); err != nil {
		return TaskRun{}, err
	}
	s.runs[run.ID] = run
	return run, nil
}

func (s *graphMemoryRuntimeStore) GetTaskRun(_ context.Context, id string) (TaskRun, bool, error) {
	run, ok := s.runs[id]
	return run, ok, nil
}

func (s *graphMemoryRuntimeStore) ListTaskRuns(_ context.Context, filter TaskRunListFilter) ([]TaskRun, error) {
	var out []TaskRun
	for _, run := range s.runs {
		if filter.WorkspaceID != "" && run.WorkspaceID != filter.WorkspaceID {
			continue
		}
		if filter.Status != "" && run.Status != filter.Status {
			continue
		}
		out = append(out, run)
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateTaskStep(_ context.Context, step TaskStep) (TaskStep, error) {
	if err := validateTaskStep(step); err != nil {
		return TaskStep{}, err
	}
	s.steps[step.ID] = step
	return step, nil
}

func (s *graphMemoryRuntimeStore) GetTaskStep(_ context.Context, id string) (TaskStep, bool, error) {
	step, ok := s.steps[id]
	return step, ok, nil
}

func (s *graphMemoryRuntimeStore) ListTaskSteps(_ context.Context, runID string) ([]TaskStep, error) {
	var out []TaskStep
	for _, step := range s.steps {
		if step.RunID == runID {
			out = append(out, step)
		}
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateLifecycleEvent(_ context.Context, event TaskRunLifecycleEvent) (TaskRunLifecycleEvent, error) {
	if err := validateLifecycleEvent(event); err != nil {
		return TaskRunLifecycleEvent{}, err
	}
	if event.ID == "" {
		event.ID = "event-" + event.EventType
	}
	s.events[event.ID] = event
	return event, nil
}

func (s *graphMemoryRuntimeStore) GetLifecycleEvent(_ context.Context, id string) (TaskRunLifecycleEvent, bool, error) {
	event, ok := s.events[id]
	return event, ok, nil
}

func (s *graphMemoryRuntimeStore) ListLifecycleEventsByRun(_ context.Context, runID string) ([]TaskRunLifecycleEvent, error) {
	var out []TaskRunLifecycleEvent
	for _, event := range s.events {
		if event.RunID == runID {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) ListLifecycleEventsBySubject(_ context.Context, subjectType string, subjectID string) ([]TaskRunLifecycleEvent, error) {
	var out []TaskRunLifecycleEvent
	for _, event := range s.events {
		if event.SubjectType == subjectType && event.SubjectID == subjectID {
			out = append(out, event)
		}
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateRuntimeTrace(_ context.Context, trace RuntimeTrace) (RuntimeTrace, error) {
	if err := validateRuntimeTrace(trace); err != nil {
		return RuntimeTrace{}, err
	}
	if trace.ID == "" {
		trace.ID = fmt.Sprintf("trace-%s-%d", trace.RunID, len(s.traces)+1)
	}
	s.traces[trace.ID] = trace
	return trace, nil
}

func (s *graphMemoryRuntimeStore) GetRuntimeTrace(_ context.Context, id string) (RuntimeTrace, bool, error) {
	trace, ok := s.traces[id]
	return trace, ok, nil
}

func (s *graphMemoryRuntimeStore) ListRuntimeTraces(_ context.Context, filter RuntimeTraceListFilter) ([]RuntimeTrace, error) {
	var out []RuntimeTrace
	for _, trace := range s.traces {
		if filter.RunID != "" && trace.RunID != filter.RunID {
			continue
		}
		if filter.StepID != "" && trace.StepID != filter.StepID {
			continue
		}
		out = append(out, trace)
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateUsage(_ context.Context, usage Usage) (Usage, error) {
	if err := validateUsage(usage); err != nil {
		return Usage{}, err
	}
	if usage.ID == "" {
		usage.ID = fmt.Sprintf("usage-%s-%d", usage.RunID, len(s.usages)+1)
	}
	s.usages[usage.ID] = usage
	return usage, nil
}

func (s *graphMemoryRuntimeStore) GetUsage(_ context.Context, id string) (Usage, bool, error) {
	usage, ok := s.usages[id]
	return usage, ok, nil
}

func (s *graphMemoryRuntimeStore) ListUsage(_ context.Context, filter UsageListFilter) ([]Usage, error) {
	var out []Usage
	for _, usage := range s.usages {
		if filter.RunID != "" && usage.RunID != filter.RunID {
			continue
		}
		if filter.StepID != "" && usage.StepID != filter.StepID {
			continue
		}
		out = append(out, usage)
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateProjectionCandidate(_ context.Context, projection ProjectionCandidate) (ProjectionCandidate, error) {
	projection = normalizeProjectionCandidate(projection)
	if err := validateProjectionCandidate(projection); err != nil {
		return ProjectionCandidate{}, err
	}
	if projection.ID == "" {
		projection.ID = fmt.Sprintf("projection-%s-%d", projection.RunID, len(s.projections)+1)
	}
	s.projections[projection.ID] = projection
	return projection, nil
}

func (s *graphMemoryRuntimeStore) GetProjectionCandidate(_ context.Context, id string) (ProjectionCandidate, bool, error) {
	projection, ok := s.projections[id]
	return projection, ok, nil
}

func (s *graphMemoryRuntimeStore) ListProjectionCandidates(_ context.Context, filter ProjectionCandidateListFilter) ([]ProjectionCandidate, error) {
	var out []ProjectionCandidate
	for _, projection := range s.projections {
		if filter.RunID != "" && projection.RunID != filter.RunID {
			continue
		}
		if filter.StepID != "" && projection.StepID != filter.StepID {
			continue
		}
		out = append(out, projection)
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateRuntimeContract(_ context.Context, contract RuntimeContract) (RuntimeContract, error) {
	if err := validateRuntimeContract(contract); err != nil {
		return RuntimeContract{}, err
	}
	s.contracts[contract.ID] = contract
	return contract, nil
}

func (s *graphMemoryRuntimeStore) PutRuntimeContract(_ context.Context, contract RuntimeContract) (RuntimeContract, error) {
	if err := validateRuntimeContract(contract); err != nil {
		return RuntimeContract{}, err
	}
	if contract.ID == "" {
		contract.ID = fmt.Sprintf("contract-%d", len(s.contracts)+1)
	}
	s.contracts[contract.ID] = contract
	return contract, nil
}

func (s *graphMemoryRuntimeStore) GetRuntimeContract(_ context.Context, id string) (RuntimeContract, bool, error) {
	contract, ok := s.contracts[id]
	return contract, ok, nil
}

func (s *graphMemoryRuntimeStore) ListRuntimeContracts(_ context.Context, filter RuntimeContractListFilter) ([]RuntimeContract, error) {
	var out []RuntimeContract
	for _, contract := range s.contracts {
		if filter.TaskType != "" && contract.TaskType != filter.TaskType {
			continue
		}
		if filter.Status != "" && contract.Status != filter.Status {
			continue
		}
		out = append(out, contract)
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateTaskTypeRegistration(_ context.Context, item TaskTypeRegistration) (TaskTypeRegistration, error) {
	if err := validateTaskTypeRegistration(item); err != nil {
		return TaskTypeRegistration{}, err
	}
	if item.ID == "" {
		item.ID = "task-type-" + item.TypeKey
	}
	s.taskTypeIndex[item.TypeKey] = item
	return item, nil
}

func (s *graphMemoryRuntimeStore) PutTaskTypeRegistration(_ context.Context, item TaskTypeRegistration) (TaskTypeRegistration, error) {
	if err := validateTaskTypeRegistration(item); err != nil {
		return TaskTypeRegistration{}, err
	}
	if item.ID == "" {
		item.ID = "task-type-" + item.TypeKey
	}
	s.taskTypeIndex[item.TypeKey] = item
	return item, nil
}

func (s *graphMemoryRuntimeStore) GetTaskTypeRegistration(_ context.Context, id string) (TaskTypeRegistration, bool, error) {
	for _, item := range s.taskTypeIndex {
		if item.ID == id {
			return item, true, nil
		}
	}
	return TaskTypeRegistration{}, false, nil
}

func (s *graphMemoryRuntimeStore) GetTaskTypeRegistrationByKey(_ context.Context, typeKey string) (TaskTypeRegistration, bool, error) {
	item, ok := s.taskTypeIndex[typeKey]
	return item, ok, nil
}

func (s *graphMemoryRuntimeStore) ListTaskTypeRegistrations(_ context.Context, filter TaskTypeRegistrationListFilter) ([]TaskTypeRegistration, error) {
	var out []TaskTypeRegistration
	for _, item := range s.taskTypeIndex {
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *graphMemoryRuntimeStore) CreateHookBinding(_ context.Context, hook HookBinding) (HookBinding, error) {
	if err := validateHookBinding(hook); err != nil {
		return HookBinding{}, err
	}
	if hook.ID == "" {
		hook.ID = fmt.Sprintf("hook-%d", len(s.hookBindings)+1)
	}
	s.hookBindings[hook.ID] = hook
	return hook, nil
}

func (s *graphMemoryRuntimeStore) PutHookBinding(_ context.Context, hook HookBinding) (HookBinding, error) {
	if err := validateHookBinding(hook); err != nil {
		return HookBinding{}, err
	}
	if hook.ID == "" {
		hook.ID = fmt.Sprintf("hook-%d", len(s.hookBindings)+1)
	}
	s.hookBindings[hook.ID] = hook
	return hook, nil
}

func (s *graphMemoryRuntimeStore) GetHookBinding(_ context.Context, id string) (HookBinding, bool, error) {
	hook, ok := s.hookBindings[id]
	return hook, ok, nil
}

func (s *graphMemoryRuntimeStore) ListHookBindings(_ context.Context, filter HookBindingListFilter) ([]HookBinding, error) {
	var out []HookBinding
	for _, hook := range s.hookBindings {
		if filter.ContractID != "" && hook.ContractID != filter.ContractID {
			continue
		}
		if filter.HookPoint != "" && hook.HookPoint != filter.HookPoint {
			continue
		}
		if filter.Enabled != nil && hook.Enabled != *filter.Enabled {
			continue
		}
		out = append(out, hook)
	}
	return out, nil
}
