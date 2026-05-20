package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadFromEnvDefaultsToPostgresStores(t *testing.T) {
	t.Setenv("HTTP_PORT", "8080")
	t.Setenv("MAX_CONCURRENT_REQUESTS", "1")
	t.Setenv("MAX_CONCURRENT_TOOLS", "1")
	t.Setenv("REQUEST_TIMEOUT_SECONDS", "1")
	t.Setenv("DEFERRED_QUEUE_LIMIT", "1")
	t.Setenv("CLOSED_RESUME_TOKEN_TTL_SECONDS", "1")
	t.Setenv("SKILL_PACKAGE_REVISION_LIMIT", "1")
	t.Setenv("SESSION_STORE_POSTGRES_UPDATE_RETRIES", "1")
	t.Setenv("DB_TYPE", "postgres")
	t.Setenv("DB_HOST", "127.0.0.1")
	t.Setenv("DB_PORT", "5432")
	t.Setenv("DB_NAME", "secure_digital")
	t.Setenv("DB_USERNAME", "postgres")
	t.Setenv("DB_MAX_IDLE_CONNS", "1")
	t.Setenv("DB_MAX_OPEN_CONNS", "1")
	t.Setenv("DB_CONN_MAX_LIFETIME_SECONDS", "1")
	t.Setenv("SECURITY_ENCRYPTION_KEY", "test-encryption-key")

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd() error = %v", err)
	}
	if err := os.Chdir(filepath.Clean(filepath.Join(cwd, "..", ".."))); err != nil {
		t.Fatalf("Chdir() error = %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(cwd)
	})

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv() error = %v", err)
	}
	if cfg.Session.Driver != "postgres" {
		t.Fatalf("cfg.Session.Driver = %q, want postgres", cfg.Session.Driver)
	}
	if cfg.Model.StoreDriver != "postgres" {
		t.Fatalf("cfg.Model.StoreDriver = %q, want postgres", cfg.Model.StoreDriver)
	}
	if cfg.System.ActiveStateDir != filepath.Join("output", "system-state") {
		t.Fatalf("cfg.System.ActiveStateDir = %q, want output/system-state", cfg.System.ActiveStateDir)
	}
}

// TestConfigValidateAcceptsMemorySessionStore verifies the default in-process session store stays valid.
// TestConfigValidateAcceptsMemorySessionStore 用于验证默认内存版 session store 仍能通过配置校验。
func TestConfigValidateAcceptsMemorySessionStore(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{HTTPPort: 8080},
		Model:  ModelConfig{StoreDriver: "memory"},
		Runtime: RuntimeConfig{
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     1,
			DeferredQueueLimit:        1,
			ClosedTokenTTLSecs:        1,
			SkillPackageRevisionLimit: 1,
		},
		ControlPlane: ControlPlaneConfig{
			StorePath: filepath.Join("config", "controlplane", "overrides.json"),
		},
		System: SystemConfig{
			TruthDir:          filepath.Join("config", "system", "truth"),
			ActiveStateDir:    filepath.Join("output", "system-state"),
			CompiledAssetsDir: filepath.Join("output", "system-assets"),
		},
		Session: SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Database: DatabaseConfig{
			DBPort:          5432,
			MaxIdleConns:    1,
			MaxOpenConns:    1,
			ConnMaxLifetime: 1,
		},
	}

	if err := cfg.Validate(); err != nil {
		t.Fatalf("Validate() error = %v", err)
	}
}

// TestConfigValidateRejectsPostgresWithoutDSN verifies postgres mode requires the database block.
// TestConfigValidateRejectsPostgresWithoutDSN 用于验证 postgres 模式必须提供 database 配置块。
func TestConfigValidateRejectsPostgresWithoutDSN(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{HTTPPort: 8080},
		Model:  ModelConfig{StoreDriver: "postgres"},
		Runtime: RuntimeConfig{
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     1,
			DeferredQueueLimit:        1,
			ClosedTokenTTLSecs:        1,
			SkillPackageRevisionLimit: 1,
		},
		ControlPlane: ControlPlaneConfig{
			StorePath: filepath.Join("config", "controlplane", "overrides.json"),
		},
		System: SystemConfig{
			TruthDir:          filepath.Join("config", "system", "truth"),
			ActiveStateDir:    filepath.Join("output", "system-state"),
			CompiledAssetsDir: filepath.Join("output", "system-assets"),
		},
		Session: SessionConfig{
			Driver:                "postgres",
			PostgresUpdateRetries: 1,
		},
		Database: DatabaseConfig{
			DBType:          "postgres",
			DBPort:          5432,
			MaxIdleConns:    1,
			MaxOpenConns:    1,
			ConnMaxLifetime: 1,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() expected error when postgres dsn is missing")
	}
}

