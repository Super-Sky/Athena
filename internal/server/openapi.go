// openapi.go builds and serves the repository OpenAPI document and Swagger UI payload.
// openapi.go 负责构建并输出仓库的 OpenAPI 文档和 Swagger UI 载荷。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	"moss/internal/config"
)

func handleSwaggerUI(cfg config.Config) hertzapp.HandlerFunc {
	specURL := "/swagger/openapi.json"
	cssURL := "/swagger/assets/swagger-ui.css"
	bundleURL := "/swagger/assets/swagger-ui-bundle.js"
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <title>Athena Swagger UI</title>
  <meta name="viewport" content="width=device-width, initial-scale=1" />
  <link rel="stylesheet" href=%q />
  <style>
    html, body { margin: 0; padding: 0; background: #faf7ef; }
    #swagger-ui { max-width: 1280px; margin: 0 auto; }
  </style>
</head>
<body>
  <div id="swagger-ui"></div>
  <script src=%q></script>
  <script>
    window.ui = SwaggerUIBundle({
      url: %q,
      dom_id: '#swagger-ui',
      deepLinking: true,
      presets: [SwaggerUIBundle.presets.apis],
      layout: "BaseLayout"
    });

    (function stabilizeSwaggerBodyEditors() {
      function syncControlledTextarea(textarea) {
        if (!textarea) {
          return;
        }
        const descriptor = Object.getOwnPropertyDescriptor(window.HTMLTextAreaElement.prototype, 'value');
        if (!descriptor || typeof descriptor.set !== 'function') {
          return;
        }
        const currentValue = textarea.value;
        descriptor.set.call(textarea, currentValue);
        textarea.dispatchEvent(new InputEvent('input', {
          bubbles: true,
          inputType: 'insertText',
          data: currentValue
        }));
        textarea.dispatchEvent(new Event('change', { bubbles: true }));
      }

      document.addEventListener('click', function onExecuteClick(event) {
        const trigger = event.target && event.target.closest('button');
        if (!trigger) {
          return;
        }
        if (trigger.textContent && trigger.textContent.trim() === 'Execute') {
          document.querySelectorAll('textarea.body-param__text').forEach(syncControlledTextarea);
        }
      }, true);
    })();
  </script>
</body>
</html>`, cssURL, bundleURL, specURL)

	return func(_ context.Context, c *hertzapp.RequestContext) {
		c.Data(consts.StatusOK, "text/html; charset=utf-8", []byte(html))
	}
}

func handleOpenAPISpec(cfg config.Config) hertzapp.HandlerFunc {
	spec := buildOpenAPISpec(cfg)
	return func(_ context.Context, c *hertzapp.RequestContext) {
		applyControlPlaneCORS(c, cfg)
		payload, err := json.MarshalIndent(spec, "", "  ")
		if err != nil {
			c.JSON(consts.StatusInternalServerError, map[string]string{
				"error": "failed to build openapi spec",
			})
			return
		}
		c.Data(consts.StatusOK, "application/json; charset=utf-8", payload)
	}
}

func buildOpenAPISpec(cfg config.Config) map[string]any {
	return map[string]any{
		"openapi": "3.0.3",
		"info": map[string]any{
			"title":       "Athena API",
			"version":     "v0.1.0",
			"description": "Athena 运行时、技能治理与模型治理接口。",
		},
		"servers": []map[string]any{{
			"url": fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.HTTPPort),
		}},
		"tags": []map[string]any{
			{"name": "health"},
			{"name": "sessions"},
			{"name": "chat"},
			{"name": "control-plane"},
			{"name": "runtime"},
			{"name": "skills"},
			{"name": "models"},
			{"name": "swagger"},
		},
		"paths": buildOpenAPIPaths(),
		"components": map[string]any{
			"schemas": buildOpenAPISchemas(),
		},
	}
}

func buildOpenAPIPaths() map[string]any {
	paths := map[string]any{
		"/healthz": map[string]any{
			"get": map[string]any{
				"tags":        []string{"health"},
				"summary":     "健康检查",
				"operationId": "healthz",
				"responses": map[string]any{
					"200": jsonResponse("健康检查响应", "HealthzResponse"),
				},
			},
		},
		"/swagger/openapi.json": map[string]any{
			"get": map[string]any{
				"tags":        []string{"swagger"},
				"summary":     "OpenAPI 文档",
				"operationId": "getOpenAPISpec",
				"responses": map[string]any{
					"200": map[string]any{"description": "OpenAPI 文档内容"},
				},
			},
		},
		"/api/sessions": map[string]any{
			"post": map[string]any{
				"tags":        []string{"sessions"},
				"summary":     "创建会话",
				"operationId": "createSession",
				"requestBody": jsonRequest("CreateSessionRequest", false),
				"responses": map[string]any{
					"201": jsonResponse("Session", "SessionResource"),
				},
			},
			"get": map[string]any{
				"tags":        []string{"sessions"},
				"summary":     "列出会话",
				"operationId": "listSessions",
				"responses": map[string]any{
					"200": jsonResponse("Session list", "SessionListResponse"),
				},
			},
		},
		"/api/sessions/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"sessions"},
				"summary":     "查看会话详情",
				"operationId": "getSession",
				"parameters":  pathIDParameter("id", "会话 ID"),
				"responses": map[string]any{
					"200": jsonResponse("Session", "SessionResource"),
					"404": jsonResponse("Session not found", "ErrorResponse"),
				},
			},
			"patch": map[string]any{
				"tags":        []string{"sessions"},
				"summary":     "更新会话标题",
				"operationId": "patchSession",
				"parameters":  pathIDParameter("id", "会话 ID"),
				"requestBody": jsonRequest("PatchSessionRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Session", "SessionResource"),
					"404": jsonResponse("Session not found", "ErrorResponse"),
				},
			},
		},
		"/api/sessions/{id}/archive": map[string]any{
			"post": map[string]any{
				"tags":        []string{"sessions"},
				"summary":     "归档会话",
				"operationId": "archiveSession",
				"parameters":  pathIDParameter("id", "会话 ID"),
				"responses": map[string]any{
					"200": jsonResponse("Session", "SessionResource"),
					"404": jsonResponse("Session not found", "ErrorResponse"),
				},
			},
		},
		"/api/chat/stream": map[string]any{
			"post": map[string]any{
				"tags":        []string{"chat"},
				"summary":     "通过 SSE 发起流式聊天",
				"operationId": "chatStream",
				"requestBody": jsonRequest("ChatStreamRequest", true),
				"responses": map[string]any{
					"200": map[string]any{
						"description": "SSE 事件流",
						"content": map[string]any{
							"text/event-stream": map[string]any{
								"schema": map[string]any{
									"type":    "string",
									"example": "event: message\ndata: {\"type\":\"done\"}\n\n",
								},
							},
						},
					},
					"400": jsonResponse("错误请求", "ErrorResponse"),
				},
			},
		},
		"/api/chat/respond": map[string]any{
			"post": map[string]any{
				"tags":        []string{"chat"},
				"summary":     "获取结构化聊天结果",
				"operationId": "chatRespond",
				"requestBody": jsonRequest("ChatRespondRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("结构化聊天结果", "ChatRespondResponse"),
					"400": jsonResponse("错误请求", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/auth/status": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取控制面认证状态",
				"operationId": "getControlPlaneAuthStatus",
				"responses": map[string]any{
					"200": jsonResponse("控制面认证状态", "ControlPlaneAuthStatus"),
					"423": jsonResponse("当前 IP 已被控制面登录锁定", "ControlPlaneAuthStatus"),
				},
			},
		},
		"/api/control-plane/login": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "登录控制面",
				"operationId": "loginControlPlane",
				"requestBody": jsonRequest("ControlPlaneLoginRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("控制面认证状态", "ControlPlaneAuthStatus"),
					"401": jsonResponse("控制面 token 无效", "ControlPlaneAuthStatus"),
					"423": jsonResponse("当前 IP 已被控制面登录锁定", "ControlPlaneAuthStatus"),
				},
			},
		},
		"/api/control-plane/logout": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "退出控制面",
				"operationId": "logoutControlPlane",
				"responses": map[string]any{
					"200": jsonResponse("控制面认证状态", "ControlPlaneAuthStatus"),
				},
			},
		},
		"/api/control-plane/bootstrap": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "获取控制面前端启动数据",
				"operationId": "getControlPlaneBootstrap",
				"responses": map[string]any{
					"200": jsonResponse("控制面启动数据", "ControlPlaneBootstrapResponse"),
				},
			},
		},
		"/api/control-plane/scenes": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出有效场景目录",
				"operationId": "listControlPlaneScenes",
				"responses": map[string]any{
					"200": jsonResponse("场景目录", "ControlPlaneSceneListResponse"),
				},
			},
		},
		"/api/control-plane/scenes/{id}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "更新一条场景定义",
				"operationId": "putControlPlaneScene",
				"parameters":  pathIDParameter("id", "场景 ID"),
				"requestBody": jsonRequest("ControlPlaneScene", true),
				"responses": map[string]any{
					"200": jsonResponse("场景定义", "ControlPlaneScene"),
				},
			},
		},
		"/api/control-plane/skills": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出有效 skill 目录",
				"operationId": "listControlPlaneSkills",
				"responses": map[string]any{
					"200": jsonResponse("Skill 目录", "ControlPlaneSkillListResponse"),
				},
			},
		},
		"/api/control-plane/skills/{name}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "更新一条 skill 定义",
				"operationId": "putControlPlaneSkill",
				"parameters":  pathIDParameter("name", "Skill 名称"),
				"requestBody": jsonRequest("ControlPlaneSkill", true),
				"responses": map[string]any{
					"200": jsonResponse("Skill 定义", "ControlPlaneSkill"),
				},
			},
		},
		"/api/control-plane/tools": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出有效 tool registry",
				"operationId": "listControlPlaneTools",
				"responses": map[string]any{
					"200": jsonResponse("tool registry", "ControlPlaneToolListResponse"),
				},
			},
		},
		"/api/control-plane/tools/{name}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "更新一条 tool 定义",
				"operationId": "putControlPlaneTool",
				"parameters":  pathIDParameter("name", "tool 名称"),
				"requestBody": jsonRequest("ControlPlaneTool", true),
				"responses": map[string]any{
					"200": jsonResponse("tool 定义", "ControlPlaneTool"),
				},
			},
		},
		"/api/control-plane/runtime-config": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取运行参数开关",
				"operationId": "getControlPlaneRuntimeConfig",
				"responses": map[string]any{
					"200": jsonResponse("运行参数", "ControlPlaneRuntimeConfig"),
				},
			},
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "更新运行参数开关",
				"operationId": "putControlPlaneRuntimeConfig",
				"requestBody": jsonRequest("ControlPlaneRuntimeConfig", true),
				"responses": map[string]any{
					"200": jsonResponse("运行参数", "ControlPlaneRuntimeConfig"),
				},
			},
		},
		"/api/control-plane/runtime/validation-runs": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "触发一次 runtime persistence 验证写入",
				"operationId": "createControlPlaneRuntimeValidationRun",
				"description": "通过 Eino runtime graph foundation 生成一组安全的 TaskRun、TaskStep、lifecycle、trace、usage 和 projection candidate 记录，供 System Validation 页面读回验收。",
				"requestBody": jsonRequest("RuntimeValidationRunRequest", false),
				"responses": map[string]any{
					"201": jsonResponse("runtime validation run", "RuntimeValidationRunResponse"),
					"503": jsonResponse("runtime persistence store unavailable", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出持久化 runtime run",
				"operationId": "listControlPlaneRuntimeRuns",
				"parameters": []map[string]any{
					queryStringParameter("workspace_id", "workspace 过滤条件。"),
					queryStringParameter("status", "runtime run 状态过滤条件。"),
					queryIntParameter("limit", "返回数量上限。"),
				},
				"responses": map[string]any{
					"200": jsonResponse("runtime run 列表", "RuntimeRunListResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs/{runID}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取单个持久化 runtime run",
				"operationId": "getControlPlaneRuntimeRun",
				"parameters":  pathIDParameter("runID", "runtime run ID"),
				"responses": map[string]any{
					"200": jsonResponse("runtime run", "RuntimeRun"),
					"404": jsonResponse("runtime run not found", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs/{runID}/steps": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 runtime run 下的 step",
				"operationId": "listControlPlaneRuntimeSteps",
				"parameters":  pathIDParameter("runID", "runtime run ID"),
				"responses": map[string]any{
					"200": jsonResponse("runtime step 列表", "RuntimeStepListResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs/{runID}/lifecycle": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 runtime run 生命周期事件",
				"operationId": "listControlPlaneRuntimeLifecycleEvents",
				"parameters":  pathIDParameter("runID", "runtime run ID"),
				"responses": map[string]any{
					"200": jsonResponse("runtime lifecycle event 列表", "RuntimeLifecycleEventListResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs/{runID}/traces": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 runtime run trace 摘要",
				"operationId": "listControlPlaneRuntimeTraces",
				"parameters": append(pathIDParameter("runID", "runtime run ID"),
					queryStringParameter("step_id", "可选 step 过滤条件。"),
					queryIntParameter("limit", "返回数量上限。"),
				),
				"responses": map[string]any{
					"200": jsonResponse("runtime trace 列表", "RuntimeTraceListResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs/{runID}/usage": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 runtime run 通用资源用量",
				"operationId": "listControlPlaneRuntimeUsage",
				"parameters": append(pathIDParameter("runID", "runtime run ID"),
					queryStringParameter("step_id", "可选 step 过滤条件。"),
					queryIntParameter("limit", "返回数量上限。"),
				),
				"responses": map[string]any{
					"200": jsonResponse("runtime usage 列表", "RuntimeUsageListResponse"),
				},
			},
		},
		"/api/control-plane/runtime/runs/{runID}/projections": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 runtime run 候选输出投影",
				"operationId": "listControlPlaneRuntimeProjectionCandidates",
				"parameters": append(pathIDParameter("runID", "runtime run ID"),
					queryStringParameter("step_id", "可选 step 过滤条件。"),
					queryIntParameter("limit", "返回数量上限。"),
				),
				"responses": map[string]any{
					"200": jsonResponse("runtime projection candidate 列表", "RuntimeProjectionCandidateListResponse"),
				},
			},
		},
		"/api/control-plane/runtime/contracts/foundation": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取 v2.1 runtime contract foundation",
				"operationId": "getControlPlaneRuntimeContractFoundation",
				"description": "读取 RuntimeContract、TaskTypeRegistry、HookBinding 和 System Truth active pointer 的当前 foundation 快照。",
				"responses": map[string]any{
					"200": jsonResponse("runtime contract foundation", "RuntimeContractFoundationResponse"),
					"503": jsonResponse("runtime persistence store unavailable", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/runtime/contracts/{contractID}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "创建或更新一个 runtime contract",
				"operationId": "putControlPlaneRuntimeContract",
				"parameters":  pathIDParameter("contractID", "runtime contract ID"),
				"requestBody": jsonRequest("RuntimeContractUpsertRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("runtime contract", "RuntimeContract"),
					"400": jsonResponse("invalid runtime contract payload", "ErrorResponse"),
					"503": jsonResponse("runtime persistence store unavailable", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/runtime/task-types/{typeKey}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "创建或更新一个 task type registration",
				"operationId": "putControlPlaneRuntimeTaskType",
				"parameters":  pathIDParameter("typeKey", "runtime task type key"),
				"requestBody": jsonRequest("RuntimeTaskTypeUpsertRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("runtime task type registration", "RuntimeTaskTypeRegistration"),
					"400": jsonResponse("invalid runtime task type payload", "ErrorResponse"),
					"503": jsonResponse("runtime persistence store unavailable", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/runtime/hook-bindings/{bindingID}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "创建或更新一个 hook binding",
				"operationId": "putControlPlaneRuntimeHookBinding",
				"parameters":  pathIDParameter("bindingID", "runtime hook binding ID"),
				"requestBody": jsonRequest("RuntimeHookBindingUpsertRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("runtime hook binding", "RuntimeHookBinding"),
					"400": jsonResponse("invalid runtime hook binding payload", "ErrorResponse"),
					"503": jsonResponse("runtime persistence store unavailable", "ErrorResponse"),
				},
			},
		},
		"/api/control-plane/governance": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取治理策略配置",
				"operationId": "getControlPlaneGovernance",
				"responses": map[string]any{
					"200": jsonResponse("治理策略", "ControlPlaneGovernanceConfig"),
				},
			},
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "更新治理策略配置",
				"operationId": "putControlPlaneGovernance",
				"requestBody": jsonRequest("ControlPlaneGovernanceConfig", true),
				"responses": map[string]any{
					"200": jsonResponse("治理策略", "ControlPlaneGovernanceConfig"),
				},
			},
		},
		"/api/control-plane/tool-governance/policy": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取有效 tool governance policy",
				"operationId": "getToolGovernancePolicy",
				"responses": map[string]any{
					"200": jsonResponse("有效 tool governance policy", "ToolGovernancePolicy"),
				},
			},
		},
		"/api/control-plane/tool-governance/decisions": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 tool governance 决策日志",
				"operationId": "listToolGovernanceDecisions",
				"responses": map[string]any{
					"200": jsonResponse("tool governance 决策列表", "ToolGovernanceDecisionListResponse"),
				},
			},
		},
		"/api/control-plane/tool-governance/evaluate": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "判定并持久化一次 tool governance 决策",
				"operationId": "evaluateToolGovernance",
				"requestBody": jsonRequest("ToolGovernanceDecisionRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("tool governance 决策", "ToolGovernanceDecision"),
				},
			},
		},
		"/api/control-plane/validation-mcp/server": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取 validation MCP server 描述",
				"operationId": "getValidationMCPServer",
				"responses": map[string]any{
					"200": jsonResponse("validation MCP server", "ValidationMCPServerInfo"),
				},
			},
		},
		"/api/control-plane/validation-mcp/tools": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出 validation MCP tool schemas",
				"operationId": "listValidationMCPTools",
				"responses": map[string]any{
					"200": jsonResponse("validation MCP tool schema 列表", "ValidationMCPToolListResponse"),
				},
			},
		},
		"/api/control-plane/validation-mcp/invocations": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "调用 validation MCP tool 并展示治理决策",
				"operationId": "invokeValidationMCPTool",
				"requestBody": jsonRequest("ValidationMCPInvocationRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("validation MCP 调用结果", "ValidationMCPInvocationResponse"),
				},
			},
		},
		"/api/control-plane/config-versions": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出控制面配置版本",
				"operationId": "listControlPlaneConfigVersions",
				"responses": map[string]any{
					"200": jsonResponse("配置版本列表", "ControlPlaneConfigVersionListResponse"),
				},
			},
		},
		"/api/control-plane/config-versions/{versionID}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看单个控制面配置版本",
				"operationId": "getControlPlaneConfigVersion",
				"parameters":  pathIDParameter("versionID", "配置版本 ID"),
				"responses": map[string]any{
					"200": jsonResponse("配置版本详情", "ControlPlaneConfigVersionDetail"),
				},
			},
		},
		"/api/control-plane/config-versions/{versionID}/rollback": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "回滚到指定控制面配置版本",
				"operationId": "rollbackControlPlaneConfigVersion",
				"parameters":  pathIDParameter("versionID", "配置版本 ID"),
				"responses": map[string]any{
					"200": jsonResponse("回滚后的配置版本", "ControlPlaneConfigVersionDetail"),
				},
			},
		},
		"/api/runtime/skills": map[string]any{
			"get": map[string]any{
				"tags":        []string{"runtime"},
				"summary":     "列出当前任务可见的 runtime skills",
				"operationId": "listRuntimeSkills",
				"parameters": []map[string]any{
					queryStringParameter("source", "skill 来源过滤条件。"),
					queryStringParameter("task_type", "任务主类型。"),
					queryStringParameter("task_subtype", "任务子类型。"),
					queryStringParameter("requested_output_mode", "输出契约模式。"),
				},
				"responses": map[string]any{
					"200": jsonResponse("Runtime skill list", "RuntimeSkillListResponse"),
				},
			},
		},
		"/api/runtime/respond": map[string]any{
			"post": map[string]any{
				"tags":        []string{"runtime"},
				"summary":     "通过通用 runtime 路径执行直接响应",
				"description": "Generic direct respond adapter over the common app/runtime path. Legacy scenario-shaped task_type values remain accepted by the shared task normalizer as compatibility hints and are not core-native semantics.",
				"operationId": "runtimeRespond",
				"requestBody": jsonRequest("ChatRespondRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Runtime direct response", "ChatRespondResponse"),
					"400": jsonResponse("错误请求", "ErrorResponse"),
					"404": jsonResponse("未找到相关模型或会话", "ErrorResponse"),
				},
			},
		},
		"/api/runtime/scenario/respond": map[string]any{
			"post": map[string]any{
				"tags":        []string{"runtime"},
				"summary":     "执行兼容场景 runtime judgment",
				"description": "Compatibility route for legacy RuntimeScenarioRequest / RuntimeScenarioResponse judgment flows such as OpenClaw validation packages. It is not the core direct respond adapter.",
				"operationId": "runtimeScenarioRespond",
				"requestBody": jsonRequest("RuntimeScenarioRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Runtime scenario response", "RuntimeScenarioResponse"),
					"400": jsonResponse("错误请求", "ErrorResponse"),
					"404": jsonResponse("未找到相关模型或会话", "ErrorResponse"),
				},
			},
		},
		"/api/skills": map[string]any{
			"get": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "列出当前可见技能",
				"operationId": "listSkills",
				"responses": map[string]any{
					"200": jsonResponse("可见技能列表", "SkillListResponse"),
				},
			},
		},
		"/api/skills/packages": map[string]any{
			"get": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "列出已上传技能包",
				"operationId": "listSkillPackages",
				"responses": map[string]any{
					"200": jsonResponse("Uploaded packages", "SkillPackageListResponse"),
				},
			},
			"post": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "上传技能包",
				"operationId": "uploadSkillPackage",
				"requestBody": skillPackageRequestBody(),
				"responses": map[string]any{
					"201": jsonResponse("Uploaded package", "SkillPackageResponse"),
				},
			},
		},
		"/api/skills/packages/{id}/revisions": map[string]any{
			"get": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "列出技能包版本历史",
				"operationId": "listSkillPackageRevisions",
				"parameters":  pathIDParameter("id", "Package ID"),
				"responses": map[string]any{
					"200": jsonResponse("Package revisions", "SkillPackageListResponse"),
				},
			},
		},
		"/api/skills/packages/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "查看技能包详情",
				"operationId": "getSkillPackage",
				"parameters":  pathIDParameter("id", "Package ID"),
				"responses": map[string]any{
					"200": jsonResponse("Package detail", "SkillPackageDetailResponse"),
					"404": jsonResponse("Package not found", "ErrorResponse"),
				},
			},
			"put": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "替换技能包",
				"operationId": "replaceSkillPackage",
				"parameters":  pathIDParameter("id", "Package ID"),
				"requestBody": skillPackageRequestBody(),
				"responses": map[string]any{
					"200": jsonResponse("Updated package", "SkillPackageResponse"),
				},
			},
			"patch": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "更新技能包状态",
				"operationId": "patchSkillPackage",
				"parameters":  pathIDParameter("id", "Package ID"),
				"requestBody": jsonRequest("UpdateSkillPackageStateRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Updated package", "SkillPackageResponse"),
				},
			},
			"delete": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "删除技能包",
				"operationId": "deleteSkillPackage",
				"parameters":  pathIDParameter("id", "Package ID"),
				"responses": map[string]any{
					"200": jsonResponse("Delete result", "DeleteSkillPackageResponse"),
				},
			},
		},
		"/api/skills/packages/{id}/rollback": map[string]any{
			"post": map[string]any{
				"tags":        []string{"skills"},
				"summary":     "回滚技能包版本",
				"operationId": "rollbackSkillPackage",
				"parameters":  pathIDParameter("id", "Package ID"),
				"requestBody": jsonRequest("RollbackSkillPackageRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Rollback result", "SkillPackageRollbackResponse"),
				},
			},
		},
		"/api/models/providers": map[string]any{
			"get": map[string]any{
				"tags":        []string{"models"},
				"summary":     "列出模型供应商",
				"operationId": "listModelProviders",
				"responses": map[string]any{
					"200": jsonResponse("Provider list", "ModelProviderListResponse"),
				},
			},
			"post": map[string]any{
				"tags":        []string{"models"},
				"summary":     "创建模型供应商",
				"description": "创建一个供应商，并可在同一次请求中可选地一并创建其子模型。返回结果中的每个模型记录都可以继续通过 POST /api/models/providers/{id}/models/{record_id}/test 校验是否真实可用。",
				"operationId": "createModelProvider",
				"requestBody": jsonRequest("CreateModelProviderRequest", true),
				"responses": map[string]any{
					"201": jsonResponse("Provider", "ModelProviderResponse"),
				},
			},
		},
		"/api/models/providers/{id}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"models"},
				"summary":     "更新模型供应商",
				"description": "仅替换供应商级元数据；子模型仍通过专用的 /models 接口单独治理。",
				"operationId": "updateModelProvider",
				"parameters":  pathIDParameter("id", "Provider ID"),
				"requestBody": jsonRequest("UpsertModelProviderRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Provider", "ModelProviderResponse"),
				},
			},
			"patch": map[string]any{
				"tags":        []string{"models"},
				"summary":     "局部更新模型供应商",
				"operationId": "patchModelProvider",
				"parameters":  pathIDParameter("id", "Provider ID"),
				"requestBody": jsonRequest("PatchModelProviderRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Provider", "ModelProviderResponse"),
				},
			},
			"delete": map[string]any{
				"tags":        []string{"models"},
				"summary":     "删除模型供应商",
				"operationId": "deleteModelProvider",
				"parameters":  pathIDParameter("id", "Provider ID"),
				"responses": map[string]any{
					"200": jsonResponse("Delete result", "StatusResponse"),
					"404": jsonResponse("Provider not found", "ErrorResponse"),
				},
			},
		},
		"/api/models/providers/{id}/models": map[string]any{
			"post": map[string]any{
				"tags":        []string{"models"},
				"summary":     "创建供应商模型",
				"operationId": "createProviderModel",
				"parameters":  pathIDParameter("id", "Provider ID"),
				"requestBody": jsonRequest("UpsertProviderModelRequest", true),
				"responses": map[string]any{
					"201": jsonResponse("Provider model", "ProviderModelResponse"),
				},
			},
		},
		"/api/models/providers/{id}/models/{record_id}": map[string]any{
			"put": map[string]any{
				"tags":        []string{"models"},
				"summary":     "更新供应商模型",
				"operationId": "updateProviderModel",
				"parameters":  append(pathIDParameter("id", "供应商 ID"), map[string]any{"name": "record_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}, "description": "模型记录 ID"}),
				"requestBody": jsonRequest("UpsertProviderModelRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Provider model", "ProviderModelResponse"),
				},
			},
			"patch": map[string]any{
				"tags":        []string{"models"},
				"summary":     "局部更新供应商模型",
				"operationId": "patchProviderModel",
				"parameters":  append(pathIDParameter("id", "供应商 ID"), map[string]any{"name": "record_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}, "description": "模型记录 ID"}),
				"requestBody": jsonRequest("PatchProviderModelRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("Provider model", "ProviderModelResponse"),
				},
			},
			"delete": map[string]any{
				"tags":        []string{"models"},
				"summary":     "删除供应商模型",
				"operationId": "deleteProviderModel",
				"parameters":  append(pathIDParameter("id", "供应商 ID"), map[string]any{"name": "record_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}, "description": "模型记录 ID"}),
				"responses": map[string]any{
					"200": jsonResponse("Delete result", "StatusResponse"),
					"404": jsonResponse("Provider model not found", "ErrorResponse"),
				},
			},
		},
		"/api/models/providers/{id}/models/{record_id}/test": map[string]any{
			"post": map[string]any{
				"tags":        []string{"models"},
				"summary":     "测试供应商模型可用性",
				"operationId": "testProviderModel",
				"parameters":  append(pathIDParameter("id", "供应商 ID"), map[string]any{"name": "record_id", "in": "path", "required": true, "schema": map[string]any{"type": "string"}, "description": "模型记录 ID"}),
				"responses": map[string]any{
					"200": jsonResponse("Test result", "ModelTestResponse"),
				},
			},
		},
	}
	for path, item := range buildSystemResourceOpenAPIPaths() {
		paths[path] = item
	}
	return paths
}

func buildOpenAPISchemas() map[string]any {
	schemas := map[string]any{
		"HealthzResponse": objectSchema(map[string]any{
			"status": map[string]any{"type": "string", "example": "ok"},
		}, []string{"status"}),
		"ErrorResponse": objectSchema(map[string]any{
			"error": map[string]any{"type": "string"},
		}, []string{"error"}),
		"ResumeContext": objectSchema(map[string]any{
			"stage":        map[string]any{"type": "string"},
			"resume_token": map[string]any{"type": "string"},
		}, nil),
		"CreateSessionRequest": objectSchema(map[string]any{
			"title": stringSchema("可选的会话标题。", "风险分析会话"),
		}, nil),
		"PatchSessionRequest": objectSchema(map[string]any{
			"title": stringSchema("新的会话标题。", "新的会话标题"),
		}, []string{"title"}),
		"SessionResource": objectSchema(map[string]any{
			"id":             stringSchema("会话资源 ID。", "sess_019d7286-ab7b-75eb-b043-d686a4a92851"),
			"title":          stringSchema("会话标题。", "风险分析会话"),
			"status":         map[string]any{"type": "string", "enum": []string{"active", "pending_wait", "archived"}, "description": "会话资源当前状态。", "example": "active"},
			"archived":       boolSchema("该会话当前是否已归档。", false),
			"pending_wait":   boolSchema("该会话当前是否存在活跃等待缺口。", false),
			"last_active_at": dateTimeSchema("最近一次活跃时间，RFC3339 格式。", "2026-04-09T09:35:36Z"),
			"created_at":     dateTimeSchema("创建时间，RFC3339 格式。", "2026-04-09T09:35:36Z"),
			"updated_at":     dateTimeSchema("最近更新时间，RFC3339 格式。", "2026-04-09T09:35:36Z"),
		}, []string{"id", "status", "archived", "pending_wait", "last_active_at", "created_at", "updated_at"}),
		"SessionListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("SessionResource")),
		}, []string{"items"}),
		"SupplementPayload": objectSchema(map[string]any{
			"data": map[string]any{
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
			},
			"outcome": map[string]any{
				"type": "string",
				"enum": []string{"provided", "unable_to_provide", "timeout_expired", "abandon_and_continue", "pending_human"},
			},
			"resume": refSchema("ResumeContext"),
		}, nil),
		"ChatStreamRequest": objectSchema(map[string]any{
			"task_type":                map[string]any{"type": "string", "description": "Generic runtime task type. Existing values chat, inspection_task, integration_event, scheduled_job, and workflow_step_request are accepted; scenario-shaped values are legacy-compatible or future registered semantics, not core-native task types."},
			"task_subtype":             map[string]any{"type": "string"},
			"scene":                    map[string]any{"type": "string"},
			"query":                    map[string]any{"type": "string"},
			"session_id":               map[string]any{"type": "string"},
			"main_session_id":          map[string]any{"type": "string"},
			"workspace_id":             map[string]any{"type": "string"},
			"app_instance_id":          map[string]any{"type": "string"},
			"app_session_id":           map[string]any{"type": "string"},
			"integration_instance_id":  map[string]any{"type": "string"},
			"workflow_run_id":          map[string]any{"type": "string"},
			"step_id":                  map[string]any{"type": "string"},
			"trigger_type":             map[string]any{"type": "string"},
			"automation_task_id":       map[string]any{"type": "string"},
			"user_language":            map[string]any{"type": "string"},
			"desired_output_mode":      map[string]any{"type": "string"},
			"global_context":           map[string]any{"type": "object", "additionalProperties": true},
			"app_context":              map[string]any{"type": "object", "additionalProperties": true},
			"input_payload":            map[string]any{"type": "object", "additionalProperties": true},
			"model_id":                 map[string]any{"type": "string"},
			"enabled_skills":           arraySchema(map[string]any{"type": "string"}),
			"enabled_tools":            arraySchema(map[string]any{"type": "string"}),
			"prompt_template":          map[string]any{"type": "string"},
			"context_asset_overrides":  arraySchema(map[string]any{"type": "object", "additionalProperties": true}),
			"disabled_asset_types":     arraySchema(map[string]any{"type": "string"}),
			"asset_priority_overrides": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "format": "int32"}},
			"supplement":               refSchema("SupplementPayload"),
			"supplement_outcome":       map[string]any{"type": "string"},
			"timeout_after_seconds":    map[string]any{"type": "integer", "format": "int32"},
			"resume_token":             map[string]any{"type": "string"},
			"disable_fast_path":        map[string]any{"type": "boolean"},
		}, nil),
		"ChatRespondRequest": objectSchema(map[string]any{
			"task_type":                map[string]any{"type": "string", "description": "Generic runtime task type. Existing values chat, inspection_task, integration_event, scheduled_job, and workflow_step_request are accepted; scenario-shaped values are legacy-compatible or future registered semantics, not core-native task types."},
			"task_subtype":             map[string]any{"type": "string"},
			"scene":                    map[string]any{"type": "string"},
			"query":                    map[string]any{"type": "string"},
			"session_id":               map[string]any{"type": "string"},
			"main_session_id":          map[string]any{"type": "string"},
			"workspace_id":             map[string]any{"type": "string"},
			"app_instance_id":          map[string]any{"type": "string"},
			"app_session_id":           map[string]any{"type": "string"},
			"integration_instance_id":  map[string]any{"type": "string"},
			"workflow_run_id":          map[string]any{"type": "string"},
			"step_id":                  map[string]any{"type": "string"},
			"trigger_type":             map[string]any{"type": "string"},
			"automation_task_id":       map[string]any{"type": "string"},
			"user_language":            map[string]any{"type": "string"},
			"desired_output_mode":      map[string]any{"type": "string"},
			"global_context":           map[string]any{"type": "object", "additionalProperties": true},
			"app_context":              map[string]any{"type": "object", "additionalProperties": true},
			"input_payload":            map[string]any{"type": "object", "additionalProperties": true},
			"model_id":                 map[string]any{"type": "string"},
			"enabled_skills":           arraySchema(map[string]any{"type": "string"}),
			"enabled_tools":            arraySchema(map[string]any{"type": "string"}),
			"prompt_template":          map[string]any{"type": "string"},
			"context_asset_overrides":  arraySchema(map[string]any{"type": "object", "additionalProperties": true}),
			"disabled_asset_types":     arraySchema(map[string]any{"type": "string"}),
			"asset_priority_overrides": map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "integer", "format": "int32"}},
			"supplement":               refSchema("SupplementPayload"),
			"supplement_outcome":       map[string]any{"type": "string"},
			"timeout_after_seconds":    map[string]any{"type": "integer", "format": "int32"},
			"resume_token":             map[string]any{"type": "string"},
			"disable_fast_path":        map[string]any{"type": "boolean"},
			"strict_schema_validation": map[string]any{"type": "boolean"},
			"schema_retry_count":       map[string]any{"type": "integer", "format": "int32"},
			"schema_repair_mode":       map[string]any{"type": "string", "enum": []string{"off", "basic"}},
			"schema_failure_action":    map[string]any{"type": "string", "enum": []string{"error", "partial"}},
		}, nil),
		"StructuredChatResult": objectSchema(map[string]any{
			"main_answer": map[string]any{"type": "string"},
			"structured_result": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"result_summary": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"content_cards": arraySchema(map[string]any{"type": "object"}),
			"right_panel_view": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
			"next_questions":        arraySchema(map[string]any{"type": "string"}),
			"score_delta":           map[string]any{"type": "object", "additionalProperties": true},
			"delivery_profile":      map[string]any{"type": "object", "additionalProperties": true},
			"answer":                map[string]any{"type": "string"},
			"follow_up_suggestions": arraySchema(map[string]any{"type": "string"}),
			"verdict":               map[string]any{"type": "string"},
			"decision":              map[string]any{"type": "string"},
			"reason":                map[string]any{"type": "string"},
		}, nil),
		"SchemaValidationReport": objectSchema(map[string]any{
			"strict":                map[string]any{"type": "boolean"},
			"repair_mode":           map[string]any{"type": "string"},
			"retry_count":           map[string]any{"type": "integer", "format": "int32"},
			"retries_used":          map[string]any{"type": "integer", "format": "int32"},
			"repair_attempted":      map[string]any{"type": "boolean"},
			"repair_succeeded":      map[string]any{"type": "boolean"},
			"regex_fallback_used":   map[string]any{"type": "boolean"},
			"valid":                 map[string]any{"type": "boolean"},
			"failure_stage":         map[string]any{"type": "string"},
			"last_validation_error": map[string]any{"type": "string"},
		}, []string{"strict", "valid"}),
		"ChatRespondResponse": objectSchema(map[string]any{
			"request_id":        map[string]any{"type": "string"},
			"session_id":        map[string]any{"type": "string"},
			"status":            map[string]any{"type": "string"},
			"result":            refSchema("StructuredChatResult"),
			"action_type":       map[string]any{"type": "string"},
			"action":            map[string]any{"type": "object"},
			"wait_state":        map[string]any{"type": "object"},
			"error":             map[string]any{"type": "string"},
			"error_detail":      map[string]any{"type": "object"},
			"structured_output": map[string]any{"type": "object"},
			"schema_validation": refSchema("SchemaValidationReport"),
			"detail": map[string]any{
				"type":                 "object",
				"additionalProperties": true,
			},
		}, []string{"request_id", "session_id", "status", "schema_validation"}),
		"RuntimeScenarioRequest": objectSchema(map[string]any{
			"task_type":             stringSchema("通用 runtime task type；场景化值仅为兼容或未来注册式语义。", "chat"),
			"task_subtype":          stringSchema("可选任务子类型。", "direct_response"),
			"requested_output_mode": arraySchema(map[string]any{"type": "string"}),
			"session_id":            stringSchema("Athena canonical session 标识。", "sess_019d7286-ab7b-75eb-b043-d686a4a92851"),
			"correlation_id":        stringSchema("客户端业务关联标识，不等同于 canonical session_id。", "mosi-risk-123"),
			"trace_id":              stringSchema("轻量 trace 标识。", "trace_123"),
			"connector_id":          stringSchema("宿主连接器标识。", "connector_123"),
			"host_type":             stringSchema("宿主类型。", "openclaw"),
			"hook_name":             stringSchema("宿主控制点名称。", "before_tool_call"),
			"event_type":            stringSchema("宿主事件类型。", "runtime_event"),
			"occurred_at":           stringSchema("宿主事件时间，RFC3339 或调用方自有时间格式。", "2026-04-10T10:00:00Z"),
			"model_id":              stringSchema("可选的显式模型记录 ID。", "modelrec_123"),
			"allow_user_supplement": boolSchema("是否允许把缺失信息升级成用户补问。", false),
			"available_skill_ids":   arraySchema(map[string]any{"type": "string"}),
			"raw_payload":           map[string]any{"type": "object", "additionalProperties": true},
			"normalized_context":    map[string]any{"type": "object", "additionalProperties": true},
			"evidence_supplement":   map[string]any{"type": "object", "additionalProperties": true},
			"judgment_context":      map[string]any{"type": "object", "additionalProperties": true},
			"resume_token":          stringSchema("evidence supplement 恢复当前等待缺口时使用的 token。", "resume_123"),
		}, nil),
		"RuntimeSuggestedAction": objectSchema(map[string]any{
			"skill_id":         stringSchema("建议触发的 runtime skill 标识。", "skill_mosi_email_sender_v1"),
			"execution_target": map[string]any{"type": "string", "enum": []string{"athena", "client"}},
			"operation":        stringSchema("业务动作名。", "send_email"),
			"arguments":        map[string]any{"type": "object", "additionalProperties": true},
		}, nil),
		"RuntimeHostProjection": objectSchema(map[string]any{
			"hook_action_code":  stringSchema("宿主侧动作投影代码。", "require_approval"),
			"final_decision":    stringSchema("投影后的 allow/ask/deny judgement。", "ask"),
			"user_visible_copy": stringSchema("宿主展示给最终用户的文案。", "Athena detected a runtime action that requires explicit approval before continuing."),
		}, nil),
		"RuntimeEvidenceRequest": objectSchema(map[string]any{
			"resume_token":       stringSchema("继续 evidence supplement 的恢复 token。", "resume_123"),
			"kind":               stringSchema("补全过程类型。", "evidence_supplement"),
			"missing_evidence":   arraySchema(map[string]any{"type": "string"}),
			"allowed_client_ops": arraySchema(map[string]any{"type": "string"}),
		}, nil),
		"RuntimeScenarioResponse": objectSchema(map[string]any{
			"request_id":            stringSchema("当前请求的稳定请求 ID。", "req_123"),
			"session_id":            stringSchema("Athena canonical session 标识。", "sess_019d7286-ab7b-75eb-b043-d686a4a92851"),
			"trace_id":              stringSchema("轻量 trace 标识。", "trace_123"),
			"correlation_id":        stringSchema("客户端业务关联标识。", "mosi-risk-123"),
			"task_type":             stringSchema("通用 runtime task type；场景化值仅为兼容或未来注册式语义。", "chat"),
			"task_subtype":          stringSchema("可选任务子类型。", "direct_response"),
			"requested_output_mode": arraySchema(map[string]any{"type": "string"}),
			"status":                stringSchema("当前 runtime judgment 状态。", "completed"),
			"decision":              stringSchema("Athena canonical machine-readable judgment。", "allow"),
			"decision_reason":       stringSchema("machine-readable decision reason。", "the current runtime event does not show a confirmed high-risk action under the current bounded evidence"),
			"user_visible_copy":     stringSchema("宿主或业务壳可直接展示给用户的文案。", "Athena found no confirmed high-risk signal for this runtime step."),
			"audit_summary":         stringSchema("供 audit / explanation 使用的稳定摘要。", "Runtime event allowed because the current evidence is closer to benign maintenance or low-risk execution."),
			"recommended_next_step": stringSchema("后续建议动作。", "continue execution and keep the audit trail for downstream review if needed"),
			"host_projection":       refSchema("RuntimeHostProjection"),
			"evidence_request":      refSchema("RuntimeEvidenceRequest"),
			"suggested_actions":     arraySchema(refSchema("RuntimeSuggestedAction")),
			"detail":                map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"request_id", "session_id", "task_type", "task_subtype", "requested_output_mode", "status"}),
		"RuntimeSkillMetadata": objectSchema(map[string]any{
			"id":                    stringSchema("runtime skill 的稳定标识。", "skill_mosi_audit_operator_v1"),
			"name":                  stringSchema("runtime skill 名称。", "audit-operator"),
			"description":           stringSchema("对外可见的最小用途说明。", "Query audit events and mark selected events as resolved through mosi's own services."),
			"source":                map[string]any{"type": "string", "enum": []string{"builtin", "product_managed", "client_managed"}},
			"product":               stringSchema("所属产品线。", "mosi"),
			"owner":                 stringSchema("拥有方。", "mosi"),
			"version":               stringSchema("稳定版本标识。", "v1"),
			"status":                stringSchema("当前 skill 状态。", "active"),
			"execution_target":      map[string]any{"type": "string", "enum": []string{"athena", "client"}},
			"allowed_task_types":    arraySchema(map[string]any{"type": "string"}),
			"allowed_task_subtypes": arraySchema(map[string]any{"type": "string"}),
			"allowed_output_modes":  arraySchema(map[string]any{"type": "string"}),
			"user_visible":          boolSchema("该 skill 是否应对用户直接可见。", true),
		}, []string{"id", "name", "source", "execution_target", "user_visible"}),
		"RuntimeSkillListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeSkillMetadata")),
		}, []string{"items"}),
		"RuntimeRun": objectSchema(map[string]any{
			"id":                stringSchema("runtime run ID。", "run_20260507_001"),
			"task_id":           stringSchema("runtime task ID。", "task_20260507_001"),
			"task_type":         stringSchema("任务主类型。", "runtime_validation"),
			"task_subtype":      stringSchema("任务子类型。", "graph_smoke"),
			"input_kind":        stringSchema("输入类型。", "chat"),
			"scene":             stringSchema("场景标识。", "system_validation"),
			"workspace_id":      stringSchema("workspace ID。", "ws_001"),
			"app_instance_id":   stringSchema("应用实例 ID。", "app_001"),
			"status":            stringSchema("runtime run 状态。", "completed"),
			"idempotency_scope": stringSchema("幂等范围。", "request"),
			"idempotency_key":   stringSchema("幂等键。", "req_001"),
			"retention_policy":  stringSchema("保留策略。", "default"),
			"metadata":          map[string]any{"type": "object", "additionalProperties": true},
			"created_at":        dateTimeSchema("创建时间。", "2026-05-07T10:00:00Z"),
			"updated_at":        dateTimeSchema("最近更新时间。", "2026-05-07T10:00:01Z"),
			"started_at":        dateTimeSchema("开始时间。", "2026-05-07T10:00:00Z"),
			"completed_at":      dateTimeSchema("完成时间。", "2026-05-07T10:00:01Z"),
		}, []string{"id", "task_id", "status", "created_at", "updated_at"}),
		"RuntimeRunListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeRun")),
		}, []string{"items"}),
		"RuntimeValidationRunRequest": objectSchema(map[string]any{
			"workspace_id": stringSchema("验证写入使用的 workspace 过滤值。", "system-validation"),
			"scene":        stringSchema("验证写入使用的 runtime scene。", "system_validation"),
			"prompt":       stringSchema("只用于本次 graph 验证输入，持久化 metadata 只记录长度等安全摘要。", "validate runtime persistence"),
			"metadata":     map[string]any{"type": "object", "additionalProperties": true},
		}, nil),
		"RuntimeValidationRunResponse": objectSchema(map[string]any{
			"run":                       refSchema("RuntimeRun"),
			"step":                      refSchema("RuntimeStep"),
			"events":                    arraySchema(refSchema("RuntimeLifecycleEvent")),
			"trace":                     refSchema("RuntimeTrace"),
			"usage":                     refSchema("RuntimeUsage"),
			"projection":                refSchema("RuntimeProjectionCandidate"),
			"validation_mcp":            refSchema("ValidationMCPInvocationResponse"),
			"validation_mcp_trace":      refSchema("RuntimeTrace"),
			"validation_mcp_usage":      refSchema("RuntimeUsage"),
			"validation_mcp_projection": refSchema("RuntimeProjectionCandidate"),
			"sandbox":                   refSchema("ExternalSandboxValidationResult"),
			"sandbox_event":             refSchema("RuntimeLifecycleEvent"),
			"sandbox_trace":             refSchema("RuntimeTrace"),
			"sandbox_usage":             refSchema("RuntimeUsage"),
			"sandbox_projection":        refSchema("RuntimeProjectionCandidate"),
		}, []string{"run", "step", "events", "trace", "usage", "projection", "validation_mcp", "sandbox"}),
		"ExternalSandboxRef": objectSchema(map[string]any{
			"ref_id":    stringSchema("external sandbox 引用 ID。", "external_sandbox_ref.runtime-validation-123"),
			"mode":      stringSchema("sandbox 边界模式。", "external_sandbox_ref"),
			"provider":  stringSchema("sandbox provider 标签。", "athena-validation"),
			"boundary":  stringSchema("边界名称。", "validation_control_plane"),
			"status":    stringSchema("边界状态。", "completed"),
			"operation": stringSchema("操作类型。", "validate"),
			"resource":  stringSchema("资源标签。", "runtime_validation"),
			"metadata":  map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"ref_id", "mode", "provider", "boundary", "status", "operation", "resource"}),
		"ExternalSandboxAuditSummary": objectSchema(map[string]any{
			"summary":                stringSchema("安全审计摘要。", "external_sandbox_ref completed with allow_with_redaction governance decision"),
			"credential_scope":       stringSchema("凭据暴露范围摘要。", "none_persisted"),
			"context_scope":          stringSchema("上下文暴露范围摘要。", "safe_validation_summary_only"),
			"nested_execution":       stringSchema("嵌套执行策略。", "disabled"),
			"state_integrity":        stringSchema("状态完整性检查摘要。", "input_snapshot_preserved"),
			"allowed_output_classes": arraySchema(map[string]any{"type": "string"}),
			"safe_labels":            map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
		}, []string{"summary", "credential_scope", "context_scope", "nested_execution", "state_integrity"}),
		"ExternalSandboxStructuredResult": objectSchema(map[string]any{
			"status":         stringSchema("结构化结果状态。", "success"),
			"result_type":    stringSchema("结果类型。", "external_sandbox_ref"),
			"summary":        stringSchema("结构化结果摘要。", "external_sandbox_ref produced structured validation result"),
			"output":         map[string]any{"type": "object", "additionalProperties": true},
			"redacted_input": map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"status", "result_type", "summary"}),
		"ExternalSandboxValidationResult": objectSchema(map[string]any{
			"sandbox_ref":       refSchema("ExternalSandboxRef"),
			"structured_result": refSchema("ExternalSandboxStructuredResult"),
			"audit_summary":     refSchema("ExternalSandboxAuditSummary"),
			"projection":        map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"sandbox_ref", "structured_result", "audit_summary"}),
		"RuntimeStep": objectSchema(map[string]any{
			"id":           stringSchema("runtime step ID。", "step_20260507_001"),
			"run_id":       stringSchema("runtime run ID。", "run_20260507_001"),
			"sequence":     intSchema("step 顺序。", 1),
			"step_type":    stringSchema("step 类型。", "graph_node"),
			"name":         stringSchema("step 名称。", "model_execution"),
			"status":       stringSchema("step 状态。", "success"),
			"metadata":     map[string]any{"type": "object", "additionalProperties": true},
			"created_at":   dateTimeSchema("创建时间。", "2026-05-07T10:00:00Z"),
			"updated_at":   dateTimeSchema("最近更新时间。", "2026-05-07T10:00:01Z"),
			"started_at":   dateTimeSchema("开始时间。", "2026-05-07T10:00:00Z"),
			"completed_at": dateTimeSchema("完成时间。", "2026-05-07T10:00:01Z"),
		}, []string{"id", "run_id", "sequence", "status"}),
		"RuntimeStepListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeStep")),
		}, []string{"items"}),
		"RuntimeLifecycleEvent": objectSchema(map[string]any{
			"id":           stringSchema("lifecycle event ID。", "event_20260507_001"),
			"run_id":       stringSchema("runtime run ID。", "run_20260507_001"),
			"step_id":      stringSchema("可选 step ID。", "step_20260507_001"),
			"event_type":   stringSchema("事件类型。", "step_status_changed"),
			"subject_type": stringSchema("主体类型。", "step"),
			"subject_id":   stringSchema("主体 ID。", "step_20260507_001"),
			"from_status":  stringSchema("前一状态。", "running"),
			"to_status":    stringSchema("后一状态。", "success"),
			"reason":       stringSchema("状态变化原因。", "node_completed"),
			"metadata":     map[string]any{"type": "object", "additionalProperties": true},
			"occurred_at":  dateTimeSchema("发生时间。", "2026-05-07T10:00:01Z"),
		}, []string{"id", "run_id", "event_type", "subject_type", "subject_id", "occurred_at"}),
		"RuntimeLifecycleEventListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeLifecycleEvent")),
		}, []string{"items"}),
		"RuntimeTrace": objectSchema(map[string]any{
			"id":               stringSchema("trace ID。", "trace_20260507_001"),
			"run_id":           stringSchema("runtime run ID。", "run_20260507_001"),
			"step_id":          stringSchema("可选 step ID。", "step_20260507_001"),
			"trace_type":       stringSchema("trace 类型。", "model_callback"),
			"summary":          stringSchema("白名单安全摘要。", "model callback completed"),
			"safe_labels":      map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"redacted_payload": map[string]any{"type": "object", "additionalProperties": true},
			"metadata":         map[string]any{"type": "object", "additionalProperties": true},
			"created_at":       dateTimeSchema("创建时间。", "2026-05-07T10:00:01Z"),
		}, []string{"id", "run_id", "summary", "created_at"}),
		"RuntimeTraceListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeTrace")),
		}, []string{"items"}),
		"RuntimeUsage": objectSchema(map[string]any{
			"id":            stringSchema("usage ID。", "usage_20260507_001"),
			"run_id":        stringSchema("runtime run ID。", "run_20260507_001"),
			"step_id":       stringSchema("可选 step ID。", "step_20260507_001"),
			"resource_type": stringSchema("资源类型。", "model_tokens"),
			"provider":      stringSchema("provider 名称。", "openai_compatible"),
			"resource_name": stringSchema("模型、工具或资源名称。", "gpt-4.1"),
			"unit":          stringSchema("计量单位。", "token"),
			"amount":        numberSchema("用量数值。", 42),
			"cost":          numberSchema("可选成本。", 0.012),
			"currency":      stringSchema("成本币种。", "USD"),
			"metadata":      map[string]any{"type": "object", "additionalProperties": true},
			"created_at":    dateTimeSchema("创建时间。", "2026-05-07T10:00:01Z"),
		}, []string{"id", "run_id", "resource_type", "unit", "amount", "created_at"}),
		"RuntimeUsageListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeUsage")),
		}, []string{"items"}),
		"RuntimeProjectionCandidate": objectSchema(map[string]any{
			"id":                     stringSchema("projection candidate ID。", "projection_20260507_001"),
			"run_id":                 stringSchema("runtime run ID。", "run_20260507_001"),
			"step_id":                stringSchema("可选 step ID。", "step_20260507_001"),
			"candidate_kind":         stringSchema("候选输出类型。", "assistant_message"),
			"status":                 stringSchema("候选输出状态。", "completed"),
			"summary":                stringSchema("候选输出摘要。", "assistant response candidate"),
			"schema_version":         stringSchema("projection schema 版本。", "runtime_projection.v1"),
			"redacted_payload":       map[string]any{"type": "object", "additionalProperties": true},
			"semantic_payload":       map[string]any{"type": "object", "additionalProperties": true},
			"artifact_refs":          map[string]any{"type": "object", "additionalProperties": true},
			"ui_hints":               map[string]any{"type": "object", "additionalProperties": true},
			"materialization_target": map[string]any{"type": "object", "additionalProperties": true},
			"metadata":               map[string]any{"type": "object", "additionalProperties": true},
			"created_at":             dateTimeSchema("创建时间。", "2026-05-07T10:00:01Z"),
		}, []string{"id", "run_id", "candidate_kind", "created_at"}),
		"RuntimeProjectionCandidateListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("RuntimeProjectionCandidate")),
		}, []string{"items"}),
		"RuntimeContract": objectSchema(map[string]any{
			"id":                     stringSchema("runtime contract ID。", "contract_runtime_validation_v1"),
			"name":                   stringSchema("contract 名称。", "Runtime Validation Contract"),
			"version":                stringSchema("contract 版本。", "v1"),
			"status":                 stringSchema("contract 状态。", "active"),
			"task_type":              stringSchema("绑定的 task type。", "runtime_validation"),
			"input_schema":           map[string]any{"type": "object", "additionalProperties": true},
			"execution_profile":      map[string]any{"type": "object", "additionalProperties": true},
			"exit_policy":            map[string]any{"type": "object", "additionalProperties": true},
			"capability_profile":     map[string]any{"type": "object", "additionalProperties": true},
			"governance_policy_refs": map[string]any{"type": "object", "additionalProperties": true},
			"hook_bindings":          map[string]any{"type": "object", "additionalProperties": true},
			"projection_policy":      map[string]any{"type": "object", "additionalProperties": true},
			"system_truth_refs":      map[string]any{"type": "object", "additionalProperties": true},
			"idempotency_scope":      stringSchema("幂等范围。", "contract"),
			"idempotency_key":        stringSchema("幂等键。", "runtime_validation:v1"),
			"metadata":               map[string]any{"type": "object", "additionalProperties": true},
			"created_at":             dateTimeSchema("创建时间。", "2026-05-19T10:00:00Z"),
			"updated_at":             dateTimeSchema("最近更新时间。", "2026-05-19T10:00:01Z"),
		}, []string{"id", "name", "version", "status", "task_type", "created_at", "updated_at"}),
		"RuntimeContractUpsertRequest": objectSchema(map[string]any{
			"name":                   stringSchema("contract 名称。", "Runtime Validation Contract"),
			"version":                stringSchema("contract 版本。", "v1"),
			"status":                 stringSchema("contract 状态。", "active"),
			"task_type":              stringSchema("绑定的 task type。", "runtime_validation"),
			"input_schema":           map[string]any{"type": "object", "additionalProperties": true},
			"execution_profile":      map[string]any{"type": "object", "additionalProperties": true},
			"exit_policy":            map[string]any{"type": "object", "additionalProperties": true},
			"capability_profile":     map[string]any{"type": "object", "additionalProperties": true},
			"governance_policy_refs": map[string]any{"type": "object", "additionalProperties": true},
			"hook_bindings":          map[string]any{"type": "object", "additionalProperties": true},
			"projection_policy":      map[string]any{"type": "object", "additionalProperties": true},
			"system_truth_refs":      map[string]any{"type": "object", "additionalProperties": true},
			"idempotency_scope":      stringSchema("幂等范围。", "contract"),
			"idempotency_key":        stringSchema("幂等键。", "runtime_validation:v1"),
			"metadata":               map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"name", "version", "status", "task_type"}),
		"RuntimeTaskTypeRegistration": objectSchema(map[string]any{
			"id":                  stringSchema("task type registration ID。", "task_type_runtime_validation"),
			"type_key":            stringSchema("task type 稳定键。", "runtime_validation"),
			"display_name":        stringSchema("显示名。", "Runtime Validation"),
			"description":         stringSchema("说明。", "Validates runtime persistence and projection boundaries."),
			"status":              stringSchema("注册状态。", "active"),
			"input_schema":        map[string]any{"type": "object", "additionalProperties": true},
			"validator_refs":      map[string]any{"type": "object", "additionalProperties": true},
			"default_contract_id": stringSchema("默认 runtime contract ID。", "contract_runtime_validation_v1"),
			"compatibility":       map[string]any{"type": "object", "additionalProperties": true},
			"metadata":            map[string]any{"type": "object", "additionalProperties": true},
			"created_at":          dateTimeSchema("创建时间。", "2026-05-19T10:00:00Z"),
			"updated_at":          dateTimeSchema("最近更新时间。", "2026-05-19T10:00:01Z"),
		}, []string{"id", "type_key", "status", "created_at", "updated_at"}),
		"RuntimeTaskTypeUpsertRequest": objectSchema(map[string]any{
			"id":                  stringSchema("可选稳定 ID；省略时保留现有记录或由后端生成。", "task_type_runtime_validation"),
			"display_name":        stringSchema("显示名。", "Runtime Validation"),
			"description":         stringSchema("说明。", "Validates runtime persistence and projection boundaries."),
			"status":              stringSchema("注册状态。", "active"),
			"input_schema":        map[string]any{"type": "object", "additionalProperties": true},
			"validator_refs":      map[string]any{"type": "object", "additionalProperties": true},
			"default_contract_id": stringSchema("默认 runtime contract ID。", "contract_runtime_validation_v1"),
			"compatibility":       map[string]any{"type": "object", "additionalProperties": true},
			"metadata":            map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"status"}),
		"RuntimeHookBinding": objectSchema(map[string]any{
			"id":             stringSchema("hook binding ID。", "hook_runtime_contract_guard"),
			"contract_id":    stringSchema("runtime contract ID。", "contract_runtime_validation_v1"),
			"hook_point":     stringSchema("runtime hook point。", "before_run"),
			"binding_kind":   stringSchema("binding 类型。", "eino_middleware"),
			"binding_ref":    stringSchema("allowlisted binding 引用。", "runtime_contract_guard"),
			"order_index":    intSchema("执行顺序。", 10),
			"enabled":        boolSchema("是否启用。", true),
			"failure_policy": stringSchema("失败策略。", "fail_closed"),
			"config":         map[string]any{"type": "object", "additionalProperties": true},
			"metadata":       map[string]any{"type": "object", "additionalProperties": true},
			"created_at":     dateTimeSchema("创建时间。", "2026-05-19T10:00:00Z"),
			"updated_at":     dateTimeSchema("最近更新时间。", "2026-05-19T10:00:01Z"),
		}, []string{"id", "contract_id", "hook_point", "binding_kind", "binding_ref", "enabled", "failure_policy"}),
		"RuntimeHookBindingUpsertRequest": objectSchema(map[string]any{
			"contract_id":    stringSchema("runtime contract ID。", "contract_runtime_validation_v1"),
			"hook_point":     stringSchema("runtime hook point。", "before_run"),
			"binding_kind":   stringSchema("binding 类型。", "eino_middleware"),
			"binding_ref":    stringSchema("allowlisted binding 引用。", "runtime_contract_guard"),
			"order_index":    intSchema("执行顺序。", 10),
			"enabled":        boolSchema("是否启用。", true),
			"failure_policy": stringSchema("失败策略。", "fail_closed"),
			"config":         map[string]any{"type": "object", "additionalProperties": true},
			"metadata":       map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"contract_id", "hook_point", "binding_kind", "binding_ref", "enabled", "failure_policy"}),
		"SystemTruthActiveVersion": objectSchema(map[string]any{
			"id":                stringSchema("active pointer 记录 ID。", "active_system_truth_001"),
			"asset_id":          stringSchema("system truth asset ID。", "tool_governance_policy"),
			"compile_result_id": stringSchema("compile result ID。", "compile_tool_governance_v1"),
			"draft_id":          stringSchema("draft ID。", "draft_tool_governance_v1"),
			"activated_by":      stringSchema("激活人或来源。", "system"),
			"reason":            stringSchema("激活原因。", "initial activation"),
			"rollback_from_id":  stringSchema("回滚来源 active ID。", "active_previous"),
			"metadata":          map[string]any{"type": "object", "additionalProperties": true},
			"activated_at":      dateTimeSchema("激活时间。", "2026-05-19T10:00:00Z"),
		}, []string{"id", "asset_id", "compile_result_id", "draft_id", "activated_at"}),
		"RuntimeContractFoundationResponse": objectSchema(map[string]any{
			"contracts":            arraySchema(refSchema("RuntimeContract")),
			"task_types":           arraySchema(refSchema("RuntimeTaskTypeRegistration")),
			"hook_bindings":        arraySchema(refSchema("RuntimeHookBinding")),
			"active_system_truths": arraySchema(refSchema("SystemTruthActiveVersion")),
			"store_capabilities":   arraySchema(map[string]any{"type": "string"}),
			"unavailable_surfaces": arraySchema(map[string]any{"type": "string"}),
		}, []string{"contracts", "task_types", "hook_bindings", "active_system_truths", "store_capabilities"}),
		"ControlPlaneScene": objectSchema(map[string]any{
			"id":                  stringSchema("场景稳定标识。", "security_review"),
			"description":         stringSchema("场景说明。", "面向安全审计、威胁建模和供应链分析。"),
			"keywords":            arraySchema(map[string]any{"type": "string"}),
			"default_skills":      arraySchema(map[string]any{"type": "string"}),
			"suggested_questions": arraySchema(map[string]any{"type": "string"}),
			"enabled":             boolSchema("场景当前是否启用。", true),
			"match_score":         intSchema("关键词命中时采用的默认分值。", 86),
		}, []string{"id", "enabled"}),
		"ControlPlaneSceneListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ControlPlaneScene")),
		}, []string{"items"}),
		"ControlPlaneSkill": objectSchema(map[string]any{
			"name":        stringSchema("Skill 稳定名称。", "cso_review"),
			"description": stringSchema("Skill 对外说明。", "面向安全审计与威胁建模的分析 skill。"),
			"guidance":    stringSchema("Skill guidance override。", "优先输出风险摘要、证据和下一步建议。"),
			"tool_names":  arraySchema(map[string]any{"type": "string"}),
			"enabled":     boolSchema("Skill 当前是否启用。", true),
		}, []string{"name", "enabled"}),
		"ControlPlaneSkillListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ControlPlaneSkill")),
		}, []string{"items"}),
		"ControlPlaneTool": objectSchema(map[string]any{
			"name":                  stringSchema("tool 稳定名称。", "lookup_profile"),
			"description":           stringSchema("tool 对外说明。", "Look up a user's basic profile."),
			"tool_scope":            stringSchema("tool 的作用域标签。", "read_only_lookup"),
			"requires_confirmation": boolSchema("该 tool 是否要求确认。", false),
			"side_effect_level":     stringSchema("副作用等级。", "none"),
			"input_schema_summary":  stringSchema("输入摘要。", "user_id:string"),
			"output_schema_summary": stringSchema("输出摘要。", "profile object"),
			"enabled":               boolSchema("tool 当前是否启用。", true),
		}, []string{"name", "enabled"}),
		"ControlPlaneToolListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ControlPlaneTool")),
		}, []string{"items"}),
		"ControlPlaneRuntimeConfig": objectSchema(map[string]any{
			"choice_required_enabled":     boolSchema("是否允许原生返回 choice_required。", true),
			"automation_fallback_enabled": boolSchema("是否允许自动化草案路径进入 fallback。", true),
			"planning_progress_enabled":   boolSchema("是否输出 planning progress。", true),
		}, []string{"choice_required_enabled", "automation_fallback_enabled", "planning_progress_enabled"}),
		"ControlPlaneGovernanceConfig": objectSchema(map[string]any{
			"choice_required_enabled":              boolSchema("是否允许原生返回 choice_required。", true),
			"automation_fallback_enabled":          boolSchema("是否允许自动化草案路径进入 fallback。", true),
			"planning_progress_enabled":            boolSchema("是否输出 planning progress。", true),
			"fact_quality_gate_enabled":            boolSchema("是否发射 fact_quality。", true),
			"tool_hint_emission_enabled":           boolSchema("是否发射 platform_tool_hints。", true),
			"knowledge_retrieval_emission_enabled": boolSchema("是否发射 knowledge_retrieval。", true),
			"max_planning_steps":                   intSchema("planning progress 允许暴露的最大步骤数。", 6),
			"max_tool_hints":                       intSchema("允许发射的最大 tool hint 数量。", 4),
		}, []string{"choice_required_enabled", "automation_fallback_enabled", "planning_progress_enabled", "fact_quality_gate_enabled", "tool_hint_emission_enabled", "knowledge_retrieval_emission_enabled", "max_planning_steps", "max_tool_hints"}),
		"ToolGovernanceRule": objectSchema(map[string]any{
			"rule_id":         stringSchema("规则 ID。", "redact_external_read"),
			"match_tool":      stringSchema("匹配 tool 名称，空值或 * 表示任意。", "demo_browser"),
			"match_scope":     stringSchema("匹配 tool scope。", "external_web"),
			"match_operation": stringSchema("匹配操作类型。", "read"),
			"match_risk":      stringSchema("匹配风险等级。", "medium"),
			"decision":        stringSchema("治理决策。", "allow_with_redaction"),
			"reason":          stringSchema("决策原因。", "External reads may proceed with redaction."),
			"redact_fields":   arraySchema(map[string]any{"type": "string"}),
			"sandbox_ref":     stringSchema("要求的 sandbox 引用。", "workspace-write"),
			"metadata":        map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"decision"}),
		"ToolGovernancePolicy": objectSchema(map[string]any{
			"policy_id":         stringSchema("策略 ID。", "default_tool_governance"),
			"asset_id":          stringSchema("来源 system resource ID。", "tool_governance_policy.core.default"),
			"name":              stringSchema("策略名称。", "Default Tool Governance"),
			"default_decision":  stringSchema("默认决策。", "allow"),
			"decision_model":    stringSchema("决策模型。", "first_match"),
			"rules":             arraySchema(refSchema("ToolGovernanceRule")),
			"compiled_version":  stringSchema("编译版本。", "20260422T100003.000000000Z"),
			"truth_dir_version": stringSchema("truth dir 版本。", "20260422T100003.000000000Z"),
			"source_checksum":   stringSchema("source checksum。", "sha256:abcd"),
			"updated_at":        dateTimeSchema("更新时间。", "2026-04-22T10:00:03Z"),
			"metadata":          map[string]any{"type": "object", "additionalProperties": true},
		}, nil),
		"ToolGovernanceDecisionRequest": objectSchema(map[string]any{
			"tool_name":  stringSchema("待执行 tool 名称。", "demo_browser"),
			"tool_scope": stringSchema("待执行 tool scope。", "external_web"),
			"operation":  stringSchema("待执行操作类型。", "read"),
			"risk_level": stringSchema("请求风险等级。", "medium"),
			"metadata":   map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"tool_name"}),
		"ToolGovernanceDecision": objectSchema(map[string]any{
			"decision_id":       stringSchema("决策 ID。", "tool_gov_20260422T100003.000000000Z"),
			"decision":          stringSchema("治理决策。", "allow_with_redaction"),
			"reason":            stringSchema("决策原因。", "External reads may proceed with redaction."),
			"matched_rule_id":   stringSchema("命中的规则 ID。", "redact_external_read"),
			"policy_asset_id":   stringSchema("命中的 policy asset ID。", "tool_governance_policy.core.default"),
			"policy_version":    stringSchema("policy 编译版本。", "20260422T100003.000000000Z"),
			"tool_name":         stringSchema("tool 名称。", "demo_browser"),
			"tool_scope":        stringSchema("tool scope。", "external_web"),
			"operation":         stringSchema("操作类型。", "read"),
			"risk_level":        stringSchema("风险等级。", "medium"),
			"redact_fields":     arraySchema(map[string]any{"type": "string"}),
			"sandbox_ref":       stringSchema("要求的 sandbox 引用。", "workspace-write"),
			"evaluated_at":      dateTimeSchema("判定时间。", "2026-04-22T10:00:03Z"),
			"truth_dir_version": stringSchema("truth dir 版本。", "20260422T100003.000000000Z"),
			"metadata":          map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"decision_id", "decision"}),
		"ToolGovernanceDecisionListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ToolGovernanceDecision")),
		}, []string{"items"}),
		"ValidationMCPToolSchema": objectSchema(map[string]any{
			"name":          stringSchema("MCP tool 名称。", "risk_signal_lookup"),
			"description":   stringSchema("MCP tool 描述。", "Return deterministic validation risk signals."),
			"tool_scope":    stringSchema("tool scope。", "validation_mcp"),
			"operation":     stringSchema("操作类型。", "invoke"),
			"risk_level":    stringSchema("风险等级。", "medium"),
			"input_schema":  map[string]any{"type": "object", "additionalProperties": true},
			"output_schema": map[string]any{"type": "object", "additionalProperties": true},
			"metadata":      map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"name"}),
		"ValidationMCPServerInfo": objectSchema(map[string]any{
			"server_id": stringSchema("validation MCP server ID。", "athena-validation-mcp"),
			"name":      stringSchema("server 名称。", "Athena Validation MCP"),
			"transport": stringSchema("轻量 transport 描述。", "control-plane-http-adapter"),
			"status":    stringSchema("server 状态。", "ready"),
			"tools":     arraySchema(refSchema("ValidationMCPToolSchema")),
			"metadata":  map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"server_id", "name", "transport", "status"}),
		"ValidationMCPToolListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ValidationMCPToolSchema")),
		}, []string{"items"}),
		"ValidationMCPInvocationRequest": objectSchema(map[string]any{
			"tool_name": stringSchema("validation MCP tool 名称。", "risk_signal_lookup"),
			"input":     map[string]any{"type": "object", "additionalProperties": true},
			"metadata":  map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"tool_name"}),
		"ValidationMCPInvocationTrace": objectSchema(map[string]any{
			"trace_id":         stringSchema("trace ID。", "trace_123"),
			"trace_type":       stringSchema("trace 类型。", "validation_mcp_tool_invocation"),
			"summary":          stringSchema("白名单安全摘要。", "risk signal classified as high"),
			"safe_labels":      map[string]any{"type": "object", "additionalProperties": map[string]any{"type": "string"}},
			"redacted_payload": map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"trace_id", "trace_type", "summary"}),
		"ValidationMCPInvocationResult": objectSchema(map[string]any{
			"invocation_id":     stringSchema("调用 ID。", "validation_mcp_123"),
			"server_id":         stringSchema("server ID。", "athena-validation-mcp"),
			"tool_name":         stringSchema("tool 名称。", "risk_signal_lookup"),
			"status":            stringSchema("调用状态。", "success"),
			"result_summary":    stringSchema("结果摘要。", "risk signal classified as high"),
			"output":            map[string]any{"type": "object", "additionalProperties": true},
			"trace":             refSchema("ValidationMCPInvocationTrace"),
			"applied_redaction": boolSchema("是否应用脱敏。", true),
			"metadata":          map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"invocation_id", "server_id", "tool_name", "status", "result_summary", "trace", "applied_redaction"}),
		"ValidationMCPInvocationResponse": objectSchema(map[string]any{
			"server":              refSchema("ValidationMCPServerInfo"),
			"tool":                refSchema("ValidationMCPToolSchema"),
			"request":             refSchema("ValidationMCPInvocationRequest"),
			"governance_decision": refSchema("ToolGovernanceDecision"),
			"result":              refSchema("ValidationMCPInvocationResult"),
		}, []string{"server", "tool", "request", "governance_decision", "result"}),
		"ControlPlaneConfigVersionSummary": objectSchema(map[string]any{
			"version_id": stringSchema("配置版本 ID。", "cfg_20260421T120000.000000000Z"),
			"created_at": dateTimeSchema("版本创建时间。", "2026-04-21T12:00:00Z"),
			"created_by": stringSchema("版本创建人。", "system"),
			"summary":    stringSchema("版本摘要。", "update governance"),
		}, []string{"version_id", "created_at"}),
		"ControlPlaneConfigVersionDetail": objectSchema(map[string]any{
			"version_id": stringSchema("配置版本 ID。", "cfg_20260421T120000.000000000Z"),
			"created_at": dateTimeSchema("版本创建时间。", "2026-04-21T12:00:00Z"),
			"created_by": stringSchema("版本创建人。", "system"),
			"summary":    stringSchema("版本摘要。", "update governance"),
			"document": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scenes":     arraySchema(refSchema("ControlPlaneScene")),
					"skills":     arraySchema(refSchema("ControlPlaneSkill")),
					"tools":      arraySchema(refSchema("ControlPlaneTool")),
					"governance": refSchema("ControlPlaneGovernanceConfig"),
					"runtime":    refSchema("ControlPlaneGovernanceConfig"),
				},
			},
		}, []string{"version_id", "created_at", "document"}),
		"ControlPlaneConfigVersionListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ControlPlaneConfigVersionSummary")),
		}, []string{"items"}),
		"TruthDirInfo": objectSchema(map[string]any{
			"path":    stringSchema("当前 active truth dir 的本地路径。", "<truth_dir>"),
			"version": stringSchema("当前 active truth dir 版本。", "truth-20260422T120000.000000000Z"),
		}, nil),
		"ControlPlaneAuthStatus": objectSchema(map[string]any{
			"authenticated":      boolSchema("当前控制台请求是否已通过控制面认证。", true),
			"lock_state":         stringSchema("当前 IP 对应的锁定状态。disabled 表示未启用 token，active 表示可继续尝试，locked 表示已锁定。", "active"),
			"remaining_attempts": intSchema("距离触发 IP 锁定前还剩多少次失败机会。", 4),
			"failed_attempts":    intSchema("当前 IP 已累计的失败次数。", 1),
			"session_expires_at": dateTimeSchema("当前控制面认证 session 的失效时间。", "2026-04-22T20:00:00Z"),
			"truth_dir":          refSchema("TruthDirInfo"),
		}, []string{"authenticated"}),
		"ControlPlaneLoginRequest": objectSchema(map[string]any{
			"token": stringSchema("控制面登录 token。", "control-plane-demo-token"),
		}, []string{"token"}),
		"ControlPlaneBootstrapResponse": objectSchema(map[string]any{
			"scenes":           arraySchema(refSchema("ControlPlaneScene")),
			"skills":           arraySchema(refSchema("ControlPlaneSkill")),
			"tools":            arraySchema(refSchema("ControlPlaneTool")),
			"system_resources": arraySchema(refSchema("SystemResourceSummary")),
			"governance":       refSchema("ControlPlaneGovernanceConfig"),
			"runtime":          refSchema("ControlPlaneGovernanceConfig"),
			"config_versions":  arraySchema(refSchema("ControlPlaneConfigVersionSummary")),
			"swagger_spec_url": stringSchema("前端控制台展示 Swagger 时使用的 OpenAPI spec 地址。", "/swagger/openapi.json"),
		}, []string{"scenes", "skills", "tools", "governance", "runtime", "config_versions", "swagger_spec_url"}),
		"UpdateSkillPackageStateRequest": objectSchema(map[string]any{
			"enabled": map[string]any{"type": "boolean"},
		}, []string{"enabled"}),
		"RollbackSkillPackageRequest": objectSchema(map[string]any{
			"revision": map[string]any{"type": "integer", "format": "int32"},
		}, []string{"revision"}),
		"SkillPackageFileMap": map[string]any{
			"type":                 "object",
			"description":          "技能包文件路径到文本内容的映射，必须包含 SKILL.md。",
			"additionalProperties": map[string]any{"type": "string"},
			"example": map[string]string{
				"SKILL.md":            "---\nname: uploaded-example\ndescription: Uploaded example skill.\n---\n\n# Uploaded example\n\nGuide the agent.",
				"references/guide.md": "Reference material",
			},
		},
		"SkillPackageFilesRequest": objectSchema(map[string]any{
			"name":    stringSchema("当 SKILL.md 未声明 name 时使用的后备技能包名称。", "uploaded-example"),
			"enabled": boolSchema("上传后的期望启用状态；无效包仍会被强制禁用。", true),
			"files":   refSchema("SkillPackageFileMap"),
		}, []string{"files"}),
		"ValidationResult": objectSchema(map[string]any{
			"valid":    boolSchema("治理校验是否通过。", true),
			"errors":   withDescription(arraySchema(map[string]any{"type": "string"}), "会阻止技能包启用的校验错误列表。"),
			"warnings": withDescription(arraySchema(map[string]any{"type": "string"}), "仅用于提示的非阻断性校验警告。"),
		}, []string{"valid"}),
		"SkillItem": objectSchema(map[string]any{
			"name":        map[string]any{"type": "string"},
			"source":      map[string]any{"type": "string"},
			"description": map[string]any{"type": "string"},
			"tool_names":  arraySchema(map[string]any{"type": "string"}),
			"guidance":    map[string]any{"type": "string"},
		}, []string{"name", "source"}),
		"SkillListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("SkillItem")),
		}, []string{"items"}),
		"SkillPackageResponse": objectSchema(map[string]any{
			"id":          stringSchema("上传技能包的稳定 ID。", "pkg_123"),
			"name":        stringSchema("从 SKILL.md 中提取出的技能包名称。", "uploaded-example"),
			"revision":    intSchema("当前技能包版本号。", 2),
			"file_count":  intSchema("当前技能包内保存的文件数量。", 3),
			"file_paths":  withDescription(arraySchema(map[string]any{"type": "string"}), "当前技能包内的已排序文件路径列表。"),
			"enabled":     boolSchema("该上传技能包当前是否对运行时可见。", true),
			"validation":  refSchema("ValidationResult"),
			"uploaded_at": dateTimeSchema("上传时间，RFC3339 格式。", "2026-04-09T06:30:00Z"),
		}, []string{"id", "name", "revision", "file_count", "enabled", "validation"}),
		"SkillPackageDetailResponse": objectSchema(map[string]any{
			"metadata": refSchema("SkillPackageResponse"),
			"files":    refSchema("SkillPackageFileMap"),
		}, []string{"metadata", "files"}),
		"SkillPackageListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("SkillPackageResponse")),
		}, []string{"items"}),
		"SkillPackageRollbackResponse": objectSchema(map[string]any{
			"metadata":         refSchema("SkillPackageResponse"),
			"rolled_back_from": intSchema("本次回滚所基于的历史版本号。", 1),
			"current_revision": intSchema("回滚后新生成的当前版本号。", 3),
		}, []string{"metadata", "rolled_back_from", "current_revision"}),
		"DeleteSkillPackageResponse": objectSchema(map[string]any{
			"id":     stringSchema("被删除的技能包 ID。", "pkg_123"),
			"status": stringSchema("删除结果。", "deleted"),
		}, []string{"id", "status"}),
		"StatusResponse": objectSchema(map[string]any{
			"status": stringSchema("操作结果。", "deleted"),
		}, []string{"status"}),
		"ProviderHeadersInput": map[string]any{
			"description": "可选的供应商级静态请求头。既支持 JSON 对象，也支持 key/value 数组形式，方便不同客户端提交。",
			"example": map[string]any{
				"Accept-Encoding": "identity",
				"X-Client":        "athena-swagger",
			},
			"oneOf": []any{
				map[string]any{
					"description":          "对象写法：key 为请求头名，value 为请求头值。",
					"type":                 "object",
					"additionalProperties": map[string]any{"type": "string"},
				},
				map[string]any{
					"description": "数组写法：适合偏好显式 key/value 行结构的调用方。",
					"type":        "array",
					"items": objectSchema(map[string]any{
						"key":   stringSchema("HTTP 请求头名称。", "Accept-Encoding"),
						"value": stringSchema("HTTP 请求头取值。", "identity"),
					}, []string{"key", "value"}),
				},
			},
		},
		"CreateModelProviderRequest": objectSchema(map[string]any{
			"name":                    stringSchema("供应商显示名称，必须全局唯一。", "openrouter-main"),
			"base_url":                stringSchema("供应商 API 基础地址。仅当所选协议存在内置默认地址时才可留空。", "https://openrouter.ai/api/v1"),
			"protocol":                providerProtocolSchema("Athena 构建聊天客户端时使用的传输协议。留空时默认按 openai_compatible 处理。"),
			"api_key":                 stringSchema("供应商 API Key。响应中不会回传该明文值。", "sk-demo"),
			"request_timeout_seconds": intSchema("模型请求超时时间，单位秒。非正数会回退到系统默认值。", 45),
			"headers":                 refSchema("ProviderHeadersInput"),
			"enabled":                 boolSchema("该供应商当前是否允许被运行时治理选择。", true),
			"models":                  withDescription(arraySchema(refSchema("UpsertProviderModelRequest")), "可选的子模型定义列表，可与供应商一起创建。创建成功后可继续通过 POST /api/models/providers/{id}/models/{record_id}/test 测试模型可用性。"),
		}, []string{"name", "protocol"}),
		"UpsertModelProviderRequest": objectSchema(map[string]any{
			"name":                    stringSchema("供应商显示名称，必须全局唯一。", "openrouter-main"),
			"base_url":                stringSchema("供应商 API 基础地址。仅当所选协议存在内置默认地址时才可留空。", "https://openrouter.ai/api/v1"),
			"protocol":                providerProtocolSchema("Athena 构建聊天客户端时使用的传输协议。留空时默认按 openai_compatible 处理。"),
			"api_key":                 stringSchema("供应商 API Key。响应中不会回传该明文值。", "sk-demo"),
			"request_timeout_seconds": intSchema("模型请求超时时间，单位秒。非正数会回退到系统默认值。", 45),
			"headers":                 refSchema("ProviderHeadersInput"),
			"enabled":                 boolSchema("该供应商当前是否允许被运行时治理选择。", true),
		}, []string{"name", "protocol"}),
		"PatchModelProviderRequest": objectSchema(map[string]any{
			"enabled": boolSchema("在不整体替换供应商定义的前提下，启用或禁用该供应商。", false),
		}, nil),
		"UpsertProviderModelRequest": objectSchema(map[string]any{
			"model_id":     stringSchema("发送给上游模型服务的原生模型标识。", "gpt-4o-mini"),
			"display_name": stringSchema("用于治理界面与调试输出的人类可读名称。", "GPT-4o Mini"),
			"enabled":      boolSchema("该模型记录当前是否允许被运行时选择。", true),
			"is_default":   boolSchema("当请求未显式传入 model_id 时，是否将该模型作为全局默认模型。", true),
			"is_fallback":  boolSchema("当隐式选模失败时，是否将该模型作为全局技术兜底模型。", false),
		}, []string{"model_id", "display_name"}),
		"PatchProviderModelRequest": objectSchema(map[string]any{
			"enabled":     boolSchema("在不替换标识字段的前提下，启用或禁用该模型记录。", false),
			"is_default":  boolSchema("重分配或取消该记录的全局默认模型角色。", false),
			"is_fallback": boolSchema("重分配或取消该记录的全局兜底模型角色。", false),
		}, nil),
		"ProviderModelResponse": objectSchema(map[string]any{
			"id":           stringSchema("该供应商模型记录的稳定治理 ID。", "modelrec_123"),
			"provider_id":  stringSchema("拥有该模型记录的供应商稳定 ID。", "provider_123"),
			"model_id":     stringSchema("实际推理时发送给上游的原生模型标识。", "gpt-4o-mini"),
			"display_name": stringSchema("面向运营和 UI 的可读名称。", "GPT-4o Mini"),
			"enabled":      boolSchema("该模型记录当前是否允许被运行时选择。", true),
			"is_default":   boolSchema("该模型当前是否担任全局默认模型。", true),
			"is_fallback":  boolSchema("该模型当前是否担任全局技术兜底模型。", false),
			"created_at":   dateTimeSchema("创建时间，RFC3339 格式。", "2026-04-09T06:00:00Z"),
			"updated_at":   dateTimeSchema("最近更新时间，RFC3339 格式。", "2026-04-09T06:30:00Z"),
		}, []string{"id", "model_id", "display_name"}),
		"ModelProviderResponse": objectSchema(map[string]any{
			"id":                      stringSchema("供应商的稳定治理 ID。", "provider_123"),
			"name":                    stringSchema("供应商显示名称。", "openrouter-main"),
			"base_url":                stringSchema("当前治理层保存的供应商 API 基础地址。", "https://openrouter.ai/api/v1"),
			"protocol":                providerProtocolSchema("当前治理层保存的供应商传输协议。"),
			"request_timeout_seconds": intSchema("Athena 调用该供应商时的单次请求超时，单位秒。", 45),
			"enabled":                 boolSchema("该供应商当前是否允许被运行时选择。", true),
			"api_key_configured":      boolSchema("Athena 当前是否为该供应商保存了一份 API Key。仅表示已配置，不代表一定可用。", true),
			"api_key_masked":          stringSchema("仅供确认的脱敏 API Key 预览。", "sk-***demo"),
			"headers": map[string]any{
				"description":          "治理层当前保存的供应商级静态请求头。",
				"type":                 "object",
				"additionalProperties": map[string]any{"type": "string"},
				"example": map[string]any{
					"Accept-Encoding": "identity",
				},
			},
			"models":     withDescription(arraySchema(refSchema("ProviderModelResponse")), "挂在该供应商下的子模型记录列表。"),
			"created_at": dateTimeSchema("创建时间，RFC3339 格式。", "2026-04-09T06:00:00Z"),
			"updated_at": dateTimeSchema("最近更新时间，RFC3339 格式。", "2026-04-09T06:30:00Z"),
		}, []string{"id", "name", "protocol"}),
		"ModelProviderListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("ModelProviderResponse")),
		}, []string{"items"}),
		"ModelTestResponse": objectSchema(map[string]any{
			"provider_id":     stringSchema("被测试模型所属供应商的稳定 ID。", "provider_123"),
			"model_record_id": stringSchema("该模型在治理层中的稳定记录 ID。", "modelrec_123"),
			"provider_name":   stringSchema("供应商显示名称。", "openrouter-main"),
			"model_id":        stringSchema("本次测试实际使用的原生模型标识。", "gpt-4o-mini"),
			"display_name":    stringSchema("被测试模型的可读名称。", "GPT-4o Mini"),
			"available":       boolSchema("本次在线探测是否成功。", true),
			"duration_ms":     intSchema("本次在线探测的端到端耗时，单位毫秒。", 842),
			"error":           stringSchema("当探测失败时返回的失败摘要。", "dial tcp: i/o timeout"),
		}, []string{"provider_id", "model_record_id", "provider_name", "model_id", "display_name", "available", "duration_ms"}),
	}
	for name, schema := range buildSystemResourceOpenAPISchemas() {
		schemas[name] = schema
	}

	keys := make([]string, 0, len(schemas))
	for key := range schemas {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	sorted := make(map[string]any, len(schemas))
	for _, key := range keys {
		sorted[key] = schemas[key]
	}
	return sorted
}

func jsonRequest(schema string, required bool) map[string]any {
	return map[string]any{
		"required": required,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": refSchema(schema),
			},
		},
	}
}

func multipartRequest() map[string]any {
	return map[string]any{
		"required": true,
		"content": map[string]any{
			"multipart/form-data": map[string]any{
				"schema": objectSchema(map[string]any{
					"bundle": map[string]any{"type": "string", "format": "binary"},
				}, []string{"bundle"}),
			},
		},
	}
}

func skillPackageRequestBody() map[string]any {
	body := multipartRequest()
	content := body["content"].(map[string]any)
	content["application/json"] = map[string]any{"schema": refSchema("SkillPackageFilesRequest")}
	return body
}

func jsonResponse(description, schema string) map[string]any {
	return map[string]any{
		"description": description,
		"content": map[string]any{
			"application/json": map[string]any{
				"schema": refSchema(schema),
			},
		},
	}
}

func pathIDParameter(name, description string) []map[string]any {
	return []map[string]any{{
		"name":        name,
		"in":          "path",
		"required":    true,
		"description": description,
		"schema": map[string]any{
			"type": "string",
		},
	}}
}

func queryStringParameter(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"required":    false,
		"description": description,
		"schema": map[string]any{
			"type": "string",
		},
	}
}

func queryIntParameter(name, description string) map[string]any {
	return map[string]any{
		"name":        name,
		"in":          "query",
		"required":    false,
		"description": description,
		"schema": map[string]any{
			"type":   "integer",
			"format": "int32",
		},
	}
}

func refSchema(name string) map[string]any {
	return map[string]any{
		"$ref": "#/components/schemas/" + name,
	}
}

func objectSchema(properties map[string]any, required []string) map[string]any {
	schema := map[string]any{
		"type":       "object",
		"properties": properties,
	}
	if len(required) > 0 {
		schema["required"] = required
	}
	return schema
}

func arraySchema(items any) map[string]any {
	return map[string]any{
		"type":  "array",
		"items": items,
	}
}

func withDescription(schema map[string]any, description string) map[string]any {
	schema["description"] = description
	return schema
}

func stringSchema(description, example string) map[string]any {
	schema := map[string]any{
		"type":        "string",
		"description": description,
	}
	if example != "" {
		schema["example"] = example
	}
	return schema
}

func intSchema(description string, example int) map[string]any {
	return map[string]any{
		"type":        "integer",
		"format":      "int32",
		"description": description,
		"example":     example,
	}
}

func numberSchema(description string, example float64) map[string]any {
	return map[string]any{
		"type":        "number",
		"format":      "double",
		"description": description,
		"example":     example,
	}
}

func boolSchema(description string, example bool) map[string]any {
	return map[string]any{
		"type":        "boolean",
		"description": description,
		"example":     example,
	}
}

func dateTimeSchema(description, example string) map[string]any {
	return map[string]any{
		"type":        "string",
		"format":      "date-time",
		"description": description,
		"example":     example,
	}
}

func providerProtocolSchema(description string) map[string]any {
	return map[string]any{
		"type":        "string",
		"description": description,
		"enum":        []string{"openai_compatible", "ark", "anthropic"},
		"example":     "openai_compatible",
	}
}

func buildSystemResourceOpenAPIPaths() map[string]any {
	return map[string]any{
		"/api/system-resources": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出当前 active truth dir 中的 system resources",
				"operationId": "listSystemResources",
				"responses": map[string]any{
					"200": jsonResponse("system resource list", "SystemResourceListResponse"),
				},
			},
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "创建一条 system resource 并触发默认 parse/compile/activate pipeline",
				"operationId": "createSystemResource",
				"requestBody": jsonRequest("SystemResourceCreateRequest", true),
				"responses": map[string]any{
					"200": jsonResponse("system resource mutation result", "SystemResourceMutationResult"),
					"400": jsonResponse("invalid system resource request", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/export": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "导出当前 truth dir 的 system resources 快照",
				"operationId": "exportSystemResources",
				"responses": map[string]any{
					"200": map[string]any{"description": "zip export payload"},
				},
			},
		},
		"/api/system-resources/{id}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看单条 system resource 详情",
				"operationId": "getSystemResource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource detail", "SystemResourceDetail"),
					"404": jsonResponse("system resource not found", "ErrorResponse"),
				},
			},
			"delete": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "删除一条 system resource",
				"operationId": "deleteSystemResource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("delete result", "DeleteSystemResourceResponse"),
					"404": jsonResponse("system resource not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/versions": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "列出一条 system resource 的版本快照",
				"operationId": "listSystemResourceVersions",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource version list", "SystemResourceVersionListResponse"),
					"404": jsonResponse("system resource not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/versions/{versionID}": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看一条 system resource 的版本快照详情",
				"operationId": "getSystemResourceVersion",
				"parameters": append(pathIDParameter("id", "system resource ID"), map[string]any{
					"name":        "versionID",
					"in":          "path",
					"required":    true,
					"description": "版本快照 ID。",
					"schema":      map[string]any{"type": "string"},
				}),
				"responses": map[string]any{
					"200": jsonResponse("system resource version detail", "SystemResourceVersionDetail"),
					"404": jsonResponse("system resource version not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/versions/{versionID}/rollback": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "回滚一条 system resource 到指定版本并重新激活",
				"operationId": "rollbackSystemResourceVersion",
				"parameters": append(pathIDParameter("id", "system resource ID"), map[string]any{
					"name":        "versionID",
					"in":          "path",
					"required":    true,
					"description": "需要恢复的版本快照 ID。",
					"schema":      map[string]any{"type": "string"},
				}),
				"responses": map[string]any{
					"200": jsonResponse("system resource mutation result", "SystemResourceMutationResult"),
					"400": jsonResponse("invalid rollback request", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/audit": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看一条 system resource 的审计轨迹",
				"operationId": "listSystemResourceAudit",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource audit entries", "SystemResourceAuditResponse"),
					"404": jsonResponse("system resource not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/metadata": map[string]any{
			"patch": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "更新一条 system resource 的安全元数据",
				"operationId": "patchSystemResourceMetadata",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"requestBody": jsonRequest("SystemResourceMetadataPatch", true),
				"responses": map[string]any{
					"200": jsonResponse("system resource detail", "SystemResourceDetail"),
					"400": jsonResponse("invalid metadata patch", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/source": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "读取一条 system resource 的 source 内容",
				"operationId": "getSystemResourceSource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource source", "SystemResourceSource"),
					"404": jsonResponse("system resource not found", "ErrorResponse"),
				},
			},
			"put": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "替换一条 system resource 的 source 内容并重新触发 pipeline",
				"operationId": "putSystemResourceSource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"requestBody": jsonRequest("SystemResourceSource", true),
				"responses": map[string]any{
					"200": jsonResponse("system resource mutation result", "SystemResourceMutationResult"),
					"400": jsonResponse("invalid source payload", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/parse": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "重新执行 parse pipeline",
				"operationId": "parseSystemResource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource mutation result", "SystemResourceMutationResult"),
					"400": jsonResponse("invalid parse request", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/compile": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "重新执行 compile pipeline",
				"operationId": "compileSystemResource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource mutation result", "SystemResourceMutationResult"),
					"400": jsonResponse("invalid compile request", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/activate": map[string]any{
			"post": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "执行 activate pipeline",
				"operationId": "activateSystemResource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource mutation result", "SystemResourceMutationResult"),
					"400": jsonResponse("invalid activate request", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/pipeline": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看一条 system resource 的 pipeline 状态",
				"operationId": "getSystemResourcePipeline",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource pipeline", "SystemResourcePipeline"),
					"404": jsonResponse("system resource pipeline not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/parse-result": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看一条 system resource 的 parse 结果",
				"operationId": "getSystemResourceParseResult",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource parse result", "SystemResourceParseResult"),
					"404": jsonResponse("system resource parse result not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/compile-result": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "查看一条 system resource 的 compile 结果",
				"operationId": "getSystemResourceCompileResult",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": jsonResponse("system resource compile result", "SystemResourceCompileResult"),
					"404": jsonResponse("system resource compile result not found", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/debug-payload": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "生成一条 system resource 的调试请求载荷",
				"operationId": "getSystemResourceDebugPayload",
				"parameters": []map[string]any{
					pathIDParameter("id", "system resource ID")[0],
					queryStringParameter("endpoint", "目标调试接口，默认 /api/chat/respond。"),
				},
				"responses": map[string]any{
					"200": jsonResponse("system resource debug payload", "SystemResourceDebugPayload"),
					"400": jsonResponse("invalid debug payload request", "ErrorResponse"),
				},
			},
		},
		"/api/system-resources/{id}/download": map[string]any{
			"get": map[string]any{
				"tags":        []string{"control-plane"},
				"summary":     "下载一条 system resource 的 source 文件",
				"operationId": "downloadSystemResource",
				"parameters":  pathIDParameter("id", "system resource ID"),
				"responses": map[string]any{
					"200": map[string]any{"description": "markdown source payload"},
					"404": jsonResponse("system resource not found", "ErrorResponse"),
				},
			},
		},
	}
}

func buildSystemResourceOpenAPISchemas() map[string]any {
	return map[string]any{
		"SystemResourceSummary": objectSchema(map[string]any{
			"asset_id":          stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"asset_type":        stringSchema("资产类型。", "policy_rule"),
			"asset_name":        stringSchema("资产显示名称。", "Safety Constitution"),
			"scope":             stringSchema("资产作用域。", "system"),
			"source_kind":       stringSchema("source 来源标签。", "control_plane_upload"),
			"status":            stringSchema("当前状态。", "ready"),
			"truth_dir_version": stringSchema("当前 truth dir 版本。", "truth_20260422_100000"),
			"compiled_version":  stringSchema("当前编译版本。", "2026-04-22.100000"),
			"updated_at":        dateTimeSchema("最近更新时间。", "2026-04-22T10:00:03Z"),
			"read_only":         boolSchema("该资源是否只读。", false),
		}, []string{"asset_id", "asset_type"}),
		"SystemResourceDetail": objectSchema(map[string]any{
			"asset_id":          stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"asset_type":        stringSchema("资产类型。", "policy_rule"),
			"asset_name":        stringSchema("资产显示名称。", "Safety Constitution"),
			"scope":             stringSchema("资产作用域。", "system"),
			"source_kind":       stringSchema("source 来源标签。", "control_plane_upload"),
			"status":            stringSchema("当前状态。", "ready"),
			"truth_dir_version": stringSchema("当前 truth dir 版本。", "truth_20260422_100000"),
			"compiled_version":  stringSchema("当前编译版本。", "2026-04-22.100000"),
			"updated_at":        dateTimeSchema("最近更新时间。", "2026-04-22T10:00:03Z"),
			"read_only":         boolSchema("该资源是否只读。", false),
			"source_path":       stringSchema("truth dir 中的 source 文件路径。", "sources/core/policy_rule/safety_constitution.md"),
			"metadata":          map[string]any{"type": "object", "additionalProperties": true},
			"parse_result":      refSchema("SystemResourceParseResult"),
			"compile_result":    refSchema("SystemResourceCompileResult"),
			"pipeline":          refSchema("SystemResourcePipeline"),
		}, []string{"asset_id", "asset_type"}),
		"SystemResourceListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("SystemResourceSummary")),
		}, []string{"items"}),
		"SystemResourceVersionSummary": objectSchema(map[string]any{
			"version_id":        stringSchema("版本快照 ID。", "asset_20260422T100003.000000000Z"),
			"asset_id":          stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"action":            stringSchema("触发该版本的动作。", "activated"),
			"summary":           stringSchema("版本摘要。", "activated compiled system resource"),
			"created_at":        dateTimeSchema("版本创建时间。", "2026-04-22T10:00:03Z"),
			"truth_dir_version": stringSchema("快照对应 truth dir 版本。", "20260422T100003.000000000Z"),
			"compiled_version":  stringSchema("快照对应 compiled 版本。", "20260422T100003.000000000Z"),
			"source_checksum":   stringSchema("快照对应 source checksum。", "sha256:abcd"),
			"compiled_checksum": stringSchema("快照对应 compiled checksum。", "sha256:efgh"),
			"rolled_back_from":  stringSchema("若本次为回滚生成，则记录来源版本。", "asset_20260421T235959.000000000Z"),
		}, []string{"version_id", "asset_id"}),
		"SystemResourceVersionDetail": objectSchema(map[string]any{
			"version_id":        stringSchema("版本快照 ID。", "asset_20260422T100003.000000000Z"),
			"asset_id":          stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"action":            stringSchema("触发该版本的动作。", "activated"),
			"summary":           stringSchema("版本摘要。", "activated compiled system resource"),
			"created_at":        dateTimeSchema("版本创建时间。", "2026-04-22T10:00:03Z"),
			"truth_dir_version": stringSchema("快照对应 truth dir 版本。", "20260422T100003.000000000Z"),
			"compiled_version":  stringSchema("快照对应 compiled 版本。", "20260422T100003.000000000Z"),
			"source_checksum":   stringSchema("快照对应 source checksum。", "sha256:abcd"),
			"compiled_checksum": stringSchema("快照对应 compiled checksum。", "sha256:efgh"),
			"rolled_back_from":  stringSchema("若本次为回滚生成，则记录来源版本。", "asset_20260421T235959.000000000Z"),
			"resource":          refSchema("SystemResourceDetail"),
			"source_content":    stringSchema("该版本对应的 source 内容。", "---\nid: safety_constitution\nname: Safety Constitution\nsummary: Non-negotiable safety boundaries\nseverity: critical\ncheckpoints:\n  - pre_inference\n  - pre_tool_call\n  - pre_finalize\non_fail: deny\n---\n\n## Purpose\nProtect core safety boundaries."),
		}, []string{"version_id", "asset_id", "resource"}),
		"SystemResourceVersionListResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("SystemResourceVersionSummary")),
		}, []string{"items"}),
		"SystemResourceAuditEntry": objectSchema(map[string]any{
			"event_id":          stringSchema("审计事件 ID。", "asset_20260422T100003.000000000Z"),
			"asset_id":          stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"action":            stringSchema("审计动作。", "activated"),
			"summary":           stringSchema("审计摘要。", "activated compiled system resource"),
			"created_at":        dateTimeSchema("审计时间。", "2026-04-22T10:00:03Z"),
			"truth_dir_version": stringSchema("对应 truth dir 版本。", "20260422T100003.000000000Z"),
			"compiled_version":  stringSchema("对应 compiled 版本。", "20260422T100003.000000000Z"),
			"source_checksum":   stringSchema("对应 source checksum。", "sha256:abcd"),
			"compiled_checksum": stringSchema("对应 compiled checksum。", "sha256:efgh"),
			"rolled_back_from":  stringSchema("若为回滚事件，则记录来源版本。", "asset_20260421T235959.000000000Z"),
			"detail":            map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"event_id", "asset_id"}),
		"SystemResourceAuditResponse": objectSchema(map[string]any{
			"items": arraySchema(refSchema("SystemResourceAuditEntry")),
		}, []string{"items"}),
		"SystemResourceSource": objectSchema(map[string]any{
			"asset_id":       stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"source_content": stringSchema("当前 source 原文。", "---\nid: safety_constitution\nname: Safety Constitution\nsummary: Non-negotiable safety boundaries\nseverity: critical\ncheckpoints:\n  - pre_inference\n  - pre_tool_call\n  - pre_finalize\non_fail: deny\n---\n\n## Purpose\nProtect core safety boundaries."),
			"message":        stringSchema("本次 source 修改摘要。", "tighten core safety boundary wording"),
			"updated_at":     dateTimeSchema("最近更新时间。", "2026-04-22T10:00:03Z"),
		}, nil),
		"SystemResourceMetadataPatch": objectSchema(map[string]any{
			"asset_type":  stringSchema("新的资产类型。", "policy_rule"),
			"asset_name":  stringSchema("新的显示名称。", "Safety Constitution"),
			"scope":       stringSchema("新的作用域。", "system"),
			"source_kind": stringSchema("新的 source 来源标签。", "control_plane_upload"),
			"read_only":   boolSchema("是否切换为只读。", false),
			"metadata":    map[string]any{"type": "object", "additionalProperties": true},
		}, nil),
		"SystemResourceCreateRequest": objectSchema(map[string]any{
			"asset_id":       stringSchema("system resource 稳定标识。", "policy_rule.core.safety_constitution"),
			"asset_type":     stringSchema("资产类型。", "policy_rule"),
			"asset_name":     stringSchema("资产显示名称。", "Safety Constitution"),
			"scope":          stringSchema("资产作用域。", "system"),
			"source_kind":    stringSchema("source 来源标签。", "control_plane_upload"),
			"read_only":      boolSchema("该资源是否只读。", false),
			"source_content": stringSchema("初始化 source 原文。", "---\nid: safety_constitution\nname: Safety Constitution\nsummary: Non-negotiable safety boundaries\nseverity: critical\ncheckpoints:\n  - pre_inference\n  - pre_tool_call\n  - pre_finalize\non_fail: deny\n---\n\n## Purpose\nProtect core safety boundaries."),
			"message":        stringSchema("本次创建摘要。", "seed core safety policy"),
			"metadata":       map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"asset_id"}),
		"SystemResourceParseResult": objectSchema(map[string]any{
			"asset_id":    stringSchema("system resource 稳定标识。", "spec.issue7"),
			"status":      stringSchema("parse 状态。", "parsed"),
			"summary":     stringSchema("parse 提取的摘要。", "Issue 7 Rule Spec"),
			"warnings":    arraySchema(map[string]any{"type": "string"}),
			"errors":      arraySchema(map[string]any{"type": "string"}),
			"parsed":      map[string]any{"type": "object", "additionalProperties": true},
			"source_hash": stringSchema("source 内容 checksum。", "sha256:abcd"),
			"updated_at":  dateTimeSchema("最近更新时间。", "2026-04-22T10:00:03Z"),
		}, nil),
		"SystemResourceCompileResult": objectSchema(map[string]any{
			"asset_id":          stringSchema("system resource 稳定标识。", "spec.issue7"),
			"status":            stringSchema("compile 状态。", "compiled"),
			"summary":           stringSchema("编译摘要。", "Issue 7 Rule Spec"),
			"guidance_text":     stringSchema("运行时 guidance 文本。", "Rule spec Issue 7 ..."),
			"source_checksum":   stringSchema("source checksum。", "sha256:abcd"),
			"compiled_checksum": stringSchema("compiled checksum。", "sha256:efgh"),
			"compiled_version":  stringSchema("编译版本。", "2026-04-22.100000"),
			"truth_dir_version": stringSchema("truth dir 版本。", "truth_20260422_100000"),
			"payload":           map[string]any{"type": "object", "additionalProperties": true},
			"updated_at":        dateTimeSchema("最近更新时间。", "2026-04-22T10:00:03Z"),
		}, nil),
		"SystemResourcePipeline": objectSchema(map[string]any{
			"pipeline_id":      stringSchema("pipeline 稳定 ID。", "pipe_20260422_001"),
			"asset_id":         stringSchema("system resource 稳定标识。", "spec.issue7"),
			"status":           stringSchema("pipeline 状态。", "ready"),
			"current_step":     stringSchema("当前阶段。", "activate"),
			"progress_percent": intSchema("当前进度百分比。", 100),
			"started_at":       dateTimeSchema("开始时间。", "2026-04-22T10:00:00Z"),
			"updated_at":       dateTimeSchema("最近更新时间。", "2026-04-22T10:00:03Z"),
			"warnings":         arraySchema(map[string]any{"type": "string"}),
			"errors":           arraySchema(map[string]any{"type": "string"}),
		}, nil),
		"SystemResourceMutationResult": objectSchema(map[string]any{
			"asset_id": stringSchema("system resource 稳定标识。", "spec.issue7"),
			"accepted": boolSchema("本次变更是否被接受。", true),
			"pipeline": refSchema("SystemResourcePipeline"),
		}, []string{"accepted", "pipeline"}),
		"SystemResourceDebugPayload": objectSchema(map[string]any{
			"endpoint": stringSchema("目标调试接口。", "/api/chat/respond"),
			"payload":  map[string]any{"type": "object", "additionalProperties": true},
		}, []string{"endpoint", "payload"}),
		"DeleteSystemResourceResponse": objectSchema(map[string]any{
			"deleted":  boolSchema("该资源是否已删除。", true),
			"asset_id": stringSchema("被删除的 system resource ID。", "spec.issue7"),
		}, []string{"deleted", "asset_id"}),
	}
}
