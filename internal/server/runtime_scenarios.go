// runtime_scenarios.go exposes the HTTP transport entrypoints for runtime judgment and runtime skill listing.
// runtime_scenarios.go 负责暴露 runtime judgment 和 runtime skill 列表的 HTTP 传输入口。
package server

import (
	"context"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/runtimeassets"
)

// handleListRuntimeSkills returns runtime skill metadata filtered by source and task context.
// handleListRuntimeSkills 会按来源和任务上下文返回 runtime skill 元数据。
func handleListRuntimeSkills(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	items, err := application.ListRuntimeSkills(ctx, runtimeassets.SkillFilter{
		Source:              runtimeassets.SkillSource(strings.TrimSpace(c.Query("source"))),
		TaskType:            strings.TrimSpace(c.Query("task_type")),
		TaskSubtype:         strings.TrimSpace(c.Query("task_subtype")),
		RequestedOutputMode: strings.TrimSpace(c.Query("requested_output_mode")),
	})
	if err != nil {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, map[string]any{
		"items": items,
	})
}

// handleRuntimeScenarioRespond keeps the legacy scenario judgment adapter on an explicit compatibility route.
// handleRuntimeScenarioRespond 会在显式兼容路由上保留旧场景 runtime judgment 适配器。
func handleRuntimeScenarioRespond(ctx context.Context, c *hertzapp.RequestContext, application *appcore.Service) {
	var req appcore.RuntimeScenarioRequest
	if err := c.BindAndValidate(&req); err != nil {
		c.JSON(consts.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	requestID := newRequestID()
	response, err := application.AnalyzeRuntimeScenario(ctx, requestID, req)
	if err != nil {
		status := runtimeScenarioHTTPStatus(err)
		c.JSON(status, map[string]string{"error": err.Error()})
		return
	}
	c.JSON(consts.StatusOK, response)
}

func runtimeScenarioHTTPStatus(err error) int {
	if err == nil {
		return consts.StatusOK
	}
	var invalidSessionErr *appcore.InvalidSessionError
	if errors.As(err, &invalidSessionErr) {
		return consts.StatusNotFound
	}
	var invalidResumeErr *appcore.InvalidResumeTokenError
	if errors.As(err, &invalidResumeErr) {
		return consts.StatusBadRequest
	}
	var invalidTaskErr *appcore.InvalidTaskRequestError
	if errors.As(err, &invalidTaskErr) {
		switch strings.TrimSpace(invalidTaskErr.Reason) {
		case "runtime_assets_not_found":
			return consts.StatusNotFound
		default:
			return consts.StatusBadRequest
		}
	}
	return consts.StatusInternalServerError
}
