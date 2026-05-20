// report.go defines the structured inspection progress and report contracts.
// report.go 定义结构化体检进度与报告契约。
package inspection

import "moss/internal/observability"

// ProgressEvent captures one incremental inspection progress update emitted to the platform.
// ProgressEvent 描述一次向平台发出的增量体检进度更新。
type ProgressEvent struct {
	EventID  string         `json:"event_id,omitempty"`
	Phase    string         `json:"phase,omitempty"`
	Status   string         `json:"status,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Progress int            `json:"progress,omitempty"`
	Detail   map[string]any `json:"detail,omitempty"`
}

// Report captures the structured inspection report returned after inspection analysis.
// Report 描述体检分析完成后返回的结构化体检报告。
type Report struct {
	ReportID string         `json:"report_id,omitempty"`
	Title    string         `json:"title,omitempty"`
	Summary  string         `json:"summary,omitempty"`
	Severity string         `json:"severity,omitempty"`
	Findings []Finding      `json:"findings,omitempty"`
	Detail   map[string]any `json:"detail,omitempty"`
}

// Finding captures one inspection finding inside the report.
// Finding 描述体检报告中的单条发现项。
type Finding struct {
	FindingID string         `json:"finding_id,omitempty"`
	Title     string         `json:"title,omitempty"`
	Severity  string         `json:"severity,omitempty"`
	Summary   string         `json:"summary,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
}

// BuildReport returns a minimal structured inspection report from summary inputs.
// BuildReport 会根据摘要输入构建最小结构化体检报告。
func BuildReport(title, summary, severity, answer string) *Report {
	report := &Report{
		ReportID: "report-summary",
		Title:    title,
		Summary:  summary,
		Severity: severity,
		Findings: []Finding{{
			FindingID: "finding-summary",
			Title:     title,
			Severity:  severity,
			Summary:   answer,
		}},
	}
	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "inspection",
		Action: "build_report",
		Step:   "completed",
		Status: "ok",
		Reason: "report_built",
		Detail: map[string]any{
			"report_id": report.ReportID,
			"title":     title,
			"severity":  severity,
			"findings":  len(report.Findings),
		},
	})
	return report
}
