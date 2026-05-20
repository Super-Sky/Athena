// context_assets.go prepares request-scoped context_assets before runtime execution.
// context_assets.go 负责在 runtime 执行前预处理请求级 context_assets。
package server

import (
	"context"
	"strings"

	appcore "moss/internal/app"
	"moss/internal/contextassets"
	"moss/internal/customization"
)

type contextAssetsPreparation struct {
	GlobalContext map[string]any
	Bundle        *contextassets.Bundle
	Trace         contextassets.UsageTrace
	Effective     contextassets.EffectiveViews
}

func prepareContextAssets(ctx context.Context, application *appcore.Service, globalContext map[string]any, custom customization.UserCustomization, input contextassets.UsageInput) contextAssetsPreparation {
	prepared := contextAssetsPreparation{
		GlobalContext: cloneAnyMap(globalContext),
	}
	if prepared.GlobalContext == nil {
		prepared.GlobalContext = map[string]any{}
	}
	injectDefaultSystemContextAssets(ctx, application, prepared.GlobalContext)
	prepared.Bundle = contextassets.BuildBundle(prepared.GlobalContext)
	if prepared.Bundle == nil {
		if len(custom.ContextAssetOverrides) == 0 {
			return prepared
		}
		prepared.Bundle = (&contextassets.Bundle{}).ApplyOverrides(contextassets.BindingOverrides{
			ContextAssetOverrides:  append([]contextassets.Asset(nil), custom.ContextAssetOverrides...),
			DisabledAssetTypes:     append([]string(nil), custom.DisabledAssetTypes...),
			AssetPriorityOverrides: cloneIntMap(custom.AssetPriorityOverrides),
		})
	}
	if prepared.Bundle != nil {
		prepared.Bundle = prepared.Bundle.ApplyOverrides(contextassets.BindingOverrides{
			ContextAssetOverrides:  append([]contextassets.Asset(nil), custom.ContextAssetOverrides...),
			DisabledAssetTypes:     append([]string(nil), custom.DisabledAssetTypes...),
			AssetPriorityOverrides: cloneIntMap(custom.AssetPriorityOverrides),
		})
		if len(prepared.Bundle.Assets) > 0 {
			prepared.GlobalContext["context_assets"] = toAnySlice(contextassets.AssetMaps(prepared.Bundle.Assets))
			delete(prepared.GlobalContext, "context_assets_resolved")
		}
	}
	if len(prepared.Bundle.Assets) > 0 {
		resolved, err := application.ResolveContextAssets(ctx, prepared.Bundle.Assets)
		if err == nil && len(resolved) > 0 {
			prepared.GlobalContext["context_assets_resolved"] = toAnySlice(contextassets.ResolvedMaps(resolved))
			prepared.Bundle = contextassets.BuildBundle(prepared.GlobalContext)
		}
	}
	if prepared.Bundle != nil {
		prepared.Trace = prepared.Bundle.ResolveUsage(input)
		prepared.Effective = contextassets.BuildEffectiveViews(prepared.Bundle, prepared.Trace)
		for key, value := range prepared.Effective.AsGlobalContext() {
			prepared.GlobalContext[key] = value
		}
	}
	return prepared
}

func injectDefaultSystemContextAssets(ctx context.Context, application *appcore.Service, globalContext map[string]any) {
	if application == nil || globalContext == nil {
		return
	}
	defaults, err := application.DefaultSystemContextAssets(ctx)
	if err != nil || len(defaults) == 0 {
		return
	}
	existingBundle := contextassets.BuildBundle(globalContext)
	var existing []contextassets.Asset
	if existingBundle != nil {
		existing = append(existing, existingBundle.Assets...)
	}
	merged := mergeDefaultContextAssets(existing, defaults)
	if len(merged) == 0 {
		return
	}
	globalContext["context_assets"] = toAnySlice(contextassets.AssetMaps(merged))
}

func mergeDefaultContextAssets(existing []contextassets.Asset, defaults []contextassets.Asset) []contextassets.Asset {
	if len(existing) == 0 {
		return append([]contextassets.Asset(nil), defaults...)
	}
	result := append([]contextassets.Asset(nil), existing...)
	existingByID := map[string]struct{}{}
	existingSingletonTypes := map[string]struct{}{}
	for _, item := range existing {
		assetID := strings.TrimSpace(item.AssetID)
		if assetID != "" {
			existingByID[assetID] = struct{}{}
		}
		assetType := strings.TrimSpace(item.AssetType)
		if isSingletonContextAssetType(assetType) {
			existingSingletonTypes[assetType] = struct{}{}
		}
	}
	for _, item := range defaults {
		assetID := strings.TrimSpace(item.AssetID)
		if assetID == "" {
			continue
		}
		if _, exists := existingByID[assetID]; exists {
			continue
		}
		assetType := strings.TrimSpace(item.AssetType)
		if _, overridden := existingSingletonTypes[assetType]; overridden {
			continue
		}
		result = append(result, item)
	}
	return result
}

func isSingletonContextAssetType(assetType string) bool {
	switch strings.TrimSpace(assetType) {
	case "persona", "agent_profile", "user_profile", "memory_view", "scene", "workflow":
		return true
	default:
		return false
	}
}

func cloneIntMap(input map[string]int) map[string]int {
	if len(input) == 0 {
		return nil
	}
	result := make(map[string]int, len(input))
	for key, value := range input {
		result[key] = value
	}
	return result
}

func toAnySlice(items []map[string]any) []any {
	if len(items) == 0 {
		return nil
	}
	result := make([]any, 0, len(items))
	for _, item := range items {
		result = append(result, item)
	}
	return result
}

func cloneAnySlice(input []map[string]any) []map[string]any {
	if len(input) == 0 {
		return nil
	}
	result := make([]map[string]any, 0, len(input))
	for _, item := range input {
		result = append(result, cloneAnyMap(item))
	}
	return result
}
