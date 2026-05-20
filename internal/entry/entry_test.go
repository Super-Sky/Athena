package entry

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"moss/internal/app"
	"moss/internal/config"
	"moss/internal/model"
	"moss/internal/observability"
	"moss/internal/session"
)

func mustPortFromURL(t *testing.T, raw string) int {
	t.Helper()

	parsed, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("url.Parse(%q) error = %v", raw, err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("Atoi(%q) error = %v", parsed.Port(), err)
	}
	return port
}

type failingResolveStore struct {
	model.Store
	err error
}

func (s failingResolveStore) Resolve(context.Context, string) (*model.Selection, error) {
	return nil, s.err
}

// TestNewSessionStoreReturnsMemoryStore verifies the default session store wiring stays in-memory.
// TestNewSessionStoreReturnsMemoryStore 用于验证默认 session store 装配仍然返回内存实现。
func TestNewSessionStoreReturnsMemoryStore(t *testing.T) {
	store, err := NewSessionStore(config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit: 1,
			ClosedTokenTTLSecs: 1,
		},
		Session: config.SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
	})
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}
	if store == nil {
		t.Fatalf("NewSessionStore() returned nil store")
	}
}

func TestNewSessionStoreRejectsUnavailablePostgres(t *testing.T) {
	_, err := NewSessionStore(config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit: 1,
			ClosedTokenTTLSecs: 1,
		},
		Session: config.SessionConfig{
			Driver:                "postgres",
			PostgresUpdateRetries: 1,
		},
		Database: config.DatabaseConfig{
			DBType:          "postgres",
			DBHost:          "127.0.0.1",
			DBPort:          1,
			DBName:          "secure_digital",
			DBUsername:      "postgres",
			DBConfig:        "sslmode=disable search_path=athena",
			MaxIdleConns:    1,
			MaxOpenConns:    1,
			ConnMaxLifetime: 1,
		},
	})
	if err == nil {
		t.Fatalf("expected postgres connectivity error")
	}
}

func TestNewModelStoreRejectsUnavailablePostgres(t *testing.T) {
	_, err := NewModelStore(config.Config{
		Model: config.ModelConfig{
			StoreDriver: model.StoreDriverPostgres,
		},
		Session: config.SessionConfig{
			PostgresUpdateRetries: 1,
		},
		Database: config.DatabaseConfig{
			DBType:          "postgres",
			DBHost:          "127.0.0.1",
			DBPort:          1,
			DBName:          "secure_digital",
			DBUsername:      "postgres",
			DBConfig:        "sslmode=disable search_path=athena",
			MaxIdleConns:    1,
			MaxOpenConns:    1,
			ConnMaxLifetime: 1,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "entry-test-encryption-key",
		},
	})
	if err == nil {
		t.Fatalf("expected postgres connectivity error")
	}
}

// TestMigrateStoresSkipsMemory verifies migrate keeps memory-backed stores as a no-op.
// TestMigrateStoresSkipsMemory 用于验证 migrate 对内存版 store 会保持空操作。
func TestMigrateStoresSkipsMemory(t *testing.T) {
	err := MigrateStores(context.Background(), config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        1,
			ClosedTokenTTLSecs:        1,
			SkillPackageRevisionLimit: 1,
		},
		Session: config.SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Model: config.ModelConfig{
			StoreDriver: "memory",
		},
	})
	if err != nil {
		t.Fatalf("MigrateStores() error = %v", err)
	}
}

// TestNewWithObservabilityUsesInjectedManager verifies the entry layer keeps observability selection outside app/runtime wiring.
// TestNewWithObservabilityUsesInjectedManager 用于验证入口层会保留注入的 observability manager，而不是在下层重新创建。
func TestNewWithObservabilityUsesInjectedManager(t *testing.T) {
	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        1,
			ClosedTokenTTLSecs:        1,
			SkillPackageRevisionLimit: 1,
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     1,
		},
		Session: config.SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Model: config.ModelConfig{
			StoreDriver: model.StoreDriverMemory,
		},
		Observability: config.ObservabilityConfig{
			LogLevel: "warn",
		},
	}

	injected := observability.NewNoopManager()
	bootstrap, err := NewWithObservability(cfg, injected)
	if err != nil {
		t.Fatalf("NewWithObservability() error = %v", err)
	}
	if bootstrap.App.Observability != injected {
		t.Fatalf("bootstrap.App.Observability was not the injected manager")
	}
}

