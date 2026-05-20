package alerts

import "testing"

// TestAlertStructStoresSeverity verifies the alert contract preserves severity information.
// TestAlertStructStoresSeverity 用于验证告警契约会保留严重度信息。
func TestAlertStructStoresSeverity(t *testing.T) {
	alert := Alert{AlertID: "alert-1", Severity: "high"}
	if alert.Severity != "high" {
		t.Fatalf("Severity = %q, want high", alert.Severity)
	}
}

// TestBuildAlertCreatesAlert verifies BuildAlert creates one structured alert.
// TestBuildAlertCreatesAlert 用于验证 BuildAlert 会创建结构化告警对象。
func TestBuildAlertCreatesAlert(t *testing.T) {
	alert := BuildAlert("title", "summary", "medium", "runtime")
	if alert == nil || alert.Title != "title" {
		t.Fatalf("unexpected alert = %#v", alert)
	}
}
