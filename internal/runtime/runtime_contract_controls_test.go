package runtime

import "testing"

func TestApplyResolvedRuntimeContractProjectsControlConstraints(t *testing.T) {
	t.Parallel()

	spec := &ExecutionSpec{Metadata: ExecutionMetadata{Constraints: map[string]any{}}}
	contract := &RuntimeContract{
		ID:      "contract-chat-v1",
		Name:    "Chat Contract",
		Version: "v1",
		Status:  RuntimeContractStatusActive,
		ExecutionProfile: map[string]any{
			"agent_runtime_controls": map[string]any{
				"cancel": map[string]any{
					"enabled":              true,
					"mode":                 "immediate",
					"checkpoint_on_cancel": false,
				},
				"model_retry": map[string]any{
					"max_attempts": float64(2),
				},
				"model_failover": map[string]any{
					"enabled": false,
				},
				"after_agent": map[string]any{
					"enabled":      true,
					"summary_mode": "terminal_projection",
				},
			},
		},
		ExitPolicy: map[string]any{
			"model_retry": map[string]any{
				"max_attempts": float64(2),
			},
		},
	}
	taskType := &TaskTypeRegistration{ID: "task-type-chat", Status: TaskTypeStatusActive}
	ApplyResolvedRuntimeContract(spec, contract, taskType)

	if got := spec.Metadata.Constraints["runtime_contract_id"]; got != "contract-chat-v1" {
		t.Fatalf("runtime_contract_id = %#v, want contract-chat-v1", got)
	}
	if got := spec.Metadata.Constraints["runtime_model_retry_max_attempts"]; got != 2 {
		t.Fatalf("runtime_model_retry_max_attempts = %#v, want 2", got)
	}
	if got := spec.Metadata.Constraints["runtime_model_failover_enabled"]; got != false {
		t.Fatalf("runtime_model_failover_enabled = %#v, want false", got)
	}
	if got := spec.Metadata.Constraints["runtime_after_agent_enabled"]; got != true {
		t.Fatalf("runtime_after_agent_enabled = %#v, want true", got)
	}
	settings := ResolveAgentRuntimeControlSettings(spec)
	if settings.CancelMode != "immediate" || settings.CancelCheckpointEnabled || settings.ModelRetryMaxAttempts != 2 || settings.ModelFailoverEnabled || !settings.AfterAgentEnabled {
		t.Fatalf("settings = %#v", settings)
	}
}
