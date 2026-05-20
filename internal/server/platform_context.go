// platform_context.go preloads platform context details before runtime execution when summaries are insufficient.
// platform_context.go 负责在 summary 不足时，于 runtime 执行前预取 platform context detail。
package server

import (
	"context"
	"strings"
	"time"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"moss/internal/config"
	platformcontext "moss/internal/extensions/platform/context"
)

type platformContextPreparation struct {
	GlobalContext            map[string]any
	Bundle                   *platformcontext.Bundle
	Trace                    platformcontext.UsageTrace
	ContextDetailsLoaded     []string
	ContextDetailFetchErrors map[string]string
}

func preparePlatformContext(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, globalContext map[string]any, input platformcontext.UsageInput) platformContextPreparation {
	prepared := platformContextPreparation{
		GlobalContext: cloneAnyMap(globalContext),
	}
	prepared.Bundle = platformcontext.BuildBundle(prepared.GlobalContext)
	if prepared.Bundle == nil {
		return prepared
	}
	prepared.Trace = prepared.Bundle.ResolveUsage(input)
	if len(prepared.Trace.ContextDetailsRequested) == 0 {
		return prepared
	}
	timeoutSeconds := cfg.PlatformContext.DetailTimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 5
	}
	hydrated, err := platformcontext.HydrateDetails(ctx, platformcontext.ClientConfig{
		BaseURL:              strings.TrimSpace(cfg.PlatformContext.BaseURL),
		Timeout:              time.Duration(timeoutSeconds) * time.Second,
		AuthHeader:           strings.TrimSpace(cfg.PlatformContext.AuthHeader),
		AuthToken:            strings.TrimSpace(cfg.PlatformContext.AuthToken),
		ForwardAuthorization: cfg.PlatformContext.ForwardAuthorization,
	}, prepared.GlobalContext, prepared.Trace.ContextDetailsRequested, platformcontext.RequestMetadata{
		Authorization:      headerValue(c, "Authorization"),
		ContextAccessToken: contextAccessToken(prepared.Bundle),
		RequestID:          headerValue(c, "X-Request-ID"),
	})
	if err != nil {
		return prepared
	}
	prepared.GlobalContext = hydrated.GlobalContext
	prepared.Bundle = platformcontext.BuildBundle(prepared.GlobalContext)
	prepared.ContextDetailsLoaded = append([]string(nil), hydrated.LoadedTypes...)
	prepared.ContextDetailFetchErrors = hydrated.FailedTypes
	if prepared.Bundle != nil {
		updatedTrace := prepared.Bundle.ResolveUsage(input)
		updatedTrace.ContextDetailsRequested = append([]string(nil), prepared.Trace.ContextDetailsRequested...)
		prepared.Trace = updatedTrace
	}
	return prepared
}

func contextAccessToken(bundle *platformcontext.Bundle) string {
	if bundle == nil {
		return ""
	}
	return bundle.ContextAccessToken()
}

func headerValue(c *hertzapp.RequestContext, name string) string {
	if c == nil || strings.TrimSpace(name) == "" {
		return ""
	}
	return strings.TrimSpace(string(c.Request.Header.Peek(name)))
}
