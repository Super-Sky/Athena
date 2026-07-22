package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/common/ut"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/runtime"
	runtimetask "moss/internal/runtime/task"
	"moss/internal/session"
)

func TestParseAgentRunStartRequestAcceptsOpenAITools(t *testing.T) {
	ctx := newAgentRunJSONRequestContext(`{
		"goal":"Build a concise fund briefing",
		"success_criteria":["has risks","has next action"],
		"budget":{"max_duration_ms":30000,"max_model_calls":4,"max_tool_calls":8,"max_tokens":4096},
		"tools":[
			{"type":"function","function":{"name":"web_search","parameters":{"type":"object"}}},
			"calculator"
		],
		"tool_choice":{"type":"function","function":{"name":"web_search"}},
		"resume_token":"resume-1",
		"supplement":{"data":{"risk":"medium"}}
	}`)

	req, err := parseAgentRunStartRequest(ctx)
	if err != nil {
		t.Fatalf("parseAgentRunStartRequest() error = %v", err)
	}
	names := agentRunToolNames(req.Tools)
	if strings.Join(names, ",") != "web_search,calculator" {
		t.Fatalf("tool names = %#v, want web_search and calculator", names)
	}
	if len(req.CanonicalTools) != 2 || req.CanonicalTools[0].Function.Name != "web_search" {
		t.Fatalf("canonical tools = %#v, want OpenAI tools converted", req.CanonicalTools)
	}
	if req.CanonicalToolChoice.ToolName != "web_search" {
		t.Fatalf("canonical tool choice = %#v, want web_search", req.CanonicalToolChoice)
	}
	if req.Supplement == nil || req.Supplement.Outcome != runtime.SupplementOutcomeProvided {
		t.Fatalf("unexpected supplement = %#v", req.Supplement)
	}
	if req.Supplement.Resume == nil || req.Supplement.Resume.ResumeToken != "resume-1" {
		t.Fatalf("unexpected resume context = %#v", req.Supplement.Resume)
	}
	if budget, err := runtime.ParseExecutionBudget(req.Budget); err != nil || budget.MaxModelCalls != 4 {
		t.Fatalf("unexpected execution budget = %#v, error=%v", req.Budget, err)
	}
}

func TestParseAgentRunStartRequestRejectsInvalidBudget(t *testing.T) {
	for _, body := range []string{
		`{"goal":"test","budget":{"max_model_calls":1.5}}`,
		`{"goal":"test","budget":{"max_model_call":3}}`,
	} {
		_, err := parseAgentRunStartRequest(newAgentRunJSONRequestContext(body))
		if err == nil || !strings.Contains(err.Error(), "budget.max_model_call") {
			t.Fatalf("parse error = %v, want invalid model call budget", err)
		}
	}
}

