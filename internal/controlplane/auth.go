// auth.go implements the minimal control-plane login, session, and IP lock state.
// auth.go 负责实现控制面的最小登录、会话和 IP 锁定状态。
package controlplane

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
)

const ControlPlaneSessionCookie = "athena_control_plane_session"

var (
	// ErrControlPlaneAuthRequired means the current request is not authenticated.
	// ErrControlPlaneAuthRequired 表示当前请求未通过控制面认证。
	ErrControlPlaneAuthRequired = errors.New("control plane authentication is required")
	// ErrControlPlaneAuthLocked means the client IP is locked after repeated failures.
	// ErrControlPlaneAuthLocked 表示客户端 IP 因重复失败已被锁定。
	ErrControlPlaneAuthLocked = errors.New("control plane login is locked for this ip")
	// ErrControlPlaneAuthInvalidToken means the submitted login token is invalid.
	// ErrControlPlaneAuthInvalidToken 表示登录 token 无效。
	ErrControlPlaneAuthInvalidToken = errors.New("control plane token is invalid")
)

type authState struct {
	Sessions map[string]authSession `json:"sessions,omitempty"`
	Locks    map[string]authLock    `json:"locks,omitempty"`
}

type authSession struct {
	SessionID string `json:"session_id,omitempty"`
	IP        string `json:"ip,omitempty"`
	CreatedAt string `json:"created_at,omitempty"`
	ExpiresAt string `json:"expires_at,omitempty"`
}

type authLock struct {
	IP             string `json:"ip,omitempty"`
	FailedAttempts int    `json:"failed_attempts,omitempty"`
	Locked         bool   `json:"locked,omitempty"`
	LastFailureAt  string `json:"last_failure_at,omitempty"`
	LockedAt       string `json:"locked_at,omitempty"`
}

type authConfig struct {
	Token             string
	SessionTTL        time.Duration
	MaxFailedAttempts int
}

// SetAuthConfig installs the minimal control-plane auth policy.
// SetAuthConfig 安装控制面的最小认证策略。
func (m *Manager) SetAuthConfig(token string, sessionTTL time.Duration, maxFailedAttempts int) {
	if m == nil {
		return
	}
	if sessionTTL <= 0 {
		sessionTTL = 8 * time.Hour
	}
	if maxFailedAttempts <= 0 {
		maxFailedAttempts = 5
	}
	m.auth = authConfig{
		Token:             strings.TrimSpace(token),
		SessionTTL:        sessionTTL,
		MaxFailedAttempts: maxFailedAttempts,
	}
}

// Login validates the configured control-plane token and opens one cookie-backed session.
// Login 校验控制面 token 并创建一条基于 cookie 的会话。
func (m *Manager) Login(ctx context.Context, token, remoteIP string) (AuthStatus, string, error) {
	if m == nil || strings.TrimSpace(m.auth.Token) == "" {
		return m.authStatusWithState(authState{}, "", strings.TrimSpace(remoteIP), true), "", nil
	}

	state, err := m.loadAuthState(ctx)
	if err != nil {
		return AuthStatus{}, "", err
	}
	state = pruneExpiredAuthState(state, time.Now().UTC())

	ip := strings.TrimSpace(remoteIP)
	if locked := state.Locks[ip]; locked.Locked {
		if err := m.saveAuthState(ctx, state); err != nil {
			return AuthStatus{}, "", err
		}
		return m.authStatusWithState(state, "", ip, false), "", ErrControlPlaneAuthLocked
	}
	if strings.TrimSpace(token) != strings.TrimSpace(m.auth.Token) {
		entry := state.Locks[ip]
		entry.IP = ip
		entry.FailedAttempts++
		entry.LastFailureAt = time.Now().UTC().Format(time.RFC3339)
		if entry.FailedAttempts >= m.auth.MaxFailedAttempts {
			entry.Locked = true
			entry.LockedAt = entry.LastFailureAt
		}
		state.Locks[ip] = entry
		if err := m.saveAuthState(ctx, state); err != nil {
			return AuthStatus{}, "", err
		}
		if entry.Locked {
			return m.authStatusWithState(state, "", ip, false), "", ErrControlPlaneAuthLocked
		}
		return m.authStatusWithState(state, "", ip, false), "", ErrControlPlaneAuthInvalidToken
	}

	delete(state.Locks, ip)
	sessionID := "cpauth_" + uuid.NewString()
	now := time.Now().UTC()
	state.Sessions[sessionID] = authSession{
		SessionID: sessionID,
		IP:        ip,
		CreatedAt: now.Format(time.RFC3339),
		ExpiresAt: now.Add(m.auth.SessionTTL).Format(time.RFC3339),
	}
	if err := m.saveAuthState(ctx, state); err != nil {
		return AuthStatus{}, "", err
	}
	return m.authStatusWithState(state, sessionID, ip, true), sessionID, nil
}

// Logout closes one control-plane auth session.
// Logout 关闭一条控制面认证会话。
func (m *Manager) Logout(ctx context.Context, sessionID, remoteIP string) (AuthStatus, error) {
	if m == nil || strings.TrimSpace(m.auth.Token) == "" {
		return m.authStatusWithState(authState{}, "", strings.TrimSpace(remoteIP), true), nil
	}
	state, err := m.loadAuthState(ctx)
	if err != nil {
		return AuthStatus{}, err
	}
	state = pruneExpiredAuthState(state, time.Now().UTC())
	delete(state.Sessions, strings.TrimSpace(sessionID))
	if err := m.saveAuthState(ctx, state); err != nil {
		return AuthStatus{}, err
	}
	return m.authStatusWithState(state, "", strings.TrimSpace(remoteIP), false), nil
}

