// customization.go defines request-level customization inputs used by app and runtime orchestration.
// customization.go 定义 app 和 runtime 编排使用的请求级 customization 输入。
package customization

import "moss/internal/contextassets"

// UserCustomization captures the request-level prompt, skill, and tool customization knobs.
// UserCustomization 描述请求级 prompt、skill、tool 以及 context asset binding 的自定义开关。
type UserCustomization struct {
	PromptTemplate         string
	EnabledSkills          []string
	EnabledTools           []string
	ContextAssetOverrides  []contextassets.Asset
	DisabledAssetTypes     []string
	AssetPriorityOverrides map[string]int
}

// DefaultUserCustomization returns the empty customization baseline used by the repository.
// DefaultUserCustomization 返回仓库默认使用的空 customization 基线。
func DefaultUserCustomization() UserCustomization {
	return UserCustomization{}
}