func TestParseAgentRunStartRequestRejectsInvalidToolContracts(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "non object schema",
			body: `{"goal":"test","tools":[{"type":"function","function":{"name":"lookup","parameters":{"type":"string"}}}]}`,
			want: "parameters must use an object JSON schema",
		},
		{
			name: "duplicate name",
			body: `{"goal":"test","tools":["lookup","lookup"]}`,
			want: "is duplicated",
		},
		{
			name: "unknown selected tool",
			body: `{"goal":"test","tools":["lookup"],"tool_choice":{"type":"function","function":{"name":"other"}}}`,
			want: "is not declared in tools",
		},
		{
			name: "required without tools",
			body: `{"goal":"test","tool_choice":"required"}`,
			want: "requires at least one tool",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseAgentRunStartRequest(newAgentRunJSONRequestContext(test.body))
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Fatalf("parse error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestParseAgentRunStartRequestDefaultsToolChoiceByToolPresence(t *testing.T) {
	withoutTools, err := parseAgentRunStartRequest(newAgentRunJSONRequestContext(`{"goal":"test"}`))
	if err != nil {
		t.Fatalf("parse without tools error = %v", err)
	}
	if withoutTools.CanonicalToolChoice.Kind != "none" {
		t.Fatalf("tool choice without tools = %#v, want none", withoutTools.CanonicalToolChoice)
	}
	withTools, err := parseAgentRunStartRequest(newAgentRunJSONRequestContext(`{"goal":"test","tools":["lookup_profile"]}`))
	if err != nil {
		t.Fatalf("parse with tools error = %v", err)
	}
	if withTools.CanonicalToolChoice.Kind != "auto" {
		t.Fatalf("tool choice with tools = %#v, want auto", withTools.CanonicalToolChoice)
	}
}

func TestAgentRunToolTranscriptDTOsCorrelateResults(t *testing.T) {
	calls, results := agentRunToolTranscriptDTOs([]runtime.ToolCall{
		{
			Index:     0,
			Round:     0,
			ID:        "call_lookup",
			Type:      runtime.ToolTypeFunction,
			Name:      "lookup",
			Arguments: `{"query":"athena"}`,
			Status:    runtime.ToolCallStatusCompleted,
			Result: &runtime.ToolResult{
				ToolCallID: "call_lookup",
				Name:       "lookup",
				Content:    `{"answer":"ok"}`,
			},
		},
	})
	if len(calls) != 1 || calls[0].ID != "call_lookup" || calls[0].Function.Arguments != `{"query":"athena"}` {
		t.Fatalf("tool calls = %#v, want OpenAI-compatible call", calls)
	}
	if len(results) != 1 || results[0].Role != "tool" || results[0].ToolCallID != calls[0].ID {
		t.Fatalf("tool results = %#v, want correlated tool message", results)
	}
	messages := agentRunToolMessages([]runtime.ToolCall{
		{
			Index:     0,
			Round:     0,
			ID:        "call_lookup",
			Type:      runtime.ToolTypeFunction,
			Name:      "lookup",
			Arguments: `{"query":"athena"}`,
			Status:    runtime.ToolCallStatusCompleted,
			Result:    &runtime.ToolResult{ToolCallID: "call_lookup", Name: "lookup", Content: `{"answer":"ok"}`},
		},
	}, "done")
	if len(messages) != 3 || messages[0].Role != "assistant" || messages[1].Role != "tool" || messages[2].Role != "assistant" {
		t.Fatalf("messages = %#v, want assistant/tool/final assistant order", messages)
	}
	if messages[1].ToolCallID != messages[0].ToolCalls[0].ID {
		t.Fatalf("tool message call id = %q, want %q", messages[1].ToolCallID, messages[0].ToolCalls[0].ID)
	}
}

func TestAgentRunEndpointsCreateReadTraceAndCancel(t *testing.T) {
	store := &testRuntimeReadStore{}
	httpServer := newAgentRunHTTPServer(t, store)
	body := bytes.NewBufferString(`{
		"goal":"Summarize portfolio risk and next action",
		"workspace_id":"workspace-agent-run",
		"app_instance_id":"fund-assistant",
		"context_assets":[{"asset_id":"policy.guardrail","asset_type":"policy","content":{"summary":"test"}}]
	}`)

	create := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/agent/runs", &ut.Body{Body: body, Len: body.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if create.Code != consts.StatusCreated {
		t.Fatalf("create status = %d, want %d; body=%s", create.Code, consts.StatusCreated, create.Body.String())
	}
	var created agentRunResponse
	if err := json.Unmarshal(create.Body.Bytes(), &created); err != nil {
		t.Fatalf("unmarshal create response error = %v; body=%s", err, create.Body.String())
	}
	if created.RunID == "" || created.Status != "completed" || !created.TraceAvailable {
		t.Fatalf("unexpected create response = %#v", created)
	}
	if created.StopReason != string(runtime.ExecutionStopSuccess) {
		t.Fatalf("stop reason = %q, want %q", created.StopReason, runtime.ExecutionStopSuccess)
	}
	if created.TraceSummary == nil || created.TraceSummary.TraceCount < 2 {
		t.Fatalf("trace summary = %#v, want writer and terminal traces", created.TraceSummary)
	}
	if len(created.Messages) != 1 || created.Messages[0].Role != "assistant" || created.Messages[0].Content == nil {
		t.Fatalf("messages = %#v, want final assistant message", created.Messages)
	}

	read := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/agent/runs/"+created.RunID, nil)
	if read.Code != consts.StatusOK {
		t.Fatalf("read status = %d, want %d; body=%s", read.Code, consts.StatusOK, read.Body.String())
	}
	if !strings.Contains(read.Body.String(), `"trace_available":true`) {
		t.Fatalf("read body missing trace availability: %s", read.Body.String())
	}

	trace := ut.PerformRequest(httpServer.engine.Engine, http.MethodGet, "/api/agent/runs/"+created.RunID+"/trace", nil)
	if trace.Code != consts.StatusOK {
		t.Fatalf("trace status = %d, want %d; body=%s", trace.Code, consts.StatusOK, trace.Body.String())
	}
	if !strings.Contains(trace.Body.String(), `"terminal_output_summary"`) {
		t.Fatalf("trace body missing terminal output summary: %s", trace.Body.String())
	}

	resumeBody := bytes.NewBufferString(`{
		"goal":"Continue after supplied facts",
		"session_id":"` + created.SessionID + `",
		"supplement":{"data":{"risk":"medium"}}
	}`)
	resume := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/agent/runs/"+created.RunID+"/resume", &ut.Body{Body: resumeBody, Len: resumeBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if resume.Code != consts.StatusCreated {
		t.Fatalf("resume status = %d, want %d; body=%s", resume.Code, consts.StatusCreated, resume.Body.String())
	}
	var resumed agentRunResponse
	if err := json.Unmarshal(resume.Body.Bytes(), &resumed); err != nil {
		t.Fatalf("unmarshal resume response error = %v; body=%s", err, resume.Body.String())
	}
	if resumed.ResumedFromRunID != created.RunID || resumed.RunID == "" || resumed.RunID == created.RunID {
		t.Fatalf("unexpected resume response = %#v", resumed)
	}

	missingResumeBody := bytes.NewBufferString(`{"goal":"Continue anyway"}`)
	missingResume := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/agent/runs/missing-run/resume", &ut.Body{Body: missingResumeBody, Len: missingResumeBody.Len()}, ut.Header{Key: "Content-Type", Value: "application/json"})
	if missingResume.Code != consts.StatusNotFound {
		t.Fatalf("missing resume status = %d, want %d; body=%s", missingResume.Code, consts.StatusNotFound, missingResume.Body.String())
	}

	cancel := ut.PerformRequest(httpServer.engine.Engine, http.MethodPost, "/api/agent/runs/"+created.RunID+"/cancel", nil)
	if cancel.Code != consts.StatusConflict {
		t.Fatalf("cancel status = %d, want %d; body=%s", cancel.Code, consts.StatusConflict, cancel.Body.String())
	}
	if !strings.Contains(cancel.Body.String(), `"stop_reason":"success"`) {
		t.Fatalf("cancel body = %s, want stable terminal reason", cancel.Body.String())
	}
}

