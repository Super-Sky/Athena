// source.go defines skill sources backed by the repository truth dir and uploaded packages.
// source.go 定义基于仓库 truth dir 与上传包的 skill source。
package skills

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"moss/internal/systemtruth"
)

// Source loads declaration-time skills from one origin such as repository truth sources or uploaded packages after adaptation.
// Source 负责从某个来源加载声明态 skill，例如仓库 truth source，或上传 skill 包经适配后的结果。
type Source interface {
	SourceName() string
	Load(context.Context) ([]Definition, error)
}

// TruthDirSource loads built-in skill definitions from the repository truth dir.
// TruthDirSource 负责从仓库 truth dir 读取内置 skill 定义。
type TruthDirSource struct {
	name     string
	truthDir string
}

// StaticSource serves a fixed set of declaration-time skills.
// StaticSource 负责提供一组固定的声明态 skill。
type StaticSource struct {
	name        string
	definitions []Definition
}

// NewStaticSource creates a fixed skill source with the provided name and definitions.
// NewStaticSource 会根据名称和 skill 列表创建一个固定 skill source。
func NewStaticSource(name string, defs []Definition) Source {
	return StaticSource{
		name:        name,
		definitions: cloneDefinitions(defs),
	}
}

// NewBuiltinSource creates the repository truth-backed built-in skill source.
// NewBuiltinSource 创建由仓库 truth dir 驱动的内置 skill source。
func NewBuiltinSource() Source {
	return NewBuiltinSourceWithTruthDir(systemtruth.DefaultTruthDir())
}

// NewBuiltinSourceWithTruthDir creates one built-in skill source for the provided truth dir.
// NewBuiltinSourceWithTruthDir 会基于给定 truth dir 创建一份内置 skill source。
func NewBuiltinSourceWithTruthDir(truthDir string) Source {
	return TruthDirSource{
		name:     "builtin_truth",
		truthDir: strings.TrimSpace(truthDir),
	}
}

// SourceName returns the stable name of the current skill source.
// SourceName 返回当前 skill source 的稳定名称。
func (s StaticSource) SourceName() string {
	return s.name
}

// Load returns a defensive copy of all definitions exposed by this source.
// Load 返回当前 source 暴露的全部 skill 定义副本。
func (s StaticSource) Load(context.Context) ([]Definition, error) {
	return cloneDefinitions(s.definitions), nil
}

// SourceName returns the stable name of the current truth-dir skill source.
// SourceName 返回当前 truth-dir skill source 的稳定名称。
func (s TruthDirSource) SourceName() string {
	return s.name
}

// Load reads all skill packages under scenes/*/skills from the truth source.
// Load 会从 truth source 的 scenes/*/skills 读取全部 skill package。
func (s TruthDirSource) Load(context.Context) ([]Definition, error) {
	sourcesRoot := systemtruth.SourcesRoot(s.truthDir)
	sceneRoot := filepath.Join(sourcesRoot, "scenes")
	sceneEntries, err := os.ReadDir(sceneRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defs := make([]Definition, 0)
	for _, sceneEntry := range sceneEntries {
		if !sceneEntry.IsDir() {
			continue
		}
		skillsRoot := filepath.Join(sceneRoot, sceneEntry.Name(), "skills")
		skillEntries, err := os.ReadDir(skillsRoot)
		if err != nil {
			continue
		}
		for _, skillEntry := range skillEntries {
			if !skillEntry.IsDir() {
				continue
			}
			docPath := filepath.Join(skillsRoot, skillEntry.Name(), "SKILL.md")
			doc, err := systemtruth.ReadMarkdownDocument(docPath)
			if err != nil {
				continue
			}
			sections := systemtruth.ParseMarkdownSections(doc.Body)
			name := firstNonEmpty(
				systemtruth.FrontmatterString(doc.Frontmatter, "id"),
				systemtruth.NormalizeID(skillEntry.Name()),
			)
			defs = append(defs, Definition{
				Name:        name,
				Description: firstNonEmpty(systemtruth.FrontmatterString(doc.Frontmatter, "description"), systemtruth.FrontmatterString(doc.Frontmatter, "summary")),
				ToolNames:   systemtruth.CompactStrings(systemtruth.FrontmatterStrings(doc.Frontmatter, "allowed_tools")),
				Guidance:    buildSkillGuidance(doc, sections),
			})
		}
	}
	sort.Slice(defs, func(i, j int) bool { return defs[i].Name < defs[j].Name })
	return defs, nil
}

func buildSkillGuidance(doc systemtruth.MarkdownDocument, sections map[string]systemtruth.MarkdownSection) string {
	parts := []string{
		firstNonEmpty(systemtruth.FrontmatterString(doc.Frontmatter, "summary"), systemtruth.FrontmatterString(doc.Frontmatter, "description")),
		systemtruth.SectionText(sections, "when_to_use"),
		systemtruth.SectionText(sections, "process"),
		systemtruth.SectionText(sections, "output"),
	}
	return strings.TrimSpace(strings.Join(systemtruth.CompactStrings(parts), "\n"))
}

func cloneDefinitions(defs []Definition) []Definition {
	cloned := make([]Definition, 0, len(defs))
	for _, def := range defs {
		cloned = append(cloned, cloneDefinition(def))
	}
	return cloned
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
