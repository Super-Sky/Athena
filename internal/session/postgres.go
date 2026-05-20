// postgres.go implements the PostgreSQL-backed session store and deferred-message persistence paths.
// postgres.go 实现基于 PostgreSQL 的 session store 与 deferred message 持久化路径。
package session

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"moss/internal/contextassets"
	"moss/internal/postgresutil"
)

const (
	// SessionStateSchemaVersion tracks the persisted aggregate shape for future migrations.
	// SessionStateSchemaVersion 用于标记当前持久化 session 聚合结构的版本，方便后续迁移。
	SessionStateSchemaVersion = 3

	// DeferredMessageStatusQueued marks one deferred item as still waiting for consumption.
	// DeferredMessageStatusQueued 表示一条 deferred 消息仍在等待被消费。
	DeferredMessageStatusQueued = "queued"

	// DefaultPostgresUpdateRetries bounds optimistic-lock retries for one aggregate update.
	// DefaultPostgresUpdateRetries 用于限制单次聚合更新的 optimistic locking 重试次数。
	DefaultPostgresUpdateRetries = 3
)

// ErrConcurrentSessionUpdate signals that one aggregate update lost the optimistic-lock race.
// ErrConcurrentSessionUpdate 表示一次聚合更新在 optimistic locking 竞争中失败。
var ErrConcurrentSessionUpdate = errors.New("concurrent session update")

// PostgresSessionModel is the GORM model used by migrate and runtime persistence.
// PostgresSessionModel 是供 migrate 和运行时持久化共用的 GORM 模型。
type PostgresSessionModel struct {
	ID                  string    `gorm:"column:id;type:text;primaryKey"`
	Title               string    `gorm:"column:title;type:text;not null;default:''"`
	Archived            bool      `gorm:"column:archived;type:boolean;not null;default:false;index:idx_sessions_archived"`
	LastActiveAt        time.Time `gorm:"column:last_active_at;type:timestamptz;not null;default:CURRENT_TIMESTAMP;index:idx_sessions_last_active_at"`
	StateSchemaVersion  int       `gorm:"column:state_schema_version;type:integer;not null;default:3"`
	MessagesJSON        []byte    `gorm:"column:messages_json;type:jsonb;not null"`
	ContextAssetsJSON   []byte    `gorm:"column:context_assets_json;type:jsonb;not null"`
	ContextBindingsJSON []byte    `gorm:"column:context_asset_bindings_json;type:jsonb;not null"`
	CompiledRefsJSON    []byte    `gorm:"column:compiled_asset_refs_json;type:jsonb;not null"`
	PendingJSON         []byte    `gorm:"column:pending_json;type:jsonb"`
	ClosedTokensJSON    []byte    `gorm:"column:closed_tokens_json;type:jsonb;not null"`
	Version             int64     `gorm:"column:version;type:bigint;not null;default:1"`
	CreatedAt           time.Time `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt           time.Time `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime;index:idx_sessions_updated_at"`
}

// TableName keeps the session aggregate root persisted in the sessions table.
// TableName 用于把 session 聚合根固定到 sessions 主表。
func (PostgresSessionModel) TableName() string {
	return "sessions"
}

