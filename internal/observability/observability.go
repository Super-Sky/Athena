// observability.go provides the repository-wide event, trace, metric, and audit entrypoints.
// observability.go 提供仓库级事件、trace、metric 和 audit 统一入口。
package observability

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// LogLevel describes the verbosity threshold used by the default log-backed observability sinks.
// LogLevel 描述默认日志型 observability sink 使用的详细程度阈值。
type LogLevel string

const (
	LogLevelDebug LogLevel = "debug"
	LogLevelInfo  LogLevel = "info"
	LogLevelWarn  LogLevel = "warn"
	LogLevelError LogLevel = "error"
)

// Event describes one structured runtime event emitted by the scaffold.
// Event 描述脚手架发出的单条结构化运行事件。
type Event struct {
	Name      string         `json:"name"`
	RequestID string         `json:"request_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Turn      int            `json:"turn,omitempty"`
	Stage     string         `json:"stage,omitempty"`
	Level     string         `json:"level,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
	At        time.Time      `json:"at"`
}

// AuditRecord captures a state transition or delivery-relevant action for later inspection.
// AuditRecord 用于记录状态切换或交付相关动作，便于后续审计与回溯。
type AuditRecord struct {
	Action    string         `json:"action"`
	RequestID string         `json:"request_id,omitempty"`
	SessionID string         `json:"session_id,omitempty"`
	Level     string         `json:"level,omitempty"`
	Detail    map[string]any `json:"detail,omitempty"`
	At        time.Time      `json:"at"`
}

// TraceRecord stores one trace span marker emitted by the scaffold.
// TraceRecord 保存脚手架发出的单条 trace 标记。
type TraceRecord struct {
	Name  string            `json:"name"`
	Attrs map[string]string `json:"attrs,omitempty"`
	Level string            `json:"level,omitempty"`
	At    time.Time         `json:"at"`
}

// MetricSample stores one counter or observation point in memory.
// MetricSample 用于在内存中保存一次计数或观测样本。
type MetricSample struct {
	Name   string            `json:"name"`
	Kind   string            `json:"kind"`
	Value  float64           `json:"value"`
	Level  string            `json:"level,omitempty"`
	Labels map[string]string `json:"labels,omitempty"`
	At     time.Time         `json:"at"`
}

// ActionLog captures one structured module/action/step log record with outcome and timing data.
// ActionLog 描述一条带结果与耗时信息的结构化模块/动作/步骤日志。
type ActionLog struct {
	Module     string         `json:"module,omitempty"`
	Action     string         `json:"action,omitempty"`
	Step       string         `json:"step,omitempty"`
	Status     string         `json:"status,omitempty"`
	RequestID  string         `json:"request_id,omitempty"`
	SessionID  string         `json:"session_id,omitempty"`
	Reason     string         `json:"reason,omitempty"`
	ErrorCode  string         `json:"error_code,omitempty"`
	DurationMS int64          `json:"duration_ms,omitempty"`
	Detail     map[string]any `json:"detail,omitempty"`
	At         time.Time      `json:"at"`
}

// EventEmitter emits structured events.
// EventEmitter 负责发出结构化事件。
type EventEmitter interface {
	Emit(context.Context, Event)
}

// Tracer records trace-like markers for important path transitions.
// Tracer 负责记录关键路径上的 trace 标记。
type Tracer interface {
	Trace(context.Context, string, map[string]string)
}

// MetricsRecorder records counters and observations.
// MetricsRecorder 负责记录计数与观测指标。
type MetricsRecorder interface {
	Inc(string, map[string]string)
	Observe(string, float64, map[string]string)
}

// AuditRecorder stores auditable state transitions.
// AuditRecorder 负责记录可审计的状态切换。
type AuditRecorder interface {
	Record(context.Context, AuditRecord)
}

// Manager is the observability facade used by app, runtime, and transport.
// Manager 是 app、runtime 和 transport 共用的观测门面。
type Manager struct {
	Emitter EventEmitter
	Tracer  Tracer
	Metrics MetricsRecorder
	Audit   AuditRecorder
}

// NewDefaultManager creates a fanout manager that logs and keeps in-memory snapshots at info level.
// NewDefaultManager 创建默认的 fanout manager，默认按 info 级别输出日志并保留内存快照。
func NewDefaultManager() *Manager {
	return NewDefaultManagerWithLevel(LogLevelInfo)
}

