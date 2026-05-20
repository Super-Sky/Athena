// control_plane.go exposes the standalone control-plane HTTP handlers for scenes, skills, runtime tuning, and docs bootstrap.
// control_plane.go 负责暴露独立控制面的 HTTP 处理器，包括场景、skill、运行开关和文档启动载荷。
package server

import (
	"context"
	"errors"
	"net/http"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
	"moss/internal/controlplane"
	"moss/internal/validationmcp"
)

type controlPlaneHandler func(context.Context, *hertzapp.RequestContext, config.Config, *appcore.Service)

func handleControlPlaneAuthStatus(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	status, err := application.GetControlPlaneAuthStatus(ctx, string(c.Cookie(controlplane.ControlPlaneSessionCookie)), c.ClientIP())
	if err != nil && !errors.Is(err, controlplane.ErrControlPlaneAuthRequired) && !errors.Is(err, controlplane.ErrControlPlaneAuthLocked) {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if errors.Is(err, controlplane.ErrControlPlaneAuthLocked) {
		c.JSON(consts.StatusLocked, status)
		return
	}
	c.JSON(consts.StatusOK, status)
}

func handleControlPlaneLogin(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.LoginRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	status, sessionID, err := application.LoginControlPlane(ctx, strings.TrimSpace(req.Token), c.ClientIP())
	if err != nil {
		switch {
		case errors.Is(err, controlplane.ErrControlPlaneAuthLocked):
			c.JSON(consts.StatusLocked, status)
		case errors.Is(err, controlplane.ErrControlPlaneAuthInvalidToken):
			c.JSON(consts.StatusUnauthorized, status)
		default:
			c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		}
		return
	}
	if sessionID != "" {
		c.SetCookie(
			controlplane.ControlPlaneSessionCookie,
			sessionID,
			cfg.ControlPlane.SessionTTLSecs,
			"/",
			"",
			protocol.CookieSameSiteLaxMode,
			false,
			true,
		)
	}
	c.JSON(consts.StatusOK, status)
}

func handleControlPlaneLogout(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	status, err := application.LogoutControlPlane(ctx, string(c.Cookie(controlplane.ControlPlaneSessionCookie)), c.ClientIP())
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.SetCookie(
		controlplane.ControlPlaneSessionCookie,
		"",
		-1,
		"/",
		"",
		protocol.CookieSameSiteLaxMode,
		false,
		true,
	)
	c.JSON(consts.StatusOK, status)
}

func withControlPlaneAuth(cfg config.Config, application *appcore.Service, next controlPlaneHandler) controlPlaneHandler {
	return func(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
		applyControlPlaneCORS(c, cfg)
		if string(c.Method()) == http.MethodOptions {
			c.Status(consts.StatusNoContent)
			return
		}
		if application == nil || application.ControlPlane == nil {
			next(ctx, c, cfg, application)
			return
		}
		status, err := application.ControlPlane.Authorize(ctx, string(c.Cookie(controlplane.ControlPlaneSessionCookie)), c.ClientIP())
		if err != nil {
			switch {
			case errors.Is(err, controlplane.ErrControlPlaneAuthLocked):
				c.JSON(consts.StatusLocked, status)
			case errors.Is(err, controlplane.ErrControlPlaneAuthRequired):
				c.JSON(consts.StatusUnauthorized, status)
			default:
				c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
			}
			return
		}
		next(ctx, c, cfg, application)
	}
}

func handleControlPlaneBootstrap(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	payload, err := application.ControlPlaneBootstrap(ctx, "/swagger/openapi.json")
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, payload)
}

