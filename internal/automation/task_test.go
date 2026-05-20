package automation

import "testing"

// TestTaskStructStoresPayload verifies automation tasks can carry an execution payload.
// TestTaskStructStoresPayload 用于验证自动化任务可以携带执行载荷。
func TestTaskStructStoresPayload(t *testing.T) {
	task := Task{TaskID: "task-1", Payload: map[string]any{"job": "scan"}}
	if task.Payload["job"] != "scan" {
		t.Fatalf("Payload = %#v", task.Payload)
	}
}

// TestBuildTaskCreatesManualAction verifies BuildTask defaults to manual action mode.
// TestBuildTaskCreatesManualAction 用于验证 BuildTask 默认采用 manual action 模式。
func TestBuildTaskCreatesManualAction(t *testing.T) {
	task := BuildTask("task-1", "title", "summary", nil)
	if task == nil || task.ExecuteMode != "manual_action" {
		t.Fatalf("unexpected task = %#v", task)
	}
}