// NewDefaultManagerWithLevel creates a fanout manager with configurable log verbosity.
// NewDefaultManagerWithLevel 会按指定日志级别创建默认的 fanout manager。
func NewDefaultManagerWithLevel(level LogLevel) *Manager {
	level = normalizeLogLevel(level)
	return &Manager{
		Emitter: MultiEmitter{LogEmitter{Level: level}, NewMemoryEmitter()},
		Tracer:  MultiTracer{LogTracer{Level: level}, NewMemoryTracer()},
		Metrics: MultiMetrics{LogMetrics{Level: level}, NewMemoryMetrics()},
		Audit:   MultiAuditRecorder{LogAuditRecorder{Level: level}, NewMemoryAuditRecorder()},
	}
}

// NewNoopManager creates a manager that drops all observability output.
// NewNoopManager 创建一个会忽略所有观测输出的 manager。
func NewNoopManager() *Manager {
	return &Manager{
		Emitter: NoopEmitter{},
		Tracer:  NoopTracer{},
		Metrics: NoopMetrics{},
		Audit:   NoopAuditRecorder{},
	}
}

// Emit records a structured event.
// Emit 负责记录一条结构化事件。
func (m *Manager) Emit(ctx context.Context, event Event) {
	if m == nil || m.Emitter == nil {
		return
	}
	if event.At.IsZero() {
		event.At = time.Now()
	}
	if strings.TrimSpace(event.Level) == "" {
		event.Level = string(LogLevelInfo)
	}
	m.Emitter.Emit(ctx, event)
}

// Trace records a trace marker.
// Trace 负责记录一条 trace 标记。
func (m *Manager) Trace(ctx context.Context, name string, attrs map[string]string) {
	if m == nil || m.Tracer == nil {
		return
	}
	m.Tracer.Trace(ctx, name, attrs)
}

// Inc increments a counter-like metric.
// Inc 负责记录一次计数型指标。
func (m *Manager) Inc(name string, labels map[string]string) {
	if m == nil || m.Metrics == nil {
		return
	}
	m.Metrics.Inc(name, labels)
}

// Observe records one sampled metric value.
// Observe 负责记录一次采样型指标。
func (m *Manager) Observe(name string, value float64, labels map[string]string) {
	if m == nil || m.Metrics == nil {
		return
	}
	m.Metrics.Observe(name, value, labels)
}

// RecordAudit stores one audit record.
// RecordAudit 负责记录一条审计记录。
func (m *Manager) RecordAudit(ctx context.Context, record AuditRecord) {
	if m == nil || m.Audit == nil {
		return
	}
	if record.At.IsZero() {
		record.At = time.Now()
	}
	if strings.TrimSpace(record.Level) == "" {
		record.Level = string(LogLevelInfo)
	}
	m.Audit.Record(ctx, record)
}

// LogAction records one structured action log through the event pipeline.
// LogAction 会通过事件通道记录一条结构化动作日志。
func (m *Manager) LogAction(ctx context.Context, level LogLevel, record ActionLog) {
	if m == nil {
		LogAction(level, record)
		return
	}
	m.Emit(ctx, Event{
		Name:      buildActionEventName(record),
		RequestID: strings.TrimSpace(record.RequestID),
		SessionID: strings.TrimSpace(record.SessionID),
		Stage:     strings.TrimSpace(record.Step),
		Level:     string(normalizeLogLevel(level)),
		Detail:    actionDetail(record),
		At:        ensureActionTime(record.At),
	})
}

// SnapshotEvents returns any in-memory events exposed by the current emitter chain.
// SnapshotEvents 返回当前 emitter 链中可用的内存事件快照。
func (m *Manager) SnapshotEvents() []Event {
	if m == nil {
		return nil
	}
	return snapshotEventsFromEmitter(m.Emitter)
}

// SnapshotTraces returns any in-memory trace records exposed by the current tracer chain.
// SnapshotTraces 返回当前 tracer 链中可用的内存 trace 快照。
func (m *Manager) SnapshotTraces() []TraceRecord {
	if m == nil {
		return nil
	}
	return snapshotTracesFromTracer(m.Tracer)
}

// SnapshotMetrics returns any in-memory metric samples exposed by the current metrics chain.
// SnapshotMetrics 返回当前 metrics 链中可用的内存指标快照。
func (m *Manager) SnapshotMetrics() []MetricSample {
	if m == nil {
		return nil
	}
	return snapshotMetricsFromRecorder(m.Metrics)
}