// PostgresDeferredMessageModel stores one queued follow-up input as an independent row.
// PostgresDeferredMessageModel 用独立行保存一条排队中的后续输入。
type PostgresDeferredMessageModel struct {
	ID                     int64      `gorm:"column:id;type:bigserial;primaryKey"`
	SessionID              string     `gorm:"column:session_id;type:text;not null;index:idx_session_deferred_queue,priority:1"`
	Sequence               int64      `gorm:"column:sequence;type:bigint;not null;index:idx_session_deferred_queue,priority:3"`
	Query                  string     `gorm:"column:query;type:text;not null"`
	ModelID                string     `gorm:"column:model_id;type:text;not null;default:''"`
	PromptTemplate         string     `gorm:"column:prompt_template;type:text;not null;default:''"`
	EnabledSkills          []byte     `gorm:"column:enabled_skills_json;type:jsonb;not null"`
	EnabledTools           []byte     `gorm:"column:enabled_tools_json;type:jsonb;not null"`
	ContextAssetOverrides  []byte     `gorm:"column:context_asset_overrides_json;type:jsonb;not null"`
	DisabledAssetTypes     []byte     `gorm:"column:disabled_asset_types_json;type:jsonb;not null"`
	AssetPriorityOverrides []byte     `gorm:"column:asset_priority_overrides_json;type:jsonb;not null"`
	ContextAssets          []byte     `gorm:"column:context_assets_json;type:jsonb;not null"`
	ContextBindings        []byte     `gorm:"column:context_asset_bindings_json;type:jsonb;not null"`
	CompiledRefs           []byte     `gorm:"column:compiled_asset_refs_json;type:jsonb;not null"`
	DisableFastPath        bool       `gorm:"column:disable_fast_path;type:boolean;not null;default:false"`
	Status                 string     `gorm:"column:status;type:text;not null;index:idx_session_deferred_queue,priority:2"`
	ReceivedAt             time.Time  `gorm:"column:received_at;type:timestamptz;not null;index:idx_session_deferred_received_at"`
	ConsumedAt             *time.Time `gorm:"column:consumed_at;type:timestamptz"`
	CreatedAt              time.Time  `gorm:"column:created_at;type:timestamptz;not null;autoCreateTime"`
	UpdatedAt              time.Time  `gorm:"column:updated_at;type:timestamptz;not null;autoUpdateTime"`
}

// TableName keeps queued follow-up inputs in their own waiting queue table.
// TableName 用于把排队中的后续输入固定到独立的等待队列表。
func (PostgresDeferredMessageModel) TableName() string {
	return "session_deferred_messages"
}

// legacyPostgresSessionQueueModel keeps the deprecated deferred_queue_json column visible during migration.
// legacyPostgresSessionQueueModel 用于在迁移阶段临时读取已废弃的 deferred_queue_json 列。
type legacyPostgresSessionQueueModel struct {
	ID                string `gorm:"column:id"`
	DeferredQueueJSON []byte `gorm:"column:deferred_queue_json"`
}

// TableName points the legacy queue reader at the same sessions table.
// TableName 用于把 legacy queue 读取模型指向同一张 sessions 表。
func (legacyPostgresSessionQueueModel) TableName() string {
	return "sessions"
}

// PostgresStore is the PostgreSQL-backed session.Store implementation.
// PostgresStore 是 PostgreSQL 版 session.Store 实现。
type PostgresStore struct {
	db             *gorm.DB
	updateRetries  int
	deferredLimit  int
	closedTokenTTL time.Duration
}

// NewPostgresDB opens a PostgreSQL-backed GORM database handle for session persistence.
// NewPostgresDB 会创建一个面向 session 持久化的 PostgreSQL GORM 连接。
func NewPostgresDB(dsn string, opts ...func(*gorm.Config)) (*gorm.DB, error) {
	cfg := &gorm.Config{}
	for _, opt := range opts {
		if opt != nil {
			opt(cfg)
		}
	}
	return gorm.Open(postgres.Open(dsn), cfg)
}

// NewPostgresStore creates a PostgreSQL-backed store with the same queue and tombstone defaults as MemoryStore.
// NewPostgresStore 会创建一个 PostgreSQL 版 store，并沿用与 MemoryStore 一致的 queue 和 tombstone 默认值。
func NewPostgresStore(db *gorm.DB, queueLimit int, closedTokenTTL time.Duration) *PostgresStore {
	return NewPostgresStoreWithOptions(db, queueLimit, closedTokenTTL, DefaultPostgresUpdateRetries)
}

