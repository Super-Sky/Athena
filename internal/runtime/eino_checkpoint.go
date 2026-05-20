// eino_checkpoint.go maps Athena wait/resume state to Eino checkpoint identifiers.
// eino_checkpoint.go 将 Athena 等待/恢复状态映射为 Eino checkpoint 标识。
package runtime

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ErrRuntimeCheckpointRejected marks a checkpoint payload rejected before persistence.
// ErrRuntimeCheckpointRejected 表示 checkpoint payload 在持久化前被拒绝。
var ErrRuntimeCheckpointRejected = errors.New("runtime checkpoint rejected")

// RuntimeGraphCheckpointRef is Athena's stable reference for one Eino checkpoint boundary.
// RuntimeGraphCheckpointRef 表示 Athena 对一个 Eino checkpoint 边界的稳定引用。
type RuntimeGraphCheckpointRef struct {
	CheckpointID string
	RequestID    string
	SessionID    string
	RunID        string
	Stage        RuntimeStage
	ResumeToken  string
}

// RuntimeGraphCheckpointByteStore is the private Eino checkpoint byte-store boundary.
// RuntimeGraphCheckpointByteStore 是 runtime 私有的 Eino checkpoint 字节存储边界。
type RuntimeGraphCheckpointByteStore interface {
	Get(context.Context, string) ([]byte, bool, error)
	Set(context.Context, string, []byte) error
}