// SnapshotAudits returns any in-memory audit records exposed by the current audit chain.
// SnapshotAudits 返回当前 audit 链中可用的内存审计快照。
func (m *Manager) SnapshotAudits() []AuditRecord {
	if m == nil {
		return nil
	}
	return snapshotAuditsFromRecorder(m.Audit)
}

// MemoryEventSource exposes in-memory event snapshots.
// MemoryEventSource 暴露内存事件快照。
type MemoryEventSource interface {
	SnapshotEvents() []Event
}

// MemoryTraceSource exposes in-memory trace snapshots.
// MemoryTraceSource 暴露内存 trace 快照。
type MemoryTraceSource interface {
	SnapshotTraces() []TraceRecord
}

// MemoryMetricSource exposes in-memory metric snapshots.
// MemoryMetricSource 暴露内存指标快照。
type MemoryMetricSource interface {
	SnapshotMetrics() []MetricSample
}

// MemoryAuditSource exposes in-memory audit snapshots.
// MemoryAuditSource 暴露内存审计快照。
type MemoryAuditSource interface {
	SnapshotAudits() []AuditRecord
}

// MultiEmitter fans one event out to multiple emitters.
// MultiEmitter 会把一条事件同时分发给多个 emitter。
type MultiEmitter []EventEmitter

// Emit fans the event out to all configured emitters.
// Emit 会把事件广播给所有配置好的 emitter。
func (m MultiEmitter) Emit(ctx context.Context, event Event) {
	for _, emitter := range m {
		if emitter != nil {
			emitter.Emit(ctx, event)
		}
	}
}

// MultiTracer fans one trace marker out to multiple tracers.
// MultiTracer 会把一条 trace 标记同时分发给多个 tracer。
type MultiTracer []Tracer

// Trace fans the trace marker out to all configured tracers.
// Trace 会把 trace 标记广播给所有配置好的 tracer。
func (m MultiTracer) Trace(ctx context.Context, name string, attrs map[string]string) {
	for _, tracer := range m {
		if tracer != nil {
			tracer.Trace(ctx, name, attrs)
		}
	}
}

// MultiMetrics fans metrics out to multiple recorders.
// MultiMetrics 会把指标同时写入多个 recorder。
type MultiMetrics []MetricsRecorder

// Inc fans the increment out to all configured metric recorders.
// Inc 会把计数指标广播给所有配置好的 metric recorder。
func (m MultiMetrics) Inc(name string, labels map[string]string) {
	for _, metrics := range m {
		if metrics != nil {
			metrics.Inc(name, labels)
		}
	}
}

// Observe fans the observation out to all configured metric recorders.
// Observe 会把采样指标广播给所有配置好的 metric recorder。
func (m MultiMetrics) Observe(name string, value float64, labels map[string]string) {
	for _, metrics := range m {
		if metrics != nil {
			metrics.Observe(name, value, labels)
		}
	}
}

// MultiAuditRecorder fans audit records out to multiple recorders.
// MultiAuditRecorder 会把审计记录同时写入多个 recorder。
type MultiAuditRecorder []AuditRecorder

// Record fans the audit record out to all configured audit recorders.
// Record 会把审计记录广播给所有配置好的 audit recorder。
func (m MultiAuditRecorder) Record(ctx context.Context, record AuditRecord) {
	for _, audit := range m {
		if audit != nil {
			audit.Record(ctx, record)
		}
	}
}

// MemoryEmitter keeps recent events in memory for tests and local inspection.
// MemoryEmitter 会在内存中保留事件，便于测试和本地检查。
type MemoryEmitter struct {
	mu     sync.Mutex
	events []Event
}

// NewMemoryEmitter constructs an in-memory event sink.
// NewMemoryEmitter 创建一个内存事件接收器。
func NewMemoryEmitter() *MemoryEmitter {
	return &MemoryEmitter{}
}

// Emit stores a copy of the event in memory.
// Emit 会把事件副本保存在内存中。
func (m *MemoryEmitter) Emit(_ context.Context, event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, cloneEvent(event))
}

// SnapshotEvents returns a defensive copy of all recorded events.
// SnapshotEvents 返回全部已记录事件的防御性拷贝。
func (m *MemoryEmitter) SnapshotEvents() []Event {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]Event, 0, len(m.events))
	for _, event := range m.events {
		out = append(out, cloneEvent(event))
	}
	return out
}

// MemoryTracer keeps trace markers in memory for tests and local inspection.
// MemoryTracer 会在内存中保留 trace 标记，便于测试和本地检查。
type MemoryTracer struct {
	mu     sync.Mutex
	traces []TraceRecord
}

