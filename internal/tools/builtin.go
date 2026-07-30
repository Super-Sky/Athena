// builtin.go defines deterministic, provider-neutral tools that are safe to expose from Athena core.
// builtin.go 定义可从 Athena core 暴露的确定性、与供应商无关的内置工具。
// It owns calculator, clock, and schema-subset tool construction plus their bounded execution helpers.
// 它负责 calculator、时钟和 schema 子集工具的构造，以及受限执行辅助逻辑。
// Changes here affect catalog metadata, tool schemas, and Agent Run execution behavior.
// 修改这里会影响目录元数据、工具 schema 和 Agent Run 执行行为。
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/cloudwego/eino/components/tool"
	tu "github.com/cloudwego/eino/components/tool/utils"
)

const (
	builtinSchemaVersion = "json_schema_subset.v1"
	maxCalculatorRunes   = 512
	maxSchemaBytes       = 64 * 1024
	maxSchemaDepth       = 24
)

// CalculatorRequest contains a limited arithmetic expression.
// CalculatorRequest 包含一条受限的算术表达式。
type CalculatorRequest struct {
	Expression string `json:"expression" jsonschema_description:"Arithmetic expression using numbers, +, -, *, /, %, and parentheses."`
}

// CalculatorResult records one evaluated arithmetic expression.
// CalculatorResult 记录一条已计算的算术表达式。
type CalculatorResult struct {
	Expression string  `json:"expression"`
	Value      float64 `json:"value"`
}

// TimeRequest selects the IANA timezone used for a current-time readout.
// TimeRequest 选择当前时间读数使用的 IANA 时区。
type TimeRequest struct {
	Timezone string `json:"timezone,omitempty" jsonschema_description:"IANA timezone such as Asia/Shanghai or America/New_York. Defaults to UTC."`
}

// TimeResult returns a normalized clock readout with explicit timezone metadata.
// TimeResult 返回带明确时区元数据的标准化时钟读数。
type TimeResult struct {
	Timezone string `json:"timezone"`
	RFC3339  string `json:"rfc3339"`
	Unix     int64  `json:"unix"`
}

// JSONSchemaValidateRequest validates a JSON-compatible value against Athena's documented schema subset.
// JSONSchemaValidateRequest 使用 Athena 已文档化的 schema 子集校验 JSON 兼容值。
type JSONSchemaValidateRequest struct {
	Schema map[string]any `json:"schema" jsonschema_description:"JSON Schema subset: type, properties, required, additionalProperties, items, enum, minimum, maximum, minLength, maxLength."`
	Value  any            `json:"value" jsonschema_description:"JSON-compatible value to validate."`
}

// JSONSchemaValidationError explains one deterministic schema mismatch without exposing unrelated input.
// JSONSchemaValidationError 解释一条确定性 schema 不匹配，不暴露无关输入。
type JSONSchemaValidationError struct {
	Path    string `json:"path"`
	Keyword string `json:"keyword"`
	Message string `json:"message"`
}

// JSONSchemaValidateResult reports schema validation outcomes.
// JSONSchemaValidateResult 报告 schema 校验结果。
type JSONSchemaValidateResult struct {
	Valid         bool                        `json:"valid"`
	Errors        []JSONSchemaValidationError `json:"errors,omitempty"`
	SchemaVersion string                      `json:"schema_version"`
}

// BuiltinToolRegistry returns executable deterministic tools keyed by canonical runtime name.
// BuiltinToolRegistry 返回按 runtime 标准名称索引的可执行确定性内置工具。
func BuiltinToolRegistry() (map[string]tool.BaseTool, error) {
	return builtinToolRegistryWithClock(time.Now)
}

// BuiltinToolNames returns the stable names reserved for Athena Core deterministic tools.
// BuiltinToolNames 返回为 Athena Core 确定性工具保留的稳定名称。
func BuiltinToolNames() []string {
	return []string{"calculator", "current_time", "json_schema_validate"}
}

func builtinToolRegistryWithClock(now func() time.Time) (map[string]tool.BaseTool, error) {
	calculator, err := tu.InferTool("calculator",
		"Evaluate a bounded arithmetic expression. Use this for deterministic numeric calculations; it cannot access files, networks, or application data.",
		calculateExpression)
	if err != nil {
		return nil, err
	}
	clock, err := tu.InferTool("current_time",
		"Read the current time in an IANA timezone. Use this when a task needs an explicit current timestamp or timezone conversion anchor.",
		func(ctx context.Context, input TimeRequest) (TimeResult, error) {
			return currentTime(ctx, input, now)
		})
	if err != nil {
		return nil, err
	}
	schemaValidator, err := tu.InferTool("json_schema_validate",
		"Validate a JSON-compatible value with Athena's deterministic JSON Schema subset. Use this before trusting structured tool inputs or outputs.",
		validateJSONSchema)
	if err != nil {
		return nil, err
	}
	return map[string]tool.BaseTool{
		"calculator":           calculator,
		"current_time":         clock,
		"json_schema_validate": schemaValidator,
	}, nil
}

