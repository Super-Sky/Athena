package observability

import (
	"bytes"
	"context"
	"log"
	"strings"
	"testing"
)

// TestDefaultManagerSnapshotsCaptureSignals ensures the default manager fans out into log and memory collectors.
// TestDefaultManagerSnapshotsCaptureSignals 用于验证默认 manager 会同时把信号写入日志和内存收集器。
func TestDefaultManagerSnapshotsCaptureSignals(t *testing.T) {
	t.Parallel()

	manager := NewDefaultManager()
	ctx := context.Background()

	manager.Emit(ctx, Event{
		Name:      "runtime.waiting_for_information",
		RequestID: "req-1",
		SessionID: "sess-1",
		Detail: map[string]any{
			"resume_token": "resume-1",
		},
	})
	manager.Trace(ctx, "runtime.prepare.started", map[string]string{
		"request_id": "req-1",
	})
	manager.Inc("runtime_waiting_total", map[string]string{
		"skill": "user_overview",
	})
	manager.Observe("runtime_turn_prepare_ms", 12, map[string]string{
		"skill": "user_overview",
	})
	manager.RecordAudit(ctx, AuditRecord{
		Action:    "information_request_created",
		RequestID: "req-1",
		SessionID: "sess-1",
		Detail: map[string]any{
			"missing_count": 1,
		},
	})

	if got := len(manager.SnapshotEvents()); got != 1 {
		t.Fatalf("expected 1 event snapshot, got %d", got)
	}
	if got := len(manager.SnapshotTraces()); got != 1 {
		t.Fatalf("expected 1 trace snapshot, got %d", got)
	}
	if got := len(manager.SnapshotMetrics()); got != 2 {
		t.Fatalf("expected 2 metric snapshots, got %d", got)
	}
	if got := len(manager.SnapshotAudits()); got != 1 {
		t.Fatalf("expected 1 audit snapshot, got %d", got)
	}
}

// TestSnapshotCopiesAreDefensive ensures in-memory snapshots cannot mutate recorder state from the outside.
// TestSnapshotCopiesAreDefensive 用于验证内存快照是防御性拷贝，不会被外部修改污染。
func TestSnapshotCopiesAreDefensive(t *testing.T) {
	t.Parallel()

	manager := NewDefaultManager()
	ctx := context.Background()

	manager.Emit(ctx, Event{
		Name: "event-1",
		Detail: map[string]any{
			"nested": map[string]any{"status": "waiting"},
		},
	})
	manager.Trace(ctx, "trace-1", map[string]string{"key": "value"})
	manager.Inc("metric-1", map[string]string{"label": "one"})
	manager.RecordAudit(ctx, AuditRecord{
		Action: "audit-1",
		Detail: map[string]any{
			"count": 1,
		},
	})

	eventSnapshots := manager.SnapshotEvents()
	traceSnapshots := manager.SnapshotTraces()
	metricSnapshots := manager.SnapshotMetrics()
	auditSnapshots := manager.SnapshotAudits()

	eventSnapshots[0].Detail["nested"] = "changed"
	traceSnapshots[0].Attrs["key"] = "changed"
	metricSnapshots[0].Labels["label"] = "changed"
	auditSnapshots[0].Detail["count"] = 99

	if got := manager.SnapshotEvents()[0].Detail["nested"]; got == "changed" {
		t.Fatalf("expected event detail to remain immutable")
	}
	if got := manager.SnapshotTraces()[0].Attrs["key"]; got != "value" {
		t.Fatalf("expected trace attrs to remain unchanged, got %q", got)
	}
	if got := manager.SnapshotMetrics()[0].Labels["label"]; got != "one" {
		t.Fatalf("expected metric labels to remain unchanged, got %q", got)
	}
	if got := manager.SnapshotAudits()[0].Detail["count"]; got != 1 {
		t.Fatalf("expected audit detail to remain unchanged, got %#v", got)
	}
}

