// session.go defines the session domain model, pending state, and session store interfaces.
// session.go 定义 session 领域模型、pending state 与 session store 接口。
package session

import (
	"context"
	"fmt"
	"moss/internal/contextassets"
	"moss/internal/observability"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// DefaultDeferredQueueLimit bounds how many ordinary follow-up messages one waiting session can hold.
	// DefaultDeferredQueueLimit 用于限制一个 waiting session 最多暂存多少条普通后续消息。
	DefaultDeferredQueueLimit = 32

	// DefaultClosedResumeTokenTTL bounds how long closed-token tombstones stay queryable.
	// DefaultClosedResumeTokenTTL 用于限制已关闭 token tombstone 的默认保留时长。
	DefaultClosedResumeTokenTTL = 24 * time.Hour
)

// Message stores a consumed dialogue item in the main session history.
// Message 表示已经进入主链消费的会话消息。
type Message struct {
	Role    string
	Content string
}

// PreservedContext captures the minimum continuity state needed to safely resume one waiting gap.
// PreservedContext 描述安全恢复一个 waiting gap 所需的最小 continuity 上下文。
type PreservedContext struct {
	Goal               string
	LastUserIntent     string
	MissingFields      []string
	Facts              map[string]string
	WaitStage          string
	ResumeToken        string
	TimeoutAt          time.Time
	TimeoutAfter       time.Duration
	DegradeReason      string
	CloseReason        string
	PendingHumanReason string
}

// PendingState captures the single active waiting gap for a session.
// PendingState 描述一个 session 当前唯一活跃的等待缺口。
type PendingState struct {
	Stage         string
	Status        string
	ActionType    string
	ResumeToken   string
	ModelID       string
	TimeoutAt     time.Time
	TimeoutAfter  time.Duration
	MissingFields []string
	Preserved     *PreservedContext
}

// DeferredMessage stores a normal follow-up input that arrived during waiting and must be consumed later.
// DeferredMessage 保存 waiting 期间到达、但要等后续再消费的普通消息。
type DeferredMessage struct {
	Query                  string
	ModelID                string
	PromptTemplate         string
	EnabledSkills          []string
	EnabledTools           []string
	ContextAssetOverrides  []contextassets.Asset
	DisabledAssetTypes     []string
	AssetPriorityOverrides map[string]int
	ContextAssets          []contextassets.Asset
	ContextBindings        []contextassets.ResolvedAsset
	CompiledRefs           []contextassets.Ref
	DisableFastPath        bool
	ReceivedAt             time.Time
}

// ClosedResumeToken keeps a short-lived tombstone for a gap that has already been closed.
// ClosedResumeToken 为已关闭 gap 的 token 保留一份 tombstone 记录。
type ClosedResumeToken struct {
	Token    string
	Reason   string
	ClosedAt time.Time
}

// Session is the server-side source of truth for history, waiting state, deferred queue, and closed tokens.
// Session 是服务端关于主 history、waiting 状态、排队消息和已关闭 token 的统一真相。
type Session struct {
	ID                   string
	Title                string
	Archived             bool
	LastActiveAt         time.Time
	CreatedAt            time.Time
	UpdatedAt            time.Time
	Messages             []Message
	ContextAssets        []contextassets.Asset
	ContextAssetBindings []contextassets.ResolvedAsset
	CompiledAssetRefs    []contextassets.Ref
	Pending              *PendingState
	DeferredQueue        []DeferredMessage
	ClosedTokens         []ClosedResumeToken
}

// ListFilter captures the minimal server-side filters for session resource listing.
// ListFilter 描述 session 资源列表查询所需的最小过滤条件。
type ListFilter struct {
	Archived *bool
	Status   string
	Limit    int
	Offset   int
}

// Store abstracts session persistence away from HTTP and runtime details.
// Store 将 session 持久化与 HTTP/runtime 细节解耦。
type Store interface {
	Get(context.Context, string) (*Session, bool)
	Put(context.Context, *Session) error
	Update(context.Context, string, func(*Session) error) (*Session, error)
	List(context.Context, ListFilter) ([]*Session, error)
}