// NewPostgresStoreWithOptions creates a PostgreSQL-backed store with configurable limits and retry count.
// NewPostgresStoreWithOptions 会创建一个可配置限制和重试次数的 PostgreSQL 版 store。
func NewPostgresStoreWithOptions(db *gorm.DB, queueLimit int, closedTokenTTL time.Duration, updateRetries int) *PostgresStore {
	if queueLimit <= 0 {
		queueLimit = DefaultDeferredQueueLimit
	}
	if closedTokenTTL <= 0 {
		closedTokenTTL = DefaultClosedResumeTokenTTL
	}
	if updateRetries <= 0 {
		updateRetries = DefaultPostgresUpdateRetries
	}
	return &PostgresStore{
		db:             db,
		updateRetries:  updateRetries,
		deferredLimit:  queueLimit,
		closedTokenTTL: closedTokenTTL,
	}
}

// AutoMigrate creates or updates the session root table and deferred queue table.
// AutoMigrate 会创建或更新 session 主表和 deferred queue 表结构。
func (s *PostgresStore) AutoMigrate(ctx context.Context) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres session store is not configured")
	}
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := migrateSessionJSONColumns(tx); err != nil {
			return err
		}
		if err := migrateDeferredMessageJSONColumns(tx); err != nil {
			return err
		}
		if err := tx.AutoMigrate(&PostgresSessionModel{}, &PostgresDeferredMessageModel{}); err != nil {
			return err
		}
		return s.migrateLegacyDeferredQueue(ctx, tx)
	})
}

// Get loads one session aggregate from PostgreSQL and rehydrates queued deferred messages from the queue table.
// Get 会从 PostgreSQL 读取一个 session 聚合，并从独立队列表中重新组装 deferred queue。
func (s *PostgresStore) Get(ctx context.Context, id string) (*Session, bool) {
	if s == nil || s.db == nil {
		return nil, false
	}

	var model PostgresSessionModel
	err := s.db.WithContext(ctx).First(&model, "id = ?", id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false
		}
		return nil, false
	}

	queueRows, err := s.loadQueuedDeferredMessages(ctx, s.db, id)
	if err != nil {
		return nil, false
	}
	session, err := s.decodeAggregate(model, queueRows)
	if err != nil {
		return nil, false
	}
	return session, true
}

// Put upserts one complete session aggregate snapshot and replaces the queued deferred rows for that session.
// Put 会整块 upsert 一份完整的 session 聚合快照，并替换该 session 的排队消息行。
func (s *PostgresStore) Put(ctx context.Context, session *Session) error {
	if s == nil || s.db == nil {
		return fmt.Errorf("postgres session store is not configured")
	}
	if session == nil {
		return fmt.Errorf("session is required")
	}

	normalized := session.Clone()
	normalized.Normalize(time.Now(), s.deferredLimit, s.closedTokenTTL)

	return postgresutil.WithRetry(ctx, s.updateRetries, func() error {
		return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
			currentVersion := int64(0)
			var existing PostgresSessionModel
			err := tx.Select("version", "created_at").First(&existing, "id = ?", normalized.ID).Error
			switch {
			case err == nil:
				currentVersion = existing.Version
			case errors.Is(err, gorm.ErrRecordNotFound):
			default:
				return err
			}

			record, err := s.encodeSession(normalized, currentVersion+1, zeroIfMissing(currentVersion > 0, existing.CreatedAt))
			if err != nil {
				return err
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "id"}},
				UpdateAll: true,
			}).Create(&record).Error; err != nil {
				return err
			}
			return s.replaceDeferredQueueRows(ctx, tx, normalized.ID, normalized.DeferredQueue)
		})
	})
}

