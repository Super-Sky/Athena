// registry.go defines tool metadata wrappers for Athena demo and deterministic built-in tools.
// registry.go 定义 Athena demo 与确定性内置工具的元数据包装结构。
package tools

import (
	"sort"

	"github.com/cloudwego/eino/components/tool"
)

// Definition captures one tool together with the metadata the runtime needs to consume it.
// Definition 描述 runtime 消费某个 tool 时需要的工具对象和元数据。
type Definition struct {
	Name                 string
	Description          string
	BaseTool             tool.BaseTool
	RequiredInputs       []string
	ToolScope            string
	RequiresConfirmation bool
	SideEffectLevel      string
	InputSchemaSummary   string
	OutputSchemaSummary  string
}

// DemoDefinitions returns the runtime-facing metadata map for the repository demo tools.
// DemoDefinitions 返回仓库 demo tools 的 runtime 侧元数据映射。
func DemoDefinitions() (map[string]Definition, error) {
	registry, err := DemoToolRegistry()
	if err != nil {
		return nil, err
	}
	builtinRegistry, err := BuiltinToolRegistry()
	if err != nil {
		return nil, err
	}

	return map[string]Definition{
		"calculator": {
			Name:                 "calculator",
			Description:          "Evaluate a bounded arithmetic expression.",
			BaseTool:             builtinRegistry["calculator"],
			ToolScope:            "builtin_deterministic",
			RequiresConfirmation: false,
			SideEffectLevel:      "none",
			InputSchemaSummary:   "expression:string",
			OutputSchemaSummary:  "expression and finite numeric value",
		},
		"current_time": {
			Name:                 "current_time",
			Description:          "Read the current time in an IANA timezone.",
			BaseTool:             builtinRegistry["current_time"],
			ToolScope:            "builtin_deterministic",
			RequiresConfirmation: false,
			SideEffectLevel:      "none",
			InputSchemaSummary:   "timezone:IANA string optional",
			OutputSchemaSummary:  "timezone, RFC3339 time, unix seconds",
		},
		"json_schema_validate": {
			Name:                 "json_schema_validate",
			Description:          "Validate a JSON-compatible value against the Athena JSON Schema subset.",
			BaseTool:             builtinRegistry["json_schema_validate"],
			ToolScope:            "builtin_deterministic",
			RequiresConfirmation: false,
			SideEffectLevel:      "none",
			InputSchemaSummary:   "schema:object, value:JSON-compatible",
			OutputSchemaSummary:  "valid boolean and validation errors",
		},
		"lookup_profile": {
			Name:                 "lookup_profile",
			Description:          "Look up a user's basic profile.",
			BaseTool:             registry["lookup_profile"],
			RequiredInputs:       []string{"user_id"},
			ToolScope:            "read_only_lookup",
			RequiresConfirmation: false,
			SideEffectLevel:      "none",
			InputSchemaSummary:   "user_id:string",
			OutputSchemaSummary:  "profile object",
		},
		"lookup_orders": {
			Name:                 "lookup_orders",
			Description:          "Look up a user's open orders.",
			BaseTool:             registry["lookup_orders"],
			RequiredInputs:       []string{"user_id"},
			ToolScope:            "read_only_lookup",
			RequiresConfirmation: false,
			SideEffectLevel:      "none",
			InputSchemaSummary:   "user_id:string",
			OutputSchemaSummary:  "open orders list",
		},
		"lookup_risk_flags": {
			Name:                 "lookup_risk_flags",
			Description:          "Look up a user's risk flags.",
			BaseTool:             registry["lookup_risk_flags"],
			RequiredInputs:       []string{"user_id"},
			ToolScope:            "read_only_lookup",
			RequiresConfirmation: false,
			SideEffectLevel:      "none",
			InputSchemaSummary:   "user_id:string",
			OutputSchemaSummary:  "risk flags list",
		},
	}, nil
}

// DemoDefinitionList returns the sorted tool definition list used by control-plane views.
// DemoDefinitionList 返回控制面视图使用的已排序 tool 定义列表。
func DemoDefinitionList() ([]Definition, error) {
	defs, err := DemoDefinitions()
	if err != nil {
		return nil, err
	}
	result := make([]Definition, 0, len(defs))
	for _, item := range defs {
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}
