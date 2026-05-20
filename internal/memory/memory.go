// memory.go implements context preparation and preserved-context compression helpers for runtime turns.
// memory.go 实现 runtime 单轮所需的上下文准备与 preserved context 压缩辅助逻辑。
package memory

import (
	"fmt"
	"sort"
	"strings"

	"moss/internal/session"
)

// ContextPolicy controls how historical messages are compressed before runtime consumption.
// ContextPolicy 控制 runtime 消费前历史消息如何被压缩整理。
type ContextPolicy struct {
	EnableSummaryCompression bool
	EnableLongTermRetrieval  bool
	EnableToolResultOffload  bool
	CompressionThreshold     int
}

// DefaultContextPolicy returns the repository default policy for history preparation.
// DefaultContextPolicy 返回仓库默认使用的历史准备策略。
func DefaultContextPolicy() ContextPolicy {
	return ContextPolicy{
		EnableSummaryCompression: true,
		EnableLongTermRetrieval:  true,
		EnableToolResultOffload:  true,
		CompressionThreshold:     12000,
	}
}

// PrepareHistory compresses long message history into a shorter runtime-ready sequence when needed.
// PrepareHistory 会在需要时把长消息历史压缩为更短的 runtime 可消费序列。
func PrepareHistory(messages []session.Message, policy ContextPolicy) []session.Message {
	if !policy.EnableSummaryCompression {
		return append([]session.Message(nil), messages...)
	}

	total := 0
	for _, msg := range messages {
		total += len(msg.Content)
	}
	if total <= policy.CompressionThreshold || len(messages) <= 6 {
		return append([]session.Message(nil), messages...)
	}

	recentStart := len(messages) - 4
	if recentStart < 0 {
		recentStart = 0
	}

	older := messages[:recentStart]
	recent := messages[recentStart:]

	var summaryParts []string
	for _, msg := range older {
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		if len(content) > 80 {
			content = content[:80] + "..."
		}
		summaryParts = append(summaryParts, fmt.Sprintf("%s: %s", msg.Role, content))
	}

	if len(summaryParts) == 0 {
		return append([]session.Message(nil), recent...)
	}

	summary := session.Message{
		Role:    "user",
		Content: "Earlier conversation summary:\n" + strings.Join(summaryParts, "\n"),
	}

	result := make([]session.Message, 0, 1+len(recent))
	result = append(result, summary)
	result = append(result, recent...)
	return result
}

// PrepareHistoryWithPreservedContext prepends preserved continuity context onto prepared history.
// PrepareHistoryWithPreservedContext 会把 preserved continuity context 前置到已整理历史之前。
func PrepareHistoryWithPreservedContext(messages []session.Message, preserved *session.PreservedContext, policy ContextPolicy) []session.Message {
	prepared := PrepareHistory(messages, policy)
	summary := formatPreservedContext(preserved)
	if summary == "" {
		return prepared
	}

	result := make([]session.Message, 0, len(prepared)+1)
	result = append(result, session.Message{
		Role:    "user",
		Content: summary,
	})
	result = append(result, prepared...)
	return result
}

func formatPreservedContext(preserved *session.PreservedContext) string {
	if preserved == nil {
		return ""
	}

	lines := []string{"Preserved continuity context:"}
	if goal := strings.TrimSpace(preserved.Goal); goal != "" {
		lines = append(lines, fmt.Sprintf("Goal: %s", goal))
	}
	if intent := strings.TrimSpace(preserved.LastUserIntent); intent != "" {
		lines = append(lines, fmt.Sprintf("Last user intent: %s", intent))
	}
	if len(preserved.Facts) > 0 {
		keys := make([]string, 0, len(preserved.Facts))
		for key := range preserved.Facts {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		lines = append(lines, "Known facts:")
		for _, key := range keys {
			lines = append(lines, fmt.Sprintf("- %s=%s", key, preserved.Facts[key]))
		}
	}
	if len(preserved.MissingFields) > 0 {
		lines = append(lines, fmt.Sprintf("Missing fields: %s", strings.Join(preserved.MissingFields, ", ")))
	}
	if reason := strings.TrimSpace(preserved.DegradeReason); reason != "" {
		lines = append(lines, fmt.Sprintf("Degrade reason: %s", reason))
	}
	if reason := strings.TrimSpace(preserved.PendingHumanReason); reason != "" {
		lines = append(lines, fmt.Sprintf("Pending human reason: %s", reason))
	}
	if len(lines) == 1 {
		return ""
	}
	return strings.Join(lines, "\n")
}
