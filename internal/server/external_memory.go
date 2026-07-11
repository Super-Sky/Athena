// external_memory.go exposes app-owned memory and context assembly over HTTP.
// external_memory.go 通过 HTTP 暴露应用拥有的记忆与上下文组装能力。
package server

import (
	"context"
	"encoding/json"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/contextassets"
	"moss/internal/memory"
)

// externalMemoryWriteRequest intentionally contains only generic summary fields.
// externalMemoryWriteRequest 有意只包含通用摘要字段。
type externalMemoryWriteRequest struct {
	AppID         string            `json:"app_id"`
	OwnerID       string            `json:"owner_id"`
	Scope         string            `json:"scope"`
	Kind          string            `json:"kind"`
	Summary       string            `json:"summary"`
	SchemaVersion string            `json:"schema_version,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

type externalMemoryQueryRequest struct {
	AppID   string `json:"app_id"`
	OwnerID string `json:"owner_id"`
	Scope   string `json:"scope"`
	Limit   int    `json:"limit,omitempty"`
}

type externalContextAssembleRequest struct {
	externalMemoryQueryRequest
	Query             string                `json:"query,omitempty"`
	TaskType          string                `json:"task_type,omitempty"`
	Scene             string                `json:"scene,omitempty"`
	DesiredOutputMode string                `json:"desired_output_mode,omitempty"`
	Assets            []contextassets.Asset `json:"assets,omitempty"`
}

type externalContextResponse struct {
	Assets             []contextassets.Asset        `json:"assets"`
	MemoryTrace        memory.ExternalTrace         `json:"memory_trace"`
	ContextTrace       contextassets.UsageTrace     `json:"context_trace"`
	EffectiveViews     contextassets.EffectiveViews `json:"effective_views"`
	ContextCompression contextCompressionSummary    `json:"context_compression"`
}

type contextCompressionSummary struct {
	SchemaVersion     string `json:"schema_version"`
	SummaryCount      int    `json:"summary_count"`
	SummaryCharacters int    `json:"summary_characters"`
	Truncated         bool   `json:"truncated"`
}

func handleWriteExternalMemory(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req externalMemoryWriteRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid external memory payload"})
		return
	}
	item, trace, err := application.WriteExternalMemory(ctx, memory.ExternalRecord{
		AppID:         req.AppID,
		OwnerID:       req.OwnerID,
		Scope:         req.Scope,
		Kind:          req.Kind,
		Summary:       req.Summary,
		SchemaVersion: req.SchemaVersion,
		Metadata:      req.Metadata,
	})
	if err != nil {
		c.JSON(externalMemoryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusCreated, map[string]any{"item": item, "trace": trace})
}

func handleQueryExternalMemory(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req externalMemoryQueryRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid external memory query"})
		return
	}
	items, trace, err := queryExternalMemory(ctx, application, req)
	if err != nil {
		c.JSON(externalMemoryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items, "trace": trace})
}

func handleResolveExternalContextAssets(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req externalMemoryQueryRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid context asset resolve request"})
		return
	}
	items, trace, err := queryExternalMemory(ctx, application, req)
	if err != nil {
		c.JSON(externalMemoryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"assets": externalMemoryAssets(items), "memory_trace": trace})
}

func handleAssembleExternalContextAssets(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req externalContextAssembleRequest
	if err := json.Unmarshal(c.Request.Body(), &req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": "invalid context asset assemble request"})
		return
	}
	items, memoryTrace, err := queryExternalMemory(ctx, application, req.externalMemoryQueryRequest)
	if err != nil {
		c.JSON(externalMemoryStatus(err), map[string]string{"error": err.Error()})
		return
	}
	assets := append(externalMemoryAssets(items), req.Assets...)
	bundle := &contextassets.Bundle{Assets: assets}
	usage := bundle.ResolveUsage(contextassets.UsageInput{
		Query:             strings.TrimSpace(req.Query),
		TaskType:          strings.TrimSpace(req.TaskType),
		Scene:             strings.TrimSpace(req.Scene),
		DesiredOutputMode: strings.TrimSpace(req.DesiredOutputMode),
	})
	c.JSON(consts.StatusOK, externalContextResponse{
		Assets:             assets,
		MemoryTrace:        memoryTrace,
		ContextTrace:       usage,
		EffectiveViews:     contextassets.BuildEffectiveViews(bundle, usage),
		ContextCompression: summarizeExternalMemory(items),
	})
}

func queryExternalMemory(ctx context.Context, application *appcore.Service, req externalMemoryQueryRequest) ([]memory.ExternalRecord, memory.ExternalTrace, error) {
	return application.QueryExternalMemory(ctx, req.AppID, req.OwnerID, req.Scope, req.Limit)
}

func externalMemoryAssets(items []memory.ExternalRecord) []contextassets.Asset {
	assets := make([]contextassets.Asset, 0, len(items))
	for _, item := range items {
		assets = append(assets, contextassets.Asset{
			AssetID:    "app_memory." + item.ID,
			AssetType:  "memory_view",
			AssetName:  item.Kind,
			Scope:      item.Scope,
			SourceKind: "app_memory",
			ReadOnly:   true,
			Content: map[string]any{
				"record_id":      item.ID,
				"summary":        item.Summary,
				"schema_version": item.SchemaVersion,
			},
			Resolution: contextassets.Resolution{AllowInlineFallback: true, ResidentHint: true},
			Metadata:   contextassets.Metadata{SourceLabel: item.AppID, Tags: []string{"app_owned", "memory"}},
		})
	}
	return assets
}

func summarizeExternalMemory(items []memory.ExternalRecord) contextCompressionSummary {
	characters := 0
	for _, item := range items {
		characters += len([]rune(item.Summary))
	}
	return contextCompressionSummary{
		SchemaVersion:     "external_context_compression.v1",
		SummaryCount:      len(items),
		SummaryCharacters: characters,
		Truncated:         false,
	}
}

func externalMemoryStatus(err error) int {
	if err == nil {
		return consts.StatusOK
	}
	if strings.Contains(err.Error(), "not configured") {
		return consts.StatusServiceUnavailable
	}
	return consts.StatusBadRequest
}
