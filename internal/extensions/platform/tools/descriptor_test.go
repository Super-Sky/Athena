// descriptor_test.go verifies platform tool descriptors and invocation hints stay structured and deterministic.
// descriptor_test.go 用于验证 platform tool 描述符与调用提示保持结构化且可确定。
package tools

import (
	"testing"

	platformautomation "moss/internal/extensions/platform/automation"
)

// TestDefaultDescriptorsExposeStructuredContracts verifies the default descriptors remain explicit and typed.
// TestDefaultDescriptorsExposeStructuredContracts 用于验证默认描述符保持显式且带类型。
func TestDefaultDescriptorsExposeStructuredContracts(t *testing.T) {
	descriptors := DefaultDescriptors()
	if len(descriptors) < 4 {
		t.Fatalf("descriptors len = %d, want at least 4", len(descriptors))
	}
	if descriptors[0].Name == "" {
		t.Fatalf("descriptor = %#v", descriptors[0])
	}
	foundTyped := false
	for _, descriptor := range descriptors {
		if len(descriptor.InputSchema) > 0 {
			foundTyped = true
			break
		}
	}
	if !foundTyped {
		t.Fatalf("descriptors = %#v, want at least one descriptor with input schema", descriptors)
	}
}

// TestBuildAutomationHintsReturnsCreateTool verifies confirmed automation creation gets a stable tool hint.
// TestBuildAutomationHintsReturnsCreateTool 用于验证确认后的自动化创建会得到稳定的 tool 提示。
func TestBuildAutomationHintsReturnsCreateTool(t *testing.T) {
	hints := BuildAutomationHints(&platformautomation.CreatePayload{Title: "draft"})
	if len(hints) != 1 {
		t.Fatalf("hints len = %d, want 1", len(hints))
	}
	if hints[0].ToolName != "create_automation_from_payload" {
		t.Fatalf("tool_name = %q", hints[0].ToolName)
	}
}
