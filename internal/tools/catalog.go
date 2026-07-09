// catalog.go provides the thread-safe tool definition catalog shared by resolution and execution.
// catalog.go 提供能力解析与执行共同使用的线程安全工具定义目录。
package tools

import (
	"fmt"
	"strings"
	"sync"
)

// Catalog stores built-in and dynamically registered tool definitions.
// Catalog 保存内置与动态注册的工具定义。
type Catalog struct {
	mu          sync.RWMutex
	definitions map[string]Definition
}

// NewCatalog creates a catalog from an initial definition snapshot.
// NewCatalog 根据初始工具定义快照创建目录。
func NewCatalog(initial map[string]Definition) *Catalog {
	catalog := &Catalog{definitions: make(map[string]Definition, len(initial))}
	for name, definition := range initial {
		name = strings.TrimSpace(name)
		if name == "" {
			name = strings.TrimSpace(definition.Name)
		}
		if name == "" {
			continue
		}
		definition.Name = name
		catalog.definitions[name] = definition
	}
	return catalog
}

// Register validates and atomically replaces one tool definition.
// Register 校验并原子替换一条工具定义。
func (c *Catalog) Register(definition Definition) error {
	if c == nil {
		return fmt.Errorf("tool catalog is not configured")
	}
	name := strings.TrimSpace(definition.Name)
	if name == "" {
		return fmt.Errorf("tool name is required")
	}
	if definition.BaseTool == nil {
		return fmt.Errorf("tool %q implementation is required", name)
	}
	definition.Name = name
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.definitions == nil {
		c.definitions = make(map[string]Definition)
	}
	c.definitions[name] = definition
	return nil
}

// Delete removes one dynamically managed definition by name.
// Delete 按名称移除一条动态管理的工具定义。
func (c *Catalog) Delete(name string) bool {
	if c == nil {
		return false
	}
	name = strings.TrimSpace(name)
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.definitions[name]; !ok {
		return false
	}
	delete(c.definitions, name)
	return true
}

// Get returns one stable definition copy.
// Get 返回一条稳定的工具定义副本。
func (c *Catalog) Get(name string) (Definition, bool) {
	if c == nil {
		return Definition{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	definition, ok := c.definitions[strings.TrimSpace(name)]
	return definition, ok
}

// Snapshot returns a stable map copy for one resolver or executor operation.
// Snapshot 返回供单次解析或执行使用的稳定 map 副本。
func (c *Catalog) Snapshot() map[string]Definition {
	if c == nil {
		return map[string]Definition{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	snapshot := make(map[string]Definition, len(c.definitions))
	for name, definition := range c.definitions {
		snapshot[name] = definition
	}
	return snapshot
}