// TestNewWithObservabilityWithoutDefaultModelKeepsPreparedExecutionUnavailable verifies startup no longer seeds a default model from legacy config.
// TestNewWithObservabilityWithoutDefaultModelKeepsPreparedExecutionUnavailable 用于验证启动阶段不再从旧配置自动植入默认模型。
func TestNewWithObservabilityWithoutDefaultModelKeepsPreparedExecutionUnavailable(t *testing.T) {
	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        2,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 1,
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     30,
		},
		Session: config.SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Model: config.ModelConfig{
			StoreDriver: model.StoreDriverMemory,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "entry-test-encryption-key",
		},
	}

	bootstrap, err := NewWithObservability(cfg, observability.NewNoopManager())
	if err != nil {
		t.Fatalf("NewWithObservability() error = %v", err)
	}

	if _, err := bootstrap.ModelStore.Resolve(context.Background(), ""); err == nil {
		t.Fatalf("expected missing default model")
	}
	providers, err := bootstrap.ModelStore.ListProviders(context.Background())
	if err != nil {
		t.Fatalf("ListProviders() error = %v", err)
	}
	if len(providers) != 0 {
		t.Fatalf("expected no bootstrap providers, got %#v", providers)
	}
}

// TestBootstrapStartupWarnsAndContinuesWithoutDefaultModel verifies startup probing stays non-blocking when no default model exists.
// TestBootstrapStartupWarnsAndContinuesWithoutDefaultModel 用于验证默认模型缺失时，启动探测会告警并继续，而不会阻断服务启动。
func TestBootstrapStartupWarnsAndContinuesWithoutDefaultModel(t *testing.T) {
	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        2,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 1,
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     30,
		},
		Session: config.SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Model: config.ModelConfig{
			StoreDriver: model.StoreDriverMemory,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "entry-test-encryption-key",
		},
	}

	bootstrap, err := NewWithObservability(cfg, observability.NewNoopManager())
	if err != nil {
		t.Fatalf("NewWithObservability() error = %v", err)
	}

	done := make(chan struct{})
	go func() {
		bootstrap.Startup(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Startup() should not block when default model is missing")
	}
}

// TestBootstrapStartupWarnsAndContinuesOnResolveFailure verifies startup probing still returns when model resolution fails unexpectedly.
// TestBootstrapStartupWarnsAndContinuesOnResolveFailure 用于验证模型解析出现异常时，启动探测仍会告警并继续返回。
func TestBootstrapStartupWarnsAndContinuesOnResolveFailure(t *testing.T) {
	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        2,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 1,
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     30,
		},
		Session: config.SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Security: config.SecurityConfig{
			EncryptionKey: "entry-test-encryption-key",
		},
	}

	baseStore := model.NewMemoryStore(cfg.Security.EncryptionKey)
	service := app.NewServiceWithDependencies(
		cfg,
		observability.NewNoopManager(),
		session.NewMemoryStoreWithOptions(session.DefaultDeferredQueueLimit, session.DefaultClosedResumeTokenTTL),
		failingResolveStore{Store: baseStore, err: errors.New("resolve failed")},
	)
	bootstrap := &Bootstrap{
		Config:       cfg,
		App:          service,
		SessionStore: service.SessionStore,
		ModelStore:   service.ModelStore,
	}

	done := make(chan struct{})
	go func() {
		bootstrap.Startup(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("Startup() should not block on resolve failures")
	}
}

