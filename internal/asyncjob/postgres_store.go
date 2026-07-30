// postgres_store.go persists authoritative asynchronous job, outbox, and lease state.
// postgres_store.go 持久化权威异步 job、outbox 与 lease 状态。
package asyncjob

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type postgresJobModel struct {
	ID                string     `gorm:"column:id;type:text;primaryKey"`
	RunID             string     `gorm:"column:run_id;type:text;not null;uniqueIndex"`
	AppID             string     `gorm:"column:app_id;type:text;not null;default:'';index"`
	WorkspaceID       string     `gorm:"column:workspace_id;type:text;not null;default:'';index"`
	AppInstanceID     string     `gorm:"column:app_instance_id;type:text;not null;default:'';index"`
	Kind              string     `gorm:"column:kind;type:text;not null;index"`
	Status            string     `gorm:"column:status;type:text;not null;index"`
	RequestSHA256     string     `gorm:"column:request_sha256;type:text;not null"`
	EncryptedRequest  string     `gorm:"column:encrypted_request;type:text;not null"`
	EncryptedResult   string     `gorm:"column:encrypted_result;type:text;not null;default:''"`
	Attempt           int        `gorm:"column:attempt;type:integer;not null;default:0"`
	MaxAttempts       int        `gorm:"column:max_attempts;type:integer;not null"`
	IdempotencyScope  string     `gorm:"column:idempotency_scope;type:text;not null;default:''"`
	IdempotencyKey    string     `gorm:"column:idempotency_key;type:text;not null;default:''"`
	CheckpointID      string     `gorm:"column:checkpoint_id;type:text;not null;default:''"`
	LeaseOwner        string     `gorm:"column:lease_owner;type:text;not null;default:''"`
	LeaseToken        string     `gorm:"column:lease_token;type:text;not null;default:''"`
	LeaseUntil        *time.Time `gorm:"column:lease_until;type:timestamptz"`
	CancelRequestedAt *time.Time `gorm:"column:cancel_requested_at;type:timestamptz"`
	CancelReason      string     `gorm:"column:cancel_reason;type:text;not null;default:''"`
	LastErrorCode     string     `gorm:"column:last_error_code;type:text;not null;default:''"`
	LastErrorMessage  string     `gorm:"column:last_error_message;type:text;not null;default:''"`
	NextAttemptAt     *time.Time `gorm:"column:next_attempt_at;type:timestamptz"`
	CreatedAt         time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt         time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresJobModel) TableName() string { return "runtime_async_jobs" }