// Update atomically mutates one session aggregate via optimistic locking and queue-table replacement.
// Update 会通过 optimistic locking 原子更新一个 session 聚合，并同步替换队列表。
func (s *PostgresStore) Update(ctx context.Context, id string, mutator func(*Session) error) (*Session, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres session store is not configured")
	}
	if mutator == nil {
		return nil, fmt.Errorf("session mutator is required")
	}

	for attempt := 0; attempt < s.updateRetries; attempt++ {
		var (
			current       *Session
			currentRecord PostgresSessionModel
			currentVer    int64
			found         bool
			createdAt     time.Time
		)

		err := s.db.WithContext(ctx).First(&currentRecord, "id = ?", id).Error
		switch {
		case err == nil:
			queueRows, queueErr := s.loadQueuedDeferredMessages(ctx, s.db, id)
			if queueErr != nil {
				return nil, queueErr
			}
			current, err = s.decodeAggregate(currentRecord, queueRows)
			if err != nil {
				return nil, err
			}
			currentVer = currentRecord.Version
			createdAt = currentRecord.CreatedAt
			found = true
		case errors.Is(err, gorm.ErrRecordNotFound):
			current = (&Session{ID: id}).Clone()
		default:
			return nil, err
		}

		if err := mutator(current); err != nil {
			return nil, err
		}
		current.Normalize(time.Now(), s.deferredLimit, s.closedTokenTTL)

		record, err := s.encodeSession(current, currentVer+1, zeroIfMissing(found, createdAt))
		if err != nil {
			return nil, err
		}

		err = postgresutil.WithRetry(ctx, s.updateRetries, func() error {
			return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				if !found {
					if err := tx.Create(&record).Error; err != nil {
						if isDuplicateKeyError(err) {
							return ErrConcurrentSessionUpdate
						}
						return err
					}
					return s.replaceDeferredQueueRows(ctx, tx, current.ID, current.DeferredQueue)
				}

				result := tx.Model(&PostgresSessionModel{}).
					Where("id = ? AND version = ?", id, currentVer).
					Updates(map[string]any{
						"title":                       record.Title,
						"archived":                    record.Archived,
						"last_active_at":              record.LastActiveAt,
						"state_schema_version":        record.StateSchemaVersion,
						"messages_json":               record.MessagesJSON,
						"context_assets_json":         record.ContextAssetsJSON,
						"context_asset_bindings_json": record.ContextBindingsJSON,
						"compiled_asset_refs_json":    record.CompiledRefsJSON,
						"pending_json":                record.PendingJSON,
						"closed_tokens_json":          record.ClosedTokensJSON,
						"version":                     record.Version,
						"updated_at":                  record.UpdatedAt,
					})
				if result.Error != nil {
					return result.Error
				}
				if result.RowsAffected != 1 {
					return ErrConcurrentSessionUpdate
				}
				return s.replaceDeferredQueueRows(ctx, tx, current.ID, current.DeferredQueue)
			})
		})
		if err == nil {
			return current.Clone(), nil
		}
		if errors.Is(err, ErrConcurrentSessionUpdate) {
			continue
		}
		return nil, err
	}

	return nil, ErrConcurrentSessionUpdate
}

func (s *PostgresStore) decodeAggregate(model PostgresSessionModel, queueRows []PostgresDeferredMessageModel) (*Session, error) {
	session := &Session{
		ID:           model.ID,
		Title:        model.Title,
		Archived:     model.Archived,
		LastActiveAt: model.LastActiveAt,
		CreatedAt:    model.CreatedAt,
		UpdatedAt:    model.UpdatedAt,
	}
	if err := decodeJSONField(model.MessagesJSON, &session.Messages, []Message{}); err != nil {
		return nil, err
	}
	if err := decodeJSONField(model.ContextAssetsJSON, &session.ContextAssets, []contextassets.Asset{}); err != nil {
		return nil, err
	}
	if err := decodeJSONField(model.ContextBindingsJSON, &session.ContextAssetBindings, []contextassets.ResolvedAsset{}); err != nil {
		return nil, err
	}
	if err := decodeJSONField(model.CompiledRefsJSON, &session.CompiledAssetRefs, []contextassets.Ref{}); err != nil {
		return nil, err
	}
	if len(model.PendingJSON) > 0 {
		var pending PendingState
		if err := json.Unmarshal(model.PendingJSON, &pending); err != nil {
			return nil, fmt.Errorf("decode pending session state failed: %w", err)
		}
		session.Pending = &pending
	}
	if err := decodeJSONField(model.ClosedTokensJSON, &session.ClosedTokens, []ClosedResumeToken{}); err != nil {
		return nil, err
	}

	deferred, err := decodeDeferredQueueRows(queueRows)
	if err != nil {
		return nil, err
	}
	session.DeferredQueue = deferred
	session.Normalize(time.Now(), s.deferredLimit, s.closedTokenTTL)
	return session, nil
}

