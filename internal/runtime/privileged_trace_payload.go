// privileged_trace_payload.go defines encrypted, separately authorized runtime trace payloads.
// privileged_trace_payload.go 定义独立授权读取的加密 runtime trace payload。
package runtime

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

const (
	// PrivilegedTracePayloadSchemaVersion is the encrypted payload envelope version.
	// PrivilegedTracePayloadSchemaVersion 是加密 payload 包络版本。
	PrivilegedTracePayloadSchemaVersion = "runtime_privileged_trace_payload.v1"
	privilegedTracePayloadRefPrefix     = "ptp_"
)

// ErrInvalidPrivilegedTracePayload marks rejected payload policy, crypto, or storage input.
// ErrInvalidPrivilegedTracePayload 表示 payload 策略、加密或存储输入被拒绝。
var ErrInvalidPrivilegedTracePayload = errors.New("invalid privileged trace payload")

// ErrPrivilegedTracePayloadRunBudgetExceeded marks a payload rejected by the per-run byte budget.
// ErrPrivilegedTracePayloadRunBudgetExceeded 表示 payload 因单次运行字节预算不足而被拒绝。
var ErrPrivilegedTracePayloadRunBudgetExceeded = errors.New("privileged trace payload run budget exceeded")

// PrivilegedTracePayloadPolicy controls opt-in capture, sampling, retention, size, and encryption.
// PrivilegedTracePayloadPolicy 控制显式开启、采样、保留期、大小和加密。
type PrivilegedTracePayloadPolicy struct {
	Enabled               bool
	CaptureModel          bool
	CaptureTools          bool
	CaptureContextSummary bool
	SampleRate            float64
	Retention             time.Duration
	MaxPayloadBytes       int
	MaxRunBytes           int64
	MaxFieldBytes         int
	KeyID                 string
	EncryptionKey         []byte
	DisabledWorkspaceIDs  map[string]struct{}
	ExtraRedactedFields   []string
}

// PrivilegedTracePayloadCoordinates provide stable correlation inputs for one payload.
// PrivilegedTracePayloadCoordinates 提供一条 payload 的稳定关联输入。
type PrivilegedTracePayloadCoordinates struct {
	RunID         string
	StepID        string
	TraceType     string
	Source        string
	CorrelationID string
}

// PrivilegedTracePayload stores only encrypted redacted bytes and safe correlation metadata.
// PrivilegedTracePayload 只保存加密后的脱敏字节与安全关联元数据。
type PrivilegedTracePayload struct {
	PayloadRef         string
	RunID              string
	StepID             string
	TraceType          string
	Source             string
	CorrelationID      string
	SchemaVersion      string
	KeyID              string
	Nonce              []byte
	Ciphertext         []byte
	PayloadSize        int
	RedactedFieldCount int
	RunBudgetBytes     int64
	PayloadSHA256      string
	CreatedAt          time.Time
	ExpiresAt          time.Time
}

// PrivilegedTracePayloadAccess describes one audited read attempt.
// PrivilegedTracePayloadAccess 描述一次需要审计的读取尝试。
type PrivilegedTracePayloadAccess struct {
	ActorID     string
	ActorType   string
	WorkspaceID string
	Reason      string
	RequestID   string
	AccessedAt  time.Time
}

// PrivilegedTracePayloadAccessAudit is the immutable audit record for one payload read.
// PrivilegedTracePayloadAccessAudit 是一次 payload 读取对应的不可变审计记录。
type PrivilegedTracePayloadAccessAudit struct {
	ID                 string
	PayloadRef         string
	RunID              string
	RequestID          string
	ActorSessionHash   string
	RemoteIP           string
	Outcome            string
	Reason             string
	ReturnedBytes      int
	RedactedFieldCount int
	AccessedAt         time.Time
}

// PrivilegedTracePayloadStore is an optional storage boundary separate from core runtime persistence.
// PrivilegedTracePayloadStore 是独立于核心 runtime persistence 的可选存储边界。
type PrivilegedTracePayloadStore interface {
	CreatePrivilegedTracePayload(context.Context, PrivilegedTracePayload) (PrivilegedTracePayload, error)
	GetPrivilegedTracePayload(context.Context, string, string) (PrivilegedTracePayload, bool, error)
	CreatePrivilegedTracePayloadAccessAudit(context.Context, PrivilegedTracePayloadAccessAudit) error
	DeletePrivilegedTracePayload(context.Context, string) error
	DeleteExpiredPrivilegedTracePayloads(context.Context, time.Time) (int64, error)
	ListPrivilegedTracePayloadAccessAudits(context.Context, string, int) ([]PrivilegedTracePayloadAccessAudit, error)
}