// TestNewWithDependenciesPostgresStoresFeedPreparedExecution verifies postgres-backed entry wiring can resolve one default governance model into runtime prepared spec.
// TestNewWithDependenciesPostgresStoresFeedPreparedExecution 用于验证 postgres 版入口装配可以把默认治理模型真正送入 runtime prepared spec。
func TestNewWithDependenciesPostgresStoresFeedPreparedExecution(t *testing.T) {
	t.Parallel()

	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres entry integration test: ATHENA_PG_TEST_DSN is not set")
	}

	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        2,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 1,
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     30,
		},
		Session: config.SessionConfig{
			Driver:                "postgres",
			PostgresUpdateRetries: 1,
		},
		Model: config.ModelConfig{
			StoreDriver: model.StoreDriverPostgres,
		},
		Database: buildDatabaseConfigFromDSN(t, dsn),
		Security: config.SecurityConfig{
			EncryptionKey: "postgres-entry-test-encryption-key",
		},
	}

	if err := MigrateStores(context.Background(), cfg); err != nil {
		t.Fatalf("MigrateStores() error = %v", err)
	}

	modelStore, err := NewModelStore(cfg)
	if err != nil {
		t.Fatalf("NewModelStore() error = %v", err)
	}
	sessionStore, err := NewSessionStore(cfg)
	if err != nil {
		t.Fatalf("NewSessionStore() error = %v", err)
	}

	provider, modelRecord := seedPostgresEntryModel(t, modelStore)

	bootstrap := NewWithDependencies(cfg, observability.NewNoopManager(), sessionStore, modelStore)
	chatSession, err := bootstrap.App.OpenChatSession(context.Background(), "req-entry-postgres", appRequestForModelVerification("", modelRecord.ID))
	if err != nil {
		t.Fatalf("OpenChatSession() error = %v", err)
	}
	defer cleanupPostgresEntryArtifacts(t, dsn, provider.ID, chatSession.SessionID)
	defer chatSession.Release()

	if chatSession.Prepared == nil {
		t.Fatalf("expected prepared execution")
	}
	if chatSession.Prepared.Spec == nil {
		t.Fatalf("expected prepared spec, got status=%q initial=%#v error=%#v", chatSession.Prepared.InitialStatus, chatSession.Prepared.Initial, chatSession.Prepared.InitialError)
	}
	if got := chatSession.Prepared.Spec.Model.Requested.ModelRecordID; got != modelRecord.ID {
		t.Fatalf("prepared requested model record id = %q, want %q", got, modelRecord.ID)
	}
	if got := chatSession.Prepared.Spec.Model.Requested.ProviderModelID; got != modelRecord.ModelID {
		t.Fatalf("prepared requested provider model id = %q, want %q", got, modelRecord.ModelID)
	}
	if got := chatSession.Prepared.Spec.Model.Executed.ModelRecordID; got != modelRecord.ID {
		t.Fatalf("prepared executed model record id = %q, want %q", got, modelRecord.ID)
	}
	if chatSession.Prepared.Initial != nil {
		t.Fatalf("expected graph-prepared execution without initial action, got %#v", chatSession.Prepared.Initial)
	}
	if chatSession.Prepared.Runner == nil {
		t.Fatalf("expected graph-prepared execution to include Eino runner")
	}
}

// TestMigrateStoresCreatesRuntimePersistenceTables verifies runtime persistence participates in the existing migration path.
// TestMigrateStoresCreatesRuntimePersistenceTables 用于验证 runtime 持久化会接入现有 migration 路径。
func TestMigrateStoresCreatesRuntimePersistenceTables(t *testing.T) {
	t.Parallel()

	dsn := strings.TrimSpace(os.Getenv("ATHENA_PG_TEST_DSN"))
	if dsn == "" {
		t.Skip("skip postgres entry integration test: ATHENA_PG_TEST_DSN is not set")
	}

	cfg := config.Config{
		Runtime: config.RuntimeConfig{
			DeferredQueueLimit:        2,
			ClosedTokenTTLSecs:        int(session.DefaultClosedResumeTokenTTL / time.Second),
			SkillPackageRevisionLimit: 1,
		},
		Session: config.SessionConfig{
			Driver:                "postgres",
			PostgresUpdateRetries: 1,
		},
		Model: config.ModelConfig{
			StoreDriver: model.StoreDriverPostgres,
		},
		Database: buildDatabaseConfigFromDSN(t, dsn),
		Security: config.SecurityConfig{
			EncryptionKey: "postgres-entry-test-encryption-key",
		},
	}

	if err := MigrateStores(context.Background(), cfg); err != nil {
		t.Fatalf("MigrateStores() error = %v", err)
	}

	db, err := session.NewPostgresDB(cfg.PostgresDSN())
	if err != nil {
		t.Fatalf("NewPostgresDB() error = %v", err)
	}
	for _, tableName := range []string{
		"runtime_contracts",
		"runtime_task_types",
		"runtime_hook_bindings",
		"system_truth_sources",
		"system_truth_drafts",
		"system_truth_compile_results",
		"system_truth_active_versions",
		"task_runs",
		"task_steps",
		"runtime_traces",
		"runtime_usage",
		"task_run_lifecycle_events",
		"runtime_projections",
	} {
		if !db.Migrator().HasTable(tableName) {
			t.Fatalf("runtime migration did not create table %q", tableName)
		}
	}
}

