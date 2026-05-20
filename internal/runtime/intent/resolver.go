// resolver.go resolves one request into a stable interaction mode and selected route.
// resolver.go 负责把一次请求解析成稳定的交互模式与目标路由。
package intent

import "strings"

// Resolve returns the canonical interaction resolution for one request.
// Resolve 返回一次请求的标准交互解析结果。
func Resolve(ctx Context) Resolution {
	if ctx.Scene == "" {
		ctx.Scene = "default"
	}
	if selected := strings.TrimSpace(string(ctx.UserSelectedMode)); selected != "" {
		switch UserSelectedMode(selected) {
		case UserSelectedModeAutomationDraft:
			return Resolution{
				InteractionMode:       InteractionModeAutomationDraft,
				Scene:                 ctx.Scene,
				PrimarySkill:          strings.TrimSpace(ctx.PrimarySkill),
				AllowedTools:          append([]string(nil), ctx.AllowedTools...),
				SelectedRoute:         "automation_draft",
				RequiresClarification: false,
				Reason:                "the user explicitly selected the automation draft route",
			}
		case UserSelectedModeChat:
			return Resolution{
				InteractionMode:       InteractionModeChat,
				Scene:                 ctx.Scene,
				PrimarySkill:          strings.TrimSpace(ctx.PrimarySkill),
				AllowedTools:          append([]string(nil), ctx.AllowedTools...),
				SelectedRoute:         "chat",
				RequiresClarification: false,
				Reason:                "the user explicitly selected the normal chat route",
			}
		}
	}

	switch strings.TrimSpace(string(ctx.EntryMode)) {
	case string(EntryModeAutomationCreate), string(EntryModeAutomationConfirm):
		return Resolution{
			InteractionMode:       InteractionModeAutomationDraft,
			Scene:                 ctx.Scene,
			PrimarySkill:          "automation_planner",
			AllowedTools:          append([]string(nil), ctx.AllowedTools...),
			SelectedRoute:         "automation_draft",
			RequiresClarification: false,
			Reason:                "platform explicitly routed this request through the automation entry",
		}
	case string(EntryModeResultExplanation):
		return Resolution{
			InteractionMode:       InteractionModeResultExplanation,
			Scene:                 ctx.Scene,
			PrimarySkill:          strings.TrimSpace(ctx.PrimarySkill),
			AllowedTools:          append([]string(nil), ctx.AllowedTools...),
			SelectedRoute:         "result_explanation",
			RequiresClarification: false,
			Reason:                "platform explicitly routed this request as a result explanation interaction",
		}
	}

	if shouldRouteToAutomationDraft(ctx) {
		mode := InteractionModeAutomationDraft
		reason := "the request explicitly asks Athena to produce an automation draft before any execution or creation"
		if asksUserToChooseRoute(ctx.Query) || (looksLikeAutomationIntent(ctx.Query) && !hasStrongAutomationDraftSignal(ctx.Query)) {
			mode = InteractionModeChoiceRequired
			reason = "user explicitly asks to choose between continuing discussion and generating an automation draft"
		}
		return Resolution{
			InteractionMode:       mode,
			Scene:                 ctx.Scene,
			PrimarySkill:          "automation_planner",
			AllowedTools:          append([]string(nil), ctx.AllowedTools...),
			SelectedRoute:         selectedRouteForMode(mode),
			RequiresClarification: mode == InteractionModeChoiceRequired || mode == InteractionModeClarification,
			Reason:                reason,
		}
	}

	if looksLikeAutomationIntent(ctx.Query) {
		mode := InteractionModeChoiceRequired
		reason := "the request may mean either a normal discussion or creating an automation draft, so Athena needs the user to choose the route"
		if asksUserToChooseRoute(ctx.Query) {
			reason = "user explicitly asks to choose between continuing discussion and generating an automation draft"
		} else if hasStrongAutomationDraftSignal(ctx.Query) {
			mode = InteractionModeAutomationDraft
			reason = "the request clearly describes a recurring automation draft and confirmation-before-execution interaction"
		}
		return Resolution{
			InteractionMode:       mode,
			Scene:                 ctx.Scene,
			PrimarySkill:          "automation_planner",
			AllowedTools:          append([]string(nil), ctx.AllowedTools...),
			SelectedRoute:         selectedRouteForMode(mode),
			RequiresClarification: mode == InteractionModeChoiceRequired,
			Reason:                reason,
		}
	}

	return Resolution{
		InteractionMode:       InteractionModeChat,
		Scene:                 ctx.Scene,
		PrimarySkill:          strings.TrimSpace(ctx.PrimarySkill),
		AllowedTools:          append([]string(nil), ctx.AllowedTools...),
		SelectedRoute:         "chat",
		RequiresClarification: false,
		Reason:                "the request should continue through the normal chat interaction path",
	}
}

// BuildOptions returns stable route options when the interaction mode requires user choice.
// BuildOptions 会在交互模式需要用户选择时返回稳定候选路由项。
func BuildOptions(resolution Resolution) []Option {
	if resolution.InteractionMode != InteractionModeChoiceRequired {
		return nil
	}
	return []Option{
		{
			ID:          "continue_chat",
			Title:       "继续普通讨论",
			Description: "先继续澄清目标和范围，不生成自动化草案。",
		},
		{
			ID:          "automation_draft",
			Title:       "生成自动化草案",
			Description: "生成待确认的自动化计划草案，不直接创建或执行自动化。",
		},
	}
}

func shouldRouteToAutomationDraft(ctx Context) bool {
	if strings.EqualFold(strings.TrimSpace(ctx.DesiredOutputMode), "automation_plan_draft") {
		return true
	}
	return strings.TrimSpace(ctx.TaskType) == "scheduled_job" && strings.TrimSpace(ctx.AutomationTaskID) == ""
}

func looksLikeAutomationIntent(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return false
	}
	return containsAny(normalized,
		"自动化", "automation", "schedule", "scheduled", "recurring", "periodic",
		"每天", "每周", "每日", "每月", "按周期", "周期", "定时", "daily", "weekly", "monthly")
}

func hasStrongAutomationDraftSignal(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return false
	}
	hasDraft := containsAny(normalized, "草案", "draft", "plan", "方案", "设计")
	hasConfirm := containsAny(normalized, "确认后", "确认之后", "先确认", "confirm", "不直接执行", "不要直接执行")
	hasRecurring := containsAny(normalized, "每天", "每周", "每日", "每月", "按周期", "周期", "定时", "daily", "weekly", "monthly", "schedule", "scheduled", "automation", "自动化")
	return hasRecurring && (hasDraft || hasConfirm)
}

func asksUserToChooseRoute(query string) bool {
	normalized := strings.ToLower(strings.TrimSpace(query))
	if normalized == "" {
		return false
	}
	return containsAny(normalized,
		"还没决定", "先帮我判断", "还是直接", "还是先继续", "继续聊细节", "继续讨论还是",
		"choose", "decide whether", "continue discussing or", "or directly generate")
}

func selectedRouteForMode(mode InteractionMode) string {
	switch mode {
	case InteractionModeAutomationDraft, InteractionModeChoiceRequired:
		return "automation_draft"
	case InteractionModeResultExplanation:
		return "result_explanation"
	default:
		return "chat"
	}
}

func containsAny(input string, patterns ...string) bool {
	for _, pattern := range patterns {
		if strings.Contains(input, pattern) {
			return true
		}
	}
	return false
}
