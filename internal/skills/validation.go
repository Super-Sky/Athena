// validation.go validates uploaded skill packages before they become runtime-visible.
// validation.go 负责在 uploaded skill package 变成 runtime 可见前进行校验。
package skills

import (
	"fmt"
	"path"
	"strings"
)

// ValidatePackage applies the scaffold's current governance rules to one uploaded skill package.
// ValidatePackage 会对一份上传 skill 包执行当前脚手架的治理校验规则。
func ValidatePackage(pkg Package, adapter PackageAdapter) ValidationResult {
	result := ValidationResult{Valid: true}
	if adapter == nil {
		result.Valid = false
		result.Errors = append(result.Errors, "package adapter is not configured")
		return result
	}
	if len(pkg.Files) == 0 {
		result.Valid = false
		result.Errors = append(result.Errors, "package bundle is empty")
		return result
	}

	allowedRoots := map[string]struct{}{
		"SKILL.md":   {},
		"scripts":    {},
		"references": {},
		"assets":     {},
	}
	for filePath := range pkg.Files {
		normalized := strings.TrimSpace(filePath)
		if normalized == "" {
			result.Valid = false
			result.Errors = append(result.Errors, "package contains an empty file path")
			continue
		}
		if strings.HasPrefix(normalized, "/") || strings.Contains(normalized, `\`) {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("package contains an unsupported file path %q", normalized))
			continue
		}
		cleaned := path.Clean(normalized)
		if cleaned == "." || strings.HasPrefix(cleaned, "../") || cleaned == ".." {
			result.Valid = false
			result.Errors = append(result.Errors, fmt.Sprintf("package contains an unsafe file path %q", normalized))
			continue
		}
		root, _, _ := strings.Cut(cleaned, "/")
		if _, ok := allowedRoots[root]; !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("package contains an unsupported top-level path %q", root))
		}
	}

	def, err := adapter.AdaptPackage(pkg)
	if err != nil {
		result.Valid = false
		result.Errors = append(result.Errors, err.Error())
		return result
	}
	if strings.TrimSpace(def.Name) == "" {
		result.Valid = false
		result.Errors = append(result.Errors, "adapted skill definition is missing a name")
	}
	if strings.TrimSpace(def.Description) == "" {
		result.Warnings = append(result.Warnings, "skill description is empty")
	}
	if strings.TrimSpace(def.Guidance) == "" {
		result.Warnings = append(result.Warnings, "skill guidance is empty")
	}
	return result
}
