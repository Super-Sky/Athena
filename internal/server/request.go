// request.go defines transport-layer request helpers shared by HTTP handlers.
// request.go 定义 HTTP handlers 共用的传输层请求辅助结构和逻辑。
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

type requestIDKey struct{}

func withRequestID(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, requestIDKey{}, requestID)
}

func requestIDFromContext(ctx context.Context) string {
	requestID, _ := ctx.Value(requestIDKey{}).(string)
	return requestID
}

func newRequestID() string {
	return fmt.Sprintf("req-%d-%s", time.Now().UnixNano(), randomRequestIDSuffix())
}

func randomRequestIDSuffix() string {
	bytes := make([]byte, 4)
	if _, err := rand.Read(bytes); err != nil {
		return "fallback"
	}
	return hex.EncodeToString(bytes)
}