// MemoryStore is the in-process reference implementation used by the scaffold.
// MemoryStore 是当前脚手架使用的进程内参考实现。
type MemoryStore struct {
	mu                 sync.RWMutex
	sessions           map[string]*Session
	deferredQueueLimit int
	closedTokenTTL     time.Duration
}

// NewMemoryStore constructs the default in-memory session store.
// NewMemoryStore 创建默认的内存版 session store。
func NewMemoryStore() *MemoryStore {
	return NewMemoryStoreWithOptions(DefaultDeferredQueueLimit, DefaultClosedResumeTokenTTL)
}

// NewMemoryStoreWithOptions constructs an in-memory store with configurable queue and tombstone limits.
// NewMemoryStoreWithOptions 创建一个可配置 queue 上限和 tombstone 保留时长的内存版 store。
func NewMemoryStoreWithOptions(queueLimit int, closedTokenTTL time.Duration) *MemoryStore {
	if queueLimit <= 0 {
		queueLimit = DefaultDeferredQueueLimit
	}
	if closedTokenTTL <= 0 {
		closedTokenTTL = DefaultClosedResumeTokenTTL
	}
	return &MemoryStore{
		sessions:           make(map[string]*Session),
		deferredQueueLimit: queueLimit,
		closedTokenTTL:     closedTokenTTL,
	}
}

// Normalize enforces the scaffold defaults for queue length and closed-token retention.
// Normalize 会按脚手架默认规则收紧 queue 长度和 closed token 保留时长。
func (s *Session) Normalize(now time.Time, queueLimit int, closedTokenTTL time.Duration) {
	if s == nil {
		return
	}
	if s.CreatedAt.IsZero() {
		s.CreatedAt = now
	}
	if s.UpdatedAt.IsZero() {
		s.UpdatedAt = now
	}
	if s.LastActiveAt.IsZero() {
		s.LastActiveAt = now
	}
	if queueLimit > 0 && len(s.DeferredQueue) > queueLimit {
		s.DeferredQueue = append([]DeferredMessage(nil), s.DeferredQueue[len(s.DeferredQueue)-queueLimit:]...)
	}
	if closedTokenTTL <= 0 || len(s.ClosedTokens) == 0 {
		return
	}

	filtered := s.ClosedTokens[:0]
	for _, token := range s.ClosedTokens {
		if token.ClosedAt.IsZero() || now.Sub(token.ClosedAt) < closedTokenTTL {
			filtered = append(filtered, token)
		}
	}
	s.ClosedTokens = append([]ClosedResumeToken(nil), filtered...)
}

// NewID returns a distributed-safe session resource identifier.
// NewID 会返回一个适合分布式部署的 session 资源标识。
func NewID() string {
	id, err := uuid.NewV7()
	if err != nil {
		return fmt.Sprintf("sess_%s", uuid.NewString())
	}
	return "sess_" + id.String()
}

// Status derives the current session resource status from archived and waiting state.
// Status 会从 archived 与 waiting 状态推导当前 session 资源状态。
func (s *Session) Status() string {
	if s == nil {
		return "active"
	}
	if s.Archived {
		return "archived"
	}
	if s.Pending != nil {
		return "pending_wait"
	}
	return "active"
}

// PendingWait reports whether the session currently exposes an active waiting gap.
// PendingWait 用于判断该 session 当前是否存在活跃等待缺口。
func (s *Session) PendingWait() bool {
	return s != nil && !s.Archived && s.Pending != nil
}

