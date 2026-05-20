// store.go defines the file-backed control-plane override store.
// store.go 定义文件化控制面 override 存储。
package controlplane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Store persists control-plane overrides.
// Store 负责持久化控制面 override。
type Store interface {
	Load(context.Context) (Document, error)
	Save(context.Context, Document) error
	ListVersions(context.Context) ([]ConfigVersionSummary, error)
	LoadVersion(context.Context, string) (ConfigVersionDetail, error)
	SaveVersion(context.Context, ConfigVersionDetail) error
}

// FileStore keeps control-plane overrides in one JSON file.
// FileStore 会把控制面 override 保存在单个 JSON 文件中。
type FileStore struct {
	path string
	mu   sync.Mutex
}

// NewFileStore creates one file-backed control-plane store.
// NewFileStore 创建一个文件化控制面存储。
func NewFileStore(path string) *FileStore {
	return &FileStore{path: strings.TrimSpace(path)}
}

// Load returns the persisted document or an empty default document when the file is missing.
// Load 会返回已持久化文档；文件不存在时返回空默认文档。
func (s *FileStore) Load(context.Context) (Document, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.path) == "" {
		return Document{Runtime: DefaultRuntimeTuning()}, nil
	}
	payload, err := os.ReadFile(filepath.Clean(s.path))
	if err != nil {
		if os.IsNotExist(err) {
			return Document{Runtime: DefaultRuntimeTuning()}, nil
		}
		return Document{}, err
	}
	if len(payload) == 0 {
		return Document{Runtime: DefaultRuntimeTuning()}, nil
	}
	var doc Document
	if err := json.Unmarshal(payload, &doc); err != nil {
		return Document{}, err
	}
	doc = mergeDocument(doc)
	return doc, nil
}

// Save writes the full control-plane document back to disk.
// Save 会把完整控制面文档写回磁盘。
func (s *FileStore) Save(_ context.Context, doc Document) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	doc = mergeDocument(doc)
	if err := os.MkdirAll(filepath.Dir(filepath.Clean(s.path)), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Clean(s.path), payload, 0o644)
}

// ListVersions returns all persisted control-plane version summaries in reverse chronological order.
// ListVersions 会按时间倒序返回已持久化的控制面版本摘要。
func (s *FileStore) ListVersions(context.Context) ([]ConfigVersionSummary, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.versionDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	items := make([]ConfigVersionSummary, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		payload, err := os.ReadFile(filepath.Join(s.versionDir(), entry.Name()))
		if err != nil {
			return nil, err
		}
		var detail ConfigVersionDetail
		if err := json.Unmarshal(payload, &detail); err != nil {
			return nil, err
		}
		items = append(items, detail.ConfigVersionSummary)
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].CreatedAt > items[j].CreatedAt
	})
	return items, nil
}

// LoadVersion returns one persisted control-plane version detail.
// LoadVersion 会返回单个已持久化控制面版本详情。
func (s *FileStore) LoadVersion(_ context.Context, versionID string) (ConfigVersionDetail, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	payload, err := os.ReadFile(filepath.Join(s.versionDir(), filepath.Base(strings.TrimSpace(versionID))+".json"))
	if err != nil {
		return ConfigVersionDetail{}, err
	}
	var detail ConfigVersionDetail
	if err := json.Unmarshal(payload, &detail); err != nil {
		return ConfigVersionDetail{}, err
	}
	detail.Document = mergeDocument(detail.Document)
	return detail, nil
}

// SaveVersion persists one version snapshot next to the primary override file.
// SaveVersion 会把一个版本快照持久化到主 override 文件旁边。
func (s *FileStore) SaveVersion(_ context.Context, detail ConfigVersionDetail) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if strings.TrimSpace(s.path) == "" {
		return nil
	}
	detail.Document = mergeDocument(detail.Document)
	if strings.TrimSpace(detail.VersionID) == "" {
		detail.VersionID = newVersionID()
	}
	if strings.TrimSpace(detail.CreatedAt) == "" {
		detail.CreatedAt = time.Now().UTC().Format(time.RFC3339)
	}
	if err := os.MkdirAll(s.versionDir(), 0o755); err != nil {
		return err
	}
	payload, err := json.MarshalIndent(detail, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.versionDir(), detail.VersionID+".json"), payload, 0o644)
}

func (s *FileStore) versionDir() string {
	if strings.TrimSpace(s.path) == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(filepath.Clean(s.path)), "versions")
}

func mergeDocument(input Document) Document {
	merged := input
	merged.Governance = mergeRuntimeTuning(resolveGovernance(input))
	merged.Runtime = merged.Governance
	return merged
}

func resolveGovernance(input Document) GovernanceConfig {
	if input.Governance != (GovernanceConfig{}) {
		return input.Governance
	}
	return input.Runtime
}

func mergeRuntimeTuning(input GovernanceConfig) GovernanceConfig {
	defaults := DefaultRuntimeTuning()
	if !input.ChoiceRequiredEnabled {
		defaults.ChoiceRequiredEnabled = false
	}
	if !input.AutomationFallbackEnabled {
		defaults.AutomationFallbackEnabled = false
	}
	if !input.PlanningProgressEnabled {
		defaults.PlanningProgressEnabled = false
	}
	if !input.FactQualityGateEnabled {
		defaults.FactQualityGateEnabled = false
	}
	if !input.ToolHintEmissionEnabled {
		defaults.ToolHintEmissionEnabled = false
	}
	if !input.KnowledgeRetrievalEnabled {
		defaults.KnowledgeRetrievalEnabled = false
	}
	if input.MaxPlanningSteps > 0 {
		defaults.MaxPlanningSteps = input.MaxPlanningSteps
	}
	if input.MaxToolHints > 0 {
		defaults.MaxToolHints = input.MaxToolHints
	}
	return defaults
}

func newVersionID() string {
	return fmt.Sprintf("cfg_%s", time.Now().UTC().Format("20060102T150405.000000000Z"))
}
