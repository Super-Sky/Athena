package inspection

import "testing"

// TestReportStructCarriesFindings verifies the structured report contract can carry findings.
// TestReportStructCarriesFindings 用于验证结构化报告契约可以携带发现项。
func TestReportStructCarriesFindings(t *testing.T) {
	report := Report{
		ReportID: "report-1",
		Findings: []Finding{{FindingID: "finding-1", Title: "high-risk config"}},
	}
	if len(report.Findings) != 1 {
		t.Fatalf("Findings length = %d, want 1", len(report.Findings))
	}
}

// TestBuildReportCreatesSummaryFinding verifies BuildReport emits one summary finding.
// TestBuildReportCreatesSummaryFinding 用于验证 BuildReport 会输出一条摘要发现项。
func TestBuildReportCreatesSummaryFinding(t *testing.T) {
	report := BuildReport("inspection", "summary", "medium", "answer")
	if report == nil || len(report.Findings) != 1 {
		t.Fatalf("unexpected report = %#v", report)
	}
}