// Clone creates a defensive session copy so callers can safely mutate snapshots.
// Clone 会创建一份防御性拷贝，方便调用方安全修改 session 快照。
func (s *Session) Clone() *Session {
	if s == nil {
		return nil
	}
	copied := *s
	copied.Messages = append([]Message(nil), s.Messages...)
	copied.ContextAssets = cloneContextAssets(s.ContextAssets)
	copied.ContextAssetBindings = cloneResolvedAssets(s.ContextAssetBindings)
	copied.CompiledAssetRefs = cloneAssetRefs(s.CompiledAssetRefs)
	if s.Pending != nil {
		pending := *s.Pending
		pending.MissingFields = append([]string(nil), s.Pending.MissingFields...)
		pending.Preserved = clonePreservedContext(s.Pending.Preserved)
		copied.Pending = &pending
	}
	if len(s.DeferredQueue) > 0 {
		copied.DeferredQueue = make([]DeferredMessage, 0, len(s.DeferredQueue))
		for _, item := range s.DeferredQueue {
			copied.DeferredQueue = append(copied.DeferredQueue, DeferredMessage{
				Query:                  item.Query,
				ModelID:                item.ModelID,
				PromptTemplate:         item.PromptTemplate,
				EnabledSkills:          append([]string(nil), item.EnabledSkills...),
				EnabledTools:           append([]string(nil), item.EnabledTools...),
				ContextAssetOverrides:  cloneContextAssets(item.ContextAssetOverrides),
				DisabledAssetTypes:     append([]string(nil), item.DisabledAssetTypes...),
				AssetPriorityOverrides: cloneIntMap(item.AssetPriorityOverrides),
				ContextAssets:          cloneContextAssets(item.ContextAssets),
				ContextBindings:        cloneResolvedAssets(item.ContextBindings),
				CompiledRefs:           cloneAssetRefs(item.CompiledRefs),
				DisableFastPath:        item.DisableFastPath,
				ReceivedAt:             item.ReceivedAt,
			})
		}
	}
	if len(s.ClosedTokens) > 0 {
		copied.ClosedTokens = append([]ClosedResumeToken(nil), s.ClosedTokens...)
	}
	return &copied
}

func clonePreservedContext(ctx *PreservedContext) *PreservedContext {
	if ctx == nil {
		return nil
	}
	cloned := *ctx
	cloned.MissingFields = append([]string(nil), ctx.MissingFields...)
	if len(ctx.Facts) > 0 {
		cloned.Facts = make(map[string]string, len(ctx.Facts))
		for key, value := range ctx.Facts {
			cloned.Facts[key] = value
		}
	}
	return &cloned
}

func cloneIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]int, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func cloneContextAssets(items []contextassets.Asset) []contextassets.Asset {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]contextassets.Asset, 0, len(items))
	for _, item := range items {
		next := item
		next.Content = cloneStringAnyMap(item.Content)
		if item.Ref != nil {
			ref := *item.Ref
			next.Ref = &ref
		}
		next.Metadata.Tags = append([]string(nil), item.Metadata.Tags...)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneResolvedAssets(items []contextassets.ResolvedAsset) []contextassets.ResolvedAsset {
	if len(items) == 0 {
		return nil
	}
	cloned := make([]contextassets.ResolvedAsset, 0, len(items))
	for _, item := range items {
		next := item
		next.Asset.Content = cloneStringAnyMap(item.Asset.Content)
		if item.Asset.Ref != nil {
			ref := *item.Asset.Ref
			next.Asset.Ref = &ref
		}
		next.Asset.Metadata.Tags = append([]string(nil), item.Asset.Metadata.Tags...)
		next.Payload = cloneStringAnyMap(item.Payload)
		cloned = append(cloned, next)
	}
	return cloned
}

func cloneAssetRefs(items []contextassets.Ref) []contextassets.Ref {
	if len(items) == 0 {
		return nil
	}
	return append([]contextassets.Ref(nil), items...)
}

func cloneStringAnyMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = value
	}
	return cloned
}

// Get returns a defensive copy so callers cannot mutate shared session state by accident.
// Get 返回防御性拷贝，避免调用方意外修改共享 session 状态。
func (s *MemoryStore) Get(_ context.Context, id string) (*Session, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		observability.LogAction(observability.LogLevelDebug, observability.ActionLog{
			Module: "session",
			Action: "memory_store_get",
			Step:   "completed",
			Status: "miss",
			Reason: "session_not_found",
			Detail: map[string]any{"session_id": id},
		})
		return nil, false
	}

	copied := session.Clone()
	copied.Normalize(time.Now(), s.deferredQueueLimit, s.closedTokenTTL)
	observability.LogAction(observability.LogLevelDebug, observability.ActionLog{
		Module: "session",
		Action: "memory_store_get",
		Step:   "completed",
		Status: "hit",
		Reason: "session_loaded",
		Detail: map[string]any{"session_id": id},
	})
	return copied, true
}

