// contract_foundation_test.go verifies runtime contract foundation invariants.
// contract_foundation_test.go 验证 runtime contract foundation 的核心约束。
package runtime

import (
	"errors"
	"strings"
	"testing"
)

// TestValidateTaskTypeRegistrationRequiresActiveValidatorRefs verifies active task types expose an inspectable validator contract.
// TestValidateTaskTypeRegistrationRequiresActiveValidatorRefs 用于验证 active task type 必须暴露可检查的 validator contract。
func TestValidateTaskTypeRegistrationRequiresActiveValidatorRefs(t *testing.T) {
	err := ValidateTaskTypeRegistration(TaskTypeRegistration{
		TypeKey:           "inspection_task",
		Status:            TaskTypeStatusActive,
		InputSchema:       map[string]any{"type": "object"},
		DefaultContractID: "contract-1",
	})
	if !errors.Is(err, ErrInvalidRuntimePersistenceInput) || !strings.Contains(err.Error(), "validator_refs.validators") {
		t.Fatalf("ValidateTaskTypeRegistration() error = %v, want validator refs error", err)
	}
}

// TestValidateTaskTypeRegistrationAllowsDraftWithoutValidatorRefs verifies draft records can be staged before validator refs exist.
// TestValidateTaskTypeRegistrationAllowsDraftWithoutValidatorRefs 用于验证 draft 记录可在 validator refs 补齐前暂存。
func TestValidateTaskTypeRegistrationAllowsDraftWithoutValidatorRefs(t *testing.T) {
	err := ValidateTaskTypeRegistration(TaskTypeRegistration{
		TypeKey: "inspection_task",
		Status:  TaskTypeStatusDraft,
	})
	if err != nil {
		t.Fatalf("ValidateTaskTypeRegistration() error = %v", err)
	}
}

// TestValidateTaskTypeRegistrationAcceptsActiveValidatorContract verifies the minimal active contract shape stays valid.
// TestValidateTaskTypeRegistrationAcceptsActiveValidatorContract 用于验证最小 active validator contract 形态保持可用。
func TestValidateTaskTypeRegistrationAcceptsActiveValidatorContract(t *testing.T) {
	err := ValidateTaskTypeRegistration(TaskTypeRegistration{
		TypeKey:           "workflow_step_request",
		Status:            TaskTypeStatusActive,
		InputSchema:       map[string]any{"type": "object"},
		DefaultContractID: "contract-1",
		ValidatorRefs: map[string]any{
			"validators": []any{"registered_task_type_input_schema"},
			"status":     "ready",
		},
		Compatibility: map[string]any{
			"required_fields_mode": "advisory",
		},
	})
	if err != nil {
		t.Fatalf("ValidateTaskTypeRegistration() error = %v", err)
	}
}