func (s *PostgresStore) encodeSession(session *Session, version int64, createdAt time.Time) (PostgresSessionModel, error) {
	if session == nil {
		return PostgresSessionModel{}, fmt.Errorf("session is required")
	}

	messagesJSON, err := encodeJSONField(session.Messages)
	if err != nil {
		return PostgresSessionModel{}, fmt.Errorf("encode session messages failed: %w", err)
	}
	contextAssetsJSON, err := encodeJSONField(session.ContextAssets)
	if err != nil {
		return PostgresSessionModel{}, fmt.Errorf("encode session context assets failed: %w", err)
	}
	contextBindingsJSON, err := encodeJSONField(session.ContextAssetBindings)
	if err != nil {
		return PostgresSessionModel{}, fmt.Errorf("encode session context bindings failed: %w", err)
	}
	compiledRefsJSON, err := encodeJSONField(session.CompiledAssetRefs)
	if err != nil {
		return PostgresSessionModel{}, fmt.Errorf("encode session compiled refs failed: %w", err)
	}
	pendingJSON, err := encodeOptionalJSONField(session.Pending)
	if err != nil {
		return PostgresSessionModel{}, fmt.Errorf("encode pending session state failed: %w", err)
	}
	closedJSON, err := encodeJSONField(session.ClosedTokens)
	if err != nil {
		return PostgresSessionModel{}, fmt.Errorf("encode closed token tombstones failed: %w", err)
	}

	now := time.Now()
	if createdAt.IsZero() {
		createdAt = now
	}
	return PostgresSessionModel{
		ID:                  session.ID,
		Title:               session.Title,
		Archived:            session.Archived,
		LastActiveAt:        session.LastActiveAt,
		StateSchemaVersion:  SessionStateSchemaVersion,
		MessagesJSON:        messagesJSON,
		ContextAssetsJSON:   contextAssetsJSON,
		ContextBindingsJSON: contextBindingsJSON,
		CompiledRefsJSON:    compiledRefsJSON,
		PendingJSON:         pendingJSON,
		ClosedTokensJSON:    closedJSON,
		Version:             version,
		CreatedAt:           createdAt,
		UpdatedAt:           now,
	}, nil
}

func migrateSessionJSONColumns(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("sessions") {
		return nil
	}
	defaults := map[string]string{
		"context_assets_json":         "[]",
		"context_asset_bindings_json": "[]",
		"compiled_asset_refs_json":    "[]",
	}
	for column, defaultJSON := range defaults {
		if err := migrateJSONBNotNullColumn(tx, "sessions", column, defaultJSON); err != nil {
			return err
		}
	}
	return nil
}

func migrateDeferredMessageJSONColumns(tx *gorm.DB) error {
	if !tx.Migrator().HasTable("session_deferred_messages") {
		return nil
	}
	defaults := map[string]string{
		"enabled_skills_json":           "[]",
		"enabled_tools_json":            "[]",
		"context_asset_overrides_json":  "[]",
		"disabled_asset_types_json":     "[]",
		"asset_priority_overrides_json": "{}",
		"context_assets_json":           "[]",
		"context_asset_bindings_json":   "[]",
		"compiled_asset_refs_json":      "[]",
	}
	for column, defaultJSON := range defaults {
		if err := migrateJSONBNotNullColumn(tx, "session_deferred_messages", column, defaultJSON); err != nil {
			return err
		}
	}
	return nil
}