func appRequestForModelVerification(sessionID string, modelID string) app.ChatRequest {
	return app.ChatRequest{
		Query:     "show user order summary",
		SessionID: sessionID,
		ModelID:   modelID,
	}
}

func buildDatabaseConfigFromDSN(t *testing.T, dsn string) config.DatabaseConfig {
	t.Helper()

	parsed, err := url.Parse(strings.TrimSpace(dsn))
	if err != nil {
		t.Fatalf("Parse(ATHENA_PG_TEST_DSN) error = %v", err)
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil {
		t.Fatalf("Atoi(ATHENA_PG_TEST_DSN port) error = %v", err)
	}
	queryParts := make([]string, 0, len(parsed.Query()))
	for key, values := range parsed.Query() {
		for _, value := range values {
			queryParts = append(queryParts, key+"="+value)
		}
	}

	password, _ := parsed.User.Password()
	return config.DatabaseConfig{
		DBType:          "postgres",
		DBHost:          parsed.Hostname(),
		DBPort:          port,
		DBConfig:        strings.Join(queryParts, " "),
		DBName:          strings.TrimPrefix(parsed.Path, "/"),
		DBUsername:      parsed.User.Username(),
		DBPassword:      password,
		MaxIdleConns:    2,
		MaxOpenConns:    4,
		ConnMaxLifetime: 60,
	}
}

func seedPostgresEntryModel(t *testing.T, store model.Store) (model.ProviderDefinition, model.ProviderModelRecord) {
	t.Helper()
	ctx := context.Background()
	uniqueSuffix := time.Now().UTC().Format("20060102150405.000000000")
	provider, err := store.CreateProvider(ctx, model.ProviderUpsertInput{
		Name:                  "entry-itest-provider-" + uniqueSuffix,
		BaseURL:               "https://example.com/v1",
		Protocol:              model.ProtocolOpenAICompatible,
		APIKey:                "sk-entry-itest",
		RequestTimeoutSeconds: 30,
		Enabled:               true,
	})
	if err != nil {
		t.Fatalf("CreateProvider() error = %v", err)
	}
	modelRecord, err := store.CreateProviderModel(ctx, provider.ID, model.ProviderModelUpsertInput{
		ModelID:     "entry-model-" + uniqueSuffix,
		DisplayName: "Entry Model " + uniqueSuffix,
		Enabled:     true,
		IsDefault:   true,
	})
	if err != nil {
		t.Fatalf("CreateProviderModel() error = %v", err)
	}
	return provider, modelRecord
}

func cleanupPostgresEntryArtifacts(t *testing.T, dsn string, providerID string, sessionID string) {
	t.Helper()
	modelDB, err := model.NewPostgresDB(dsn)
	if err != nil {
		t.Fatalf("model.NewPostgresDB() error = %v", err)
	}
	if err := modelDB.Exec(`DELETE FROM model_provider_models WHERE provider_id = ?`, providerID).Error; err != nil {
		t.Fatalf("cleanup model_provider_models failed: %v", err)
	}
	if err := modelDB.Exec(`DELETE FROM model_providers WHERE id = ?`, providerID).Error; err != nil {
		t.Fatalf("cleanup model_providers failed: %v", err)
	}

	sessionDB, err := session.NewPostgresDB(dsn)
	if err != nil {
		t.Fatalf("session.NewPostgresDB() error = %v", err)
	}
	if err := sessionDB.WithContext(context.Background()).Where("session_id = ?", sessionID).Delete(&session.PostgresDeferredMessageModel{}).Error; err != nil {
		t.Fatalf("cleanup session deferred rows failed: %v", err)
	}
	if err := sessionDB.WithContext(context.Background()).Where("id = ?", sessionID).Delete(&session.PostgresSessionModel{}).Error; err != nil {
		t.Fatalf("cleanup sessions failed: %v", err)
	}
}
