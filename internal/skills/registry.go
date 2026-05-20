// registry.go defines the in-memory skill registry used after loader resolution.
// registry.go 定义 loader 解析完成后使用的内存态 skill registry。
package skills

import (
	"context"
	"sort"
)

// Definition is the declaration-time skill object used before runtime adaptation.
// Definition 是运行时适配前使用的声明态 skill 对象。
type Definition struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	ToolNames   []string `json:"tool_names,omitempty"`
	Guidance    string   `json:"guidance,omitempty"`
}

// Registry stores skill definitions by name for runtime selection and adaptation.
// Registry 按名称保存 skill 定义，供 runtime 选择和适配使用。
type Registry struct {
	definitions map[string]Definition
}

// NewRegistry creates an empty skill registry.
// NewRegistry 创建一个空的 skill registry。
func NewRegistry() *Registry {
	return &Registry{
		definitions: make(map[string]Definition),
	}
}

// NewRegistryFromDefinitions creates a registry from the provided declaration-time skills.
// NewRegistryFromDefinitions 会根据给定的声明态 skill 列表创建 registry。
func NewRegistryFromDefinitions(defs []Definition) *Registry {
	registry := NewRegistry()
	for _, def := range defs {
		registry.Register(def)
	}
	return registry
}

// Register adds or replaces one skill definition in the registry.
// Register 会向 registry 中新增或替换一条 skill 定义。
func (r *Registry) Register(def Definition) {
	if r.definitions == nil {
		r.definitions = make(map[string]Definition)
	}
	r.definitions[def.Name] = def
}

// Get looks up one skill definition by name.
// Get 会按名称读取单条 skill 定义。
func (r *Registry) Get(name string) (Definition, bool) {
	def, ok := r.definitions[name]
	return def, ok
}

// List returns all registered skill definitions in stable name order.
// List 会按稳定名称顺序返回全部已注册的 skill 定义。
func (r *Registry) List() []Definition {
	result := make([]Definition, 0, len(r.definitions))
	names := make([]string, 0, len(r.definitions))
	for name := range r.definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		result = append(result, cloneDefinition(r.definitions[name]))
	}
	return result
}

// BuiltinRegistry creates a registry backed by the scaffold's built-in skill declarations.
// BuiltinRegistry 会基于脚手架内置 skill 声明创建 registry。
func BuiltinRegistry() *Registry {
	source := NewBuiltinSource()
	defs, err := source.Load(context.Background())
	if err != nil {
		panic(err)
	}
	return NewRegistryFromDefinitions(defs)
}

func cloneDefinition(def Definition) Definition {
	return Definition{
		Name:        def.Name,
		Description: def.Description,
		ToolNames:   append([]string(nil), def.ToolNames...),
		Guidance:    def.Guidance,
	}
}
