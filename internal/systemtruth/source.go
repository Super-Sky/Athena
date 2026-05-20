// source.go defines shared helpers for parsing the repository's single source of truth.
// source.go 定义仓库单一真相主源使用的共享解析辅助方法。
package systemtruth

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	defaultTruthDir   = "config/system/truth"
	systemSourcesDir  = "sources"
	frontmatterFence  = "---"
)

// MarkdownDocument captures one markdown source file with parsed frontmatter.
// MarkdownDocument 描述一份带 frontmatter 的 markdown 主源文件。
type MarkdownDocument struct {
	Path        string
	Frontmatter map[string]any
	Body        string
}

// MarkdownSection captures one normalized markdown section.
// MarkdownSection 描述一段标准化后的 markdown section。
type MarkdownSection struct {
	Title   string
	Content []string
}

// DefaultTruthDir returns the repository default truth dir.
// DefaultTruthDir 返回仓库默认的 truth dir。
func DefaultTruthDir() string {
	if filepath.IsAbs(defaultTruthDir) {
		return defaultTruthDir
	}
	if _, currentFile, _, ok := runtime.Caller(0); ok {
		repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
		candidate := filepath.Join(repoRoot, defaultTruthDir)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	if candidate := resolveTruthDirFromWorkingTree(); candidate != "" {
		return candidate
	}
	return defaultTruthDir
}

func resolveTruthDirFromWorkingTree() string {
	wd, err := os.Getwd()
	if err != nil {
		return ""
	}
	current := filepath.Clean(wd)
	for {
		if _, err := os.Stat(filepath.Join(current, "go.mod")); err == nil {
			candidate := filepath.Join(current, defaultTruthDir)
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
		parent := filepath.Dir(current)
		if parent == current {
			return ""
		}
		current = parent
	}
}

// SourcesRoot returns the normalized sources root for one truth dir.
// SourcesRoot 返回某个 truth dir 对应的标准 sources 根目录。
func SourcesRoot(truthDir string) string {
	root := strings.TrimSpace(truthDir)
	if root == "" {
		root = defaultTruthDir
	}
	return filepath.Join(root, systemSourcesDir)
}

// NormalizeID normalizes user-maintained identifiers into repository snake_case.
// NormalizeID 会把用户维护的标识统一规整成仓库使用的 snake_case。
func NormalizeID(input string) string {
	replacer := strings.NewReplacer(
		" ", "_",
		"-", "_",
		"/", "_",
		"\\", "_",
		".", "_",
		":", "_",
		"：", "_",
	)
	value := replacer.Replace(strings.TrimSpace(strings.ToLower(input)))
	value = strings.Trim(value, "_")
	for strings.Contains(value, "__") {
		value = strings.ReplaceAll(value, "__", "_")
	}
	return value
}

// ReadMarkdownDocument loads one markdown source and parses optional YAML frontmatter.
// ReadMarkdownDocument 会读取一份 markdown 主源并解析可选 YAML frontmatter。
func ReadMarkdownDocument(path string) (MarkdownDocument, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return MarkdownDocument{}, err
	}
	doc := ParseMarkdownDocumentText(path, string(payload))
	return doc, nil
}

// ParseMarkdownDocumentText parses one markdown source body with optional frontmatter.
// ParseMarkdownDocumentText 会解析一段带可选 frontmatter 的 markdown 文本。
func ParseMarkdownDocumentText(path string, raw string) MarkdownDocument {
	text := strings.ReplaceAll(raw, "\r\n", "\n")
	doc := MarkdownDocument{Path: path, Body: strings.TrimSpace(text)}
	if !strings.HasPrefix(text, frontmatterFence+"\n") {
		return doc
	}
	rest := strings.TrimPrefix(text, frontmatterFence+"\n")
	end := strings.Index(rest, "\n"+frontmatterFence)
	if end < 0 {
		return doc
	}
	header := strings.TrimSpace(rest[:end])
	body := strings.TrimSpace(strings.TrimPrefix(rest[end:], "\n"+frontmatterFence))
	var meta map[string]any
	if header != "" {
		if err := yaml.Unmarshal([]byte(header), &meta); err != nil {
			return MarkdownDocument{Path: path, Body: strings.TrimSpace(text)}
		}
	}
	doc.Frontmatter = meta
	doc.Body = strings.TrimSpace(body)
	return doc
}

// ParseMarkdownSections parses markdown body into normalized section blocks.
// ParseMarkdownSections 会把 markdown 正文解析成标准化 section 块。
func ParseMarkdownSections(body string) map[string]MarkdownSection {
	result := map[string]MarkdownSection{}
	currentKey := "__root__"
	current := MarkdownSection{Title: "__root__"}
	for _, raw := range strings.Split(body, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			if len(current.Content) > 0 || current.Title != "__root__" {
				result[currentKey] = current
			}
			title := strings.TrimSpace(strings.TrimLeft(line, "#"))
			currentKey = NormalizeID(title)
			current = MarkdownSection{Title: title}
			continue
		}
		current.Content = append(current.Content, line)
	}
	if len(current.Content) > 0 || current.Title != "__root__" {
		result[currentKey] = current
	}
	return result
}

