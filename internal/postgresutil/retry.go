// retry.go provides shared PostgreSQL retry helpers for optimistic-lock or transient failure paths.
// retry.go 提供 PostgreSQL 乐观锁或瞬时失败路径共用的重试辅助能力。
package postgresutil

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
)

const (
	DefaultRetryAttempts = 3
	baseRetryDelay       = 150 * time.Millisecond
)

// IsTransientError reports whether one PostgreSQL-facing error is worth retrying.
// IsTransientError 用于判断一次面向 PostgreSQL 的错误是否值得重试。
func IsTransientError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if pgconn.Timeout(err) || pgconn.SafeToRetry(err) {
		return true
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		code := pgErr.SQLState()
		if strings.HasPrefix(code, "08") {
			return true
		}
		switch code {
		case "57P01", "57P02", "57P03":
			return true
		}
	}
	return false
}

// WithRetry reruns one operation when PostgreSQL returns a transient connectivity error.
// WithRetry 会在 PostgreSQL 返回瞬时连接类错误时重试一次操作。
func WithRetry(ctx context.Context, attempts int, operation func() error) error {
	if attempts <= 0 {
		attempts = DefaultRetryAttempts
	}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		if ctx != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if err := operation(); err != nil {
			lastErr = err
			if !IsTransientError(err) || attempt == attempts-1 {
				return err
			}
			delay := time.Duration(1<<attempt) * baseRetryDelay
			timer := time.NewTimer(delay)
			select {
			case <-timer.C:
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			}
			continue
		}
		return nil
	}
	return lastErr
}
