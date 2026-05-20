package app

import (
	"testing"

	"moss/internal/runtime"
)

// TestBuildRuntimeTaskFromChatRequest verifies the app layer owns chat-to-task normalization before runtime sees the request.
// TestBuildRuntimeTaskFromChatRequest 用于验证 app 层会在 runtime 看到请求前先完成 chat 到 task 的归一化。
func TestBuildRuntimeTaskFromChatRequest(t *testing.T) {
	req := ChatRequest{
		Query: "summarize project status",
		Supplement: &runtime.SupplementPayload{
			Data: map[string]string{
				"project_id": "p1001",
			},
		},
	}

	task, err := buildRuntimeTaskFromRequest("req-task-1", req)
	if err != nil {
		t.Fatalf("buildRuntimeTaskFromRequest() error = %v", err)
	}
	if task == nil {
		t.Fatalf("buildRuntimeTaskFromRequest() returned nil")
	}
	if task.TaskID != "req-task-1" {
		t.Fatalf("TaskID = %q, want %q", task.TaskID, "req-task-1")
	}
	if task.UserGoal != "summarize project status" {
		t.Fatalf("UserGoal = %q", task.UserGoal)
	}
	if task.KnownFacts["project_id"] != "p1001" {
		t.Fatalf("KnownFacts = %#v", task.KnownFacts)
	}
}

// TestBuildRuntimeTaskFromRequestAcceptsLegacyWorkflowShape verifies app normalization keeps legacy task types compatible.
// TestBuildRuntimeTaskFromRequestAcceptsLegacyWorkflowShape 用于验证 app 归一化保持旧任务类型兼容。
func TestBuildRuntimeTaskFromRequestAcceptsLegacyWorkflowShape(t *testing.T) {
	task, err := buildRuntimeTaskFromRequest("req-task-2", ChatRequest{
		TaskType:      "workflow_step_request",
		WorkspaceID:   "ws-1",
		MainSessionID: "sess-1",
	})
	if err != nil {
		t.Fatalf("buildRuntimeTaskFromRequest() error = %v", err)
	}
	if task.TaskType != "workflow_step_request" {
		t.Fatalf("TaskType = %q, want workflow_step_request", task.TaskType)
	}
	if task.WorkflowRunID != "" || task.StepID != "" {
		t.Fatalf("workflow compatibility fields should remain optional, got run=%q step=%q", task.WorkflowRunID, task.StepID)
	}
}