// TestLoadConfigFilesMergesBaseAndEnvSpecific verifies config/config.yml and config/config.<env>.yml merge in order.
// TestLoadConfigFilesMergesBaseAndEnvSpecific 用于验证基础配置与环境配置文件会按顺序合并。
func TestLoadConfigFilesMergesBaseAndEnvSpecific(t *testing.T) {
	t.Parallel()

	configDir := t.TempDir()
	base := []byte(`
server:
  http_port: 9000
runtime:
  deferred_queue_limit: 9
session:
  driver: memory
  postgres_update_retries: 5
`)
	envSpecific := []byte(`
server:
  http_port: 9100
session:
  driver: postgres
`)

	if err := os.WriteFile(filepath.Join(configDir, "config.yml"), base, 0o600); err != nil {
		t.Fatalf("WriteFile(config.yml) error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.test.yml"), envSpecific, 0o600); err != nil {
		t.Fatalf("WriteFile(config.test.yml) error = %v", err)
	}

	cfg := Config{}
	if err := loadConfigFiles(configDir, "test", &cfg); err != nil {
		t.Fatalf("loadConfigFiles() error = %v", err)
	}

	if cfg.Server.HTTPPort != 9100 {
		t.Fatalf("cfg.Server.HTTPPort = %d, want 9100", cfg.Server.HTTPPort)
	}
	if cfg.Runtime.DeferredQueueLimit != 9 {
		t.Fatalf("cfg.Runtime.DeferredQueueLimit = %d, want 9", cfg.Runtime.DeferredQueueLimit)
	}
	if cfg.Session.Driver != "postgres" {
		t.Fatalf("cfg.Session.Driver = %q, want postgres", cfg.Session.Driver)
	}
	if cfg.Session.PostgresUpdateRetries != 5 {
		t.Fatalf("cfg.Session.PostgresUpdateRetries = %d, want 5", cfg.Session.PostgresUpdateRetries)
	}
}

func TestConfigValidateRejectsOverlappingSystemOutputDirs(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{HTTPPort: 8080},
		Model:  ModelConfig{StoreDriver: "memory"},
		Runtime: RuntimeConfig{
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     1,
			DeferredQueueLimit:        1,
			ClosedTokenTTLSecs:        1,
			SkillPackageRevisionLimit: 1,
		},
		ControlPlane: ControlPlaneConfig{
			StorePath: filepath.Join("config", "controlplane", "overrides.json"),
		},
		System: SystemConfig{
			TruthDir:          filepath.Join("config", "system", "truth"),
			ActiveStateDir:    filepath.Join("output", "system"),
			CompiledAssetsDir: filepath.Join("output", "system", "assets"),
		},
		Session: SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Database: DatabaseConfig{
			DBPort:          5432,
			MaxIdleConns:    1,
			MaxOpenConns:    1,
			ConnMaxLifetime: 1,
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() expected error when system output dirs overlap")
	}
}

func TestConfigPostgresDSNBuildsFromDatabaseBlock(t *testing.T) {
	t.Parallel()

	cfg := Config{
		Session: SessionConfig{
			Driver:                "postgres",
			PostgresUpdateRetries: 1,
		},
		Database: DatabaseConfig{
			DBType:     "postgres",
			DBHost:     "192.168.5.201",
			DBPort:     5432,
			DBConfig:   "sslmode=disable search_path=athena client_encoding=UTF8",
			DBName:     "secure_digital",
			DBUsername: "postgres",
			DBPassword: "secret",
		},
	}

	got := cfg.PostgresDSN()
	want := "postgres://postgres:secret@192.168.5.201:5432/secure_digital?client_encoding=UTF8&search_path=athena&sslmode=disable"
	if got != want {
		t.Fatalf("cfg.PostgresDSN() = %q, want %q", got, want)
	}
}

// TestConfigValidateRejectsUnknownObservabilityLogLevel verifies observability log levels stay within the supported four-level set.
// TestConfigValidateRejectsUnknownObservabilityLogLevel 用于验证 observability 日志级别只能使用约定的四个等级。
func TestConfigValidateRejectsUnknownObservabilityLogLevel(t *testing.T) {
	cfg := Config{
		Server: ServerConfig{HTTPPort: 8080},
		Model:  ModelConfig{StoreDriver: "memory"},
		Runtime: RuntimeConfig{
			MaxConcurrentRequests:     1,
			MaxConcurrentTools:        1,
			RequestTimeoutSeconds:     1,
			DeferredQueueLimit:        1,
			ClosedTokenTTLSecs:        1,
			SkillPackageRevisionLimit: 1,
		},
		System: SystemConfig{
			TruthDir:          filepath.Join("config", "system", "truth"),
			ActiveStateDir:    filepath.Join("output", "system-state"),
			CompiledAssetsDir: filepath.Join("output", "system-assets"),
		},
		Session: SessionConfig{
			Driver:                "memory",
			PostgresUpdateRetries: 1,
		},
		Database: DatabaseConfig{
			DBPort:          5432,
			MaxIdleConns:    1,
			MaxOpenConns:    1,
			ConnMaxLifetime: 1,
		},
		Observability: ObservabilityConfig{
			LogLevel: "trace",
		},
	}

	if err := cfg.Validate(); err == nil {
		t.Fatalf("Validate() expected error when observability log level is unsupported")
	}
}
