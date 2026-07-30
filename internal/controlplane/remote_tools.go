// remote_tools.go persists app-owned HTTP tool registrations in the control-plane document.
// remote_tools.go 在控制面文档中持久化应用自有 HTTP 工具注册。
package controlplane

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"moss/internal/tools"
)

// ListRemoteTools returns persisted remote registrations in stable name order.
// ListRemoteTools 按名称稳定排序返回已持久化的远程工具注册。
func (m *Manager) ListRemoteTools(ctx context.Context) ([]tools.RemoteRegistration, error) {
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return nil, err
	}
	items := append([]tools.RemoteRegistration(nil), doc.RemoteTools...)
	sort.Slice(items, func(i, j int) bool { return items[i].Name < items[j].Name })
	return items, nil
}

// PutRemoteTool creates or replaces one persisted remote registration.
// PutRemoteTool 创建或替换一条持久化远程工具注册。
func (m *Manager) PutRemoteTool(ctx context.Context, name string, input tools.RemoteRegistration) (tools.RemoteRegistration, error) {
	if m != nil {
		m.remoteToolsMu.Lock()
		defer m.remoteToolsMu.Unlock()
	}
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return tools.RemoteRegistration{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return tools.RemoteRegistration{}, fmt.Errorf("remote tool name is required")
	}
	if bodyName := strings.TrimSpace(input.Name); bodyName != "" && bodyName != name {
		return tools.RemoteRegistration{}, fmt.Errorf("remote tool path name must match body name")
	}
	now := time.Now().UTC().Format(time.RFC3339)
	input.Name = name
	input.UpdatedAt = now
	updated := false
	for index := range doc.RemoteTools {
		if strings.TrimSpace(doc.RemoteTools[index].Name) != name {
			continue
		}
		if strings.TrimSpace(input.CreatedAt) == "" {
			input.CreatedAt = doc.RemoteTools[index].CreatedAt
		}
		doc.RemoteTools[index] = input
		updated = true
		break
	}
	if !updated {
		if strings.TrimSpace(input.CreatedAt) == "" {
			input.CreatedAt = now
		}
		doc.RemoteTools = append(doc.RemoteTools, input)
	}
	sort.Slice(doc.RemoteTools, func(i, j int) bool { return doc.RemoteTools[i].Name < doc.RemoteTools[j].Name })
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return tools.RemoteRegistration{}, err
		}
		if err := m.recordVersion(ctx, doc, fmt.Sprintf("update remote tool %s", name)); err != nil {
			return tools.RemoteRegistration{}, err
		}
	}
	return input, nil
}

// DeleteRemoteTool removes one persisted remote registration.
// DeleteRemoteTool 删除一条持久化远程工具注册。
func (m *Manager) DeleteRemoteTool(ctx context.Context, name string) error {
	if m != nil {
		m.remoteToolsMu.Lock()
		defer m.remoteToolsMu.Unlock()
	}
	doc, err := m.LoadDocument(ctx)
	if err != nil {
		return err
	}
	name = strings.TrimSpace(name)
	found := false
	filtered := doc.RemoteTools[:0]
	for _, item := range doc.RemoteTools {
		if strings.TrimSpace(item.Name) == name {
			found = true
			continue
		}
		filtered = append(filtered, item)
	}
	if !found {
		return fmt.Errorf("remote tool %q not found", name)
	}
	doc.RemoteTools = filtered
	if m != nil && m.store != nil {
		if err := m.store.Save(ctx, doc); err != nil {
			return err
		}
		return m.recordVersion(ctx, doc, fmt.Sprintf("delete remote tool %s", name))
	}
	return nil
}
