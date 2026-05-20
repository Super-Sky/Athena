// catalog.go defines the runtime scene catalog loaded from the repository truth source.
// catalog.go 定义从仓库真相主源加载的 runtime scene catalog。
package scene

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"moss/internal/systemtruth"
)

// Definition captures one runtime scene plus its stable metadata and routing hints.
// Definition 描述一个 runtime 场景及其稳定元数据和路由提示。
type Definition struct {
	ID                 string   `json:"id"`
	Description        string   `json:"description,omitempty"`
	Keywords           []string `json:"keywords,omitempty"`
	DefaultSkills      []string `json:"default_skills,omitempty"`
	SuggestedQuestions []string `json:"suggested_questions,omitempty"`
	Enabled            bool     `json:"enabled"`
	MatchScore         int      `json:"match_score,omitempty"`
}

var sceneSourcesRoot string

// SetSourcesRoot overrides the scene sources root used by BuiltinCatalog.
// SetSourcesRoot 用于覆盖 BuiltinCatalog 使用的 scene 主源目录。
func SetSourcesRoot(path string) {
	sceneSourcesRoot = strings.TrimSpace(path)
}

// BuiltinCatalog returns the repository scene catalog derived from the single truth source.
// BuiltinCatalog 返回从单一真相主源派生出的仓库 scene catalog。
func BuiltinCatalog() []Definition {
	root := strings.TrimSpace(sceneSourcesRoot)
	if root == "" {
		root = systemtruth.SourcesRoot(systemtruth.DefaultTruthDir())
	}
	items, err := loadCatalogFromSources(root)
	if err != nil {
		return nil
	}
	return items
}

// MergeCatalog overlays control-plane scene definitions onto one base catalog.
// MergeCatalog 会把控制面场景定义叠加到一个基础目录之上。
func MergeCatalog(base []Definition, overrides []Definition) []Definition {
	if len(overrides) == 0 {
		return cloneDefinitions(base)
	}
	result := cloneDefinitions(base)
	index := make(map[string]int, len(result))
	for i, item := range result {
		index[item.ID] = i
	}
	for _, override := range overrides {
		override.ID = strings.TrimSpace(override.ID)
		if override.ID == "" {
			continue
		}
		override.Keywords = compactStrings(override.Keywords)
		override.DefaultSkills = compactStrings(override.DefaultSkills)
		override.SuggestedQuestions = compactStrings(override.SuggestedQuestions)
		if pos, ok := index[override.ID]; ok {
			result[pos] = override
			continue
		}
		index[override.ID] = len(result)
		result = append(result, override)
	}
	return result
}

// FindDefinition looks up one scene definition by id.
// FindDefinition 会按 id 查找单条场景定义。
func FindDefinition(defs []Definition, id string) (Definition, bool) {
	trimmed := strings.TrimSpace(id)
	for _, item := range defs {
		if strings.TrimSpace(item.ID) == trimmed {
			return item, true
		}
	}
	return Definition{}, false
}

func loadCatalogFromSources(sourcesRoot string) ([]Definition, error) {
	sceneRoot := filepath.Join(strings.TrimSpace(sourcesRoot), "scenes")
	entries, err := os.ReadDir(sceneRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	result := make([]Definition, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		sceneID := systemtruth.NormalizeID(entry.Name())
		docPath := filepath.Join(sceneRoot, entry.Name(), "SCENE.md")
		doc, err := systemtruth.ReadMarkdownDocument(docPath)
		if err != nil {
			continue
		}
		sections := systemtruth.ParseMarkdownSections(doc.Body)
		result = append(result, Definition{
			ID:                 sceneID,
			Description:        firstNonEmpty(systemtruth.FrontmatterString(doc.Frontmatter, "summary"), systemtruth.SectionText(sections, "purpose"), systemtruth.SectionText(sections, "primary_outcome")),
			Keywords:           sceneKeywords(doc, sections),
			DefaultSkills:      skillNamesFromRefs(systemtruth.SectionBullets(sections, "default_assets")),
			SuggestedQuestions: firstNStrings(systemtruth.SectionBullets(sections, "examples"), 3),
			Enabled:            true,
			MatchScore:         defaultSceneMatchScore(sceneID),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].MatchScore == result[j].MatchScore {
			return result[i].ID < result[j].ID
		}
		return result[i].MatchScore > result[j].MatchScore
	})
	return result, nil
}

func sceneKeywords(doc systemtruth.MarkdownDocument, sections map[string]systemtruth.MarkdownSection) []string {
	parts := []string{
		systemtruth.FrontmatterString(doc.Frontmatter, "id"),
		systemtruth.FrontmatterString(doc.Frontmatter, "name"),
	}
	parts = append(parts, systemtruth.SectionBullets(sections, "when_it_applies")...)
	parts = append(parts, systemtruth.SectionBullets(sections, "examples")...)

	replacer := strings.NewReplacer("，", ",", "。", ",", "；", ",", "、", ",", "/", ",", "(", ",", ")", ",", "（", ",", "）", ",", "“", "", "”", "", "‘", "", "’", "", ":", ",", "：", ",")
	var keywords []string
	for _, part := range parts {
		part = strings.TrimSpace(replacer.Replace(part))
		if part == "" {
			continue
		}
		for _, token := range strings.Split(part, ",") {
			token = strings.TrimSpace(token)
			if token == "" {
				continue
			}
			keywords = append(keywords, token)
		}
	}
	return systemtruth.CompactStrings(keywords)
}

func cloneDefinitions(defs []Definition) []Definition {
	result := make([]Definition, 0, len(defs))
	for _, item := range defs {
		result = append(result, Definition{
			ID:                 item.ID,
			Description:        item.Description,
			Keywords:           append([]string(nil), item.Keywords...),
			DefaultSkills:      append([]string(nil), item.DefaultSkills...),
			SuggestedQuestions: append([]string(nil), item.SuggestedQuestions...),
			Enabled:            item.Enabled,
			MatchScore:         item.MatchScore,
		})
	}
	return result
}

func compactStrings(values []string) []string {
	return systemtruth.CompactStrings(values)
}

func skillNamesFromRefs(items []string) []string {
	result := make([]string, 0, len(items))
	for _, item := range items {
		item = strings.TrimSpace(item)
		if !strings.HasPrefix(item, "skill.") {
			continue
		}
		parts := strings.Split(item, ".")
		if len(parts) < 3 {
			continue
		}
		result = append(result, parts[len(parts)-1])
	}
	return systemtruth.CompactStrings(result)
}

func firstNonEmpty(values ...string) string {
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item != "" {
			return item
		}
	}
	return ""
}

func firstNStrings(values []string, limit int) []string {
	values = systemtruth.CompactStrings(values)
	if len(values) <= limit {
		return values
	}
	return append([]string(nil), values[:limit]...)
}

func defaultSceneMatchScore(sceneID string) int {
	switch strings.TrimSpace(sceneID) {
	case "default":
		return 10
	case "application_dialogue":
		return 65
	case "knowledge":
		return 72
	case "workflow":
		return 82
	case "alerts", "inspection":
		return 85
	case "security_review":
		return 86
	default:
		return 60
	}
}