func calculateExpression(_ context.Context, input CalculatorRequest) (CalculatorResult, error) {
	expression := strings.TrimSpace(input.Expression)
	if expression == "" {
		return CalculatorResult{}, fmt.Errorf("expression is required")
	}
	if len([]rune(expression)) > maxCalculatorRunes {
		return CalculatorResult{}, fmt.Errorf("expression exceeds %d characters", maxCalculatorRunes)
	}
	parser := arithmeticParser{input: []rune(expression)}
	value, err := parser.parseExpression()
	if err != nil {
		return CalculatorResult{}, err
	}
	parser.skipSpaces()
	if parser.pos != len(parser.input) {
		return CalculatorResult{}, fmt.Errorf("unexpected token at position %d", parser.pos+1)
	}
	if math.IsInf(value, 0) || math.IsNaN(value) {
		return CalculatorResult{}, fmt.Errorf("expression result is not finite")
	}
	return CalculatorResult{Expression: expression, Value: value}, nil
}

func currentTime(_ context.Context, input TimeRequest, now func() time.Time) (TimeResult, error) {
	timezone := strings.TrimSpace(input.Timezone)
	if timezone == "" {
		timezone = "UTC"
	}
	location, err := time.LoadLocation(timezone)
	if err != nil {
		return TimeResult{}, fmt.Errorf("invalid IANA timezone %q", timezone)
	}
	if now == nil {
		now = time.Now
	}
	value := now().In(location)
	return TimeResult{Timezone: location.String(), RFC3339: value.Format(time.RFC3339), Unix: value.Unix()}, nil
}

func validateJSONSchema(_ context.Context, input JSONSchemaValidateRequest) (JSONSchemaValidateResult, error) {
	if len(input.Schema) == 0 {
		return JSONSchemaValidateResult{}, fmt.Errorf("schema is required")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return JSONSchemaValidateResult{}, fmt.Errorf("encode schema validation input: %w", err)
	}
	if len(encoded) > maxSchemaBytes {
		return JSONSchemaValidateResult{}, fmt.Errorf("schema validation input exceeds %d bytes", maxSchemaBytes)
	}
	errors, err := validateSchemaValue(input.Schema, input.Value, "$", 0)
	if err != nil {
		return JSONSchemaValidateResult{}, err
	}
	return JSONSchemaValidateResult{Valid: len(errors) == 0, Errors: errors, SchemaVersion: builtinSchemaVersion}, nil
}

func validateSchemaValue(schema map[string]any, value any, path string, depth int) ([]JSONSchemaValidationError, error) {
	if err := validateSchemaDefinition(schema, path, depth); err != nil {
		return nil, err
	}
	return validateSchemaValueUnchecked(schema, value, path, depth)
}