// RuntimeGraphCheckpointSnapshot exposes safe checkpoint metadata without the opaque payload.
// RuntimeGraphCheckpointSnapshot 暴露不含 opaque payload 的安全 checkpoint 元数据。
type RuntimeGraphCheckpointSnapshot struct {
	CheckpointID  string
	PayloadSize   int
	PayloadSHA256 string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// MemoryRuntimeGraphCheckpointStore keeps Eino checkpoint bytes in process memory.
// MemoryRuntimeGraphCheckpointStore 在进程内内存保存 Eino checkpoint 字节。
type MemoryRuntimeGraphCheckpointStore struct {
	mu        sync.RWMutex
	payloads  map[string][]byte
	snapshots map[string]RuntimeGraphCheckpointSnapshot
	now       func() time.Time
}

// NewMemoryRuntimeGraphCheckpointStore creates an in-process Eino checkpoint store.
// NewMemoryRuntimeGraphCheckpointStore 创建进程内 Eino checkpoint store。
func NewMemoryRuntimeGraphCheckpointStore() *MemoryRuntimeGraphCheckpointStore {
	return &MemoryRuntimeGraphCheckpointStore{
		payloads:  map[string][]byte{},
		snapshots: map[string]RuntimeGraphCheckpointSnapshot{},
	}
}

// Get loads one checkpoint payload by ID.
// Get 按 ID 读取一个 checkpoint payload。
func (s *MemoryRuntimeGraphCheckpointStore) Get(_ context.Context, checkpointID string) ([]byte, bool, error) {
	if s == nil {
		return nil, false, fmt.Errorf("runtime checkpoint store is not configured")
	}
	key := strings.TrimSpace(checkpointID)
	if key == "" {
		return nil, false, fmt.Errorf("%w: checkpoint id is required", ErrRuntimeCheckpointRejected)
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	payload, ok := s.payloads[key]
	if !ok {
		return nil, false, nil
	}
	return append([]byte(nil), payload...), true, nil
}

// Set stores one checkpoint payload after lightweight credential-pattern rejection.
// Set 在轻量凭据模式拒绝检查后保存一个 checkpoint payload。
func (s *MemoryRuntimeGraphCheckpointStore) Set(_ context.Context, checkpointID string, payload []byte) error {
	if s == nil {
		return fmt.Errorf("runtime checkpoint store is not configured")
	}
	snapshot, err := runtimeGraphCheckpointSnapshot(checkpointID, payload, s.currentTime())
	if err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if existing, ok := s.snapshots[snapshot.CheckpointID]; ok {
		snapshot.CreatedAt = existing.CreatedAt
	}
	s.payloads[snapshot.CheckpointID] = append([]byte(nil), payload...)
	s.snapshots[snapshot.CheckpointID] = snapshot
	return nil
}

// Snapshot returns safe metadata for one checkpoint if it exists.
// Snapshot 返回一个 checkpoint 的安全元数据。
func (s *MemoryRuntimeGraphCheckpointStore) Snapshot(checkpointID string) (RuntimeGraphCheckpointSnapshot, bool) {
	if s == nil {
		return RuntimeGraphCheckpointSnapshot{}, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	snapshot, ok := s.snapshots[strings.TrimSpace(checkpointID)]
	return snapshot, ok
}

func (s *MemoryRuntimeGraphCheckpointStore) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

// RuntimeGraphCheckpointRefForTurn builds the deterministic checkpoint reference for turn execution.
// RuntimeGraphCheckpointRefForTurn 为 turn execution 构造确定性的 checkpoint 引用。
func RuntimeGraphCheckpointRefForTurn(state RuntimeState) RuntimeGraphCheckpointRef {
	return runtimeGraphCheckpointRef(state, "", StageTurnExecution, "")
}

// RuntimeGraphCheckpointRefFromWait builds the deterministic checkpoint reference for one wait state.
// RuntimeGraphCheckpointRefFromWait 为一个等待态构造确定性的 checkpoint 引用。
func RuntimeGraphCheckpointRefFromWait(state RuntimeState, recordSet *MinimalPersistenceRecordSet, wait *WaitState) RuntimeGraphCheckpointRef {
	runID := ""
	if recordSet != nil {
		runID = strings.TrimSpace(recordSet.Run.ID)
	}
	stage := RuntimeStage("")
	resumeToken := ""
	if wait != nil {
		stage = wait.Stage
		resumeToken = strings.TrimSpace(wait.ResumeToken)
	}
	return runtimeGraphCheckpointRef(state, runID, stage, resumeToken)
}

func runtimeGraphCheckpointRef(state RuntimeState, runID string, stage RuntimeStage, resumeToken string) RuntimeGraphCheckpointRef {
	parts := []string{
		"athena_runtime_graph",
		defaultString(strings.TrimSpace(state.SessionID), "session"),
		defaultString(strings.TrimSpace(state.RequestID), "request"),
		defaultString(strings.TrimSpace(runID), "run"),
		defaultString(string(stage), "stage"),
		defaultString(strings.TrimSpace(resumeToken), "resume"),
	}
	return RuntimeGraphCheckpointRef{
		CheckpointID: strings.Join(parts, ":"),
		RequestID:    strings.TrimSpace(state.RequestID),
		SessionID:    strings.TrimSpace(state.SessionID),
		RunID:        strings.TrimSpace(runID),
		Stage:        stage,
		ResumeToken:  strings.TrimSpace(resumeToken),
	}
}

func runtimeGraphCheckpointSnapshot(checkpointID string, payload []byte, now time.Time) (RuntimeGraphCheckpointSnapshot, error) {
	key := strings.TrimSpace(checkpointID)
	if key == "" {
		return RuntimeGraphCheckpointSnapshot{}, fmt.Errorf("%w: checkpoint id is required", ErrRuntimeCheckpointRejected)
	}
	if len(payload) == 0 {
		return RuntimeGraphCheckpointSnapshot{}, fmt.Errorf("%w: checkpoint payload is empty", ErrRuntimeCheckpointRejected)
	}
	if containsCredentialLikeCheckpointPayload(payload) {
		return RuntimeGraphCheckpointSnapshot{}, fmt.Errorf("%w: checkpoint payload contains credential-like plaintext", ErrRuntimeCheckpointRejected)
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	sum := sha256.Sum256(payload)
	return RuntimeGraphCheckpointSnapshot{
		CheckpointID:  key,
		PayloadSize:   len(payload),
		PayloadSHA256: hex.EncodeToString(sum[:]),
		CreatedAt:     now.UTC(),
		UpdatedAt:     now.UTC(),
	}, nil
}

func containsCredentialLikeCheckpointPayload(payload []byte) bool {
	lower := strings.ToLower(string(payload))
	markers := []string{
		"authorization: bearer",
		"api_key=",
		"api-key=",
		"x-api-key",
		"access_token=",
		"refresh_token=",
		"password=",
		"secret_key=",
		"sk-",
		"akia",
	}
	for _, marker := range markers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}