// AuthStatus returns the current session and lock state for one client IP.
// AuthStatus 返回某个客户端 IP 的当前会话与锁定状态。
func (m *Manager) AuthStatus(ctx context.Context, sessionID, remoteIP string) (AuthStatus, error) {
	authenticated, status, err := m.authorize(ctx, sessionID, remoteIP)
	if err != nil && !errors.Is(err, ErrControlPlaneAuthRequired) && !errors.Is(err, ErrControlPlaneAuthLocked) {
		return AuthStatus{}, err
	}
	if authenticated {
		return status, nil
	}
	return status, err
}

// Authorize verifies one control-plane auth cookie against the configured policy.
// Authorize 会按当前策略校验控制面认证 cookie。
func (m *Manager) Authorize(ctx context.Context, sessionID, remoteIP string) (AuthStatus, error) {
	_, status, err := m.authorize(ctx, sessionID, remoteIP)
	return status, err
}

func (m *Manager) authorize(ctx context.Context, sessionID, remoteIP string) (bool, AuthStatus, error) {
	if m == nil || strings.TrimSpace(m.auth.Token) == "" {
		return true, m.authStatusWithState(authState{}, "", strings.TrimSpace(remoteIP), true), nil
	}
	state, err := m.loadAuthState(ctx)
	if err != nil {
		return false, AuthStatus{}, err
	}
	state = pruneExpiredAuthState(state, time.Now().UTC())
	if err := m.saveAuthState(ctx, state); err != nil {
		return false, AuthStatus{}, err
	}

	ip := strings.TrimSpace(remoteIP)
	if locked := state.Locks[ip]; locked.Locked {
		return false, m.authStatusWithState(state, "", ip, false), ErrControlPlaneAuthLocked
	}
	entry, ok := state.Sessions[strings.TrimSpace(sessionID)]
	if !ok {
		return false, m.authStatusWithState(state, "", ip, false), ErrControlPlaneAuthRequired
	}
	if entry.IP != "" && ip != "" && entry.IP != ip {
		delete(state.Sessions, strings.TrimSpace(sessionID))
		if err := m.saveAuthState(ctx, state); err != nil {
			return false, AuthStatus{}, err
		}
		return false, m.authStatusWithState(state, "", ip, false), ErrControlPlaneAuthRequired
	}
	return true, m.authStatusWithState(state, entry.SessionID, ip, true), nil
}

func (m *Manager) authStatusWithState(state authState, sessionID, remoteIP string, authenticated bool) AuthStatus {
	if state.Sessions == nil {
		state.Sessions = map[string]authSession{}
	}
	if state.Locks == nil {
		state.Locks = map[string]authLock{}
	}
	lockState := "active"
	remainingAttempts := m.auth.MaxFailedAttempts
	failedAttempts := 0
	ip := strings.TrimSpace(remoteIP)
	if strings.TrimSpace(m.auth.Token) == "" {
		lockState = "disabled"
		remainingAttempts = 0
	}
	if entry, ok := state.Locks[ip]; ok {
		failedAttempts = entry.FailedAttempts
		if entry.Locked {
			lockState = "locked"
			remainingAttempts = 0
		} else if strings.TrimSpace(m.auth.Token) != "" {
			remainingAttempts = maxInt(m.auth.MaxFailedAttempts-entry.FailedAttempts, 0)
		}
	}
	status := AuthStatus{
		Authenticated:     authenticated,
		LockState:         lockState,
		RemainingAttempts: remainingAttempts,
		FailedAttempts:    failedAttempts,
		TruthDir:          m.truthDirInfo(),
	}
	if entry, ok := state.Sessions[strings.TrimSpace(sessionID)]; ok {
		status.SessionExpiresAt = entry.ExpiresAt
	}
	return status
}

func (m *Manager) truthDirInfo() TruthDirInfo {
	info := TruthDirInfo{
		Path: strings.TrimSpace(m.truthDir),
	}
	manifest, err := m.loadSystemManifest()
	if err == nil {
		info.Version = strings.TrimSpace(manifest.TruthDirVersion)
	}
	return info
}

func (m *Manager) loadAuthState(ctx context.Context) (authState, error) {
	return m.authStateStore().Load(ctx)
}

func (m *Manager) saveAuthState(ctx context.Context, state authState) error {
	return m.authStateStore().Save(ctx, state)
}

func (m *Manager) authStatePath() string {
	if m == nil {
		return ""
	}
	if store, ok := m.store.(*FileStore); ok && strings.TrimSpace(store.path) != "" {
		return filepath.Join(filepath.Dir(filepath.Clean(store.path)), "auth_state.json")
	}
	if strings.TrimSpace(m.truthDir) != "" {
		return filepath.Join(filepath.Clean(m.truthDir), ".controlplane_auth.json")
	}
	return ""
}

func (m *Manager) authStateStore() AuthStateStore {
	if m == nil {
		return NewFileAuthStateStore("")
	}
	if m.authStore != nil {
		return m.authStore
	}
	m.authStore = NewFileAuthStateStore(m.authStatePath())
	return m.authStore
}

func pruneExpiredAuthState(state authState, now time.Time) authState {
	state = normalizeAuthState(state)
	for key, item := range state.Sessions {
		if strings.TrimSpace(item.ExpiresAt) == "" {
			continue
		}
		expiresAt, err := time.Parse(time.RFC3339, item.ExpiresAt)
		if err != nil || !expiresAt.After(now) {
			delete(state.Sessions, key)
		}
	}
	return state
}

func maxInt(left, right int) int {
	if left > right {
		return left
	}
	return right
}
