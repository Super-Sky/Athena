// alert.go defines the structured alert contract consumed by downstream platform flows.
// alert.go 定义下游平台流程消费的结构化告警契约。
package alerts

import "moss/internal/observability"

// Alert captures one structured alert emitted by Athena.
// Alert 描述 Athena 发出的一条结构化告警。
type Alert struct {
	AlertID  string         `json:"alert_id,omitempty"`
	Title    string         `json:"title,omitempty"`
	Severity string         `json:"severity,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Category string         `json:"category,omitempty"`
	Detail   map[string]any `json:"detail,omitempty"`
}

// BuildAlert returns a minimal structured alert from runtime summary inputs.
// BuildAlert 会根据 runtime 摘要输入构建最小结构化告警。
func BuildAlert(title, summary, severity, category string) *Alert {
	alert := &Alert{
		AlertID:  "alert-summary",
		Title:    title,
		Severity: severity,
		Summary:  summary,
		Category: category,
	}
	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "alerts",
		Action: "build_alert",
		Step:   "completed",
		Status: "ok",
		Reason: "alert_built",
		Detail: map[string]any{
			"alert_id": alert.AlertID,
			"title":    title,
			"severity": severity,
			"category": category,
		},
	})
	return alert
}
