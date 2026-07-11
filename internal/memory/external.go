// external.go implements app-owned scoped memory summaries without business-domain tables.
// external.go 实现应用拥有的作用域记忆摘要，不引入业务领域表。
package memory

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ExternalRecord stores one governed summary supplied by an owning application.
// ExternalRecord 保存由拥有应用提供的一条受治理摘要。
type ExternalRecord struct {
	ID            string            `json:"id"`
	AppID         string            `json:"app_id"`
	OwnerID       string            `json:"owner_id"`
	Scope         string            `json:"scope"`
	Kind          string            `json:"kind"`
	Summary       string            `json:"summary"`
	SchemaVersion string            `json:"schema_version"`
	Metadata      map[string]string `json:"metadata,omitempty"`
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// ExternalTrace records safe operation metadata without copying a business payload.
// ExternalTrace 记录安全操作元数据，不复制业务载荷。
type ExternalTrace struct {
	Operation string    `json:"operation"`
	AppID     string    `json:"app_id"`
	OwnerID   string    `json:"owner_id"`
	Scope     string    `json:"scope"`
	RecordIDs []string  `json:"record_ids"`
	CreatedAt time.Time `json:"created_at"`
}

// ExternalStore keeps generic scoped summaries and their safe operation trace.
// ExternalStore 保存通用作用域摘要及其安全操作 trace。
type ExternalStore struct {
	mu       sync.RWMutex
	records  map[string]ExternalRecord
	traces   []ExternalTrace
	now      func() time.Time
	sequence uint64
}

// NewExternalStore creates the default in-process enhancement store.
// NewExternalStore 创建默认进程内增强 store。
func NewExternalStore() *ExternalStore {
	return &ExternalStore{records: map[string]ExternalRecord{}, now: time.Now}
}

// Write validates ownership and stores one versioned summary.
// Write 校验归属并保存一条带版本的摘要。
func (s *ExternalStore) Write(ctx context.Context, item ExternalRecord) (ExternalRecord, ExternalTrace, error) {
	if err := ctx.Err(); err != nil {
		return ExternalRecord{}, ExternalTrace{}, err
	}
	item.AppID = strings.TrimSpace(item.AppID)
	item.OwnerID = strings.TrimSpace(item.OwnerID)
	item.Scope = strings.TrimSpace(item.Scope)
	item.Kind = strings.TrimSpace(item.Kind)
	item.Summary = strings.TrimSpace(item.Summary)
	if item.AppID == "" || item.OwnerID == "" || item.Scope == "" || item.Kind == "" || item.Summary == "" {
		return ExternalRecord{}, ExternalTrace{}, fmt.Errorf("app_id, owner_id, scope, kind, and summary are required")
	}
	if item.SchemaVersion == "" {
		item.SchemaVersion = "external_memory.v1"
	}
	item.Metadata = cloneStringMap(item.Metadata)
	now := s.now().UTC()
	if item.ID == "" {
		item.ID = fmt.Sprintf("mem_%d_%d", now.UnixNano(), atomic.AddUint64(&s.sequence, 1))
		item.CreatedAt = now
	}
	item.UpdatedAt = now
	s.mu.Lock()
	defer s.mu.Unlock()
	s.records[item.ID] = item
	trace := ExternalTrace{Operation: "memory_write", AppID: item.AppID, OwnerID: item.OwnerID, Scope: item.Scope, RecordIDs: []string{item.ID}, CreatedAt: now}
	s.traces = append(s.traces, trace)
	return cloneExternalRecord(item), trace, nil
}

// Query returns only records matching the caller's explicit ownership scope.
// Query 仅返回匹配调用方显式归属作用域的记录。
func (s *ExternalStore) Query(ctx context.Context, appID, ownerID, scope string, limit int) ([]ExternalRecord, ExternalTrace, error) {
	if err := ctx.Err(); err != nil {
		return nil, ExternalTrace{}, err
	}
	appID = strings.TrimSpace(appID)
	ownerID = strings.TrimSpace(ownerID)
	scope = strings.TrimSpace(scope)
	if appID == "" || ownerID == "" || scope == "" {
		return nil, ExternalTrace{}, fmt.Errorf("app_id, owner_id, and scope are required")
	}
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]ExternalRecord, 0, limit)
	for _, v := range s.records {
		if v.AppID == appID && v.OwnerID == ownerID && v.Scope == scope {
			out = append(out, cloneExternalRecord(v))
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].UpdatedAt.After(out[j].UpdatedAt) })
	if len(out) > limit {
		out = out[:limit]
	}
	ids := make([]string, 0, len(out))
	for _, v := range out {
		ids = append(ids, v.ID)
	}
	trace := ExternalTrace{Operation: "memory_query", AppID: appID, OwnerID: ownerID, Scope: scope, RecordIDs: ids, CreatedAt: s.now().UTC()}
	s.traces = append(s.traces, trace)
	return out, trace, nil
}

func cloneExternalRecord(item ExternalRecord) ExternalRecord {
	item.Metadata = cloneStringMap(item.Metadata)
	return item
}

func cloneStringMap(input map[string]string) map[string]string {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]string, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}