func TestAgentRunTraceUsesStableStopReasonAndLatestRunStatus(t *testing.T) {
	t.Parallel()

	readout := agentRunTraceReadout{
		Run: runtimeRunDTO{Status: runtime.TaskRunStatusCompleted},
		Events: []runtimeLifecycleEventDTO{
			{SubjectType: runtime.LifecycleSubjectRun, ToStatus: runtime.TaskRunStatusCompleted, Reason: "minimal_persistence_complete"},
			{EventType: "run_terminal_observed", SubjectType: runtime.LifecycleSubjectRun, ToStatus: runtime.TaskRunStatusFailed, Reason: string(runtime.ExecutionStopUnrecoverableError)},
			{EventType: "run_completed", SubjectType: runtime.LifecycleSubjectRun, ToStatus: runtime.TaskRunStatusCompleted, Reason: "minimal_persistence_complete"},
		},
	}
	readout.Run.Status = agentRunEffectiveStatus(readout.Run.Status, readout.Events)
	if readout.Run.Status != runtime.TaskRunStatusFailed {
		t.Fatalf("effective status = %q, want failed", readout.Run.Status)
	}
	if got := agentRunStopReasonFromTrace(readout); got != string(runtime.ExecutionStopUnrecoverableError) {
		t.Fatalf("stop reason = %q, want unrecoverable_error", got)
	}

	legacy := agentRunTraceReadout{
		Run: runtimeRunDTO{Status: runtime.TaskRunStatusCompleted},
	}
	if got := agentRunStopReasonFromTrace(legacy); got != string(runtime.ExecutionStopSuccess) {
		t.Fatalf("legacy stop reason = %q, want success", got)
	}

	incomplete := agentRunTraceReadout{
		Run: runtimeRunDTO{Status: runtime.TaskRunStatusCompleted},
		Events: []runtimeLifecycleEventDTO{
			{EventType: "run_running", SubjectType: runtime.LifecycleSubjectRun, ToStatus: runtime.TaskRunStatusRunning},
			{EventType: "run_completed", SubjectType: runtime.LifecycleSubjectRun, ToStatus: runtime.TaskRunStatusCompleted, Reason: "minimal_persistence_complete"},
		},
	}
	incomplete.Run.Status = agentRunEffectiveStatus(incomplete.Run.Status, incomplete.Events)
	if incomplete.Run.Status != runtime.TaskRunStatusRunning || agentRunStopReasonFromTrace(incomplete) != "" {
		t.Fatalf("incomplete readout = %#v, want running without stop reason", incomplete)
	}

	stepOnly := agentRunTraceReadout{
		Run: runtimeRunDTO{Status: runtime.TaskRunStatusRunning},
		Events: []runtimeLifecycleEventDTO{{
			EventType: "step_terminal_observed", SubjectType: runtime.LifecycleSubjectStep,
			Reason: string(runtime.ExecutionStopUnrecoverableError),
		}},
	}
	if reason, authoritative := agentRunTerminalStopReasonFromTrace(stepOnly); authoritative || reason != "" {
		t.Fatalf("step-only terminal reason = %q, %t, want empty, false", reason, authoritative)
	}
}

