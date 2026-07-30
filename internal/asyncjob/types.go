// types.go defines the durable asynchronous job, outbox, lease, and delivery boundaries.
// types.go 定义持久异步 job、outbox、lease 与投递边界。
package asyncjob

import (
	"context"
	"errors"
	"time"
)

type Status string

const (
	StatusDeliveryPending Status = "delivery_pending"
	StatusQueued          Status = "queued"
	StatusRunning         Status = "running"
	StatusWaiting         Status = "waiting"
	StatusRetryScheduled  Status = "retry_scheduled"
	StatusCompleted       Status = "completed"
	StatusFailed          Status = "failed"
	StatusCancelled       Status = "cancelled"
)

// EventType identifies one authoritative asynchronous job lifecycle event.
// EventType 标识一条权威的异步 job 生命周期事件。
type EventType string

const (
	EventTypeCreated         EventType = "created"
	EventTypeQueued          EventType = "queued"
	EventTypeRunning         EventType = "running"
	EventTypeRetryScheduled  EventType = "retry_scheduled"
	EventTypeWaiting         EventType = "waiting"
	EventTypeResumed         EventType = "resumed"
	EventTypeCancelRequested EventType = "cancel_requested"
	EventTypeCancelled       EventType = "cancelled"
	EventTypeCompleted       EventType = "completed"
	EventTypeFailed          EventType = "failed"
)

var (
	ErrNotFound            = errors.New("async job not found")
	ErrQueueDisabled       = errors.New("async job queue is disabled")
	ErrAlreadyTerminal     = errors.New("async job is already terminal")
	ErrIdempotencyConflict = errors.New("async job idempotency conflict")
	ErrLeaseUnavailable    = errors.New("async job lease unavailable")
)

// Job is the durable asynchronous execution state and its authorization ownership.
// Job 是持久异步执行状态及其鉴权归属信息。
type Job struct {
	ID                string    `json:"id"`
	RunID             string    `json:"run_id"`
	AppID             string    `json:"app_id"`
	WorkspaceID       string    `json:"workspace_id"`
	AppInstanceID     string    `json:"app_instance_id"`
	Kind              string    `json:"kind"`
	Status            Status    `json:"status"`
	RequestSHA256     string    `json:"request_sha256"`
	EncryptedRequest  string    `json:"-"`
	EncryptedResult   string    `json:"-"`
	Attempt           int       `json:"attempt"`
	MaxAttempts       int       `json:"max_attempts"`
	IdempotencyScope  string    `json:"idempotency_scope,omitempty"`
	IdempotencyKey    string    `json:"idempotency_key,omitempty"`
	CheckpointID      string    `json:"checkpoint_id,omitempty"`
	LeaseOwner        string    `json:"-"`
	LeaseToken        string    `json:"-"`
	LeaseUntil        time.Time `json:"lease_until,omitempty"`
	CancelRequestedAt time.Time `json:"cancel_requested_at,omitempty"`
	CancelReason      string    `json:"cancel_reason,omitempty"`
	LastErrorCode     string    `json:"last_error_code,omitempty"`
	LastErrorMessage  string    `json:"last_error_message,omitempty"`
	NextAttemptAt     time.Time `json:"next_attempt_at,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

func (j Job) Terminal() bool {
	switch j.Status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	default:
		return false
	}
}

// CreateInput carries the immutable identity and authorization ownership of a new job.
// CreateInput 携带新 job 的不可变标识与鉴权归属信息。
type CreateInput struct {
	ID               string
	RunID            string
	AppID            string
	WorkspaceID      string
	AppInstanceID    string
	Kind             string
	RequestSHA256    string
	EncryptedRequest string
	IdempotencyScope string
	IdempotencyKey   string
	MaxAttempts      int
	CheckpointID     string
	AvailableAt      time.Time
}

type Outbox struct {
	ID               string
	JobID            string
	RunID            string
	Operation        string
	AvailableAt      time.Time
	LeaseOwner       string
	LeaseUntil       time.Time
	DeliveredAt      time.Time
	DeliveryAttempts int
	LastErrorCode    string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// Event is an immutable PostgreSQL-backed lifecycle record ordered by Cursor.
// Event 是按 Cursor 排序、由 PostgreSQL 持久化的不可变生命周期记录。
type Event struct {
	Cursor     uint64    `json:"cursor"`
	JobID      string    `json:"job_id"`
	RunID      string    `json:"run_id"`
	Type       EventType `json:"type"`
	FromStatus Status    `json:"from_status,omitempty"`
	ToStatus   Status    `json:"to_status"`
	Reason     string    `json:"reason,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
}

type Completion struct {
	EncryptedResult string
	Waiting         bool
	WaitingReason   string
}

type Failure struct {
	Code            string
	Message         string
	Retryable       bool
	EncryptedResult string
}

type Store interface {
	AutoMigrate(context.Context) error
	Create(context.Context, CreateInput) (Job, bool, error)
	Get(context.Context, string) (Job, error)
	ListEvents(context.Context, string, uint64, int) ([]Event, error)
	ClaimOutbox(context.Context, string, time.Time, time.Duration, int) ([]Outbox, error)
	MarkOutboxDelivered(context.Context, Outbox, string, time.Time) error
	ReleaseOutbox(context.Context, Outbox, string, time.Time) error
	AcquireLease(context.Context, string, string, time.Duration) (Job, error)
	RenewLease(context.Context, Job, time.Duration) error
	Complete(context.Context, Job, Completion) error
	Wait(context.Context, Job, Completion) error
	Fail(context.Context, Job, Failure) error
	ScheduleRetry(context.Context, Job, Failure, time.Time) error
	Resume(context.Context, string, string, string, time.Time) (Job, error)
	RequestCancel(context.Context, string, string) (Job, error)
	CancelRequested(context.Context, string) (bool, error)
}

type Delivery struct {
	MessageID string
	OutboxID  string
	JobID     string
	RunID     string
}

type DeliveryQueue interface {
	Ping(context.Context) error
	Ensure(context.Context) error
	Publish(context.Context, Outbox) (string, error)
	Claim(context.Context, string, time.Duration, time.Duration) (Delivery, error)
	Renew(context.Context, Delivery, string) error
	Ack(context.Context, Delivery) error
	Close() error
}

type Handler func(context.Context, Job) (Completion, *Failure)
