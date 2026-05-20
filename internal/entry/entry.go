// entry.go assembles the root application graph and process entrypoints for Athena.
// entry.go 负责组装 Athena 的根应用依赖图和进程入口。
package entry

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gitee.com/super_sky/mkh_utils"
	"gorm.io/gorm"
	"moss/internal/app"
	"moss/internal/config"
	"moss/internal/model"
	"moss/internal/observability"
	runtimepkg "moss/internal/runtime"
	httpserver "moss/internal/server"
	"moss/internal/session"
)

// Bootstrap owns the top-level dependency graph for one athena process mode.
// Bootstrap 持有 athena 单个进程模式下的顶层依赖图。
type Bootstrap struct {
	Config       config.Config
	App          *app.Service
	Server       *httpserver.HTTPServer
	SessionStore session.Store
	ModelStore   model.Store
	RuntimeStore runtimepkg.RuntimePersistenceStore
}

// New builds the default bootstrap graph from config.
// New 会基于配置构建默认的启动依赖图。
func New(cfg config.Config) (*Bootstrap, error) {
	return NewWithObservability(cfg, nil)
}

// NewWithObservability builds the bootstrap graph with an injected observability manager.
// NewWithObservability 允许在入口层通过注入 observability manager 来构建完整依赖图。
func NewWithObservability(cfg config.Config, obs *observability.Manager) (*Bootstrap, error) {
	sessionStore, err := NewSessionStore(cfg)
	if err != nil {
		return nil, err
	}
	modelStore, err := NewModelStore(cfg)
	if err != nil {
		return nil, err
	}
	runtimeStore, err := NewRuntimeStore(cfg)
	if err != nil {
		return nil, err
	}
	if obs == nil {
		obs = NewDefaultObservability(cfg)
	}
	return NewWithRuntimeDependencies(cfg, obs, sessionStore, modelStore, runtimeStore), nil
}

// NewWithDependencies builds the bootstrap graph from already-resolved dependencies.
// NewWithDependencies 允许使用已经解析好的依赖来构建启动依赖图。
func NewWithDependencies(cfg config.Config, obs *observability.Manager, sessionStore session.Store, modelStore model.Store) *Bootstrap {
	return NewWithRuntimeDependencies(cfg, obs, sessionStore, modelStore, nil)
}

// NewWithRuntimeDependencies builds the bootstrap graph with an optional runtime persistence store.
// NewWithRuntimeDependencies 会使用可选的 runtime 持久化 store 构建启动依赖图。
func NewWithRuntimeDependencies(cfg config.Config, obs *observability.Manager, sessionStore session.Store, modelStore model.Store, runtimeStore runtimepkg.RuntimePersistenceStore) *Bootstrap {
	if obs == nil {
		obs = NewDefaultObservability(cfg)
	}
	application := app.NewServiceWithRuntimeStore(cfg, obs, sessionStore, modelStore, runtimeStore)
	return &Bootstrap{
		Config:       cfg,
		App:          application,
		Server:       httpserver.NewHTTPServer(cfg, application),
		SessionStore: sessionStore,
		ModelStore:   modelStore,
		RuntimeStore: runtimeStore,
	}
}

// NewDefaultObservability creates the default manager chosen by the entry layer for the current config.
// NewDefaultObservability 会按当前配置创建入口层默认采用的 observability manager。
func NewDefaultObservability(cfg config.Config) *observability.Manager {
	return observability.NewDefaultManagerWithLevel(observability.LogLevel(cfg.Observability.LogLevel))
}

// NewSessionStore resolves the configured session store implementation.
// NewSessionStore 会按配置解析当前使用的 session store 实现。
func NewSessionStore(cfg config.Config) (session.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Session.Driver)) {
	case "", "memory":
		return session.NewMemoryStoreWithOptions(
			cfg.Runtime.DeferredQueueLimit,
			time.Duration(cfg.Runtime.ClosedTokenTTLSecs)*time.Second,
		), nil
	case "postgres":
		db, err := session.NewPostgresDB(cfg.PostgresDSN(), postgresGORMConfig(cfg.Database.DBLogMode, cfg.Database.LogZap))
		if err != nil {
			return nil, err
		}
		if err := configurePostgresDB(cfg, db); err != nil {
			return nil, err
		}
		if err := validatePostgresConnectivity("session store", db); err != nil {
			return nil, err
		}
		return session.NewPostgresStoreWithOptions(
			db,
			cfg.Runtime.DeferredQueueLimit,
			time.Duration(cfg.Runtime.ClosedTokenTTLSecs)*time.Second,
			cfg.Session.PostgresUpdateRetries,
		), nil
	default:
		return nil, fmt.Errorf("unsupported session store driver: %s", cfg.Session.Driver)
	}
}

