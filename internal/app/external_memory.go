// external_memory.go exposes generic app-owned memory operations to transport adapters.
// external_memory.go 向 transport 适配层暴露通用应用拥有 memory 操作。
package app

import (
	"context"
	"fmt"
	"moss/internal/memory"
)

// WriteExternalMemory stores one scoped app summary.
// WriteExternalMemory 保存一条作用域应用摘要。
func (s *Service) WriteExternalMemory(ctx context.Context, item memory.ExternalRecord) (memory.ExternalRecord, memory.ExternalTrace, error) {
	if s == nil || s.ExternalMemory == nil {
		return memory.ExternalRecord{}, memory.ExternalTrace{}, fmt.Errorf("external memory store is not configured")
	}
	return s.ExternalMemory.Write(ctx, item)
}

// QueryExternalMemory reads summaries constrained by app ownership.
// QueryExternalMemory 按应用归属约束读取摘要。
func (s *Service) QueryExternalMemory(ctx context.Context, appID, ownerID, scope string, limit int) ([]memory.ExternalRecord, memory.ExternalTrace, error) {
	if s == nil || s.ExternalMemory == nil {
		return nil, memory.ExternalTrace{}, fmt.Errorf("external memory store is not configured")
	}
	return s.ExternalMemory.Query(ctx, appID, ownerID, scope, limit)
}
