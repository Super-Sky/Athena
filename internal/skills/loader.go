// loader.go implements the unified skill loading chain across builtin and uploaded sources.
// loader.go 实现 builtin 与 uploaded 来源统一的 skill 加载链。
package skills

import "context"

// Loader builds one registry from builtin and persisted declaration-time skill sources.
// Loader 负责把内置与持久化的声明态 skill 来源合并成一个 registry。
type Loader interface {
	Load(context.Context) (*Registry, error)
}

// MergedLoader merges multiple sources and an optional store into one registry.
// MergedLoader 会把多个 source 和可选 store 合并成一个 registry。
type MergedLoader struct {
	Sources []Source
	Store   Store
}

// NewLoader creates the scaffold's unified skill loading chain.
// NewLoader 创建当前脚手架统一的 skill 加载链。
func NewLoader(sources []Source, store Store) Loader {
	return MergedLoader{
		Sources: append([]Source(nil), sources...),
		Store:   store,
	}
}

// Load builds one registry, letting later sources and the store override earlier declarations.
// Load 会构建一个 registry，并允许后续 source 与 store 覆盖更早的声明。
func (l MergedLoader) Load(ctx context.Context) (*Registry, error) {
	registry := NewRegistry()
	for _, source := range l.Sources {
		if source == nil {
			continue
		}
		defs, err := source.Load(ctx)
		if err != nil {
			return nil, err
		}
		for _, def := range defs {
			registry.Register(def)
		}
	}
	if l.Store == nil {
		return registry, nil
	}
	defs, err := l.Store.List(ctx)
	if err != nil {
		return nil, err
	}
	for _, def := range defs {
		registry.Register(def)
	}
	return registry, nil
}