func validateSchemaValueUnchecked(schema map[string]any, value any, path string, depth int) ([]JSONSchemaValidationError, error) {
	if depth > maxSchemaDepth {
		return nil, fmt.Errorf("schema exceeds maximum depth %d", maxSchemaDepth)
	}
	errors := make([]JSONSchemaValidationError, 0)
	if enum, ok := schema["enum"]; ok {
		items, ok := enum.([]any)
		if !ok {
			return nil, fmt.Errorf("schema enum at %s must be an array", path)
		}
		matched := false
		for _, candidate := range items {
			if reflect.DeepEqual(candidate, value) {
				matched = true
				break
			}
		}
		if !matched {
			errors = append(errors, schemaError(path, "enum", "value is not an allowed enum member"))
		}
	}
	typeName := ""
	if rawType, specified := schema["type"]; specified {
		var ok bool
		typeName, ok = rawType.(string)
		if !ok {
			return nil, fmt.Errorf("schema type at %s must be a string", path)
		}
	}
	switch typeName {
	case "", "any":
	case "object":
		object, ok := value.(map[string]any)
		if !ok {
			return append(errors, schemaError(path, "type", "expected object")), nil
		}
		required, err := schemaStringList(schema["required"], path, "required")
		if err != nil {
			return nil, err
		}
		for _, key := range required {
			if _, exists := object[key]; !exists {
				errors = append(errors, schemaError(path+"."+key, "required", "required property is missing"))
			}
		}
		properties, err := schemaObjectMap(schema["properties"], path, "properties")
		if err != nil {
			return nil, err
		}
		additional, additionalSpecified := schema["additionalProperties"]
		if additionalSpecified {
			if _, ok := additional.(bool); !ok {
				return nil, fmt.Errorf("schema additionalProperties at %s must be a boolean", path)
			}
		}
		for key, child := range object {
			childSchema, configured := properties[key]
			if configured {
				childErrors, err := validateSchemaValueUnchecked(childSchema, child, path+"."+key, depth+1)
				if err != nil {
					return nil, err
				}
				errors = append(errors, childErrors...)
				continue
			}
			if additionalSpecified && additional == false {
				errors = append(errors, schemaError(path+"."+key, "additionalProperties", "additional property is not allowed"))
			}
		}
	case "array":
		items, ok := value.([]any)
		if !ok {
			return append(errors, schemaError(path, "type", "expected array")), nil
		}
		if rawItems, exists := schema["items"]; exists {
			itemSchema, ok := rawItems.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("schema items at %s must be an object", path)
			}
			for index, item := range items {
				itemErrors, err := validateSchemaValueUnchecked(itemSchema, item, fmt.Sprintf("%s[%d]", path, index), depth+1)
				if err != nil {
					return nil, err
				}
				errors = append(errors, itemErrors...)
			}
		}
	case "string":
		text, ok := value.(string)
		if !ok {
			return append(errors, schemaError(path, "type", "expected string")), nil
		}
		bounds, err := validateStringBounds(schema, text, path)
		if err != nil {
			return nil, err
		}
		errors = append(errors, bounds...)
	case "number", "integer":
		number, ok := asJSONNumber(value)
		if !ok {
			return append(errors, schemaError(path, "type", "expected number")), nil
		}
		if typeName == "integer" && math.Trunc(number) != number {
			errors = append(errors, schemaError(path, "type", "expected integer"))
		}
		bounds, err := validateNumberBounds(schema, number, path)
		if err != nil {
			return nil, err
		}
		errors = append(errors, bounds...)
	case "boolean":
		if _, ok := value.(bool); !ok {
			return append(errors, schemaError(path, "type", "expected boolean")), nil
		}
	case "null":
		if value != nil {
			return append(errors, schemaError(path, "type", "expected null")), nil
		}
	default:
		return nil, fmt.Errorf("unsupported schema type %q at %s", typeName, path)
	}
	return errors, nil
}

func validateSchemaDefinition(schema map[string]any, path string, depth int) error {
	if depth > maxSchemaDepth {
		return fmt.Errorf("schema exceeds maximum depth %d", maxSchemaDepth)
	}
	if rawType, specified := schema["type"]; specified {
		typeName, ok := rawType.(string)
		if !ok {
			return fmt.Errorf("schema type at %s must be a string", path)
		}
		switch typeName {
		case "", "any", "object", "array", "string", "number", "integer", "boolean", "null":
		default:
			return fmt.Errorf("unsupported schema type %q at %s", typeName, path)
		}
	}
	if enum, specified := schema["enum"]; specified {
		if _, ok := enum.([]any); !ok {
			return fmt.Errorf("schema enum at %s must be an array", path)
		}
	}
	for _, keyword := range []string{"minimum", "maximum", "minLength", "maxLength"} {
		if _, _, err := numericKeyword(schema, keyword, path); err != nil {
			return err
		}
	}
	if rawAdditional, specified := schema["additionalProperties"]; specified {
		if _, ok := rawAdditional.(bool); !ok {
			return fmt.Errorf("schema additionalProperties at %s must be a boolean", path)
		}
	}
	if _, err := schemaStringList(schema["required"], path, "required"); err != nil {
		return err
	}
	properties, err := schemaObjectMap(schema["properties"], path, "properties")
	if err != nil {
		return err
	}
	for name, child := range properties {
		if err := validateSchemaDefinition(child, path+"."+name, depth+1); err != nil {
			return err
		}
	}
	if rawItems, specified := schema["items"]; specified {
		items, ok := rawItems.(map[string]any)
		if !ok {
			return fmt.Errorf("schema items at %s must be an object", path)
		}
		if err := validateSchemaDefinition(items, path+"[]", depth+1); err != nil {
			return err
		}
	}
	return nil
}

func schemaStringList(raw any, path, keyword string) ([]string, error) {
	if raw == nil {
		return nil, nil
	}
	values, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("schema %s at %s must be an array", keyword, path)
	}
	result := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok || strings.TrimSpace(text) == "" {
			return nil, fmt.Errorf("schema %s at %s must contain non-empty strings", keyword, path)
		}
		result = append(result, text)
	}
	return result, nil
}

func schemaObjectMap(raw any, path, keyword string) (map[string]map[string]any, error) {
	if raw == nil {
		return map[string]map[string]any{}, nil
	}
	values, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("schema %s at %s must be an object", keyword, path)
	}
	result := make(map[string]map[string]any, len(values))
	for key, value := range values {
		child, ok := value.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("schema property %s at %s must be an object", key, path)
		}
		result[key] = child
	}
	return result, nil
}

