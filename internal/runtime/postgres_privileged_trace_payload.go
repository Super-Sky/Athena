// postgres_privileged_trace_payload.go persists encrypted trace details and immutable access audits.
// postgres_privileged_trace_payload.go 持久化加密 trace 明细与不可变访问审计。
package runtime

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresPrivilegedTracePayloadModel struct {
	PayloadRef         string    `gorm:"column:payload_ref;type:text;primaryKey"`
	RunID              string    `gorm:"column:run_id;type:text;not null;index:idx_privileged_trace_payload_run"`
	StepID             string    `gorm:"column:step_id;type:text;not null;default:''"`
	TraceType          string    `gorm:"column:trace_type;type:text;not null"`
	Source             string    `gorm:"column:source;type:text;not null"`
	CorrelationID      string    `gorm:"column:correlation_id;type:text;not null"`
	SchemaVersion      string    `gorm:"column:schema_version;type:text;not null"`
	KeyID              string    `gorm:"column:key_id;type:text;not null"`
	Nonce              []byte    `gorm:"column:nonce;type:bytea;not null"`
	Ciphertext         []byte    `gorm:"column:ciphertext;type:bytea;not null"`
	PayloadSize        int       `gorm:"column:payload_size;type:integer;not null"`
	RedactedFieldCount int       `gorm:"column:redacted_field_count;type:integer;not null;default:0"`
	PayloadSHA256      string    `gorm:"column:payload_sha256;type:text;not null"`
	CreatedAt          time.Time `gorm:"column:created_at;type:timestamptz;not null;index:idx_privileged_trace_payload_created"`
	ExpiresAt          time.Time `gorm:"column:expires_at;type:timestamptz;not null;index:idx_privileged_trace_payload_expires"`
}

func (postgresPrivilegedTracePayloadModel) TableName() string {
	return "runtime_privileged_trace_payloads"
}

type postgresPrivilegedTracePayloadAccessAuditModel struct {
	ID                 string    `gorm:"column:id;type:text;primaryKey"`
	PayloadRef         string    `gorm:"column:payload_ref;type:text;not null;index:idx_privileged_trace_audit_ref"`
	RunID              string    `gorm:"column:run_id;type:text;not null;index:idx_privileged_trace_audit_run"`
	RequestID          string    `gorm:"column:request_id;type:text;not null"`
	ActorSessionHash   string    `gorm:"column:actor_session_hash;type:text;not null"`
	RemoteIP           string    `gorm:"column:remote_ip;type:text;not null"`
	Outcome            string    `gorm:"column:outcome;type:text;not null;index:idx_privileged_trace_audit_outcome"`
	Reason             string    `gorm:"column:reason;type:text;not null;default:''"`
	ReturnedBytes      int       `gorm:"column:returned_bytes;type:integer;not null;default:0"`
	RedactedFieldCount int       `gorm:"column:redacted_field_count;type:integer;not null;default:0"`
	AccessedAt         time.Time `gorm:"column:accessed_at;type:timestamptz;not null;index:idx_privileged_trace_audit_accessed"`
}

func (postgresPrivilegedTracePayloadAccessAuditModel) TableName() string {
	return "runtime_privileged_trace_payload_access_audits"
}