// NewMemoryTracer constructs an in-memory trace sink.
// NewMemoryTracer 创建一个内存 trace 接收器。
func NewMemoryTracer() *MemoryTracer {
	return &MemoryTracer{}
}

// Trace stores one trace marker in memory.
// Trace 会把一条 trace 标记保存在内存中。
func (m *MemoryTracer) Trace(_ context.Context, name string, attrs map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.traces = append(m.traces, TraceRecord{
		Name:  name,
		Attrs: cloneStringMap(attrs),
		Level: string(LogLevelDebug),
		At:    time.Now(),
	})
}

// SnapshotTraces returns a defensive copy of recorded trace markers.
// SnapshotTraces 返回已记录 trace 标记的防御性拷贝。
func (m *MemoryTracer) SnapshotTraces() []TraceRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]TraceRecord, 0, len(m.traces))
	for _, trace := range m.traces {
		out = append(out, TraceRecord{
			Name:  trace.Name,
			Attrs: cloneStringMap(trace.Attrs),
			Level: trace.Level,
			At:    trace.At,
		})
	}
	return out
}

// MemoryMetrics keeps metric samples in memory for tests and local inspection.
// MemoryMetrics 会在内存中保留指标样本，便于测试和本地检查。
type MemoryMetrics struct {
	mu      sync.Mutex
	samples []MetricSample
}

// NewMemoryMetrics constructs an in-memory metrics sink.
// NewMemoryMetrics 创建一个内存指标接收器。
func NewMemoryMetrics() *MemoryMetrics {
	return &MemoryMetrics{}
}

// Inc stores one counter sample in memory.
// Inc 会把一次计数指标保存在内存中。
func (m *MemoryMetrics) Inc(name string, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.samples = append(m.samples, MetricSample{
		Name:   name,
		Kind:   "counter",
		Value:  1,
		Level:  string(LogLevelDebug),
		Labels: cloneStringMap(labels),
		At:     time.Now(),
	})
}

// Observe stores one observation sample in memory.
// Observe 会把一次观测指标保存在内存中。
func (m *MemoryMetrics) Observe(name string, value float64, labels map[string]string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.samples = append(m.samples, MetricSample{
		Name:   name,
		Kind:   "observation",
		Value:  value,
		Level:  string(LogLevelDebug),
		Labels: cloneStringMap(labels),
		At:     time.Now(),
	})
}

// SnapshotMetrics returns a defensive copy of all metric samples.
// SnapshotMetrics 返回全部指标样本的防御性拷贝。
func (m *MemoryMetrics) SnapshotMetrics() []MetricSample {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]MetricSample, 0, len(m.samples))
	for _, sample := range m.samples {
		out = append(out, MetricSample{
			Name:   sample.Name,
			Kind:   sample.Kind,
			Value:  sample.Value,
			Level:  sample.Level,
			Labels: cloneStringMap(sample.Labels),
			At:     sample.At,
		})
	}
	return out
}

// MemoryAuditRecorder keeps audit records in memory for tests and local inspection.
// MemoryAuditRecorder 会在内存中保留审计记录，便于测试和本地检查。
type MemoryAuditRecorder struct {
	mu      sync.Mutex
	records []AuditRecord
}

// NewMemoryAuditRecorder constructs an in-memory audit sink.
// NewMemoryAuditRecorder 创建一个内存审计接收器。
func NewMemoryAuditRecorder() *MemoryAuditRecorder {
	return &MemoryAuditRecorder{}
}

// Record stores one audit record in memory.
// Record 会把一条审计记录保存在内存中。
func (m *MemoryAuditRecorder) Record(_ context.Context, record AuditRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.records = append(m.records, cloneAuditRecord(record))
}

// SnapshotAudits returns a defensive copy of all audit records.
// SnapshotAudits 返回全部审计记录的防御性拷贝。
func (m *MemoryAuditRecorder) SnapshotAudits() []AuditRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]AuditRecord, 0, len(m.records))
	for _, record := range m.records {
		out = append(out, cloneAuditRecord(record))
	}
	return out
}

// LogEmitter writes events to the standard logger.
// LogEmitter 负责把事件写入标准日志。
type LogEmitter struct {
	Level LogLevel
}

// Emit writes one event log line.
// Emit 会输出一条事件日志。
func (l LogEmitter) Emit(_ context.Context, event Event) {
	level := normalizeLogLevelFromString(event.Level)
	if !shouldLog(l.Level, level) {
		return
	}
	payload, _ := json.Marshal(event)
	log.Printf("[%s][event] %s", level, payload)
}