// BuildPrivilegedTracePayload redacts, bounds, and encrypts one payload before persistence.
// BuildPrivilegedTracePayload 在持久化前对 payload 执行脱敏、限长和加密。
func BuildPrivilegedTracePayload(coordinates PrivilegedTracePayloadCoordinates, payload any, policy PrivilegedTracePayloadPolicy, now time.Time) (PrivilegedTracePayload, string, error) {
	if !policy.Enabled {
		return PrivilegedTracePayload{}, "disabled", nil
	}
	coordinates = normalizePrivilegedTraceCoordinates(coordinates)
	if coordinates.RunID == "" || coordinates.TraceType == "" || coordinates.Source == "" || coordinates.CorrelationID == "" {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: run_id, trace_type, source, and correlation_id are required", ErrInvalidPrivilegedTracePayload)
	}
	payloadRef := PrivilegedTracePayloadRef(coordinates)
	if !privilegedTracePayloadSampled(payloadRef, policy.SampleRate) {
		return PrivilegedTracePayload{}, "sampled_out", nil
	}
	if policy.Retention <= 0 {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: retention must be positive", ErrInvalidPrivilegedTracePayload)
	}
	if policy.MaxPayloadBytes <= 0 {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: max payload bytes must be positive", ErrInvalidPrivilegedTracePayload)
	}
	redacted, redactedFieldCount := RedactPrivilegedTracePayloadWithFields(payload, policy.MaxFieldBytes, policy.ExtraRedactedFields)
	plaintext, err := json.Marshal(redacted)
	if err != nil {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: marshal redacted payload: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	if len(plaintext) > policy.MaxPayloadBytes {
		return PrivilegedTracePayload{}, "size_exceeded", nil
	}
	block, err := aes.NewCipher(policy.EncryptionKey)
	if err != nil {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: encryption key: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: initialize AES-GCM: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return PrivilegedTracePayload{}, "", fmt.Errorf("%w: create nonce: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	digest := sha256.Sum256(plaintext)
	createdAt := now.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	createdAt = createdAt.Truncate(time.Microsecond)
	record := PrivilegedTracePayload{
		PayloadRef: payloadRef, RunID: coordinates.RunID, StepID: coordinates.StepID,
		TraceType: coordinates.TraceType, Source: coordinates.Source, CorrelationID: coordinates.CorrelationID,
		SchemaVersion: PrivilegedTracePayloadSchemaVersion, KeyID: strings.TrimSpace(policy.KeyID), Nonce: nonce,
		PayloadSize: len(plaintext), PayloadSHA256: hex.EncodeToString(digest[:]), CreatedAt: createdAt,
		ExpiresAt: createdAt.Add(policy.Retention).Truncate(time.Microsecond), RunBudgetBytes: policy.MaxRunBytes,
	}
	if redactedFieldCount > 0 {
		record.RedactedFieldCount = redactedFieldCount
	}
	record.Ciphertext = gcm.Seal(nil, nonce, plaintext, privilegedTracePayloadAAD(record))
	return record, "captured", nil
}

// DecryptPrivilegedTracePayload authenticates and decrypts one stored payload.
// DecryptPrivilegedTracePayload 验证并解密一条已存储 payload。
func DecryptPrivilegedTracePayload(record PrivilegedTracePayload, key []byte) (map[string]any, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("%w: decryption key: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("%w: initialize AES-GCM: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	plaintext, err := gcm.Open(nil, record.Nonce, record.Ciphertext, privilegedTracePayloadAAD(record))
	if err != nil {
		return nil, fmt.Errorf("%w: authenticate ciphertext", ErrInvalidPrivilegedTracePayload)
	}
	digest := sha256.Sum256(plaintext)
	if record.PayloadSHA256 != hex.EncodeToString(digest[:]) || record.PayloadSize != len(plaintext) {
		return nil, fmt.Errorf("%w: payload integrity metadata mismatch", ErrInvalidPrivilegedTracePayload)
	}
	var output map[string]any
	if err := json.Unmarshal(plaintext, &output); err != nil {
		return nil, fmt.Errorf("%w: decode plaintext: %v", ErrInvalidPrivilegedTracePayload, err)
	}
	return output, nil
}

// PrivilegedTracePayloadRef derives a deterministic opaque reference from safe correlation coordinates.
// PrivilegedTracePayloadRef 根据安全关联坐标生成确定性的 opaque reference。
func PrivilegedTracePayloadRef(coordinates PrivilegedTracePayloadCoordinates) string {
	coordinates = normalizePrivilegedTraceCoordinates(coordinates)
	digest := sha256.Sum256([]byte(strings.Join([]string{coordinates.RunID, coordinates.StepID, coordinates.TraceType, coordinates.Source, coordinates.CorrelationID}, "\x00")))
	return privilegedTracePayloadRefPrefix + hex.EncodeToString(digest[:16])
}

func normalizePrivilegedTraceCoordinates(input PrivilegedTracePayloadCoordinates) PrivilegedTracePayloadCoordinates {
	input.RunID = strings.TrimSpace(input.RunID)
	input.StepID = strings.TrimSpace(input.StepID)
	input.TraceType = strings.TrimSpace(input.TraceType)
	input.Source = strings.TrimSpace(input.Source)
	input.CorrelationID = strings.TrimSpace(input.CorrelationID)
	return input
}

func privilegedTracePayloadSampled(payloadRef string, sampleRate float64) bool {
	if sampleRate <= 0 {
		return false
	}
	if sampleRate >= 1 {
		return true
	}
	digest := sha256.Sum256([]byte(payloadRef))
	value := uint64(digest[0])<<8 | uint64(digest[1])
	return float64(value)/65535 < sampleRate
}

// DerivePrivilegedTracePayloadKey derives an AES-256 key from a dedicated deployment secret.
// DerivePrivilegedTracePayloadKey 从独立部署密钥派生 AES-256 key。
func DerivePrivilegedTracePayloadKey(secret string) []byte {
	digest := sha256.Sum256([]byte(strings.TrimSpace(secret)))
	return append([]byte(nil), digest[:]...)
}

func privilegedTracePayloadAAD(record PrivilegedTracePayload) []byte {
	return []byte(strings.Join([]string{
		record.PayloadRef, record.RunID, record.StepID, record.TraceType, record.Source, record.CorrelationID,
		record.SchemaVersion, record.KeyID, strconv.Itoa(record.PayloadSize), strconv.Itoa(record.RedactedFieldCount),
		record.PayloadSHA256, record.CreatedAt.UTC().Format(time.RFC3339Nano), record.ExpiresAt.UTC().Format(time.RFC3339Nano),
	}, "\x00"))
}
