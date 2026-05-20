// assets.go loads embedded runtime task bundles and skill metadata into one queryable registry.
// assets.go 负责把嵌入式 runtime task bundle 与 skill 元数据加载成可查询 registry。
package runtimeassets

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed builtin/tasks/*.json builtin/skills/*.json
var embeddedFS embed.FS

// Registry loads task bundles and runtime skill metadata embedded at build time.
// Registry 负责加载在构建期嵌入二进制的任务资产与 runtime skill 元数据。
type Registry struct {
	taskBundles map[string]TaskAssetBundle
	skills      map[string]SkillMetadata
}

// NewRegistry creates one embedded runtime asset registry.
// NewRegistry 会创建一份嵌入式 runtime 资产 registry。
func NewRegistry() (*Registry, error) {
	taskBundles, err := loadTaskBundles()
	if err != nil {
		return nil, err
	}
	skills, err := loadSkills()
	if err != nil {
		return nil, err
	}
	return &Registry{
		taskBundles: taskBundles,
		skills:      skills,
	}, nil
}

// SelectTaskBundle returns the single task bundle that matches the current task and output contract.
// SelectTaskBundle 会返回与当前任务和输出契约匹配的唯一任务资产。
func (r *Registry) SelectTaskBundle(taskType string, taskSubtype string, requestedOutputModes []string) (TaskAssetBundle, bool) {
	if r == nil {
		return TaskAssetBundle{}, false
	}
	key := strings.TrimSpace(taskType) + "::" + strings.TrimSpace(taskSubtype)
	bundle, ok := r.taskBundles[key]
	if !ok {
		return TaskAssetBundle{}, false
	}
	if len(requestedOutputModes) == 0 {
		return bundle, true
	}
	allowed := make(map[string]struct{}, len(bundle.RequestedOutputModes))
	for _, item := range bundle.RequestedOutputModes {
		allowed[strings.TrimSpace(item)] = struct{}{}
	}
	for _, item := range requestedOutputModes {
		if _, exists := allowed[strings.TrimSpace(item)]; !exists {
			return TaskAssetBundle{}, false
		}
	}
	return bundle, true
}

// ListSkills returns runtime skill metadata narrowed by the provided filter.
// ListSkills 会按过滤条件返回 runtime skill 元数据。
func (r *Registry) ListSkills(_ context.Context, filter SkillFilter) []SkillMetadata {
	if r == nil {
		return nil
	}
	result := make([]SkillMetadata, 0, len(r.skills))
	for _, item := range r.skills {
		if filter.Source != "" && item.Source != filter.Source {
			continue
		}
		if strings.TrimSpace(filter.TaskType) != "" && !contains(item.AllowedTaskTypes, filter.TaskType) {
			continue
		}
		if strings.TrimSpace(filter.TaskSubtype) != "" && !contains(item.AllowedTaskSubtypes, filter.TaskSubtype) {
			continue
		}
		if strings.TrimSpace(filter.RequestedOutputMode) != "" && !contains(item.AllowedOutputModes, filter.RequestedOutputMode) {
			continue
		}
		result = append(result, item)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].ID < result[j].ID
	})
	return result
}

// ResolveVisibleSkills converts a strict allowlist into visible runtime skills for one task.
// ResolveVisibleSkills 会把严格 allowlist 转换为当前任务可见的 runtime skills。
func (r *Registry) ResolveVisibleSkills(availableSkillIDs []string, taskType string, taskSubtype string, requestedOutputModes []string) ([]SkillMetadata, error) {
	if r == nil {
		return nil, nil
	}
	if len(availableSkillIDs) == 0 {
		return nil, nil
	}
	requestedMode := ""
	if len(requestedOutputModes) > 0 {
		requestedMode = strings.TrimSpace(requestedOutputModes[0])
	}
	filter := SkillFilter{
		TaskType:            taskType,
		TaskSubtype:         taskSubtype,
		RequestedOutputMode: requestedMode,
	}
	visible := make([]SkillMetadata, 0, len(availableSkillIDs))
	for _, id := range availableSkillIDs {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		item, ok := r.skills[id]
		if !ok {
			return nil, fmt.Errorf("runtime skill %q was not found", id)
		}
		if filter.Source != "" && item.Source != filter.Source {
			continue
		}
		if strings.TrimSpace(filter.TaskType) != "" && !contains(item.AllowedTaskTypes, filter.TaskType) {
			return nil, fmt.Errorf("runtime skill %q is not allowed for task_type %q", id, taskType)
		}
		if strings.TrimSpace(filter.TaskSubtype) != "" && !contains(item.AllowedTaskSubtypes, filter.TaskSubtype) {
			return nil, fmt.Errorf("runtime skill %q is not allowed for task_subtype %q", id, taskSubtype)
		}
		if strings.TrimSpace(filter.RequestedOutputMode) != "" && !contains(item.AllowedOutputModes, filter.RequestedOutputMode) {
			return nil, fmt.Errorf("runtime skill %q is not allowed for requested_output_mode %q", id, requestedMode)
		}
		visible = append(visible, item)
	}
	sort.Slice(visible, func(i, j int) bool {
		return visible[i].ID < visible[j].ID
	})
	return visible, nil
}

func loadTaskBundles() (map[string]TaskAssetBundle, error) {
	items, err := readJSONObjects[TaskAssetBundle]("builtin/tasks")
	if err != nil {
		return nil, err
	}
	result := make(map[string]TaskAssetBundle, len(items))
	for _, item := range items {
		key := strings.TrimSpace(item.TaskType) + "::" + strings.TrimSpace(item.TaskSubtype)
		result[key] = item
	}
	return result, nil
}

func loadSkills() (map[string]SkillMetadata, error) {
	items, err := readJSONObjects[SkillMetadata]("builtin/skills")
	if err != nil {
		return nil, err
	}
	result := make(map[string]SkillMetadata, len(items))
	for _, item := range items {
		result[strings.TrimSpace(item.ID)] = item
	}
	return result, nil
}

func readJSONObjects[T any](dir string) ([]T, error) {
	entries, err := fs.ReadDir(embeddedFS, dir)
	if err != nil {
		return nil, err
	}
	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		names = append(names, entry.Name())
	}
	sort.Strings(names)
	result := make([]T, 0, len(names))
	for _, name := range names {
		payload, err := fs.ReadFile(embeddedFS, dir+"/"+name)
		if err != nil {
			return nil, err
		}
		var item T
		if err := json.Unmarshal(payload, &item); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, nil
}

func contains(values []string, want string) bool {
	want = strings.TrimSpace(want)
	for _, item := range values {
		if strings.TrimSpace(item) == want {
			return true
		}
	}
	return false
}
