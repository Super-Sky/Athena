// scene.go defines the scene-match and scene-switch suggestion contract used by scene runtime flows.
// scene.go 定义场景运行时流使用的场景命中和场景切换建议契约。
package scene

const (
	// HighConfidenceThreshold marks the minimum score that can produce a strong scene switch suggestion.
	// HighConfidenceThreshold 表示可以产生强场景切换建议的最低分数阈值。
	HighConfidenceThreshold = 80

	// WeakConfidenceThreshold marks the minimum score that can still produce a weak scene hint.
	// WeakConfidenceThreshold 表示仍可给出弱场景提示的最低分数阈值。
	WeakConfidenceThreshold = 60
)

// MatchStrength captures the external-facing strength bucket for one internal scene score.
// MatchStrength 描述某个内部场景评分对外对应的强度档位。
type MatchStrength string

const (
	// MatchStrengthHigh means the runtime can return a scene switch suggestion.
	// MatchStrengthHigh 表示运行时可以返回场景切换建议。
	MatchStrengthHigh MatchStrength = "high"

	// MatchStrengthWeak means the runtime only returns a weak hint or rationale.
	// MatchStrengthWeak 表示运行时只返回弱提示或理由说明。
	MatchStrengthWeak MatchStrength = "weak"

	// MatchStrengthNone means the runtime should not suggest a scene switch.
	// MatchStrengthNone 表示运行时不应建议切换场景。
	MatchStrengthNone MatchStrength = "none"
)

// MatchResult captures the internal scene scoring outcome used by later runtime stages.
// MatchResult 描述后续 runtime 阶段使用的内部场景评分结果。
type MatchResult struct {
	Scene               string        `json:"scene,omitempty"`
	Score               int           `json:"score,omitempty"`
	Strength            MatchStrength `json:"strength,omitempty"`
	Reason              string        `json:"reason,omitempty"`
	TargetAppInstanceID string        `json:"target_app_instance_id,omitempty"`
}

// SwitchSuggestion captures the transport-safe suggestion emitted after strict scene matching.
// SwitchSuggestion 描述严格场景命中后发出的 transport-safe 切换建议。
type SwitchSuggestion struct {
	TargetAppInstanceID string        `json:"target_app_instance_id,omitempty"`
	Strength            MatchStrength `json:"strength,omitempty"`
	Reason              string        `json:"reason,omitempty"`
}

// ClassifyScore maps one internal scene score into the repository's three-stage confidence buckets.
// ClassifyScore 会把内部场景评分映射到仓库三段式置信度档位。
func ClassifyScore(score int) MatchStrength {
	switch {
	case score >= HighConfidenceThreshold:
		return MatchStrengthHigh
	case score >= WeakConfidenceThreshold:
		return MatchStrengthWeak
	default:
		return MatchStrengthNone
	}
}

// BuildSwitchSuggestion returns a transport-safe switch suggestion from one scene score and target app.
// BuildSwitchSuggestion 会根据场景评分和目标 App 构建 transport-safe 的切换建议。
func BuildSwitchSuggestion(score int, targetAppInstanceID string, reason string) *SwitchSuggestion {
	strength := ClassifyScore(score)
	if strength == MatchStrengthNone {
		return nil
	}
	return &SwitchSuggestion{
		TargetAppInstanceID: targetAppInstanceID,
		Strength:            strength,
		Reason:              reason,
	}
}