func migrateJSONBNotNullColumn(tx *gorm.DB, table string, column string, defaultJSON string) error {
	if tx.Migrator().HasColumn(table, column) {
		return nil
	}
	return tx.Exec(fmt.Sprintf(`ALTER TABLE "%s" ADD COLUMN "%s" jsonb NOT NULL DEFAULT '%s'::jsonb`, table, column, defaultJSON)).Error
}

func (s *PostgresStore) loadQueuedDeferredMessages(ctx context.Context, db *gorm.DB, sessionID string) ([]PostgresDeferredMessageModel, error) {
	var rows []PostgresDeferredMessageModel
	if err := db.WithContext(ctx).
		Where("session_id = ? AND status = ?", sessionID, DeferredMessageStatusQueued).
		Order("sequence ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	return rows, nil
}

// List returns session resources ordered by last_active_at desc then created_at desc.
// List 会返回按 last_active_at 与 created_at 倒序排列的 session 资源列表。
func (s *PostgresStore) List(ctx context.Context, filter ListFilter) ([]*Session, error) {
	if s == nil || s.db == nil {
		return nil, fmt.Errorf("postgres session store is not configured")
	}

	query := s.db.WithContext(ctx).Model(&PostgresSessionModel{}).Order("last_active_at DESC").Order("created_at DESC")
	if filter.Archived != nil {
		query = query.Where("archived = ?", *filter.Archived)
	}
	if filter.Limit > 0 {
		query = query.Limit(filter.Limit)
	}
	if filter.Offset > 0 {
		query = query.Offset(filter.Offset)
	}

	var rows []PostgresSessionModel
	if err := query.Find(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}

	sessionIDs := make([]string, 0, len(rows))
	for _, row := range rows {
		sessionIDs = append(sessionIDs, row.ID)
	}
	queueRowsBySession, err := s.loadQueuedDeferredMessagesBySession(ctx, s.db, sessionIDs)
	if err != nil {
		return nil, err
	}

	result := make([]*Session, 0, len(rows))
	for _, row := range rows {
		item, err := s.decodeAggregate(row, queueRowsBySession[row.ID])
		if err != nil {
			return nil, err
		}
		if filter.Status != "" && item.Status() != filter.Status {
			continue
		}
		result = append(result, item)
	}
	return result, nil
}

func (s *PostgresStore) loadQueuedDeferredMessagesBySession(ctx context.Context, db *gorm.DB, sessionIDs []string) (map[string][]PostgresDeferredMessageModel, error) {
	result := map[string][]PostgresDeferredMessageModel{}
	if len(sessionIDs) == 0 {
		return result, nil
	}

	var rows []PostgresDeferredMessageModel
	if err := db.WithContext(ctx).
		Where("session_id IN ? AND status = ?", sessionIDs, DeferredMessageStatusQueued).
		Order("sequence ASC").
		Find(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		result[row.SessionID] = append(result[row.SessionID], row)
	}
	return result, nil
}

func (s *PostgresStore) replaceDeferredQueueRows(ctx context.Context, tx *gorm.DB, sessionID string, queue []DeferredMessage) error {
	if err := tx.WithContext(ctx).Where("session_id = ?", sessionID).Delete(&PostgresDeferredMessageModel{}).Error; err != nil {
		return err
	}
	if len(queue) == 0 {
		return nil
	}

	rows, err := buildDeferredQueueRows(sessionID, queue)
	if err != nil {
		return err
	}
	return tx.WithContext(ctx).Create(&rows).Error
}

func (s *PostgresStore) migrateLegacyDeferredQueue(ctx context.Context, tx *gorm.DB) error {
	hasLegacyColumn := tx.Migrator().HasColumn("sessions", "deferred_queue_json")
	if !hasLegacyColumn {
		return nil
	}

	var legacyRows []legacyPostgresSessionQueueModel
	if err := tx.WithContext(ctx).
		Where("deferred_queue_json IS NOT NULL").
		Find(&legacyRows).Error; err != nil {
		return err
	}

	for _, legacyRow := range legacyRows {
		if len(legacyRow.DeferredQueueJSON) == 0 || string(legacyRow.DeferredQueueJSON) == "null" {
			continue
		}

		var queue []DeferredMessage
		if err := decodeJSONField(legacyRow.DeferredQueueJSON, &queue, []DeferredMessage{}); err != nil {
			return fmt.Errorf("decode legacy deferred queue failed: %w", err)
		}
		if len(queue) == 0 {
			continue
		}

		var existing int64
		if err := tx.WithContext(ctx).
			Model(&PostgresDeferredMessageModel{}).
			Where("session_id = ?", legacyRow.ID).
			Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			continue
		}
		if err := s.replaceDeferredQueueRows(ctx, tx, legacyRow.ID, queue); err != nil {
			return err
		}
	}

	return tx.Migrator().DropColumn("sessions", "deferred_queue_json")
}