// NewModelStore resolves the configured model governance store implementation.
// NewModelStore 会按配置解析当前使用的模型治理 store 实现。
func NewModelStore(cfg config.Config) (model.Store, error) {
	switch strings.ToLower(strings.TrimSpace(cfg.Model.StoreDriver)) {
	case "", model.StoreDriverMemory:
		return model.NewMemoryStore(cfg.Security.EncryptionKey), nil
	case model.StoreDriverPostgres:
		db, err := model.NewPostgresDB(cfg.PostgresDSN(), postgresGORMConfig(cfg.Database.DBLogMode, cfg.Database.LogZap))
		if err != nil {
			return nil, err
		}
		if err := configurePostgresDB(cfg, db); err != nil {
			return nil, err
		}
		if err := validatePostgresConnectivity("model store", db); err != nil {
			return nil, err
		}
		return model.NewPostgresStore(db, cfg.Security.EncryptionKey), nil
	default:
		return nil, fmt.Errorf("unsupported model store driver: %s", cfg.Model.StoreDriver)
	}
}

// NewRuntimeStore resolves the configured runtime persistence store implementation.
// NewRuntimeStore 会按配置解析当前使用的 runtime 持久化 store 实现。
func NewRuntimeStore(cfg config.Config) (runtimepkg.RuntimePersistenceStore, error) {
	if !runtimePostgresEnabled(cfg) {
		return nil, nil
	}
	db, err := session.NewPostgresDB(cfg.PostgresDSN(), postgresGORMConfig(cfg.Database.DBLogMode, cfg.Database.LogZap))
	if err != nil {
		return nil, err
	}
	if err := configurePostgresDB(cfg, db); err != nil {
		return nil, err
	}
	if err := validatePostgresConnectivity("runtime store", db); err != nil {
		return nil, err
	}
	return runtimepkg.NewPostgresRuntimeStore(db), nil
}

func runtimePostgresEnabled(cfg config.Config) bool {
	if strings.TrimSpace(cfg.PostgresDSN()) == "" {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(cfg.Session.Driver), "postgres") ||
		strings.EqualFold(strings.TrimSpace(cfg.Model.StoreDriver), model.StoreDriverPostgres)
}

// configurePostgresDB applies pool settings onto the resolved GORM postgres connection.
// configurePostgresDB 会把连接池参数应用到当前解析出的 GORM postgres 连接上。
func configurePostgresDB(cfg config.Config, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(cfg.Database.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.Database.MaxOpenConns)
	sqlDB.SetConnMaxLifetime(cfg.DatabaseConnMaxLifetime())
	return nil
}

// validatePostgresConnectivity fails fast during startup when the configured PostgreSQL backend is unreachable.
// validatePostgresConnectivity 会在启动阶段尽早校验 PostgreSQL 可达性，避免服务启动后才暴露数据库不可用问题。
func validatePostgresConnectivity(name string, db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := sqlDB.PingContext(ctx); err != nil {
		return fmt.Errorf("%s postgres connectivity check failed: %w", name, err)
	}
	return nil
}

// MigrateSessionStore runs storage migration for the configured session backend.
// MigrateSessionStore 会为当前配置的 session backend 执行存储迁移。
func MigrateStores(ctx context.Context, cfg config.Config) error {
	store, err := NewSessionStore(cfg)
	if err != nil {
		return err
	}
	if postgresStore, ok := store.(*session.PostgresStore); ok {
		if err := postgresStore.AutoMigrate(ctx); err != nil {
			return err
		}
	}
	modelStore, err := NewModelStore(cfg)
	if err != nil {
		return err
	}
	if postgresStore, ok := modelStore.(*model.PostgresStore); ok {
		if err := postgresStore.AutoMigrate(ctx); err != nil {
			return err
		}
	}
	runtimeStore, err := NewRuntimeStore(cfg)
	if err != nil {
		return err
	}
	if runtimeStore != nil {
		if err := runtimeStore.AutoMigrate(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Startup probes the configured model once before the HTTP server starts accepting requests.
// Startup 会在 HTTP 服务接收请求前先探测一次当前模型配置。
func (b *Bootstrap) Startup(ctx context.Context) {
	b.App.StartupModelGreeting(ctx)
}

// VersionString renders the current build metadata for the version command.
// VersionString 会输出当前构建元信息，供 version 命令打印。
func VersionString() string {
	return fmt.Sprintf(
		"current version:%s\ncurrent branch:%s\ncurrent commit:%s\ncurrent build time:%s\n",
		mkh_utils.Version,
		mkh_utils.Branch,
		mkh_utils.Commit,
		mkh_utils.BuildTime,
	)
}