func TestAgentRunResponseKeepsLocalWaitReasonWithoutAuthoritativeTerminalEvent(t *testing.T) {
	t.Parallel()

	readout := agentRunTraceReadout{
		Run: runtimeRunDTO{Status: runtime.TaskRunStatusCompleted},
		Events: []runtimeLifecycleEventDTO{{
			EventType: "run_completed", SubjectType: runtime.LifecycleSubjectRun,
			ToStatus: runtime.TaskRunStatusCompleted, Reason: "minimal_persistence_complete",
		}},
	}
	localReason := string(runtime.ExecutionStopAwaitingInput)
	if reason, authoritative := agentRunTerminalStopReasonFromTrace(readout); authoritative {
		localReason = reason
	}
	if localReason != string(runtime.ExecutionStopAwaitingInput) {
		t.Fatalf("response reason = %q, want awaiting_input", localReason)
	}
}

func TestAgentRunOpenErrorUsesStableStopReasons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		err  error
		want runtime.ExecutionStopReason
	}{
		{name: "generic", err: errors.New("open failed"), want: runtime.ExecutionStopUnrecoverableError},
		{name: "deadline", err: context.DeadlineExceeded, want: runtime.ExecutionStopDeadlineExceeded},
		{name: "cancelled", err: context.Canceled, want: runtime.ExecutionStopCancelled},
		{name: "pending", err: &appcore.PendingWaitError{}, want: runtime.ExecutionStopAwaitingInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			response := agentRunOpenErrorResponse("request", agentRunStartRequest{}, agentRunExecutionOptions{}, test.err)
			if response.StopReason != string(test.want) {
				t.Fatalf("stop reason = %q, want %q", response.StopReason, test.want)
			}
		})
	}
}

