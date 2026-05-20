// store.go defines persistence interfaces for runtime-visible skills and uploaded packages.
// store.go 定义 runtime 可见 skill 与 uploaded package 的持久化接口。
package skills

import (
	"context"
	"sort"
	"sync"
)

// Store persists uploaded skill declarations for future upload and management workflows.
// Store 负责持久化上传 skill 的声明数据，供未来上传与管理流程使用。
//
// Built-in embedded skills do not go through this store.
// 内置嵌入式 skill 不经过这个 store。
type Store interface {
	List(context.Context) ([]Definition, error)
	Put(context.Context, Definition) error
	Delete(context.Context, string) error
}

// MemoryStore keeps declaration-time skills in process memory.
// MemoryStore 会把声明态 skill 保存在进程内存中。
type MemoryStore struct {
	mu          sync.Mutex
	definitions map[string]Definition
}

// NewMemoryStore creates an in-memory skill store.
// NewMemoryStore 创建一个内存版 skill store。
func NewMemoryStore() Store {
	return &MemoryStore{
		definitions: make(map[string]Definition),
	}
}

// List returns all persisted skills in stable name order.
// List 会按稳定名称顺序返回全部已持久化的 skill。
func (s *MemoryStore) List(context.Context) ([]Definition, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	names := make([]string, 0, len(s.definitions))
	for name := range s.definitions {
		names = append(names, name)
	}
	sort.Strings(names)

	result := make([]Definition, 0, len(names))
	for _, name := range names {
		result = append(result, cloneDefinition(s.definitions[name]))
	}
	return result, nil
}

// Put creates or replaces one persisted skill.
// Put 会创建或替换一条已持久化的 skill。
func (s *MemoryStore) Put(_ context.Context, def Definition) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.definitions[def.Name] = cloneDefinition(def)
	return nil
}

// Delete removes one persisted skill by name.
// Delete 会按名称删除一条已持久化的 skill。
func (s *MemoryStore) Delete(_ context.Context, name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.definitions, name)
	return nil
}
