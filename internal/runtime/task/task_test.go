package task

import "testing"

// TestNormalizeChatRequestBuildsMinimalTask verifies the chat path produces the generic internal runtime task shape.
// TestNormalizeChatRequestBuildsMinimalTask 用于验证 chat 路径会生成通用内部 runtime task 模型。
func TestNormalizeChatRequestBuildsMinimalTask(t *testing.T) {
	task := NormalizeChatRequest("req-1", "summarize current project status", map[string]string{
		"project_id": "p1001",
	})

	if task == nil {
		t.Fatalf("NormalizeChatRequest() returned nil")
	}
	if task.TaskID != "req-1" {
		t.Fatalf("TaskID = %q, want %q", task.TaskID, "req-1")
	}
	if task.InputKind != InputKindChat {
		t.Fatalf("InputKind = %q, want %q", task.InputKind, InputKindChat)
	}
	if task.UserGoal != "summarize current project status" {
		t.Fatalf("UserGoal = %q, want %q", task.UserGoal, "summarize current project status")
	}
	if task.KnownFacts["project_id"] != "p1001" {
		t.Fatalf("KnownFacts = %#v", task.KnownFacts)
	}
	if len(task.MissingFacts) != 0 {
		t.Fatalf("MissingFacts = %#v, want empty", task.MissingFacts)
	}
	if task.OutputMode != DefaultOutputModeText {
		t.Fatalf("OutputMode = %q, want %q", task.OutputMode, DefaultOutputModeText)
	}
}

// TestNormalizeChatRequestDoesNotRequireUserIDByDefault verifies default chat no longer records user_id as a global missing fact.
// TestNormalizeChatRequestDoesNotRequireUserIDByDefault 用于验证默认 chat 不再把 user_id 记录为全局缺失事实。
func TestNormalizeChatRequestDoesNotRequireUserIDByDefault(t *testing.T) {
	task := NormalizeChatRequest("", "", nil)

	if task == nil {
		t.Fatalf("NormalizeChatRequest() returned nil")
	}
	if task.UserGoal != "continue the current runtime task" {
		t.Fatalf("UserGoal = %q, want default continuation goal", task.UserGoal)
	}
	if len(task.MissingFacts) != 0 {
		t.Fatalf("MissingFacts = %#v, want empty", task.MissingFacts)
	}
	if task.Constraints["entry_path"] != "chat" {
		t.Fatalf("Constraints = %#v", task.Constraints)
	}
}

// TestNormalizeRequestAcceptsLegacyInspectionTaskType verifies legacy task types remain compatible.
// TestNormalizeRequestAcceptsLegacyInspectionTaskType 用于验证旧任务类型仍保持兼容。
func TestNormalizeRequestAcceptsLegacyInspectionTaskType(t *testing.T) {
	task, err := NormalizeRequest("req-inspect-1", NormalizationInput{
		TaskType:              InputKindInspectionTask,
		WorkspaceID:           "ws-1",
		MainSessionID:         "sess-1",
		IntegrationInstanceID: "integration-1",
		TriggerType:           "manual_inspection",
		InputPayload: map[string]any{
			"target": "asset-1",
		},
		GlobalContext: map[string]any{"org_id": "org-1"},
		AppContext:    map[string]any{"app_mode": "inspection"},
	})
	if err != nil {
		t.Fatalf("NormalizeRequest() error = %v", err)
	}
	if task.TaskType != InputKindInspectionTask {
		t.Fatalf("TaskType = %q, want %q", task.TaskType, InputKindInspectionTask)
	}
	if task.TaskSubtype != "manual_inspection" {
		t.Fatalf("TaskSubtype = %q, want manual_inspection", task.TaskSubtype)
	}
	if task.Scene != "inspection" {
		t.Fatalf("Scene = %q, want inspection", task.Scene)
	}
	if task.IntegrationInstanceID != "integration-1" {
		t.Fatalf("IntegrationInstanceID = %q", task.IntegrationInstanceID)
	}
}

// TestNormalizeRequestKeepsScenarioFieldsOutOfCoreValidation verifies legacy scenario fields are not universal core requirements.
// TestNormalizeRequestKeepsScenarioFieldsOutOfCoreValidation 用于验证旧场景字段不再是通用 core 必填项。
func TestNormalizeRequestKeepsScenarioFieldsOutOfCoreValidation(t *testing.T) {
	task, err := NormalizeRequest("req-workflow-1", NormalizationInput{
		TaskType: InputKindWorkflowStepRequest,
	})
	if err != nil {
		t.Fatalf("NormalizeRequest() error = %v", err)
	}
	if task.TaskType != InputKindWorkflowStepRequest {
		t.Fatalf("TaskType = %q, want %q", task.TaskType, InputKindWorkflowStepRequest)
	}
	if task.WorkflowRunID != "" || task.StepID != "" {
		t.Fatalf("workflow compatibility fields should remain optional, got run=%q step=%q", task.WorkflowRunID, task.StepID)
	}
}

// TestNormalizeRequestAcceptsGenericTaskType verifies Phase 0 keeps task type validation generic.
// TestNormalizeRequestAcceptsGenericTaskType 用于验证 Phase 0 保持通用 task type 校验边界。
func TestNormalizeRequestAcceptsGenericTaskType(t *testing.T) {
	task, err := NormalizeRequest("req-custom-1", NormalizationInput{
		TaskType: "custom_runtime_task",
		Query:    "handle a generic runtime task",
	})
	if err != nil {
		t.Fatalf("NormalizeRequest() error = %v", err)
	}
	if task.TaskType != "custom_runtime_task" {
		t.Fatalf("TaskType = %q, want custom_runtime_task", task.TaskType)
	}
	if task.UserGoal != "handle a generic runtime task" {
		t.Fatalf("UserGoal = %q", task.UserGoal)
	}
}