// CreatePrivilegedTracePayload stores encrypted bytes after enforcing the per-run budget under a transaction lock.
// CreatePrivilegedTracePayload 在事务锁内校验单 run 预算后存储加密字节。
func (s *PostgresRuntimeStore) CreatePrivilegedTracePayload(ctx context.Context, input PrivilegedTracePayload) (PrivilegedTracePayload, error) {
	if s == nil || s.db == nil {
		return PrivilegedTracePayload{}, fmt.Errorf("postgres runtime store is not configured")
	}
	if err := validatePrivilegedTracePayload(input); err != nil {
		return PrivilegedTracePayload{}, err
	}
	row := privilegedTracePayloadToRow(input)
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Delete(&postgresPrivilegedTracePayloadModel{}, "expires_at <= ?", time.Now().UTC()).Error; err != nil {
			return err
		}
		if err := tx.Exec("SELECT pg_advisory_xact_lock(hashtext(?))", input.RunID).Error; err != nil {
			return err
		}
		var existing int64
		if err := tx.Model(&postgresPrivilegedTracePayloadModel{}).Where("payload_ref = ?", input.PayloadRef).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return nil
		}
		if input.RunBudgetBytes > 0 {
			var total int64
			if err := tx.Model(&postgresPrivilegedTracePayloadModel{}).
				Where("run_id = ? AND expires_at > ?", input.RunID, time.Now().UTC()).
				Select("COALESCE(SUM(payload_size), 0)").Scan(&total).Error; err != nil {
				return err
			}
			if total+int64(input.PayloadSize) > input.RunBudgetBytes {
				return ErrPrivilegedTracePayloadRunBudgetExceeded
			}
		}
		return tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error
	})
	if err != nil {
		return PrivilegedTracePayload{}, err
	}
	return input, nil
}

// GetPrivilegedTracePayload loads encrypted bytes only when both run and opaque reference match.
// GetPrivilegedTracePayload 仅在 run 与 opaque reference 同时匹配时读取加密字节。
func (s *PostgresRuntimeStore) GetPrivilegedTracePayload(ctx context.Context, runID, payloadRef string) (PrivilegedTracePayload, bool, error) {
	if s == nil || s.db == nil {
		return PrivilegedTracePayload{}, false, fmt.Errorf("postgres runtime store is not configured")
	}
	var row postgresPrivilegedTracePayloadModel
	err := s.db.WithContext(ctx).Where("run_id = ? AND payload_ref = ?", strings.TrimSpace(runID), strings.TrimSpace(payloadRef)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return PrivilegedTracePayload{}, false, nil
	}
	if err != nil {
		return PrivilegedTracePayload{}, false, err
	}
	return privilegedTracePayloadFromRow(row), true, nil
}

// CreatePrivilegedTracePayloadAccessAudit durably records one read attempt without storing payload content.
// CreatePrivilegedTracePayloadAccessAudit 持久记录一次读取尝试，且不保存 payload 内容。
func (s *PostgresRuntimeStore) CreatePrivilegedTracePayloadAccessAudit(ctx context.Context, input PrivilegedTracePayloadAccessAudit) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres runtime store is not configured")
	}
	if strings.TrimSpace(input.RunID) == "" || strings.TrimSpace(input.PayloadRef) == "" || strings.TrimSpace(input.Outcome) == "" {
		return fmt.Errorf("%w: audit run_id, payload_ref, and outcome are required", ErrInvalidPrivilegedTracePayload)
	}
	if input.AccessedAt.IsZero() {
		input.AccessedAt = time.Now().UTC()
	}
	row := postgresPrivilegedTracePayloadAccessAuditModel{
		ID: defaultID(input.ID), PayloadRef: strings.TrimSpace(input.PayloadRef), RunID: strings.TrimSpace(input.RunID),
		RequestID: strings.TrimSpace(input.RequestID), ActorSessionHash: strings.TrimSpace(input.ActorSessionHash),
		RemoteIP: strings.TrimSpace(input.RemoteIP), Outcome: strings.TrimSpace(input.Outcome), Reason: strings.TrimSpace(input.Reason),
		ReturnedBytes: input.ReturnedBytes, RedactedFieldCount: input.RedactedFieldCount, AccessedAt: input.AccessedAt.UTC(),
	}
	return s.db.WithContext(ctx).Create(&row).Error
}

func (s *PostgresRuntimeStore) DeletePrivilegedTracePayload(ctx context.Context, payloadRef string) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres runtime store is not configured")
	}
	return s.db.WithContext(ctx).Delete(&postgresPrivilegedTracePayloadModel{}, "payload_ref = ?", strings.TrimSpace(payloadRef)).Error
}