func decodeDeferredQueueRows(rows []PostgresDeferredMessageModel) ([]DeferredMessage, error) {
	if len(rows) == 0 {
		return nil, nil
	}

	queue := make([]DeferredMessage, 0, len(rows))
	for _, row := range rows {
		var enabledSkills []string
		if err := decodeJSONField(row.EnabledSkills, &enabledSkills, []string{}); err != nil {
			return nil, fmt.Errorf("decode deferred enabled skills failed: %w", err)
		}
		var enabledTools []string
		if err := decodeJSONField(row.EnabledTools, &enabledTools, []string{}); err != nil {
			return nil, fmt.Errorf("decode deferred enabled tools failed: %w", err)
		}
		var contextAssetOverrides []contextassets.Asset
		if err := decodeJSONField(row.ContextAssetOverrides, &contextAssetOverrides, []contextassets.Asset{}); err != nil {
			return nil, fmt.Errorf("decode deferred context asset overrides failed: %w", err)
		}
		var disabledAssetTypes []string
		if err := decodeJSONField(row.DisabledAssetTypes, &disabledAssetTypes, []string{}); err != nil {
			return nil, fmt.Errorf("decode deferred disabled asset types failed: %w", err)
		}
		var assetPriorityOverrides map[string]int
		if err := decodeJSONField(row.AssetPriorityOverrides, &assetPriorityOverrides, map[string]int{}); err != nil {
			return nil, fmt.Errorf("decode deferred asset priority overrides failed: %w", err)
		}
		var contextAssets []contextassets.Asset
		if err := decodeJSONField(row.ContextAssets, &contextAssets, []contextassets.Asset{}); err != nil {
			return nil, fmt.Errorf("decode deferred context assets failed: %w", err)
		}
		var contextBindings []contextassets.ResolvedAsset
		if err := decodeJSONField(row.ContextBindings, &contextBindings, []contextassets.ResolvedAsset{}); err != nil {
			return nil, fmt.Errorf("decode deferred context bindings failed: %w", err)
		}
		var compiledRefs []contextassets.Ref
		if err := decodeJSONField(row.CompiledRefs, &compiledRefs, []contextassets.Ref{}); err != nil {
			return nil, fmt.Errorf("decode deferred compiled refs failed: %w", err)
		}
		queue = append(queue, DeferredMessage{
			Query:                  row.Query,
			ModelID:                row.ModelID,
			PromptTemplate:         row.PromptTemplate,
			EnabledSkills:          enabledSkills,
			EnabledTools:           enabledTools,
			ContextAssetOverrides:  contextAssetOverrides,
			DisabledAssetTypes:     disabledAssetTypes,
			AssetPriorityOverrides: assetPriorityOverrides,
			ContextAssets:          contextAssets,
			ContextBindings:        contextBindings,
			CompiledRefs:           compiledRefs,
			DisableFastPath:        row.DisableFastPath,
			ReceivedAt:             row.ReceivedAt,
		})
	}
	return queue, nil
}