type postgresOutboxModel struct {
	ID               string     `gorm:"column:id;type:text;primaryKey"`
	JobID            string     `gorm:"column:job_id;type:text;not null;index"`
	RunID            string     `gorm:"column:run_id;type:text;not null;index"`
	Operation        string     `gorm:"column:operation;type:text;not null"`
	AvailableAt      time.Time  `gorm:"column:available_at;type:timestamptz;not null;index"`
	LeaseOwner       string     `gorm:"column:lease_owner;type:text;not null;default:''"`
	LeaseUntil       *time.Time `gorm:"column:lease_until;type:timestamptz"`
	DeliveredAt      *time.Time `gorm:"column:delivered_at;type:timestamptz;index"`
	DeliveryAttempts int        `gorm:"column:delivery_attempts;type:integer;not null;default:0"`
	LastErrorCode    string     `gorm:"column:last_error_code;type:text;not null;default:''"`
	StreamEntryID    string     `gorm:"column:stream_entry_id;type:text;not null;default:''"`
	CreatedAt        time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt        time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

func (postgresOutboxModel) TableName() string { return "runtime_job_outbox" }

// postgresEventModel stores the append-only authoritative lifecycle timeline.
// postgresEventModel 存储仅追加的权威生命周期时间线。
type postgresEventModel struct {
	Cursor     uint64    `gorm:"column:cursor;type:bigserial;primaryKey;autoIncrement;index:idx_runtime_async_job_events_job_cursor,priority:2"`
	JobID      string    `gorm:"column:job_id;type:text;not null;index:idx_runtime_async_job_events_job_cursor,priority:1"`
	RunID      string    `gorm:"column:run_id;type:text;not null;index"`
	Type       string    `gorm:"column:type;type:text;not null"`
	FromStatus string    `gorm:"column:from_status;type:text;not null;default:''"`
	ToStatus   string    `gorm:"column:to_status;type:text;not null"`
	Reason     string    `gorm:"column:reason;type:text;not null;default:''"`
	OccurredAt time.Time `gorm:"column:occurred_at;type:timestamptz;not null"`
}

func (postgresEventModel) TableName() string { return "runtime_async_job_events" }

type PostgresStore struct{ db *gorm.DB }

func NewPostgresStore(db *gorm.DB) *PostgresStore { return &PostgresStore{db: db} }

func (s *PostgresStore) AutoMigrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("async job postgres database is required")
	}
	db := s.db.WithContext(ctx)
	if err := db.AutoMigrate(&postgresJobModel{}, &postgresOutboxModel{}, &postgresEventModel{}); err != nil {
		return err
	}
	return db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_runtime_async_jobs_idempotency
		ON runtime_async_jobs (idempotency_scope, idempotency_key)
		WHERE idempotency_key <> ''`).Error
}

func (s *PostgresStore) Create(ctx context.Context, in CreateInput) (Job, bool, error) {
	in.ID, in.RunID, in.Kind = strings.TrimSpace(in.ID), strings.TrimSpace(in.RunID), strings.TrimSpace(in.Kind)
	if in.ID == "" || in.RunID == "" || in.Kind == "" || strings.TrimSpace(in.RequestSHA256) == "" || strings.TrimSpace(in.EncryptedRequest) == "" {
		return Job{}, false, fmt.Errorf("async job id, run id, kind, request hash, and encrypted request are required")
	}
	if in.MaxAttempts <= 0 {
		in.MaxAttempts = 3
	}
	if in.AvailableAt.IsZero() {
		in.AvailableAt = time.Now().UTC()
	}
	occurredAt := time.Now().UTC()
	var created postgresJobModel
	reused := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if in.IdempotencyKey != "" {
			var existing postgresJobModel
			err := tx.Where("idempotency_scope = ? AND idempotency_key = ?", in.IdempotencyScope, in.IdempotencyKey).First(&existing).Error
			if err == nil {
				created = existing
				reused = true
				if existing.RequestSHA256 != in.RequestSHA256 {
					return ErrIdempotencyConflict
				}
				return nil
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
		}
		created = postgresJobModel{
			ID: in.ID, RunID: in.RunID, Kind: in.Kind, Status: string(StatusDeliveryPending), RequestSHA256: in.RequestSHA256,
			AppID: strings.TrimSpace(in.AppID), WorkspaceID: strings.TrimSpace(in.WorkspaceID), AppInstanceID: strings.TrimSpace(in.AppInstanceID),
			EncryptedRequest: in.EncryptedRequest, MaxAttempts: in.MaxAttempts, IdempotencyScope: strings.TrimSpace(in.IdempotencyScope),
			IdempotencyKey: strings.TrimSpace(in.IdempotencyKey), CheckpointID: strings.TrimSpace(in.CheckpointID),
		}
		if err := tx.Create(&created).Error; err != nil {
			return err
		}
		if err := tx.Create(&postgresOutboxModel{
			ID: uuid.NewString(), JobID: in.ID, RunID: in.RunID, Operation: "execute", AvailableAt: in.AvailableAt.UTC(),
		}).Error; err != nil {
			return err
		}
		return appendEvent(tx, Event{JobID: in.ID, RunID: in.RunID, Type: EventTypeCreated, ToStatus: StatusDeliveryPending, OccurredAt: occurredAt})
	})
	if err != nil {
		if in.IdempotencyKey != "" && !errors.Is(err, ErrIdempotencyConflict) {
			var existing postgresJobModel
			lookupErr := s.db.WithContext(ctx).Where("idempotency_scope = ? AND idempotency_key = ?", in.IdempotencyScope, in.IdempotencyKey).First(&existing).Error
			if lookupErr == nil {
				if existing.RequestSHA256 != in.RequestSHA256 {
					return Job{}, false, ErrIdempotencyConflict
				}
				return jobFromModel(existing), true, nil
			}
		}
		return Job{}, false, err
	}
	job := jobFromModel(created)
	return job, reused, nil
}

func (s *PostgresStore) Get(ctx context.Context, id string) (Job, error) {
	var row postgresJobModel
	if err := s.db.WithContext(ctx).First(&row, "id = ?", strings.TrimSpace(id)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return Job{}, ErrNotFound
		}
		return Job{}, err
	}
	return jobFromModel(row), nil
}

// ListEvents returns authoritative events strictly after afterCursor in cursor order.
// ListEvents 按 cursor 顺序返回严格晚于 afterCursor 的权威事件。
func (s *PostgresStore) ListEvents(ctx context.Context, jobID string, afterCursor uint64, limit int) ([]Event, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}
	var rows []postgresEventModel
	if err := s.db.WithContext(ctx).
		Where("job_id = ? AND cursor > ?", strings.TrimSpace(jobID), afterCursor).
		Order("cursor ASC").Limit(limit).Find(&rows).Error; err != nil {
		return nil, err
	}
	events := make([]Event, 0, len(rows))
	for _, row := range rows {
		events = append(events, eventFromModel(row))
	}
	return events, nil
}

func (s *PostgresStore) ClaimOutbox(ctx context.Context, owner string, now time.Time, lease time.Duration, limit int) ([]Outbox, error) {
	if limit <= 0 {
		limit = 100
	}
	var claimed []postgresOutboxModel
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var rows []postgresOutboxModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("delivered_at IS NULL AND available_at <= ? AND (lease_until IS NULL OR lease_until < ?)", now, now).
			Order("available_at ASC, created_at ASC").Limit(limit).Find(&rows).Error; err != nil {
			return err
		}
		until := now.Add(lease)
		for index := range rows {
			if err := tx.Model(&postgresOutboxModel{}).Where("id = ?", rows[index].ID).
				Updates(map[string]any{"lease_owner": owner, "lease_until": until, "delivery_attempts": gorm.Expr("delivery_attempts + 1")}).Error; err != nil {
				return err
			}
			rows[index].LeaseOwner = owner
			rows[index].LeaseUntil = &until
			rows[index].DeliveryAttempts++
		}
		claimed = rows
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := make([]Outbox, 0, len(claimed))
	for _, row := range claimed {
		result = append(result, outboxFromModel(row))
	}
	return result, nil
}

func (s *PostgresStore) MarkOutboxDelivered(ctx context.Context, outbox Outbox, streamID string, now time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Model(&postgresOutboxModel{}).
			Where("id = ? AND lease_owner = ? AND delivered_at IS NULL", outbox.ID, outbox.LeaseOwner).
			Updates(map[string]any{"delivered_at": now.UTC(), "stream_entry_id": streamID, "lease_owner": "", "lease_until": nil})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		var job postgresJobModel
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&job, "id = ?", outbox.JobID).Error; err != nil {
			return normalizeNotFound(err)
		}
		from := Status(job.Status)
		if from != StatusDeliveryPending && from != StatusRetryScheduled {
			return ErrLeaseUnavailable
		}
		result = tx.Model(&postgresJobModel{}).Where("id = ? AND status = ?", outbox.JobID, from).
			Updates(map[string]any{"status": StatusQueued, "updated_at": now.UTC()})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		return appendEvent(tx, Event{JobID: job.ID, RunID: job.RunID, Type: EventTypeQueued, FromStatus: from, ToStatus: StatusQueued, OccurredAt: now.UTC()})
	})
}

func (s *PostgresStore) ReleaseOutbox(ctx context.Context, outbox Outbox, errorCode string, availableAt time.Time) error {
	result := s.db.WithContext(ctx).Model(&postgresOutboxModel{}).
		Where("id = ? AND lease_owner = ? AND delivered_at IS NULL", outbox.ID, outbox.LeaseOwner).
		Updates(map[string]any{"lease_owner": "", "lease_until": nil, "last_error_code": errorCode, "available_at": availableAt.UTC()})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLeaseUnavailable
	}
	return nil
}

func (s *PostgresStore) AcquireLease(ctx context.Context, id, owner string, duration time.Duration) (Job, error) {
	now := time.Now().UTC()
	token := uuid.NewString()
	until := now.Add(duration)
	var leased postgresJobModel
	cancelledByRecovery := false
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&leased, "id = ?", strings.TrimSpace(id)).Error; err != nil {
			return normalizeNotFound(err)
		}
		if jobFromModel(leased).Terminal() {
			return ErrAlreadyTerminal
		}
		from := Status(leased.Status)
		if leased.CancelRequestedAt != nil {
			result := tx.Model(&postgresJobModel{}).Where("id = ? AND status = ?", leased.ID, from).
				Updates(map[string]any{"status": StatusCancelled, "lease_owner": "", "lease_token": "", "lease_until": nil, "updated_at": now})
			if result.Error != nil {
				return result.Error
			}
			if result.RowsAffected != 1 {
				return ErrLeaseUnavailable
			}
			reason := leased.CancelReason
			if reason == "" {
				reason = "cancel requested before lease acquire"
			}
			if err := appendEvent(tx, Event{JobID: leased.ID, RunID: leased.RunID, Type: EventTypeCancelled, FromStatus: from, ToStatus: StatusCancelled, Reason: reason, OccurredAt: now}); err != nil {
				return err
			}
			cancelledByRecovery = true
			return tx.First(&leased, "id = ?", leased.ID).Error
		}
		if (from != StatusQueued && from != StatusRetryScheduled && from != StatusDeliveryPending) || (leased.LeaseUntil != nil && !leased.LeaseUntil.Before(now)) {
			return ErrLeaseUnavailable
		}
		result := tx.Model(&postgresJobModel{}).
			Where("id = ? AND status = ? AND cancel_requested_at IS NULL AND (lease_until IS NULL OR lease_until < ?)", leased.ID, from, now).
			Updates(map[string]any{"status": StatusRunning, "lease_owner": owner, "lease_token": token, "lease_until": until, "attempt": gorm.Expr("attempt + 1"), "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		if err := appendEvent(tx, Event{JobID: leased.ID, RunID: leased.RunID, Type: EventTypeRunning, FromStatus: from, ToStatus: StatusRunning, OccurredAt: now}); err != nil {
			return err
		}
		return tx.First(&leased, "id = ?", leased.ID).Error
	})
	if err != nil {
		return Job{}, err
	}
	if cancelledByRecovery {
		return jobFromModel(leased), ErrAlreadyTerminal
	}
	return jobFromModel(leased), nil
}

func (s *PostgresStore) RenewLease(ctx context.Context, job Job, duration time.Duration) error {
	result := s.db.WithContext(ctx).Model(&postgresJobModel{}).
		Where("id = ? AND status = ? AND lease_owner = ? AND lease_token = ?", job.ID, StatusRunning, job.LeaseOwner, job.LeaseToken).
		Updates(map[string]any{"lease_until": time.Now().UTC().Add(duration)})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return ErrLeaseUnavailable
	}
	return nil
}

func (s *PostgresStore) Complete(ctx context.Context, job Job, completion Completion) error {
	return s.finish(ctx, job, StatusCompleted, "", "", completion.EncryptedResult)
}

func (s *PostgresStore) Wait(ctx context.Context, job Job, completion Completion) error {
	return s.finish(ctx, job, StatusWaiting, "awaiting_input", completion.WaitingReason, completion.EncryptedResult)
}

func (s *PostgresStore) Fail(ctx context.Context, job Job, failure Failure) error {
	status := StatusFailed
	if failure.Code == "cancelled" {
		status = StatusCancelled
	}
	return s.finish(ctx, job, status, failure.Code, failure.Message, failure.EncryptedResult)
}

func (s *PostgresStore) finish(ctx context.Context, job Job, status Status, code, message, resultPayload string) error {
	now := time.Now().UTC()
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		updates := map[string]any{"status": status, "last_error_code": code, "last_error_message": message,
			"encrypted_result": resultPayload, "lease_owner": "", "lease_token": "", "lease_until": nil, "updated_at": now}
		result := tx.Model(&postgresJobModel{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND lease_token = ?", job.ID, StatusRunning, job.LeaseOwner, job.LeaseToken).
			Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		eventType := eventTypeForStatus(status)
		reason := message
		if reason == "" {
			reason = code
		}
		return appendEvent(tx, Event{JobID: job.ID, RunID: job.RunID, Type: eventType, FromStatus: StatusRunning, ToStatus: status, Reason: reason, OccurredAt: now})
	})
}

func (s *PostgresStore) ScheduleRetry(ctx context.Context, job Job, failure Failure, availableAt time.Time) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&postgresJobModel{}).
			Where("id = ? AND status = ? AND lease_owner = ? AND lease_token = ?", job.ID, StatusRunning, job.LeaseOwner, job.LeaseToken).
			Updates(map[string]any{"status": StatusRetryScheduled, "last_error_code": failure.Code, "last_error_message": failure.Message,
				"next_attempt_at": availableAt.UTC(), "lease_owner": "", "lease_token": "", "lease_until": nil, "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		if err := tx.Create(&postgresOutboxModel{ID: uuid.NewString(), JobID: job.ID, RunID: job.RunID, Operation: "retry", AvailableAt: availableAt.UTC()}).Error; err != nil {
			return err
		}
		return appendEvent(tx, Event{JobID: job.ID, RunID: job.RunID, Type: EventTypeRetryScheduled, FromStatus: StatusRunning, ToStatus: StatusRetryScheduled, Reason: failure.Message, OccurredAt: now})
	})
}

func (s *PostgresStore) Resume(ctx context.Context, id, encryptedRequest, _ string, availableAt time.Time) (Job, error) {
	if availableAt.IsZero() {
		availableAt = time.Now().UTC()
	}
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		result := tx.Model(&postgresJobModel{}).Where("id = ? AND status = ?", id, StatusWaiting).
			Updates(map[string]any{"status": StatusDeliveryPending, "encrypted_request": encryptedRequest,
				"last_error_code": "", "last_error_message": "", "updated_at": now})
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		var job postgresJobModel
		if err := tx.First(&job, "id = ?", id).Error; err != nil {
			return err
		}
		if err := tx.Create(&postgresOutboxModel{ID: uuid.NewString(), JobID: job.ID, RunID: job.RunID, Operation: "resume", AvailableAt: availableAt.UTC()}).Error; err != nil {
			return err
		}
		return appendEvent(tx, Event{JobID: job.ID, RunID: job.RunID, Type: EventTypeResumed, FromStatus: StatusWaiting, ToStatus: StatusDeliveryPending, OccurredAt: now})
	})
	if err != nil {
		return Job{}, err
	}
	return s.Get(ctx, id)
}

func (s *PostgresStore) RequestCancel(ctx context.Context, id, reason string) (Job, error) {
	now := time.Now().UTC()
	var row postgresJobModel
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&row, "id = ?", strings.TrimSpace(id)).Error; err != nil {
			return normalizeNotFound(err)
		}
		job := jobFromModel(row)
		if job.Terminal() {
			return ErrAlreadyTerminal
		}
		if row.CancelRequestedAt != nil {
			return ErrLeaseUnavailable
		}
		from, to := Status(row.Status), Status(row.Status)
		eventType := EventTypeCancelRequested
		updates := map[string]any{"cancel_requested_at": now, "cancel_reason": strings.TrimSpace(reason), "updated_at": now}
		if from != StatusRunning {
			to = StatusCancelled
			eventType = EventTypeCancelled
			updates["status"] = to
		}
		result := tx.Model(&postgresJobModel{}).Where("id = ? AND status = ? AND cancel_requested_at IS NULL", row.ID, from).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected != 1 {
			return ErrLeaseUnavailable
		}
		if to == StatusCancelled {
			if err := tx.Model(&postgresOutboxModel{}).
				Where("job_id = ? AND delivered_at IS NULL", row.ID).
				Updates(map[string]any{"delivered_at": now, "lease_owner": "", "lease_until": nil, "last_error_code": "cancelled"}).Error; err != nil {
				return err
			}
		}
		if err := appendEvent(tx, Event{JobID: row.ID, RunID: row.RunID, Type: eventType, FromStatus: from, ToStatus: to, Reason: strings.TrimSpace(reason), OccurredAt: now}); err != nil {
			return err
		}
		return tx.First(&row, "id = ?", row.ID).Error
	})
	if err != nil {
		return Job{}, err
	}
	return jobFromModel(row), nil
}

func (s *PostgresStore) CancelRequested(ctx context.Context, id string) (bool, error) {
	job, err := s.Get(ctx, id)
	return !job.CancelRequestedAt.IsZero(), err
}

func jobFromModel(row postgresJobModel) Job {
	return Job{ID: row.ID, RunID: row.RunID, AppID: row.AppID, WorkspaceID: row.WorkspaceID, AppInstanceID: row.AppInstanceID,
		Kind: row.Kind, Status: Status(row.Status), RequestSHA256: row.RequestSHA256,
		EncryptedRequest: row.EncryptedRequest, EncryptedResult: row.EncryptedResult, Attempt: row.Attempt, MaxAttempts: row.MaxAttempts,
		IdempotencyScope: row.IdempotencyScope, IdempotencyKey: row.IdempotencyKey, CheckpointID: row.CheckpointID,
		LeaseOwner: row.LeaseOwner, LeaseToken: row.LeaseToken, LeaseUntil: valueTime(row.LeaseUntil),
		CancelRequestedAt: valueTime(row.CancelRequestedAt), CancelReason: row.CancelReason, LastErrorCode: row.LastErrorCode,
		LastErrorMessage: row.LastErrorMessage, NextAttemptAt: valueTime(row.NextAttemptAt), CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func outboxFromModel(row postgresOutboxModel) Outbox {
	return Outbox{ID: row.ID, JobID: row.JobID, RunID: row.RunID, Operation: row.Operation, AvailableAt: row.AvailableAt,
		LeaseOwner: row.LeaseOwner, LeaseUntil: valueTime(row.LeaseUntil), DeliveredAt: valueTime(row.DeliveredAt),
		DeliveryAttempts: row.DeliveryAttempts, LastErrorCode: row.LastErrorCode, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

func valueTime(value *time.Time) time.Time {
	if value == nil {
		return time.Time{}
	}
	return value.UTC()
}

// appendEvent must be called inside the transaction that wins the matching state CAS.
// appendEvent 必须在赢得对应状态 CAS 的同一事务内调用。
func appendEvent(tx *gorm.DB, event Event) error {
	return tx.Create(&postgresEventModel{JobID: event.JobID, RunID: event.RunID, Type: string(event.Type), FromStatus: string(event.FromStatus),
		ToStatus: string(event.ToStatus), Reason: strings.TrimSpace(event.Reason), OccurredAt: event.OccurredAt.UTC()}).Error
}

func eventFromModel(row postgresEventModel) Event {
	return Event{Cursor: row.Cursor, JobID: row.JobID, RunID: row.RunID, Type: EventType(row.Type), FromStatus: Status(row.FromStatus),
		ToStatus: Status(row.ToStatus), Reason: row.Reason, OccurredAt: row.OccurredAt.UTC()}
}

// eventTypeForStatus maps terminal and waiting states to their public event names.
// eventTypeForStatus 将终态与等待态映射为公开事件名称。
func eventTypeForStatus(status Status) EventType {
	switch status {
	case StatusWaiting:
		return EventTypeWaiting
	case StatusCompleted:
		return EventTypeCompleted
	case StatusCancelled:
		return EventTypeCancelled
	default:
		return EventTypeFailed
	}
}

func normalizeNotFound(err error) error {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return ErrNotFound
	}
	return err
}