// SectionText returns the first matching section as one joined line.
// SectionText 返回首个命中的 section 文本。
func SectionText(sections map[string]MarkdownSection, keys ...string) string {
	lines := SectionContent(sections, keys...)
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, " ")
}

// SectionContent returns the first matching section content.
// SectionContent 返回首个命中的 section 内容。
func SectionContent(sections map[string]MarkdownSection, keys ...string) []string {
	for _, key := range keys {
		if section, ok := sections[NormalizeID(key)]; ok && len(section.Content) > 0 {
			return append([]string(nil), section.Content...)
		}
	}
	return nil
}

// SectionBullets extracts bullet-like lines from one section.
// SectionBullets 会从 section 中提取 bullet 风格行。
func SectionBullets(sections map[string]MarkdownSection, keys ...string) []string {
	lines := SectionContent(sections, keys...)
	if len(lines) == 0 {
		return nil
	}
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		switch {
		case strings.HasPrefix(line, "- "), strings.HasPrefix(line, "* "):
			result = append(result, strings.TrimSpace(line[2:]))
		case numberedPrefixLength(line) > 0:
			result = append(result, strings.TrimSpace(line[numberedPrefixLength(line):]))
		}
	}
	if len(result) == 0 {
		result = append(result, lines...)
	}
	return CompactStrings(result)
}

// CompactStrings trims, deduplicates, and drops empty values.
// CompactStrings 会去空白、去重并丢弃空字符串。
func CompactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, item := range values {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if _, exists := seen[item]; exists {
			continue
		}
		seen[item] = struct{}{}
		result = append(result, item)
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func numberedPrefixLength(line string) int {
	for idx, ch := range line {
		if ch < '0' || ch > '9' {
			if ch == '.' && idx > 0 && idx+1 < len(line) && line[idx+1] == ' ' {
				return idx + 2
			}
			return 0
		}
	}
	return 0
}

// ReadYAMLMap loads one YAML source into a generic map.
// ReadYAMLMap 会把一份 YAML 主源读取成通用 map。
func ReadYAMLMap(path string) (map[string]any, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var result map[string]any
	if err := yaml.Unmarshal(payload, &result); err != nil {
		return nil, fmt.Errorf("parse yaml %s: %w", path, err)
	}
	return result, nil
}

// FrontmatterString returns one trimmed string field.
// FrontmatterString 返回一个去空白的 frontmatter 字段。
func FrontmatterString(meta map[string]any, key string) string {
	if len(meta) == 0 {
		return ""
	}
	value, ok := meta[strings.TrimSpace(key)]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", typed))
	}
}

// FrontmatterStrings returns one normalized string slice field.
// FrontmatterStrings 返回一个规整后的字符串切片字段。
func FrontmatterStrings(meta map[string]any, key string) []string {
	if len(meta) == 0 {
		return nil
	}
	value, ok := meta[strings.TrimSpace(key)]
	if !ok || value == nil {
		return nil
	}
	result := make([]string, 0)
	switch typed := value.(type) {
	case []any:
		for _, item := range typed {
			text := strings.TrimSpace(fmt.Sprintf("%v", item))
			if text != "" {
				result = append(result, text)
			}
		}
	case []string:
		for _, item := range typed {
			item = strings.TrimSpace(item)
			if item != "" {
				result = append(result, item)
			}
		}
	case string:
		if text := strings.TrimSpace(typed); text != "" {
			result = append(result, text)
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

// ListSceneIDs returns all scene directory ids in stable order.
// ListSceneIDs 返回全部场景目录 id，并保持稳定顺序。
func ListSceneIDs(sourcesRoot string) ([]string, error) {
	root := filepath.Join(sourcesRoot, "scenes")
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		result = append(result, NormalizeID(entry.Name()))
	}
	sort.Strings(result)
	return result, nil
}