func buildDeferredQueueRows(sessionID string, queue []DeferredMessage) ([]PostgresDeferredMessageModel, error) {
	if len(queue) == 0 {
		return nil, nil
	}

	now := time.Now()
	rows := make([]PostgresDeferredMessageModel, 0, len(queue))
	for index, item := range queue {
		skillsJSON, err := encodeJSONField(item.EnabledSkills)
		if err != nil {
			return nil, fmt.Errorf("encode deferred enabled skills failed: %w", err)
		}
		toolsJSON, err := encodeJSONField(item.EnabledTools)
		if err != nil {
			return nil, fmt.Errorf("encode deferred enabled tools failed: %w", err)
		}
		contextAssetOverridesJSON, err := encodeJSONField(item.ContextAssetOverrides)
		if err != nil {
			return nil, fmt.Errorf("encode deferred context asset overrides failed: %w", err)
		}
		disabledAssetTypesJSON, err := encodeJSONField(item.DisabledAssetTypes)
		if err != nil {
			return nil, fmt.Errorf("encode deferred disabled asset types failed: %w", err)
		}
		assetPriorityOverridesJSON, err := encodeJSONField(item.AssetPriorityOverrides)
		if err != nil {
			return nil, fmt.Errorf("encode deferred asset priority overrides failed: %w", err)
		}
		contextAssetsJSON, err := encodeJSONField(item.ContextAssets)
		if err != nil {
			return nil, fmt.Errorf("encode deferred context assets failed: %w", err)
		}
		contextBindingsJSON, err := encodeJSONField(item.ContextBindings)
		if err != nil {
			return nil, fmt.Errorf("encode deferred context bindings failed: %w", err)
		}
		compiledRefsJSON, err := encodeJSONField(item.CompiledRefs)
		if err != nil {
			return nil, fmt.Errorf("encode deferred compiled refs failed: %w", err)
		}
		receivedAt := item.ReceivedAt
		if receivedAt.IsZero() {
			receivedAt = now
		}
		rows = append(rows, PostgresDeferredMessageModel{
			SessionID:              sessionID,
			Sequence:               int64(index + 1),
			Query:                  item.Query,
			ModelID:                item.ModelID,
			PromptTemplate:         item.PromptTemplate,
			EnabledSkills:          skillsJSON,
			EnabledTools:           toolsJSON,
			ContextAssetOverrides:  contextAssetOverridesJSON,
			DisabledAssetTypes:     disabledAssetTypesJSON,
			AssetPriorityOverrides: assetPriorityOverridesJSON,
			ContextAssets:          contextAssetsJSON,
			ContextBindings:        contextBindingsJSON,
			CompiledRefs:           compiledRefsJSON,
			DisableFastPath:        item.DisableFastPath,
			Status:                 DeferredMessageStatusQueued,
			ReceivedAt:             receivedAt,
		})
	}
	return rows, nil
}

func encodeJSONField[T any](value T) ([]byte, error) {
	if isNil(value) {
		return []byte("[]"), nil
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return nil, err
	}
	if len(payload) == 0 {
		return []byte("[]"), nil
	}
	return payload, nil
}

func encodeOptionalJSONField[T any](value *T) ([]byte, error) {
	if value == nil {
		return nil, nil
	}
	return json.Marshal(value)
}

func decodeJSONField[T any](payload []byte, target *T, fallback T) error {
	if len(payload) == 0 {
		*target = fallback
		return nil
	}
	return json.Unmarshal(payload, target)
}

func isNil[T any](value T) bool {
	switch v := any(value).(type) {
	case nil:
		return true
	case []Message:
		return v == nil
	case []contextassets.Asset:
		return v == nil
	case []contextassets.ResolvedAsset:
		return v == nil
	case []contextassets.Ref:
		return v == nil
	case []string:
		return v == nil
	case []DeferredMessage:
		return v == nil
	case []ClosedResumeToken:
		return v == nil
	default:
		return false
	}
}

func zeroIfMissing(found bool, createdAt time.Time) time.Time {
	if !found {
		return time.Time{}
	}
	return createdAt
}

func isDuplicateKeyError(err error) bool {
	return err != nil && (errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate key"))
}
