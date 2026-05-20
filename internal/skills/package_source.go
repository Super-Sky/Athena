// package_source.go loads uploaded skill packages from the package store into runtime-visible definitions.
// package_source.go 负责把上传 skill package 从 package store 加载成 runtime 可见定义。
package skills

import (
	"context"
	"fmt"
	"strings"
)

// PackageAdapter converts one uploaded official skill package into a declaration-time skill definition.
// PackageAdapter 负责把一份上传的官方 skill 包转换为声明态 skill 定义。
type PackageAdapter interface {
	AdaptPackage(Package) (Definition, error)
}

// DefaultPackageAdapter extracts one declaration-time skill from the uploaded SKILL.md bundle.
// DefaultPackageAdapter 会从上传包里的 SKILL.md 提取一条声明态 skill。
type DefaultPackageAdapter struct{}

// NewPackageAdapter creates the scaffold's default uploaded-package adapter.
// NewPackageAdapter 创建当前脚手架默认使用的上传包适配器。
func NewPackageAdapter() PackageAdapter {
	return DefaultPackageAdapter{}
}

// PackageSource loads uploaded skill packages from the package store and adapts them into definitions.
// PackageSource 会从 package store 加载上传 skill 包，并把它们适配成声明态 skill。
type PackageSource struct {
	store   PackageStore
	adapter PackageAdapter
}

// NewPackageSource creates a skill source backed by uploaded official skill packages.
// NewPackageSource 创建一个基于上传官方 skill 包的 skill source。
func NewPackageSource(store PackageStore, adapter PackageAdapter) Source {
	return PackageSource{
		store:   store,
		adapter: adapter,
	}
}

// SourceName returns the stable name of the uploaded package source.
// SourceName 返回上传包 source 的稳定名称。
func (s PackageSource) SourceName() string {
	return "uploaded"
}

// Load adapts all uploaded skill packages from the package store into definitions.
// Load 会把 package store 中的全部上传 skill 包适配为声明态 skill。
func (s PackageSource) Load(ctx context.Context) ([]Definition, error) {
	if s.store == nil || s.adapter == nil {
		return nil, nil
	}
	metadata, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	defs := make([]Definition, 0, len(metadata))
	for _, item := range metadata {
		pkg, ok, err := s.store.Get(ctx, item.ID)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		if !pkg.Enabled {
			continue
		}
		def, err := s.adapter.AdaptPackage(pkg)
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return defs, nil
}

// AdaptPackage extracts name, description, and guidance from one uploaded official skill package.
// AdaptPackage 会从上传的官方 skill 包中提取名称、描述和 guidance。
func (DefaultPackageAdapter) AdaptPackage(pkg Package) (Definition, error) {
	skillDoc, ok := pkg.Files["SKILL.md"]
	if !ok {
		return Definition{}, fmt.Errorf("uploaded skill package %q is missing SKILL.md", pkg.Name)
	}
	name, description, body := parseSkillMarkdown(string(skillDoc))
	if strings.TrimSpace(name) == "" {
		name = strings.TrimSpace(pkg.Name)
	}
	if strings.TrimSpace(name) == "" {
		return Definition{}, fmt.Errorf("uploaded skill package is missing a skill name")
	}
	return Definition{
		Name:        name,
		Description: description,
		Guidance:    body,
	}, nil
}

func parseSkillMarkdown(content string) (name string, description string, body string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return "", "", ""
	}

	lines := strings.Split(trimmed, "\n")
	start := 0
	if len(lines) > 0 && strings.TrimSpace(lines[0]) == "---" {
		end := -1
		for idx := 1; idx < len(lines); idx++ {
			if strings.TrimSpace(lines[idx]) == "---" {
				end = idx
				break
			}
		}
		if end > 0 {
			for _, line := range lines[1:end] {
				key, value, ok := strings.Cut(line, ":")
				if !ok {
					continue
				}
				key = strings.TrimSpace(strings.ToLower(key))
				value = strings.TrimSpace(strings.Trim(value, `"'`))
				switch key {
				case "name":
					name = value
				case "description":
					description = value
				}
			}
			start = end + 1
		}
	}

	bodyLines := make([]string, 0, len(lines[start:]))
	for _, line := range lines[start:] {
		trimmedLine := strings.TrimSpace(line)
		if strings.HasPrefix(trimmedLine, "#") {
			trimmedLine = strings.TrimSpace(strings.TrimLeft(trimmedLine, "#"))
			if name == "" && trimmedLine != "" {
				name = trimmedLine
			}
			continue
		}
		bodyLines = append(bodyLines, line)
	}

	body = strings.TrimSpace(strings.Join(bodyLines, "\n"))
	if description == "" {
		for _, paragraph := range strings.Split(body, "\n\n") {
			paragraph = strings.TrimSpace(paragraph)
			if paragraph != "" {
				description = paragraph
				break
			}
		}
	}
	return name, description, body
}