func (s *PostgresRuntimeStore) DeleteExpiredPrivilegedTracePayloads(ctx context.Context, before time.Time) (int64, error) {
	if s == nil || s.db == nil {
		return 0, fmt.Errorf("postgres runtime store is not configured")
	}
	result := s.db.WithContext(ctx).Delete(&postgresPrivilegedTracePayloadModel{}, "expires_at <= ?", before.UTC())
	return result.RowsAffected, result.Error
}

func (s *PostgresRuntimeStore) ListPrivilegedTracePayloadAccessAudits(ctx context.Context, payloadRef string, limit int) ([]PrivilegedTracePayloadAccessAudit, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres runtime store is not configured")
	}
	query := s.db.WithContext(ctx).Where("payload_ref = ?", strings.TrimSpace(payloadRef)).Order("accessed_at desc, id desc")
	if limit > 0 {
		query = query.Limit(limit)
	}
	var rows []postgresPrivilegedTracePayloadAccessAuditModel
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	result := make([]PrivilegedTracePayloadAccessAudit, 0, len(rows))
	for _, row := range rows {
		result = append(result, PrivilegedTracePayloadAccessAudit{
			ID: row.ID, PayloadRef: row.PayloadRef, RunID: row.RunID, RequestID: row.RequestID,
			ActorSessionHash: row.ActorSessionHash, RemoteIP: row.RemoteIP, Outcome: row.Outcome, Reason: row.Reason,
			ReturnedBytes: row.ReturnedBytes, RedactedFieldCount: row.RedactedFieldCount, AccessedAt: row.AccessedAt,
		})
	}
	return result, nil
}

func validatePrivilegedTracePayload(input PrivilegedTracePayload) error {
	if strings.TrimSpace(input.PayloadRef) == "" || strings.TrimSpace(input.RunID) == "" || strings.TrimSpace(input.TraceType) == "" ||
		strings.TrimSpace(input.Source) == "" || strings.TrimSpace(input.SchemaVersion) == "" || strings.TrimSpace(input.KeyID) == "" ||
		len(input.Nonce) == 0 || len(input.Ciphertext) == 0 || input.PayloadSize <= 0 || input.CreatedAt.IsZero() || !input.ExpiresAt.After(input.CreatedAt) {
		return fmt.Errorf("%w: encrypted payload metadata is incomplete", ErrInvalidPrivilegedTracePayload)
	}
	return nil
}

func privilegedTracePayloadToRow(input PrivilegedTracePayload) postgresPrivilegedTracePayloadModel {
	return postgresPrivilegedTracePayloadModel{
		PayloadRef: input.PayloadRef, RunID: input.RunID, StepID: input.StepID, TraceType: input.TraceType,
		Source: input.Source, CorrelationID: input.CorrelationID, SchemaVersion: input.SchemaVersion, KeyID: input.KeyID,
		Nonce: append([]byte(nil), input.Nonce...), Ciphertext: append([]byte(nil), input.Ciphertext...), PayloadSize: input.PayloadSize,
		RedactedFieldCount: input.RedactedFieldCount, PayloadSHA256: input.PayloadSHA256, CreatedAt: input.CreatedAt.UTC(), ExpiresAt: input.ExpiresAt.UTC(),
	}
}

func privilegedTracePayloadFromRow(row postgresPrivilegedTracePayloadModel) PrivilegedTracePayload {
	return PrivilegedTracePayload{
		PayloadRef: row.PayloadRef, RunID: row.RunID, StepID: row.StepID, TraceType: row.TraceType,
		Source: row.Source, CorrelationID: row.CorrelationID, SchemaVersion: row.SchemaVersion, KeyID: row.KeyID,
		Nonce: append([]byte(nil), row.Nonce...), Ciphertext: append([]byte(nil), row.Ciphertext...), PayloadSize: row.PayloadSize,
		RedactedFieldCount: row.RedactedFieldCount, PayloadSHA256: row.PayloadSHA256, CreatedAt: row.CreatedAt, ExpiresAt: row.ExpiresAt,
	}
}

var _ PrivilegedTracePayloadStore = (*PostgresRuntimeStore)(nil)