// Put persists a defensive copy so deferred queues and tombstones remain internally owned.
// Put 存储防御性拷贝，确保 deferred queue 和 tombstone 仍由 store 内部持有。
func (s *MemoryStore) Put(_ context.Context, session *Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	copied := session.Clone()
	now := time.Now()
	copied.UpdatedAt = now
	copied.Normalize(now, s.deferredQueueLimit, s.closedTokenTTL)
	s.sessions[session.ID] = copied
	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "session",
		Action: "memory_store_put",
		Step:   "completed",
		Status: "ok",
		Reason: "session_stored",
		Detail: map[string]any{
			"session_id":           session.ID,
			"message_count":        len(copied.Messages),
			"deferred_queue_count": len(copied.DeferredQueue),
			"has_pending":          copied.Pending != nil,
		},
	})
	return nil
}

// Update atomically mutates one session aggregate and returns the normalized persisted snapshot.
// Update 会以原子方式修改一个 session 聚合，并返回规范化后的持久化快照。
func (s *MemoryStore) Update(_ context.Context, id string, mutator func(*Session) error) (*Session, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	current, ok := s.sessions[id]
	if !ok || current == nil {
		current = &Session{ID: id}
	}
	next := current.Clone()
	if err := mutator(next); err != nil {
		observability.LogAction(observability.LogLevelError, observability.ActionLog{
			Module:    "session",
			Action:    "memory_store_update",
			Step:      "mutate",
			Status:    "error",
			Reason:    "mutator_failed",
			ErrorCode: "session_mutator_failed",
			Detail: map[string]any{
				"session_id": id,
				"error":      err.Error(),
			},
		})
		return nil, err
	}
	now := time.Now()
	next.UpdatedAt = now
	next.Normalize(now, s.deferredQueueLimit, s.closedTokenTTL)
	s.sessions[id] = next
	observability.LogAction(observability.LogLevelInfo, observability.ActionLog{
		Module: "session",
		Action: "memory_store_update",
		Step:   "completed",
		Status: "ok",
		Reason: "session_updated",
		Detail: map[string]any{
			"session_id":           id,
			"message_count":        len(next.Messages),
			"deferred_queue_count": len(next.DeferredQueue),
			"has_pending":          next.Pending != nil,
		},
	})
	return next.Clone(), nil
}

// List returns normalized session snapshots ordered by last_active_at desc then created_at desc.
// List 会返回按 last_active_at 和 created_at 倒序排列的 session 快照列表。
func (s *MemoryStore) List(_ context.Context, filter ListFilter) ([]*Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Session, 0, len(s.sessions))
	now := time.Now()
	for _, stored := range s.sessions {
		item := stored.Clone()
		item.Normalize(now, s.deferredQueueLimit, s.closedTokenTTL)
		if filter.Archived != nil && item.Archived != *filter.Archived {
			continue
		}
		if filter.Status != "" && item.Status() != filter.Status {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].LastActiveAt.Equal(result[j].LastActiveAt) {
			return result[i].CreatedAt.After(result[j].CreatedAt)
		}
		return result[i].LastActiveAt.After(result[j].LastActiveAt)
	})
	start := filter.Offset
	if start < 0 {
		start = 0
	}
	if start > len(result) {
		start = len(result)
	}
	end := len(result)
	if filter.Limit > 0 && start+filter.Limit < end {
		end = start + filter.Limit
	}
	listed := append([]*Session(nil), result[start:end]...)
	observability.LogAction(observability.LogLevelDebug, observability.ActionLog{
		Module: "session",
		Action: "memory_store_list",
		Step:   "completed",
		Status: "ok",
		Reason: "sessions_listed",
		Detail: map[string]any{
			"count":      len(listed),
			"status":     filter.Status,
			"limit":      filter.Limit,
			"offset":     filter.Offset,
			"has_filter": filter.Archived != nil,
		},
	})
	return listed, nil
}
