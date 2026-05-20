// client.go performs same-round platform context detail loading when summaries are insufficient.
// client.go 负责在 summary 不足时，同轮读取 platform context detail。
package context

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// ClientConfig captures the HTTP settings needed to load platform context details.
// ClientConfig 描述读取 platform context detail 所需的 HTTP 设置。
type ClientConfig struct {
	BaseURL              string
	Timeout              time.Duration
	AuthHeader           string
	AuthToken            string
	ForwardAuthorization bool
}

// RequestMetadata captures request-scoped transport metadata that may need forwarding.
// RequestMetadata 描述可能需要转发给 platform detail 接口的请求级元数据。
type RequestMetadata struct {
	Authorization      string
	ContextAccessToken string
	RequestID          string
}

// HydrationResult captures the same-round detail loading outcome.
// HydrationResult 描述同轮 detail 读取的结果。
type HydrationResult struct {
	GlobalContext  map[string]any
	LoadedTypes    []string
	FailedTypes    map[string]string
	RequestedTypes []string
}

// HydrateDetails loads requested platform context details and merges them back into global context.
// HydrateDetails 负责读取所请求的 platform context detail，并把结果合并回 global_context。
func HydrateDetails(ctx context.Context, cfg ClientConfig, globalContext map[string]any, requested []string, meta RequestMetadata) (HydrationResult, error) {
	result := HydrationResult{
		GlobalContext:  cloneMap(globalContext),
		LoadedTypes:    nil,
		FailedTypes:    map[string]string{},
		RequestedTypes: compactStrings(requested),
	}
	if len(result.RequestedTypes) == 0 {
		return result, nil
	}
	client := &http.Client{Timeout: cfg.Timeout}
	for _, typ := range result.RequestedTypes {
		endpoint, err := resolveDetailEndpoint(cfg.BaseURL, result.GlobalContext["platform_context_catalog"], typ)
		if err != nil {
			result.FailedTypes[typ] = err.Error()
			continue
		}
		payload, err := fetchDetail(ctx, client, endpoint, cfg, meta)
		if err != nil {
			result.FailedTypes[typ] = err.Error()
			continue
		}
		result.GlobalContext[detailKey(Type(typ))] = payload
		result.LoadedTypes = append(result.LoadedTypes, typ)
	}
	if len(result.FailedTypes) == 0 {
		result.FailedTypes = nil
	}
	return result, nil
}

func fetchDetail(ctx context.Context, client *http.Client, endpoint string, cfg ClientConfig, meta RequestMetadata) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	if header := strings.TrimSpace(cfg.AuthHeader); header != "" {
		switch {
		case strings.TrimSpace(meta.ContextAccessToken) != "":
			req.Header.Set(header, formatAuthValue(header, strings.TrimSpace(meta.ContextAccessToken)))
		case strings.TrimSpace(cfg.AuthToken) != "":
			req.Header.Set(header, strings.TrimSpace(cfg.AuthToken))
		case cfg.ForwardAuthorization && strings.TrimSpace(meta.Authorization) != "":
			req.Header.Set(header, strings.TrimSpace(meta.Authorization))
		}
	}
	if strings.TrimSpace(meta.RequestID) != "" {
		req.Header.Set("X-Request-ID", strings.TrimSpace(meta.RequestID))
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	var payload map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return payload, nil
}

func formatAuthValue(header string, token string) string {
	if strings.TrimSpace(token) == "" {
		return ""
	}
	if !strings.EqualFold(strings.TrimSpace(header), "Authorization") {
		return token
	}
	lower := strings.ToLower(token)
	if strings.HasPrefix(lower, "bearer ") || strings.HasPrefix(lower, "basic ") {
		return token
	}
	return "Bearer " + token
}

func resolveDetailEndpoint(baseURL string, catalogValue any, typ string) (string, error) {
	catalog := normalizeCatalog(catalogValue)
	entry := catalog[Type(typ)]
	if direct := strings.TrimSpace(RenderSummary(entry["detail_url"])); direct != "" {
		return direct, nil
	}
	if direct := strings.TrimSpace(RenderSummary(entry["detail_endpoint"])); direct != "" {
		if strings.HasPrefix(direct, "http://") || strings.HasPrefix(direct, "https://") {
			return direct, nil
		}
		return joinURL(baseURL, direct)
	}
	if direct := strings.TrimSpace(RenderSummary(entry["detail_api"])); direct != "" {
		if strings.HasPrefix(direct, "http://") || strings.HasPrefix(direct, "https://") {
			return direct, nil
		}
		return joinURL(baseURL, direct)
	}
	if p := strings.TrimSpace(RenderSummary(entry["detail_path"])); p != "" {
		return joinURL(baseURL, p)
	}
	if strings.TrimSpace(baseURL) == "" {
		return "", fmt.Errorf("platform context detail base URL is not configured")
	}
	return joinURL(baseURL, "/api/v1/platform-context/"+url.PathEscape(strings.TrimSpace(typ)))
}

func joinURL(baseURL string, p string) (string, error) {
	baseURL = strings.TrimSpace(baseURL)
	if baseURL == "" {
		return "", fmt.Errorf("base URL is empty")
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}
	parsed.Path = path.Clean(strings.TrimRight(parsed.Path, "/") + p)
	return parsed.String(), nil
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
