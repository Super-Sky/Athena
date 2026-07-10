// governed_test.go verifies pre-execution governance behavior for local deterministic tools.
// governed_test.go 验证本地确定性工具的执行前治理行为。
package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	einotool "github.com/cloudwego/eino/components/tool"
)

func TestGovernedDefinitionUsesContextToolCallIDAndAllowsExecution(t *testing.T) {
	definitions, err := DemoDefinitions()
	if err != nil {
		t.Fatalf("DemoDefinitions() error = %v", err)
	}
	var request GovernedToolRequest
	var observed GovernedToolEvent
	definition, err := NewGovernedDefinition(definitions["calculator"], GovernedToolOptions{
		Evaluate: func(_ context.Context, input GovernedToolRequest) (GovernedToolDecision, error) {
			request = input
			return GovernedToolDecision{DecisionID: "decision-calc", Decision: "allow"}, nil
		},
		Observe: func(_ context.Context, event GovernedToolEvent) {
			observed = event
		},
	})
	if err != nil {
		t.Fatalf("NewGovernedDefinition() error = %v", err)
	}
	invokable, ok := definition.BaseTool.(einotool.InvokableTool)
	if !ok {
		t.Fatal("governed calculator is not invokable")
	}
	ctx := context.WithValue(context.Background(), governedToolCallIDContextKey{}, "call_calculator")
	output, err := invokable.InvokableRun(ctx, `{"expression":"6 * 7"}`)
	if err != nil {
		t.Fatalf("InvokableRun() error = %v", err)
	}
	if !strings.Contains(output, `"value":42`) {
		t.Fatalf("output = %q, want calculator result", output)
	}
	if request.ToolCallID != "call_calculator" || strings.Join(request.ArgumentKeys, ",") != "expression" {
		t.Fatalf("governance request = %#v, want stable call id and safe argument keys", request)
	}
	if observed.Status != "ok" || observed.DecisionID != "decision-calc" || observed.ToolCallID != "call_calculator" {
		t.Fatalf("observation = %#v, want allowed correlated event", observed)
	}
}

func TestGovernedDefinitionFailsClosedOnDeniedOrUnsupportedDecision(t *testing.T) {
	definitions, err := DemoDefinitions()
	if err != nil {
		t.Fatalf("DemoDefinitions() error = %v", err)
	}
	for _, decision := range []string{"deny", "allow_with_redaction", "require_sandbox_ref"} {
		t.Run(decision, func(t *testing.T) {
			definition, err := NewGovernedDefinition(definitions["calculator"], GovernedToolOptions{
				Evaluate: func(context.Context, GovernedToolRequest) (GovernedToolDecision, error) {
					return GovernedToolDecision{Decision: decision, Reason: "test policy"}, nil
				},
			})
			if err != nil {
				t.Fatal(err)
			}
			invokable := definition.BaseTool.(einotool.InvokableTool)
			_, err = invokable.InvokableRun(context.Background(), `{"expression":"6 * 7"}`)
			var governedErr *GovernedExecutionError
			if !errors.As(err, &governedErr) {
				t.Fatalf("error = %v, want GovernedExecutionError", err)
			}
			if decision == "deny" && governedErr.Code != "governance_denied" {
				t.Fatalf("error = %#v, want governance_denied", governedErr)
			}
			if decision != "deny" && governedErr.Code != "governance_decision_unsupported" {
				t.Fatalf("error = %#v, want unsupported decision", governedErr)
			}
		})
	}
}