func TestAgentRunErrorStatusPreservesCancellationSemantics(t *testing.T) {
	t.Parallel()

	if got := agentRunErrorStatus(context.DeadlineExceeded); got != string(runtime.RequestStatusTimedOut) {
		t.Fatalf("deadline status = %q, want timed_out", got)
	}
	if got := agentRunErrorStatus(context.Canceled); got != runtime.TaskRunStatusCancelled {
		t.Fatalf("cancelled status = %q, want cancelled", got)
	}
	if got := agentRunStopReason(string(runtime.RequestStatusPolicyRejected), "policy detail"); got != string(runtime.ExecutionStopGovernanceDenied) {
		t.Fatalf("policy stop reason = %q, want governance_denied", got)
	}
}

func TestAgentRunRuntimeTaskTypeDefaultsToRegisteredChat(t *testing.T) {
	t.Parallel()

	for _, taskType := range []string{"", agentRunContractTaskType} {
		req := agentRunStartRequest{TaskType: taskType}
		if got := agentRunRuntimeTaskType(req); got != defaultAgentRunRuntimeTaskType {
			t.Fatalf("agentRunRuntimeTaskType(%q) = %q, want %q", taskType, got, defaultAgentRunRuntimeTaskType)
		}
	}
}

func newAgentRunHTTPServer(t *testing.T, store runtime.RuntimePersistenceStore) *HTTPServer {
	t.Helper()
	cfg := config.Config{
		Server: config.ServerConfig{HTTPPort: 8080},
		ControlPlane: config.ControlPlaneConfig{
			StorePath: t.TempDir() + "/controlplane/overrides.json",
		},
		Runtime: config.RuntimeConfig{
			MaxConcurrentRequests:     4,
			MaxConcurrentTools:        2,
			RequestTimeoutSeconds:     30,
			DeferredQueueLimit:        4,
			ClosedTokenTTLSecs:        3600,
			SkillPackageRevisionLimit: 4,
			SharedRootDir:             "shared",
		},
	}
	application := appcore.NewServiceWithRuntimeStore(cfg, nil, nil, nil, store)
	application.FastPath = &agentRunTestFastPath{store: store}
	return NewHTTPServer(cfg, application)
}

func newAgentRunJSONRequestContext(body string) *hertzapp.RequestContext {
	ctx := hertzapp.NewContext(0)
	ctx.Request.Header.SetContentTypeBytes([]byte("application/json"))
	ctx.Request.SetBodyString(body)
	return ctx
}

type agentRunTestFastPath struct {
	store runtime.RuntimePersistenceStore
}

func (f *agentRunTestFastPath) Evaluate(ctx context.Context, _ *session.Session, req appcore.ChatRequest) (*appcore.FastPathResult, error) {
	recordSet, err := (runtime.PersistenceWriter{Store: f.store}).WriteMinimalRun(ctx, runtime.MinimalPersistenceInput{
		Task: &runtimetask.RuntimeTask{
			TaskID:        "test-agent-run-task",
			TaskType:      req.TaskType,
			TaskSubtype:   req.TaskSubtype,
			InputKind:     req.TaskType,
			Scene:         req.Scene,
			WorkspaceID:   req.WorkspaceID,
			AppInstanceID: req.AppInstanceID,
			UserGoal:      req.Query,
			OutputMode:    req.DesiredOutputMode,
			GlobalContext: req.GlobalContext,
			AppContext:    req.AppContext,
			InputPayload:  req.InputPayload,
		},
		Metadata: map[string]any{
			"test_fast_path": true,
		},
	})
	if err != nil {
		return nil, err
	}
	return &appcore.FastPathResult{
		Matched: true,
		Name:    "agent_run_test_fast_path",
		Reason:  "test_runtime_projection",
		Prepared: &runtime.PreparedExecution{
			Initial:        &runtime.TurnResult{Content: "agent run fast path output"},
			InitialStatus:  runtime.RequestStatusCompleted,
			RuntimeRecords: &recordSet,
			TerminalProjector: &runtime.RuntimeTerminalProjector{
				Store:     f.store,
				RecordSet: &recordSet,
				Metadata: map[string]any{
					"test_fast_path": true,
				},
			},
		},
	}, nil
}