// LogTracer writes trace markers to the standard logger.
// LogTracer 负责把 trace 标记写入标准日志。
type LogTracer struct {
	Level LogLevel
}

// Trace writes one trace log line.
// Trace 会输出一条 trace 日志。
func (l LogTracer) Trace(_ context.Context, name string, attrs map[string]string) {
	if !shouldLog(l.Level, LogLevelDebug) {
		return
	}
	payload, _ := json.Marshal(attrs)
	log.Printf("[%s][trace] name=%s attrs=%s", LogLevelDebug, name, payload)
}

// LogMetrics writes metric samples to the standard logger.
// LogMetrics 负责把指标样本写入标准日志。
type LogMetrics struct {
	Level LogLevel
}

// Inc writes one counter log line.
// Inc 会输出一条计数指标日志。
func (l LogMetrics) Inc(name string, labels map[string]string) {
	if !shouldLog(l.Level, LogLevelDebug) {
		return
	}
	payload, _ := json.Marshal(labels)
	log.Printf("[%s][metric-inc] name=%s labels=%s", LogLevelDebug, name, payload)
}

// Observe writes one observation log line.
// Observe 会输出一条观测指标日志。
func (l LogMetrics) Observe(name string, value float64, labels map[string]string) {
	if !shouldLog(l.Level, LogLevelDebug) {
		return
	}
	payload, _ := json.Marshal(labels)
	log.Printf("[%s][metric-observe] name=%s value=%f labels=%s", LogLevelDebug, name, value, payload)
}

// LogAuditRecorder writes audit records to the standard logger.
// LogAuditRecorder 负责把审计记录写入标准日志。
type LogAuditRecorder struct {
	Level LogLevel
}

// Record writes one audit log line.
// Record 会输出一条审计日志。
func (l LogAuditRecorder) Record(_ context.Context, record AuditRecord) {
	level := normalizeLogLevelFromString(record.Level)
	if !shouldLog(l.Level, level) {
		return
	}
	payload, _ := json.Marshal(record)
	log.Printf("[%s][audit] %s", level, payload)
}

// LogAction writes one structured module/action/step log directly to the standard logger.
// LogAction 会直接把一条结构化模块/动作/步骤日志写入标准日志。
func LogAction(level LogLevel, record ActionLog) {
	level = normalizeLogLevel(level)
	record.At = ensureActionTime(record.At)
	payload, _ := json.Marshal(record)
	log.Printf("[%s][action] %s", level, payload)
}

// NoopEmitter drops all events.
// NoopEmitter 会丢弃所有事件。
type NoopEmitter struct{}

// Emit drops the supplied event.
// Emit 会忽略传入事件。
func (NoopEmitter) Emit(context.Context, Event) {}

// NoopTracer drops all trace markers.
// NoopTracer 会丢弃所有 trace 标记。
type NoopTracer struct{}

// Trace drops the supplied trace marker.
// Trace 会忽略传入 trace 标记。
func (NoopTracer) Trace(context.Context, string, map[string]string) {}

// NoopMetrics drops all metric samples.
// NoopMetrics 会丢弃所有指标样本。
type NoopMetrics struct{}

// Inc drops the supplied counter increment.
// Inc 会忽略传入计数指标。
func (NoopMetrics) Inc(string, map[string]string) {}

// Observe drops the supplied observation.
// Observe 会忽略传入观测指标。
func (NoopMetrics) Observe(string, float64, map[string]string) {}

// NoopAuditRecorder drops all audit records.
// NoopAuditRecorder 会丢弃所有审计记录。
type NoopAuditRecorder struct{}

// Record drops the supplied audit record.
// Record 会忽略传入审计记录。
func (NoopAuditRecorder) Record(context.Context, AuditRecord) {}

func snapshotEventsFromEmitter(emitter EventEmitter) []Event {
	switch e := emitter.(type) {
	case MemoryEventSource:
		return e.SnapshotEvents()
	case MultiEmitter:
		var out []Event
		for _, item := range e {
			out = append(out, snapshotEventsFromEmitter(item)...)
		}
		return out
	default:
		return nil
	}
}

func snapshotTracesFromTracer(tracer Tracer) []TraceRecord {
	switch t := tracer.(type) {
	case MemoryTraceSource:
		return t.SnapshotTraces()
	case MultiTracer:
		var out []TraceRecord
		for _, item := range t {
			out = append(out, snapshotTracesFromTracer(item)...)
		}
		return out
	default:
		return nil
	}
}

