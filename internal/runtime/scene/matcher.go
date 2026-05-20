// matcher.go implements the repository's first-phase strict scene matching heuristics.
// matcher.go 实现仓库第一阶段严格场景命中启发式逻辑。
package scene

import "strings"

// MatchInput captures the fields used by the first-phase strict scene matcher.
// MatchInput 描述第一阶段严格场景命中使用的输入字段。
type MatchInput struct {
	TaskType      string
	Query         string
	AppInstanceID string
}

// Match performs strict scene matching and returns the internal scene result.
// Match 执行严格场景命中，并返回内部场景结果。
func Match(input MatchInput) MatchResult {
	return MatchWithCatalog(BuiltinCatalog(), input)
}

// MatchWithCatalog performs strict scene matching against the provided scene catalog.
// MatchWithCatalog 会基于给定场景目录执行严格场景命中。
func MatchWithCatalog(catalog []Definition, input MatchInput) MatchResult {
	query := strings.ToLower(strings.TrimSpace(input.Query))
	// These task types are compatibility hints until Phase 1+ introduces registered task semantics.
	// 这些任务类型在 Phase 1+ 引入注册式语义前仅作为兼容提示使用。
	switch strings.TrimSpace(input.TaskType) {
	case "inspection_task":
		return MatchResult{Scene: "inspection", Score: 90, Strength: MatchStrengthHigh, Reason: "legacy task_type inspection_task hints inspection scene"}
	case "integration_event":
		return MatchResult{Scene: "alerts", Score: 90, Strength: MatchStrengthHigh, Reason: "legacy task_type integration_event hints alerts scene"}
	case "scheduled_job":
		return MatchResult{Scene: "workflow", Score: 90, Strength: MatchStrengthHigh, Reason: "legacy task_type scheduled_job hints workflow scene"}
	case "workflow_step_request":
		return MatchResult{Scene: "workflow", Score: 90, Strength: MatchStrengthHigh, Reason: "legacy task_type workflow_step_request hints workflow scene"}
	}
	if strings.TrimSpace(input.AppInstanceID) != "" {
		return MatchResult{
			Scene:               "application_dialogue",
			Score:               90,
			Strength:            MatchStrengthHigh,
			Reason:              "explicit app instance provided",
			TargetAppInstanceID: strings.TrimSpace(input.AppInstanceID),
		}
	}
	for _, item := range catalog {
		if !item.Enabled || len(item.Keywords) == 0 {
			continue
		}
		if containsAny(query, item.Keywords...) {
			score := item.MatchScore
			if score <= 0 {
				score = 70
			}
			return MatchResult{
				Scene:    item.ID,
				Score:    score,
				Strength: ClassifyScore(score),
				Reason:   item.ID + " keywords matched strictly",
			}
		}
	}
	return MatchResult{Scene: "default", Score: 0, Strength: MatchStrengthNone, Reason: "no strict scene match"}
}

func containsAny(query string, keywords ...string) bool {
	for _, keyword := range keywords {
		if strings.Contains(query, strings.ToLower(strings.TrimSpace(keyword))) {
			return true
		}
	}
	return false
}