// TestDefaultManagerWithInfoLevelSkipsDebugStreams verifies info mode keeps key events while skipping debug-only trace and metric logs.
// TestDefaultManagerWithInfoLevelSkipsDebugStreams 用于验证 info 模式会保留关键事件日志，同时跳过仅用于调试的 trace 和 metric 日志。
func TestDefaultManagerWithInfoLevelSkipsDebugStreams(t *testing.T) {
	output := captureStandardLogger(t, func() {
		manager := NewDefaultManagerWithLevel(LogLevelInfo)
		ctx := context.Background()

		manager.Emit(ctx, Event{Name: "runtime.prepare.started"})
		manager.Trace(ctx, "runtime.prepare.trace", map[string]string{"request_id": "req-1"})
		manager.Inc("runtime_prepare_started_total", map[string]string{"stage": "prepare"})
		manager.RecordAudit(ctx, AuditRecord{Action: "runtime_prepare_started"})
	})

	if !strings.Contains(output, "[info][event]") {
		t.Fatalf("expected info event log, got %q", output)
	}
	if !strings.Contains(output, "[info][audit]") {
		t.Fatalf("expected info audit log, got %q", output)
	}
	if strings.Contains(output, "[debug][trace]") {
		t.Fatalf("did not expect debug trace log in info mode, got %q", output)
	}
	if strings.Contains(output, "[debug][metric-inc]") {
		t.Fatalf("did not expect debug metric log in info mode, got %q", output)
	}
}

// TestDefaultManagerWithDebugLevelEmitsTraceAndMetrics verifies debug mode exposes the detailed trace and metric diagnostics.
// TestDefaultManagerWithDebugLevelEmitsTraceAndMetrics 用于验证 debug 模式会输出更完整的 trace 和 metric 调试日志。
func TestDefaultManagerWithDebugLevelEmitsTraceAndMetrics(t *testing.T) {
	output := captureStandardLogger(t, func() {
		manager := NewDefaultManagerWithLevel(LogLevelDebug)
		ctx := context.Background()

		manager.Trace(ctx, "runtime.prepare.trace", map[string]string{"request_id": "req-1"})
		manager.Inc("runtime_prepare_started_total", map[string]string{"stage": "prepare"})
		manager.Observe("runtime_turn_prepare_ms", 12, map[string]string{"skill": "user_overview"})
		manager.Emit(ctx, Event{
			Name:  "runtime.queue_overflow",
			Level: string(LogLevelWarn),
		})
	})

	if !strings.Contains(output, "[debug][trace]") {
		t.Fatalf("expected debug trace log, got %q", output)
	}
	if !strings.Contains(output, "[debug][metric-inc]") {
		t.Fatalf("expected debug metric log, got %q", output)
	}
	if !strings.Contains(output, "[debug][metric-observe]") {
		t.Fatalf("expected debug metric observe log, got %q", output)
	}
	if !strings.Contains(output, "[warn][event]") {
		t.Fatalf("expected warn event log, got %q", output)
	}
}

func TestLogActionWritesStructuredActionPayload(t *testing.T) {
	output := captureStandardLogger(t, func() {
		LogAction(LogLevelWarn, ActionLog{
			Module:    "server",
			Action:    "chat_stream",
			Step:      "write_done",
			Status:    "error",
			RequestID: "req-1",
			SessionID: "sess-1",
			Reason:    "sse_write_failed",
			ErrorCode: "sse_done_write_failed",
			Detail: map[string]any{
				"duration_ms": 12,
			},
		})
	})
	if !strings.Contains(output, "[warn][action]") {
		t.Fatalf("expected warn action log, got %q", output)
	}
	if !strings.Contains(output, "\"module\":\"server\"") || !strings.Contains(output, "\"action\":\"chat_stream\"") {
		t.Fatalf("expected structured module/action payload, got %q", output)
	}
}

func captureStandardLogger(t *testing.T, fn func()) string {
	t.Helper()

	previousWriter := log.Writer()
	previousFlags := log.Flags()

	var buffer bytes.Buffer
	log.SetOutput(&buffer)
	log.SetFlags(0)
	defer func() {
		log.SetOutput(previousWriter)
		log.SetFlags(previousFlags)
	}()

	fn()
	return buffer.String()
}
