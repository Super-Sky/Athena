// sessions.go implements the governance-facing session resource operations in the app layer.
// sessions.go 实现 app 层面向治理的 session 资源操作。
package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"moss/internal/session"
)

// SessionResource captures the minimal governance-facing session resource shape.
// SessionResource 描述最小治理视角下的 session 资源结构。
type SessionResource struct {
	ID           string    `json:"id"`
	Title        string    `json:"title,omitempty"`
	Status       string    `json:"status"`
	Archived     bool      `json:"archived"`
	PendingWait  bool      `json:"pending_wait"`
	LastActiveAt time.Time `json:"last_active_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// SessionListQuery captures the minimal supported list filters.
// SessionListQuery 描述列表接口支持的最小过滤参数。
type SessionListQuery struct {
	Archived *bool
	Status   string
	Limit    int
	Offset   int
}

// CreateSession creates and persists one new formal session resource.
// CreateSession 会创建并持久化一个新的正式 session 资源。
func (s *Service) CreateSession(ctx context.Context, title string) (SessionResource, error) {
	if s.SessionStore == nil {
		return SessionResource{}, fmt.Errorf("session store is not configured")
	}
	now := time.Now().UTC()
	item := &session.Session{
		ID:           session.NewID(),
		Title:        strings.TrimSpace(title),
		LastActiveAt: now,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	if err := s.SessionStore.Put(ctx, item); err != nil {
		return SessionResource{}, err
	}
	return sessionToResource(item), nil
}

// ListSessions returns the current session resource list.
// ListSessions 会返回当前 session 资源列表。
func (s *Service) ListSessions(ctx context.Context, query SessionListQuery) ([]SessionResource, error) {
	if s.SessionStore == nil {
		return nil, fmt.Errorf("session store is not configured")
	}
	items, err := s.SessionStore.List(ctx, session.ListFilter{
		Archived: query.Archived,
		Status:   strings.TrimSpace(query.Status),
		Limit:    query.Limit,
		Offset:   query.Offset,
	})
	if err != nil {
		return nil, err
	}
	result := make([]SessionResource, 0, len(items))
	for _, item := range items {
		result = append(result, sessionToResource(item))
	}
	return result, nil
}

// GetSession returns one session resource when it exists.
// GetSession 会在 session 存在时返回其资源视图。
func (s *Service) GetSession(ctx context.Context, id string) (SessionResource, bool, error) {
	if s.SessionStore == nil {
		return SessionResource{}, false, fmt.Errorf("session store is not configured")
	}
	item, ok := s.SessionStore.Get(ctx, strings.TrimSpace(id))
	if !ok {
		return SessionResource{}, false, nil
	}
	return sessionToResource(item), true, nil
}

// ArchiveSession marks one session resource as archived.
// ArchiveSession 会把一个 session 资源标记为已归档。
func (s *Service) ArchiveSession(ctx context.Context, id string) (SessionResource, error) {
	if s.SessionStore == nil {
		return SessionResource{}, fmt.Errorf("session store is not configured")
	}
	trimmedID := strings.TrimSpace(id)
	if _, ok := s.SessionStore.Get(ctx, trimmedID); !ok {
		return SessionResource{}, &InvalidSessionError{SessionID: trimmedID, Reason: "not_found"}
	}
	updated, err := s.SessionStore.Update(ctx, trimmedID, func(current *session.Session) error {
		current.Archived = true
		current.Pending = nil
		return nil
	})
	if err != nil {
		return SessionResource{}, err
	}
	return sessionToResource(updated), nil
}

// UpdateSessionTitle updates the minimal editable session metadata in v1.
// UpdateSessionTitle 会更新 v1 允许编辑的最小 session 元数据。
func (s *Service) UpdateSessionTitle(ctx context.Context, id string, title string) (SessionResource, error) {
	if s.SessionStore == nil {
		return SessionResource{}, fmt.Errorf("session store is not configured")
	}
	trimmedID := strings.TrimSpace(id)
	if _, ok := s.SessionStore.Get(ctx, trimmedID); !ok {
		return SessionResource{}, &InvalidSessionError{SessionID: trimmedID, Reason: "not_found"}
	}
	updated, err := s.SessionStore.Update(ctx, trimmedID, func(current *session.Session) error {
		current.Title = strings.TrimSpace(title)
		return nil
	})
	if err != nil {
		return SessionResource{}, err
	}
	return sessionToResource(updated), nil
}

func sessionToResource(item *session.Session) SessionResource {
	if item == nil {
		return SessionResource{}
	}
	return SessionResource{
		ID:           item.ID,
		Title:        item.Title,
		Status:       item.Status(),
		Archived:     item.Archived,
		PendingWait:  item.PendingWait(),
		LastActiveAt: item.LastActiveAt,
		CreatedAt:    item.CreatedAt,
		UpdatedAt:    item.UpdatedAt,
	}
}
