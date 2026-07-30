// builtin_test.go verifies the bounded deterministic behavior of Athena core tools.
// builtin_test.go 验证 Athena core 工具的受限确定性行为。
package tools

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	einotool "github.com/cloudwego/eino/components/tool"
)

func TestCalculatorEvaluatesBoundedExpressions(t *testing.T) {
	result, err := calculateExpression(context.Background(), CalculatorRequest{Expression: "(2 + 3) * 4 - 5 % 3"})
	if err != nil {
		t.Fatalf("calculateExpression() error = %v", err)
	}
	if result.Value != 18 {
		t.Fatalf("result = %#v, want value 18", result)
	}
	for _, expression := range []string{"1 / 0", "1 + script()", "(1 + 2"} {
		if _, err := calculateExpression(context.Background(), CalculatorRequest{Expression: expression}); err == nil {
			t.Fatalf("calculateExpression(%q) error = nil, want bounded parser failure", expression)
		}
	}
}

func TestCurrentTimeUsesRequestedIANAZone(t *testing.T) {
	fixed := time.Date(2026, time.July, 10, 12, 30, 0, 0, time.UTC)
	result, err := currentTime(context.Background(), TimeRequest{Timezone: "Asia/Shanghai"}, func() time.Time { return fixed })
	if err != nil {
		t.Fatalf("currentTime() error = %v", err)
	}
	if result.Timezone != "Asia/Shanghai" || result.RFC3339 != "2026-07-10T20:30:00+08:00" || result.Unix != fixed.Unix() {
		t.Fatalf("result = %#v, want Asia/Shanghai converted time", result)
	}
	if _, err := currentTime(context.Background(), TimeRequest{Timezone: "not/a-real-zone"}, func() time.Time { return fixed }); err == nil {
		t.Fatal("currentTime() error = nil, want invalid timezone error")
	}
}

func TestJSONSchemaValidatorReportsStructuredMismatches(t *testing.T) {
	result, err := validateJSONSchema(context.Background(), JSONSchemaValidateRequest{
		Schema: map[string]any{
			"type":                 "object",
			"required":             []any{"name", "score"},
			"additionalProperties": false,
			"properties": map[string]any{
				"name":  map[string]any{"type": "string", "minLength": float64(3)},
				"score": map[string]any{"type": "integer", "minimum": float64(1), "maximum": float64(5)},
			},
		},
		Value: map[string]any{"name": "Li", "score": float64(5.5), "extra": true},
	})
	if err != nil {
		t.Fatalf("validateJSONSchema() error = %v", err)
	}
	if result.Valid || len(result.Errors) != 4 || result.SchemaVersion != builtinSchemaVersion {
		t.Fatalf("result = %#v, want four structured mismatches", result)
	}
	keywords := make(map[string]bool, len(result.Errors))
	for _, item := range result.Errors {
		keywords[item.Keyword] = true
	}
	for _, keyword := range []string{"minLength", "type", "maximum", "additionalProperties"} {
		if !keywords[keyword] {
			t.Fatalf("keywords = %#v, missing %q", keywords, keyword)
		}
	}
	for _, schema := range []map[string]any{
		{"type": "unsupported"},
		{"type": "number", "minimum": "one"},
		{"type": "object", "additionalProperties": "false"},
	} {
		if _, err := validateJSONSchema(context.Background(), JSONSchemaValidateRequest{Schema: schema}); err == nil {
			t.Fatalf("validateJSONSchema(%#v) error = nil, want malformed schema failure", schema)
		}
	}
}

func TestBuiltinDefinitionsAreCallableAndNonSideEffecting(t *testing.T) {
	definitions, err := DemoDefinitions()
	if err != nil {
		t.Fatalf("DemoDefinitions() error = %v", err)
	}
	for _, name := range []string{"calculator", "current_time", "json_schema_validate"} {
		definition, ok := definitions[name]
		if !ok || definition.BaseTool == nil {
			t.Fatalf("definition %q = %#v, want executable built-in tool", name, definition)
		}
		if definition.ToolScope != "builtin_deterministic" || definition.SideEffectLevel != "none" || definition.RequiresConfirmation {
			t.Fatalf("definition %q metadata = %#v, want read-only deterministic metadata", name, definition)
		}
	}
	invokable, ok := definitions["calculator"].BaseTool.(einotool.InvokableTool)
	if !ok {
		t.Fatal("calculator is not invokable")
	}
	output, err := invokable.InvokableRun(context.Background(), `{"expression":"40 + 2"}`)
	if err != nil {
		t.Fatalf("calculator InvokableRun() error = %v", err)
	}
	if !strings.Contains(output, `"value":42`) {
		t.Fatalf("calculator output = %q, want numeric result", output)
	}
	validator, ok := definitions["json_schema_validate"].BaseTool.(einotool.InvokableTool)
	if !ok {
		t.Fatal("json_schema_validate is not invokable")
	}
	validated, err := validator.InvokableRun(context.Background(), `{"schema":{"type":"object","required":["id"]},"value":{"id":"ok"}}`)
	if err != nil {
		t.Fatalf("json_schema_validate InvokableRun() error = %v", err)
	}
	var payload JSONSchemaValidateResult
	if err := json.Unmarshal([]byte(validated), &payload); err != nil || !payload.Valid {
		t.Fatalf("validator output = %q, decode error = %v", validated, err)
	}
}

func BenchmarkCalculatorExpression(b *testing.B) {
	request := CalculatorRequest{Expression: "((125.5 + 74.5) * 3 - 20) / 4"}
	b.ReportAllocs()
	for index := 0; index < b.N; index++ {
		if _, err := calculateExpression(context.Background(), request); err != nil {
			b.Fatal(err)
		}
	}
}
