// privileged_trace_redaction.go applies the mandatory denylist before privileged payload encryption.
// privileged_trace_redaction.go 在 privileged payload 加密前应用强制 denylist。
package runtime

import (
	"encoding/json"
	"regexp"
	"strings"
	"unicode/utf8"
)

const privilegedTraceRedactedValue = "[redacted]"

var privilegedTraceCredentialPattern = regexp.MustCompile(`(?i)(authorization\s*[:=]|bearer\s+[a-z0-9._~+/=-]+|(?:api[-_ ]?key|access[-_ ]?token|refresh[-_ ]?token|password|secret|credential)\s*[:=]\s*\S+|\bsk-[a-z0-9_-]{8,}|\bAKIA[A-Z0-9]{8,})`)
var privilegedTraceForbiddenContentPattern = regexp.MustCompile(`(?i)(-----BEGIN [A-Z ]*PRIVATE KEY-----|\beyJ[a-zA-Z0-9_-]{8,}\.[a-zA-Z0-9_-]{8,}\.[a-zA-Z0-9_-]{8,}\b|data:[^,;\s]+(?:;[^,\s]+)*;base64,[a-z0-9+/=]{16,}|(?:broker(?:age)?[-_ ]?account|securities[-_ ]?account|account[-_ ]?(?:id|number)|券商账?号|证券账户|资金账号|账户号码)\s*[:=：]?\s*[a-z0-9*_-]{4,})`)

// RedactPrivilegedTracePayload recursively removes forbidden fields and credential-like strings.
// RedactPrivilegedTracePayload 递归移除禁止字段和疑似凭据字符串。
func RedactPrivilegedTracePayload(value any, maxFieldBytes int) any {
	redacted, _ := RedactPrivilegedTracePayloadWithFields(value, maxFieldBytes, nil)
	return redacted
}

// RedactPrivilegedTracePayloadWithFields applies the mandatory and deployment-specific denylist and reports replacements.
// RedactPrivilegedTracePayloadWithFields 应用强制及部署自定义 denylist，并返回替换字段数量。
func RedactPrivilegedTracePayloadWithFields(value any, maxFieldBytes int, extraFields []string) (any, int) {
	return redactPrivilegedTraceValue(value, maxFieldBytes, normalizedPrivilegedTraceFields(extraFields))
}

func redactPrivilegedTraceValue(value any, maxFieldBytes int, extraFields map[string]struct{}) (any, int) {
	if value == nil {
		return nil, 0
	}
	switch typed := value.(type) {
	case map[string]any:
		output := make(map[string]any, len(typed))
		redactedCount := 0
		for key, child := range typed {
			if privilegedTraceForbiddenKey(key, extraFields) {
				output[key] = privilegedTraceRedactedValue
				redactedCount++
				continue
			}
			redacted, count := redactPrivilegedTraceValue(child, maxFieldBytes, extraFields)
			output[key] = redacted
			redactedCount += count
		}
		return output, redactedCount
	case []any:
		output := make([]any, len(typed))
		redactedCount := 0
		for index, child := range typed {
			redacted, count := redactPrivilegedTraceValue(child, maxFieldBytes, extraFields)
			output[index] = redacted
			redactedCount += count
		}
		return output, redactedCount
	case string:
		if privilegedTraceCredentialPattern.MatchString(typed) || privilegedTraceForbiddenContentPattern.MatchString(typed) {
			return privilegedTraceRedactedValue, 1
		}
		return truncatePrivilegedTraceString(typed, maxFieldBytes), 0
	default:
		raw, err := json.Marshal(value)
		if err != nil {
			return privilegedTraceRedactedValue, 1
		}
		var generic any
		if err := json.Unmarshal(raw, &generic); err != nil {
			return privilegedTraceRedactedValue, 1
		}
		return redactPrivilegedTraceValue(generic, maxFieldBytes, extraFields)
	}
}

func privilegedTraceForbiddenKey(key string, extraFields map[string]struct{}) bool {
	normalized := normalizePrivilegedTraceField(key)
	if _, found := extraFields[normalized]; found {
		return true
	}
	markers := []string{
		"reasoning", "chainofthought", "hiddenthought", "internalthought",
		"authorization", "credential", "password", "passwd", "secret", "apikey", "token",
		"cookie", "setcookie", "accountid", "accountnumber", "brokeraccount", "brokerageaccount",
		"customerid", "userid", "attachmentbody", "attachmentcontent", "filebody", "filecontent",
	}
	for _, marker := range markers {
		if strings.Contains(normalized, marker) {
			return true
		}
	}
	return false
}

func normalizedPrivilegedTraceFields(fields []string) map[string]struct{} {
	result := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		if normalized := normalizePrivilegedTraceField(field); normalized != "" {
			result[normalized] = struct{}{}
		}
	}
	return result
}

func normalizePrivilegedTraceField(value string) string {
	return strings.ToLower(strings.Map(func(r rune) rune {
		if r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		if r >= 'A' && r <= 'Z' {
			return r + ('a' - 'A')
		}
		return -1
	}, value))
}

func truncatePrivilegedTraceString(value string, maxBytes int) string {
	if maxBytes <= 0 || len(value) <= maxBytes {
		return value
	}
	var builder strings.Builder
	for _, r := range value {
		if builder.Len()+utf8.RuneLen(r) > maxBytes {
			break
		}
		builder.WriteRune(r)
	}
	return builder.String()
}