func handleListControlPlaneScenes(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListControlPlaneScenes(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handlePutControlPlaneScene(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.SceneConfig
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateControlPlaneScene(ctx, strings.TrimSpace(c.Param("id")), req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListControlPlaneSkills(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListControlPlaneSkills(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleListControlPlaneTools(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListControlPlaneTools(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handlePutControlPlaneSkill(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.SkillConfig
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateControlPlaneSkill(ctx, strings.TrimSpace(c.Param("name")), req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handlePutControlPlaneTool(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.ToolConfig
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateControlPlaneTool(ctx, strings.TrimSpace(c.Param("name")), req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetControlPlaneRuntime(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetControlPlaneRuntime(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetControlPlaneGovernance(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetControlPlaneGovernance(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handlePutControlPlaneRuntime(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.RuntimeTuning
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateControlPlaneRuntime(ctx, req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handlePutControlPlaneGovernance(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.GovernanceConfig
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateControlPlaneGovernance(ctx, req)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetToolGovernancePolicy(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.EffectiveToolGovernancePolicy(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListToolGovernanceDecisions(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListToolGovernanceDecisions(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleEvaluateToolGovernance(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.ToolGovernanceDecisionRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.EvaluateToolGovernance(ctx, req)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetValidationMCPServer(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.ValidationMCPServerInfo(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListValidationMCPTools(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListValidationMCPTools(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleInvokeValidationMCPTool(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req validationmcp.InvocationRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.InvokeValidationMCPTool(ctx, req)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListControlPlaneConfigVersions(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListControlPlaneConfigVersions(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleGetControlPlaneConfigVersion(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetControlPlaneConfigVersion(ctx, strings.TrimSpace(c.Param("versionID")))
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleRollbackControlPlaneConfigVersion(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.RollbackControlPlaneConfigVersion(ctx, strings.TrimSpace(c.Param("versionID")))
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListSystemResources(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListSystemResources(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleSyncSystemResources(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.SyncSystemResources(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleBuildSystemAssetsPackage(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	manifest, err := application.BuildSystemAssetsPackage(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, manifest)
}

func handleGetSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetSystemResource(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListSystemResourceVersions(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListSystemResourceVersions(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleGetSystemResourceVersion(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetSystemResourceVersion(ctx, strings.TrimSpace(c.Param("id")), strings.TrimSpace(c.Param("versionID")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleRollbackSystemResourceVersion(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.RollbackSystemResourceVersion(ctx, strings.TrimSpace(c.Param("id")), strings.TrimSpace(c.Param("versionID")))
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleListSystemResourceAudit(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	items, err := application.ListSystemResourceAudit(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"items": items})
}

func handleCreateSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.SystemResourceCreateRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.CreateSystemResource(ctx, req)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleDeleteSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	if err := application.DeleteSystemResource(ctx, strings.TrimSpace(c.Param("id"))); err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{"deleted": true, "asset_id": strings.TrimSpace(c.Param("id"))})
}

func handlePatchSystemResourceMetadata(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.SystemResourceMetadataPatch
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.PatchSystemResourceMetadata(ctx, strings.TrimSpace(c.Param("id")), req)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetSystemResourceSource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetSystemResourceSource(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handlePutSystemResourceSource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	var req controlplane.SystemResourceSource
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	item, err := application.UpdateSystemResourceSource(ctx, strings.TrimSpace(c.Param("id")), req)
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleParseSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.ParseSystemResource(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleCompileSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.CompileSystemResource(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleActivateSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.ActivateSystemResource(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetSystemResourcePipeline(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetSystemResourcePipeline(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetSystemResourceParseResult(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetSystemResourceParseResult(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetSystemResourceCompileResult(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.GetSystemResourceCompileResult(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleGetSystemResourceDebugPayload(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	item, err := application.BuildSystemResourceDebugPayload(ctx, strings.TrimSpace(c.Param("id")), strings.TrimSpace(c.Query("endpoint")))
	if err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, item)
}

func handleDownloadSystemResource(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	payload, filename, err := application.DownloadSystemResource(ctx, strings.TrimSpace(c.Param("id")))
	if err != nil {
		c.JSON(consts.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
	c.Data(consts.StatusOK, "text/markdown; charset=utf-8", payload)
}

func handleExportSystemResources(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
	applyControlPlaneCORS(c, cfg)
	if string(c.Method()) == http.MethodOptions {
		c.Status(consts.StatusNoContent)
		return
	}
	payload, export, err := application.ExportSystemResources(ctx)
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.Header("Content-Disposition", `attachment; filename="`+export.ExportFile+`"`)
	c.Header("X-Truth-Dir-Version", export.TruthDirVersion)
	c.Data(consts.StatusOK, "application/zip", payload)
}

func handleControlPlaneOptions(_ context.Context, c *hertzapp.RequestContext, cfg config.Config) {
	applyControlPlaneCORS(c, cfg)
	c.Status(consts.StatusNoContent)
}

func applyControlPlaneCORS(c *hertzapp.RequestContext, cfg config.Config) {
	origin := strings.TrimSpace(string(c.Request.Header.Peek("Origin")))
	if origin == "" || !originAllowed(origin, cfg.ControlPlane.AllowedOrigins) {
		return
	}
	c.Header("Access-Control-Allow-Origin", origin)
	c.Header("Vary", "Origin")
	c.Header("Access-Control-Allow-Credentials", "true")
	c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
	c.Header("Access-Control-Allow-Headers", "Content-Type,Authorization")
}

func originAllowed(origin string, allowed []string) bool {
	if origin == "" || len(allowed) == 0 {
		return false
	}
	for _, item := range allowed {
		if strings.TrimSpace(item) == "*" || strings.TrimSpace(item) == origin {
			return true
		}
	}
	return false
}