func snapshotMetricsFromRecorder(metrics MetricsRecorder) []MetricSample {
	switch m := metrics.(type) {
	case MemoryMetricSource:
		return m.SnapshotMetrics()
	case MultiMetrics:
		var out []MetricSample
		for _, item := range m {
			out = append(out, snapshotMetricsFromRecorder(item)...)
		}
		return out
	default:
		return nil
	}
}

func snapshotAuditsFromRecorder(audit AuditRecorder) []AuditRecord {
	switch a := audit.(type) {
	case MemoryAuditSource:
		return a.SnapshotAudits()
	case MultiAuditRecorder:
		var out []AuditRecord
		for _, item := range a {
			out = append(out, snapshotAuditsFromRecorder(item)...)
		}
		return out
	default:
		return nil
	}
}

func cloneEvent(event Event) Event {
	return Event{
		Name:      event.Name,
		RequestID: event.RequestID,
		SessionID: event.SessionID,
		Turn:      event.Turn,
		Stage:     event.Stage,
		Level:     event.Level,
		Detail:    cloneDetailMap(event.Detail),
		At:        event.At,
	}
}

func cloneAuditRecord(record AuditRecord) AuditRecord {
	return AuditRecord{
		Action:    record.Action,
		RequestID: record.RequestID,
		SessionID: record.SessionID,
		Level:     record.Level,
		Detail:    cloneDetailMap(record.Detail),
		At:        record.At,
	}
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func cloneDetailMap(values map[string]any) map[string]any {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(values))
	for key, value := range values {
		cloned[key] = cloneAny(value)
	}
	return cloned
}

func actionDetail(record ActionLog) map[string]any {
	detail := map[string]any{
		"module":     strings.TrimSpace(record.Module),
		"action":     strings.TrimSpace(record.Action),
		"step":       strings.TrimSpace(record.Step),
		"status":     strings.TrimSpace(record.Status),
		"reason":     strings.TrimSpace(record.Reason),
		"error_code": strings.TrimSpace(record.ErrorCode),
	}
	if record.DurationMS > 0 {
		detail["duration_ms"] = record.DurationMS
	}
	for key, value := range cloneDetailMap(record.Detail) {
		detail[key] = value
	}
	return detail
}

func buildActionEventName(record ActionLog) string {
	module := strings.TrimSpace(record.Module)
	action := strings.TrimSpace(record.Action)
	switch {
	case module != "" && action != "":
		return fmt.Sprintf("%s.%s", module, action)
	case action != "":
		return action
	default:
		return "action"
	}
}

func ensureActionTime(current time.Time) time.Time {
	if current.IsZero() {
		return time.Now()
	}
	return current
}

func cloneAny(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		return cloneDetailMap(typed)
	case map[string]string:
		return cloneStringMap(typed)
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]any, 0, len(typed))
		for _, item := range typed {
			out = append(out, cloneAny(item))
		}
		return out
	default:
		return typed
	}
}

func metricKey(name string, labels map[string]string) string {
	if len(labels) == 0 {
		return name
	}
	keys := make([]string, 0, len(labels))
	for key := range labels {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	var builder strings.Builder
	builder.WriteString(name)
	for _, key := range keys {
		builder.WriteString("|")
		builder.WriteString(key)
		builder.WriteString("=")
		builder.WriteString(labels[key])
	}
	return builder.String()
}

func normalizeLogLevel(level LogLevel) LogLevel {
	switch strings.ToLower(strings.TrimSpace(string(level))) {
	case string(LogLevelDebug):
		return LogLevelDebug
	case string(LogLevelWarn):
		return LogLevelWarn
	case string(LogLevelError):
		return LogLevelError
	default:
		return LogLevelInfo
	}
}

func normalizeLogLevelFromString(level string) LogLevel {
	return normalizeLogLevel(LogLevel(level))
}

func shouldLog(threshold LogLevel, level LogLevel) bool {
	return logLevelRank(level) >= logLevelRank(normalizeLogLevel(threshold))
}

func logLevelRank(level LogLevel) int {
	switch normalizeLogLevel(level) {
	case LogLevelDebug:
		return 10
	case LogLevelInfo:
		return 20
	case LogLevelWarn:
		return 30
	case LogLevelError:
		return 40
	default:
		return 20
	}
}