func validateStringBounds(schema map[string]any, value, path string) ([]JSONSchemaValidationError, error) {
	errors := make([]JSONSchemaValidationError, 0, 2)
	minimum, minimumSet, err := numericKeyword(schema, "minLength", path)
	if err != nil {
		return nil, err
	}
	if minimumSet && float64(len([]rune(value))) < minimum {
		errors = append(errors, schemaError(path, "minLength", "string is shorter than minLength"))
	}
	maximum, maximumSet, err := numericKeyword(schema, "maxLength", path)
	if err != nil {
		return nil, err
	}
	if maximumSet && float64(len([]rune(value))) > maximum {
		errors = append(errors, schemaError(path, "maxLength", "string is longer than maxLength"))
	}
	return errors, nil
}

func validateNumberBounds(schema map[string]any, value float64, path string) ([]JSONSchemaValidationError, error) {
	errors := make([]JSONSchemaValidationError, 0, 2)
	minimum, minimumSet, err := numericKeyword(schema, "minimum", path)
	if err != nil {
		return nil, err
	}
	if minimumSet && value < minimum {
		errors = append(errors, schemaError(path, "minimum", "number is below minimum"))
	}
	maximum, maximumSet, err := numericKeyword(schema, "maximum", path)
	if err != nil {
		return nil, err
	}
	if maximumSet && value > maximum {
		errors = append(errors, schemaError(path, "maximum", "number is above maximum"))
	}
	return errors, nil
}

func numericKeyword(schema map[string]any, keyword, path string) (float64, bool, error) {
	value, exists := schema[keyword]
	if !exists {
		return 0, false, nil
	}
	number, ok := asJSONNumber(value)
	if !ok || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, false, fmt.Errorf("schema %s at %s must be a finite number", keyword, path)
	}
	return number, true, nil
}

func asJSONNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case json.Number:
		parsed, err := typed.Float64()
		return parsed, err == nil
	default:
		return 0, false
	}
}

func schemaError(path, keyword, message string) JSONSchemaValidationError {
	return JSONSchemaValidationError{Path: path, Keyword: keyword, Message: message}
}

type arithmeticParser struct {
	input []rune
	pos   int
}

func (p *arithmeticParser) parseExpression() (float64, error) {
	value, err := p.parseTerm()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		if p.match('+') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value += right
			continue
		}
		if p.match('-') {
			right, err := p.parseTerm()
			if err != nil {
				return 0, err
			}
			value -= right
			continue
		}
		return value, nil
	}
}

func (p *arithmeticParser) parseTerm() (float64, error) {
	value, err := p.parseFactor()
	if err != nil {
		return 0, err
	}
	for {
		p.skipSpaces()
		switch {
		case p.match('*'):
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			value *= right
		case p.match('/'):
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("division by zero")
			}
			value /= right
		case p.match('%'):
			right, err := p.parseFactor()
			if err != nil {
				return 0, err
			}
			if right == 0 {
				return 0, fmt.Errorf("modulo by zero")
			}
			value = math.Mod(value, right)
		default:
			return value, nil
		}
	}
}

func (p *arithmeticParser) parseFactor() (float64, error) {
	p.skipSpaces()
	if p.match('+') {
		return p.parseFactor()
	}
	if p.match('-') {
		value, err := p.parseFactor()
		return -value, err
	}
	if p.match('(') {
		value, err := p.parseExpression()
		if err != nil {
			return 0, err
		}
		p.skipSpaces()
		if !p.match(')') {
			return 0, fmt.Errorf("missing closing parenthesis")
		}
		return value, nil
	}
	return p.parseNumber()
}

func (p *arithmeticParser) parseNumber() (float64, error) {
	p.skipSpaces()
	start := p.pos
	seenDigit := false
	seenDot := false
	for p.pos < len(p.input) {
		char := p.input[p.pos]
		switch {
		case unicode.IsDigit(char):
			seenDigit = true
			p.pos++
		case char == '.' && !seenDot:
			seenDot = true
			p.pos++
		default:
			goto parsed
		}
	}
parsed:
	if !seenDigit {
		return 0, fmt.Errorf("expected number at position %d", start+1)
	}
	value, err := strconv.ParseFloat(string(p.input[start:p.pos]), 64)
	if err != nil || math.IsInf(value, 0) || math.IsNaN(value) {
		return 0, fmt.Errorf("invalid number at position %d", start+1)
	}
	return value, nil
}

func (p *arithmeticParser) skipSpaces() {
	for p.pos < len(p.input) && unicode.IsSpace(p.input[p.pos]) {
		p.pos++
	}
}

func (p *arithmeticParser) match(expected rune) bool {
	if p.pos >= len(p.input) || p.input[p.pos] != expected {
		return false
	}
	p.pos++
	return true
}
