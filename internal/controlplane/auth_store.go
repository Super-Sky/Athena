package controlplane

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// AuthStateStore persists control-plane auth sessions and lock state.
// AuthStateStore 负责持久化控制面的认证会话和锁定状态。
type AuthStateStore interface {
	Load(context.Context) (authState, error)
	Save(context.Context, authState) error
}

type fileAuthStateStore struct {
	path string
	mu   sync.Mutex
}

type PostgresAuthSessionModel struct {
	SessionID string    `gorm:"column:session_id;type:text;primaryKey"`
	IP        string    `gorm:"column:ip;type:text;not null;index:idx_control_plane_auth_sessions_ip"`
	CreatedAt time.Time `gorm:"column:created_at;type:timestamptz;not null"`
	ExpiresAt time.Time `gorm:"column:expires_at;type:timestamptz;not null;index:idx_control_plane_auth_sessions_expires_at"`
	UpdatedAt time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (PostgresAuthSessionModel) TableName() string {
	return "control_plane_auth_sessions"
}

type PostgresAuthLockModel struct {
	IP             string     `gorm:"column:ip;type:text;primaryKey"`
	FailedAttempts int        `gorm:"column:failed_attempts;type:integer;not null;default:0"`
	Locked         bool       `gorm:"column:locked;type:boolean;not null;default:false;index:idx_control_plane_auth_locks_locked"`
	LastFailureAt  *time.Time `gorm:"column:last_failure_at;type:timestamptz"`
	LockedAt       *time.Time `gorm:"column:locked_at;type:timestamptz"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (PostgresAuthLockModel) TableName() string {
	return "control_plane_auth_locks"
}

type PostgresAuthStateStore struct {
	db *gorm.DB
}

func NewFileAuthStateStore(path string) AuthStateStore {
	return &fileAuthStateStore{path: strings.TrimSpace(path)}
}

func NewPostgresAuthStateStore(dsn string) (*PostgresAuthStateStore, error) {
	db, err := gorm.Open(postgres.Open(strings.TrimSpace(dsn)), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	store := &PostgresAuthStateStore{db: db}
	if err := store.AutoMigrate(context.Background()); err != nil {
		return nil, err
	}
	return store, nil
}

func (s *PostgresAuthStateStore) AutoMigrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.WithContext(ctx).AutoMigrate(&PostgresAuthSessionModel{}, &PostgresAuthLockModel{})
}

func (s *fileAuthStateStore) Load(_ context.Context) (authState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.path) == "" {
		return emptyAuthState(), nil
	}
	payload, err := os.ReadFile(filepath.Clean(s.path))
	if err != nil {
		if os.IsNotExist(err) {
			return emptyAuthState(), nil
		}
		return authState{}, err
	}
	if len(payload) == 0 {
		return emptyAuthState(), nil
	}
	var state authState
	if err := json.Unmarshal(payload, &state); err != nil {
		return authState{}, err
	}
	return normalizeAuthState(state), nil
}

func (s *fileAuthStateStore) Save(_ context.Context, state authState) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(s.path)), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(normalizeAuthState(state), "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(s.path), payload, 0o644)
}

func (s *PostgresAuthStateStore) Load(ctx context.Context) (authState, error) {
	if s == nil || s.db == nil {
		return emptyAuthState(), nil
	}
	state := emptyAuthState()
	var sessions []PostgresAuthSessionModel
	if err := s.db.WithContext(ctx).Find(&sessions).Error; err != nil {
		return authState{}, err
	}
	for _, item := range sessions {
		state.Sessions[item.SessionID] = authSession{
			SessionID: item.SessionID,
			IP:        item.IP,
			CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339),
			ExpiresAt: item.ExpiresAt.UTC().Format(time.RFC3339),
		}
	}
	var locks []PostgresAuthLockModel
	if err := s.db.WithContext(ctx).Find(&locks).Error; err != nil {
		return authState{}, err
	}
	for _, item := range locks {
		lock := authLock{
			IP:             item.IP,
			FailedAttempts: item.FailedAttempts,
			Locked:         item.Locked,
		}
		if item.LastFailureAt != nil {
			lock.LastFailureAt = item.LastFailureAt.UTC().Format(time.RFC3339)
		}
		if item.LockedAt != nil {
			lock.LockedAt = item.LockedAt.UTC().Format(time.RFC3339)
		}
		state.Locks[item.IP] = lock
	}
	return normalizeAuthState(state), nil
}

func (s *PostgresAuthStateStore) Save(ctx context.Context, state authState) error {
	if s == nil || s.db == nil {
		return nil
	}
	state = normalizeAuthState(state)
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&PostgresAuthSessionModel{}).Error; err != nil {
			return err
		}
		if err := tx.Session(&gorm.Session{AllowGlobalUpdate: true}).Delete(&PostgresAuthLockModel{}).Error; err != nil {
			return err
		}
		if len(state.Sessions) > 0 {
			sessions := make([]PostgresAuthSessionModel, 0, len(state.Sessions))
			for _, item := range state.Sessions {
				createdAt, _ := parseRFC3339(item.CreatedAt)
				expiresAt, _ := parseRFC3339(item.ExpiresAt)
				sessions = append(sessions, PostgresAuthSessionModel{
					SessionID: strings.TrimSpace(item.SessionID),
					IP:        strings.TrimSpace(item.IP),
					CreatedAt: createdAt,
					ExpiresAt: expiresAt,
				})
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&sessions).Error; err != nil {
				return err
			}
		}
		if len(state.Locks) > 0 {
			locks := make([]PostgresAuthLockModel, 0, len(state.Locks))
			for _, item := range state.Locks {
				lastFailureAt, _ := parseRFC3339Pointer(item.LastFailureAt)
				lockedAt, _ := parseRFC3339Pointer(item.LockedAt)
				locks = append(locks, PostgresAuthLockModel{
					IP:             strings.TrimSpace(item.IP),
					FailedAttempts: item.FailedAttempts,
					Locked:         item.Locked,
					LastFailureAt:  lastFailureAt,
					LockedAt:       lockedAt,
				})
			}
			if err := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&locks).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func parseRFC3339(value string) (time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Now().UTC(), err
	}
	return parsed.UTC(), nil
}

func parseRFC3339Pointer(value string) (*time.Time, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil, nil
	}
	parsed, err := parseRFC3339(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func emptyAuthState() authState {
	return authState{
		Sessions: map[string]authSession{},
		Locks:    map[string]authLock{},
	}
}

func normalizeAuthState(state authState) authState {
	if state.Sessions == nil {
		state.Sessions = map[string]authSession{}
	}
	if state.Locks == nil {
		state.Locks = map[string]authLock{}
	}
	return state
}
