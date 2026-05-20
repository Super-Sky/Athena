// redaction.go keeps validation MCP payloads safe before they reach traces or UI output.
// redaction.go 确保 validation MCP payload 进入 trace 或 UI 输出前已完成安全脱敏。
package validationmcp

import "strings"

const redactedValue = "[redacted]"

// RedactSensitiveMap recursively redacts credential-like keys from one map.
// RedactSensitiveMap 会递归脱敏一个 map 中类似凭证的字段。
func RedactSensitiveMap(input map[string]any) (map[string]any, bool) {
	if len(input) == 0 {
		return nil, false
	}
	result := make(map[string]any, len(input))
	var redacted bool
	for key, value := range input {
		if isSensitiveKey(key) {
			result[key] = redactedValue
			redacted = true
			continue
		}
		next, childRedacted := redactSensitiveValue(value)
		result[key] = next
		redacted = redacted || childRedacted
	}
	return result, redacted
}

func redactSensitiveValue(value any) (any, bool) {
	switch typed := value.(type) {
	case map[string]any:
		return RedactSensitiveMap(typed)
	case []any:
		result := make([]any, len(typed))
		var redacted bool
		for index, item := range typed {
			next, childRedacted := redactSensitiveValue(item)
			result[index] = next
			redacted = redacted || childRedacted
		}
		return result, redacted
	default:
		return value, false
	}
}

func isSensitiveKey(key string) bool {
	lower := strings.ToLower(strings.TrimSpace(key))
	if lower == "" {
		return false
	}
	return strings.Contains(lower, "token") ||
		strings.Contains(lower, "secret") ||
		strings.Contains(lower, "password") ||
		strings.Contains(lower, "credential") ||
		strings.Contains(lower, "api_key") ||
		strings.Contains(lower, "authorization")
}

func cloneAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]any, len(input))
	for key, value := range input {
		result[key] = cloneAnyValue(value)
	}
	return result
}

func cloneAnyValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneAnyMap(typed)
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = cloneAnyValue(item)
		}
		return result
	default:
		return value
	}
}

func valueAsString(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return ""
	}
}
