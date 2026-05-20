// config.go defines the runtime configuration model, loading order, and validation rules for Athena.
// config.go 定义 Athena 的运行时配置模型、加载顺序和校验规则。
package config

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	defaultConfigDir            = "config"
	defaultHTTPPort             = 8080
	defaultMaxConcurrentReqs    = 256
	defaultMaxConcurrentTools   = 8
	defaultRequestTimeoutSecs   = 60
	defaultDeferredQueueLimit   = 32
	defaultClosedTokenTTLSecs   = 86400
	defaultSkillPackageRevs     = 10
	defaultSharedRootDir        = "shared"
	defaultControlPlaneTTL      = 28800
	defaultControlPlaneAttempts = 5
	defaultCompressionThreshold = 12000
)

// Config captures the top-level runtime configuration assembled from files and environment overrides.
// Config 描述通过配置文件和环境变量覆盖后得到的顶层运行时配置。
type Config struct {
	Server          ServerConfig          `yaml:"server"`
	Model           ModelConfig           `yaml:"model"`
	Runtime         RuntimeConfig         `yaml:"runtime"`
	ControlPlane    ControlPlaneConfig    `yaml:"control_plane"`
	System          SystemConfig          `yaml:"system"`
	PlatformContext PlatformContextConfig `yaml:"platform_context"`
	Session         SessionConfig         `yaml:"session"`
	Database        DatabaseConfig        `yaml:"database"`
	Security        SecurityConfig        `yaml:"security"`
	Observability   ObservabilityConfig   `yaml:"observability"`
}

// ServerConfig groups HTTP server settings.
// ServerConfig 聚合 HTTP 服务相关设置。
type ServerConfig struct {
	HTTPPort int `yaml:"http_port"`
}

// ModelConfig groups model-store related configuration.
// ModelConfig 聚合模型存储相关配置。
type ModelConfig struct {
	StoreDriver string `yaml:"store_driver"`
}

// RuntimeConfig groups runtime concurrency, timeout, and package-governance settings.
// RuntimeConfig 聚合 runtime 并发、超时和 package 治理配置。
type RuntimeConfig struct {
	MaxConcurrentRequests     int    `yaml:"max_concurrent_requests"`
	MaxConcurrentTools        int    `yaml:"max_concurrent_tools"`
	RequestTimeoutSeconds     int    `yaml:"request_timeout_seconds"`
	DeferredQueueLimit        int    `yaml:"deferred_queue_limit"`
	ClosedTokenTTLSecs        int    `yaml:"closed_resume_token_ttl_seconds"`
	SkillPackageRevisionLimit int    `yaml:"skill_package_revision_limit"`
	SharedRootDir             string `yaml:"shared_root_dir"`
}

// ControlPlaneConfig groups the standalone control-plane storage path and browser access policy.
// ControlPlaneConfig 聚合独立控制面的存储路径和浏览器访问策略。
type ControlPlaneConfig struct {
	StorePath         string   `yaml:"store_path"`
	AllowedOrigins    []string `yaml:"allowed_origins"`
	AuthToken         string   `yaml:"auth_token"`
	SessionTTLSecs    int      `yaml:"session_ttl_seconds"`
	MaxFailedAttempts int      `yaml:"max_failed_attempts"`
}

// SystemConfig groups the file-backed system truth directory used by runtime context assets.
// SystemConfig 聚合运行时上下文资产使用的文件化系统真相目录。
type SystemConfig struct {
	TruthDir          string `yaml:"truth_dir"`
	ActiveStateDir    string `yaml:"active_state_dir"`
	CompiledAssetsDir string `yaml:"compiled_assets_dir"`
}

// PlatformContextConfig groups the upstream platform context detail client settings.
// PlatformContextConfig 聚合回调 platform context detail 接口的客户端配置。
type PlatformContextConfig struct {
	BaseURL              string `yaml:"base_url"`
	DetailTimeoutSeconds int    `yaml:"detail_timeout_seconds"`
	AuthHeader           string `yaml:"auth_header"`
	AuthToken            string `yaml:"auth_token"`
	ForwardAuthorization bool   `yaml:"forward_authorization"`
}

// SessionConfig groups session-store related settings.
// SessionConfig 聚合 session 存储相关设置。
type SessionConfig struct {
	Driver                string `yaml:"driver"`
	PostgresUpdateRetries int    `yaml:"postgres_update_retries"`
}

// DatabaseConfig groups shared database connectivity settings.
// DatabaseConfig 聚合共享数据库连接配置。
type DatabaseConfig struct {
	DBType          string `yaml:"db_type"`
	DBHost          string `yaml:"db_host"`
	DBPort          int    `yaml:"db_port"`
	DBConfig        string `yaml:"db_config"`
	DBName          string `yaml:"db_name"`
	DBUsername      string `yaml:"db_username"`
	DBPassword      string `yaml:"db_password"`
	MaxIdleConns    int    `yaml:"max_idle_conns"`
	MaxOpenConns    int    `yaml:"max_open_conns"`
	DBLogMode       bool   `yaml:"db_log_mode"`
	LogZap          string `yaml:"log_zap"`
	ConnMaxLifetime int    `yaml:"conn_max_lifetime_seconds"`
}

// SecurityConfig groups runtime security settings such as encryption secrets.
// SecurityConfig 聚合运行时安全配置，例如加密密钥。
type SecurityConfig struct {
	EncryptionKey string `yaml:"encryption_key"`
}

// ObservabilityConfig groups runtime log and observability settings.
// ObservabilityConfig 聚合运行时日志与可观测配置。
type ObservabilityConfig struct {
	LogLevel string `yaml:"log_level"`
}

// LoadFromEnv loads config files, applies environment overrides, and validates the final config.
// LoadFromEnv 负责加载配置文件、应用环境变量覆盖，并校验最终配置。
func LoadFromEnv() (Config, error) {
	loadLocalEnvFiles(".env.local", ".env")

	cfg := Config{
		Server: ServerConfig{
			HTTPPort: envInt("HTTP_PORT", defaultHTTPPort),
		},
		Model: ModelConfig{
			StoreDriver: defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("MODEL_STORE_DRIVER"))), "postgres"),
		},
		Runtime: RuntimeConfig{
			MaxConcurrentRequests:     envInt("MAX_CONCURRENT_REQUESTS", defaultMaxConcurrentReqs),
			MaxConcurrentTools:        envInt("MAX_CONCURRENT_TOOLS", defaultMaxConcurrentTools),
			RequestTimeoutSeconds:     envInt("REQUEST_TIMEOUT_SECONDS", defaultRequestTimeoutSecs),
			DeferredQueueLimit:        envInt("DEFERRED_QUEUE_LIMIT", defaultDeferredQueueLimit),
			ClosedTokenTTLSecs:        envInt("CLOSED_RESUME_TOKEN_TTL_SECONDS", defaultClosedTokenTTLSecs),
			SkillPackageRevisionLimit: envInt("SKILL_PACKAGE_REVISION_LIMIT", defaultSkillPackageRevs),
			SharedRootDir:             defaultString(strings.TrimSpace(os.Getenv("SHARED_ROOT_DIR")), defaultSharedRootDir),
		},
		ControlPlane: ControlPlaneConfig{
			StorePath:         defaultString(strings.TrimSpace(os.Getenv("CONTROL_PLANE_STORE_PATH")), filepath.Join(defaultConfigDir, "controlplane", "overrides.json")),
			AllowedOrigins:    envStringSlice("CONTROL_PLANE_ALLOWED_ORIGINS"),
			AuthToken:         strings.TrimSpace(os.Getenv("CONTROL_PLANE_AUTH_TOKEN")),
			SessionTTLSecs:    envInt("CONTROL_PLANE_SESSION_TTL_SECONDS", defaultControlPlaneTTL),
			MaxFailedAttempts: envInt("CONTROL_PLANE_MAX_FAILED_ATTEMPTS", defaultControlPlaneAttempts),
		},
		System: SystemConfig{
			TruthDir:          defaultString(strings.TrimSpace(os.Getenv("SYSTEM_TRUTH_DIR")), filepath.Join(defaultConfigDir, "system", "truth")),
			ActiveStateDir:    defaultString(strings.TrimSpace(os.Getenv("SYSTEM_ACTIVE_STATE_DIR")), filepath.Join("output", "system-state")),
			CompiledAssetsDir: defaultString(strings.TrimSpace(os.Getenv("SYSTEM_COMPILED_ASSETS_DIR")), filepath.Join("output", "system-assets")),
		},
		PlatformContext: PlatformContextConfig{
			BaseURL:              strings.TrimSpace(os.Getenv("PLATFORM_CONTEXT_BASE_URL")),
			DetailTimeoutSeconds: envInt("PLATFORM_CONTEXT_DETAIL_TIMEOUT_SECONDS", 5),
			AuthHeader:           defaultString(strings.TrimSpace(os.Getenv("PLATFORM_CONTEXT_AUTH_HEADER")), "Authorization"),
			AuthToken:            strings.TrimSpace(os.Getenv("PLATFORM_CONTEXT_AUTH_TOKEN")),
			ForwardAuthorization: envBool("PLATFORM_CONTEXT_FORWARD_AUTHORIZATION", true),
		},
		Session: SessionConfig{
			Driver:                defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("SESSION_STORE_DRIVER"))), "postgres"),
			PostgresUpdateRetries: envInt("SESSION_STORE_POSTGRES_UPDATE_RETRIES", 3),
		},
		Database: DatabaseConfig{
			DBType:          defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("DB_TYPE"))), "postgres"),
			DBHost:          strings.TrimSpace(os.Getenv("DB_HOST")),
			DBPort:          envInt("DB_PORT", 5432),
			DBConfig:        strings.TrimSpace(os.Getenv("DB_CONFIG")),
			DBName:          strings.TrimSpace(os.Getenv("DB_NAME")),
			DBUsername:      strings.TrimSpace(os.Getenv("DB_USERNAME")),
			DBPassword:      strings.TrimSpace(os.Getenv("DB_PASSWORD")),
			MaxIdleConns:    envInt("DB_MAX_IDLE_CONNS", 10),
			MaxOpenConns:    envInt("DB_MAX_OPEN_CONNS", 70),
			DBLogMode:       strings.EqualFold(strings.TrimSpace(os.Getenv("DB_LOG_MODE")), "true"),
			LogZap:          strings.TrimSpace(os.Getenv("DB_LOG_ZAP")),
			ConnMaxLifetime: envInt("DB_CONN_MAX_LIFETIME_SECONDS", 300),
		},
		Security: SecurityConfig{
			EncryptionKey: strings.TrimSpace(os.Getenv("SECURITY_ENCRYPTION_KEY")),
		},
		Observability: ObservabilityConfig{
			LogLevel: defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("OBSERVABILITY_LOG_LEVEL"))), "info"),
		},
	}

	if err := loadConfigFiles(defaultConfigDir, strings.TrimSpace(os.Getenv("APP_ENV")), &cfg); err != nil {
		return Config{}, err
	}

	applyEnvOverrides(&cfg)

	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

// loadConfigFiles merges base and APP_ENV-specific yaml files into one config struct.
// loadConfigFiles 会把基础配置和 APP_ENV 对应的 yaml 配置合并到同一个 config 中。
func loadConfigFiles(configDir string, appEnv string, cfg *Config) error {
	if cfg == nil {
		return fmt.Errorf("config target is required")
	}

	paths := []string{filepath.Join(configDir, "config.yml")}
	if appEnv != "" {
		paths = append(paths, filepath.Join(configDir, "config."+appEnv+".yml"))
	}

	for _, path := range paths {
		if err := mergeYAMLConfig(path, cfg); err != nil {
			return err
		}
	}

	return nil
}

// mergeYAMLConfig overlays one yaml file onto the current config struct when the file exists.
// mergeYAMLConfig 会在文件存在时把单个 yaml 文件覆盖合并到当前 config 结构中。
func mergeYAMLConfig(path string, cfg *Config) error {
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read config file %s failed: %w", path, err)
	}
	if len(payload) == 0 {
		return nil
	}
	if err := yaml.Unmarshal(payload, cfg); err != nil {
		return fmt.Errorf("parse config file %s failed: %w", path, err)
	}
	return nil
}

// applyEnvOverrides keeps env vars as the highest-priority override layer on top of yaml files.
// applyEnvOverrides 会把环境变量继续作为 yaml 配置之上的最高优先级覆盖层。
func applyEnvOverrides(cfg *Config) {
	if cfg == nil {
		return
	}

	cfg.Server.HTTPPort = envInt("HTTP_PORT", cfg.Server.HTTPPort)
	cfg.Model.StoreDriver = defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("MODEL_STORE_DRIVER"))), cfg.Model.StoreDriver)
	cfg.Runtime.MaxConcurrentRequests = envInt("MAX_CONCURRENT_REQUESTS", cfg.Runtime.MaxConcurrentRequests)
	cfg.Runtime.MaxConcurrentTools = envInt("MAX_CONCURRENT_TOOLS", cfg.Runtime.MaxConcurrentTools)
	cfg.Runtime.RequestTimeoutSeconds = envInt("REQUEST_TIMEOUT_SECONDS", cfg.Runtime.RequestTimeoutSeconds)
	cfg.Runtime.DeferredQueueLimit = envInt("DEFERRED_QUEUE_LIMIT", cfg.Runtime.DeferredQueueLimit)
	cfg.Runtime.ClosedTokenTTLSecs = envInt("CLOSED_RESUME_TOKEN_TTL_SECONDS", cfg.Runtime.ClosedTokenTTLSecs)
	cfg.Runtime.SkillPackageRevisionLimit = envInt("SKILL_PACKAGE_REVISION_LIMIT", cfg.Runtime.SkillPackageRevisionLimit)
	cfg.Runtime.SharedRootDir = defaultString(strings.TrimSpace(os.Getenv("SHARED_ROOT_DIR")), cfg.Runtime.SharedRootDir)
	cfg.ControlPlane.StorePath = defaultString(strings.TrimSpace(os.Getenv("CONTROL_PLANE_STORE_PATH")), cfg.ControlPlane.StorePath)
	if values := envStringSlice("CONTROL_PLANE_ALLOWED_ORIGINS"); len(values) > 0 {
		cfg.ControlPlane.AllowedOrigins = values
	}
	cfg.ControlPlane.AuthToken = defaultString(strings.TrimSpace(os.Getenv("CONTROL_PLANE_AUTH_TOKEN")), cfg.ControlPlane.AuthToken)
	cfg.ControlPlane.SessionTTLSecs = envInt("CONTROL_PLANE_SESSION_TTL_SECONDS", cfg.ControlPlane.SessionTTLSecs)
	cfg.ControlPlane.MaxFailedAttempts = envInt("CONTROL_PLANE_MAX_FAILED_ATTEMPTS", cfg.ControlPlane.MaxFailedAttempts)
	cfg.System.TruthDir = defaultString(strings.TrimSpace(os.Getenv("SYSTEM_TRUTH_DIR")), cfg.System.TruthDir)
	cfg.System.ActiveStateDir = defaultString(strings.TrimSpace(os.Getenv("SYSTEM_ACTIVE_STATE_DIR")), cfg.System.ActiveStateDir)
	cfg.System.CompiledAssetsDir = defaultString(strings.TrimSpace(os.Getenv("SYSTEM_COMPILED_ASSETS_DIR")), cfg.System.CompiledAssetsDir)
	cfg.PlatformContext.BaseURL = defaultString(strings.TrimSpace(os.Getenv("PLATFORM_CONTEXT_BASE_URL")), cfg.PlatformContext.BaseURL)
	cfg.PlatformContext.DetailTimeoutSeconds = envInt("PLATFORM_CONTEXT_DETAIL_TIMEOUT_SECONDS", cfg.PlatformContext.DetailTimeoutSeconds)
	cfg.PlatformContext.AuthHeader = defaultString(strings.TrimSpace(os.Getenv("PLATFORM_CONTEXT_AUTH_HEADER")), cfg.PlatformContext.AuthHeader)
	cfg.PlatformContext.AuthToken = defaultString(strings.TrimSpace(os.Getenv("PLATFORM_CONTEXT_AUTH_TOKEN")), cfg.PlatformContext.AuthToken)
	cfg.PlatformContext.ForwardAuthorization = envBool("PLATFORM_CONTEXT_FORWARD_AUTHORIZATION", cfg.PlatformContext.ForwardAuthorization)

	cfg.Session.Driver = defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("SESSION_STORE_DRIVER"))), cfg.Session.Driver)
	cfg.Session.PostgresUpdateRetries = envInt("SESSION_STORE_POSTGRES_UPDATE_RETRIES", cfg.Session.PostgresUpdateRetries)

	cfg.Database.DBType = defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("DB_TYPE"))), cfg.Database.DBType)
	cfg.Database.DBHost = defaultString(strings.TrimSpace(os.Getenv("DB_HOST")), cfg.Database.DBHost)
	cfg.Database.DBPort = envInt("DB_PORT", cfg.Database.DBPort)
	cfg.Database.DBConfig = defaultString(strings.TrimSpace(os.Getenv("DB_CONFIG")), cfg.Database.DBConfig)
	cfg.Database.DBName = defaultString(strings.TrimSpace(os.Getenv("DB_NAME")), cfg.Database.DBName)
	cfg.Database.DBUsername = defaultString(strings.TrimSpace(os.Getenv("DB_USERNAME")), cfg.Database.DBUsername)
	cfg.Database.DBPassword = defaultString(strings.TrimSpace(os.Getenv("DB_PASSWORD")), cfg.Database.DBPassword)
	cfg.Database.MaxIdleConns = envInt("DB_MAX_IDLE_CONNS", cfg.Database.MaxIdleConns)
	cfg.Database.MaxOpenConns = envInt("DB_MAX_OPEN_CONNS", cfg.Database.MaxOpenConns)
	if raw := strings.TrimSpace(os.Getenv("DB_LOG_MODE")); raw != "" {
		cfg.Database.DBLogMode = strings.EqualFold(raw, "true")
	}
	cfg.Database.LogZap = defaultString(strings.TrimSpace(os.Getenv("DB_LOG_ZAP")), cfg.Database.LogZap)
	cfg.Database.ConnMaxLifetime = envInt("DB_CONN_MAX_LIFETIME_SECONDS", cfg.Database.ConnMaxLifetime)
	cfg.Security.EncryptionKey = defaultString(strings.TrimSpace(os.Getenv("SECURITY_ENCRYPTION_KEY")), cfg.Security.EncryptionKey)
	cfg.Observability.LogLevel = defaultString(strings.ToLower(strings.TrimSpace(os.Getenv("OBSERVABILITY_LOG_LEVEL"))), cfg.Observability.LogLevel)
}

// Validate checks whether the assembled config satisfies the runtime minimum constraints.
// Validate 检查组装后的配置是否满足运行时最低约束。
func (c Config) Validate() error {
	if c.Server.HTTPPort <= 0 {
		return fmt.Errorf("HTTP_PORT must be greater than 0")
	}
	if c.Runtime.MaxConcurrentRequests <= 0 {
		return fmt.Errorf("MAX_CONCURRENT_REQUESTS must be greater than 0")
	}
	if c.Runtime.MaxConcurrentTools <= 0 {
		return fmt.Errorf("MAX_CONCURRENT_TOOLS must be greater than 0")
	}
	if c.Runtime.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("REQUEST_TIMEOUT_SECONDS must be greater than 0")
	}
	if c.Runtime.DeferredQueueLimit <= 0 {
		return fmt.Errorf("DEFERRED_QUEUE_LIMIT must be greater than 0")
	}
	if c.Runtime.ClosedTokenTTLSecs <= 0 {
		return fmt.Errorf("CLOSED_RESUME_TOKEN_TTL_SECONDS must be greater than 0")
	}
	if strings.TrimSpace(c.ControlPlane.StorePath) == "" {
		return fmt.Errorf("CONTROL_PLANE_STORE_PATH must not be empty")
	}
	if strings.TrimSpace(c.System.TruthDir) == "" {
		return fmt.Errorf("SYSTEM_TRUTH_DIR must not be empty")
	}
	if strings.TrimSpace(c.System.ActiveStateDir) == "" {
		return fmt.Errorf("SYSTEM_ACTIVE_STATE_DIR must not be empty")
	}
	if strings.TrimSpace(c.System.CompiledAssetsDir) == "" {
		return fmt.Errorf("SYSTEM_COMPILED_ASSETS_DIR must not be empty")
	}
	if overlappingCleanPath(c.System.ActiveStateDir, c.System.CompiledAssetsDir) {
		return fmt.Errorf("SYSTEM_ACTIVE_STATE_DIR must not overlap SYSTEM_COMPILED_ASSETS_DIR")
	}
	if c.ControlPlane.SessionTTLSecs < 0 {
		return fmt.Errorf("CONTROL_PLANE_SESSION_TTL_SECONDS must be greater than or equal to 0")
	}
	if c.ControlPlane.MaxFailedAttempts < 0 {
		return fmt.Errorf("CONTROL_PLANE_MAX_FAILED_ATTEMPTS must be greater than or equal to 0")
	}
	if c.PlatformContext.DetailTimeoutSeconds < 0 {
		return fmt.Errorf("PLATFORM_CONTEXT_DETAIL_TIMEOUT_SECONDS must be greater than or equal to 0")
	}
	if c.Runtime.SkillPackageRevisionLimit <= 0 {
		return fmt.Errorf("SKILL_PACKAGE_REVISION_LIMIT must be greater than 0")
	}
	if c.Session.PostgresUpdateRetries <= 0 {
		return fmt.Errorf("SESSION_STORE_POSTGRES_UPDATE_RETRIES must be greater than 0")
	}
	if c.Database.DBPort < 0 {
		return fmt.Errorf("DB_PORT must be greater than or equal to 0")
	}
	if c.Database.MaxIdleConns <= 0 {
		return fmt.Errorf("DB_MAX_IDLE_CONNS must be greater than 0")
	}
	if c.Database.MaxOpenConns <= 0 {
		return fmt.Errorf("DB_MAX_OPEN_CONNS must be greater than 0")
	}
	if c.Database.ConnMaxLifetime <= 0 {
		return fmt.Errorf("DB_CONN_MAX_LIFETIME_SECONDS must be greater than 0")
	}
	switch strings.ToLower(strings.TrimSpace(c.Observability.LogLevel)) {
	case "", "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("OBSERVABILITY_LOG_LEVEL must be one of debug, info, warn, error")
	}
	switch c.Session.Driver {
	case "memory":
	case "postgres":
		if c.PostgresDSN() == "" {
			return fmt.Errorf("database postgres config is required when SESSION_STORE_DRIVER=postgres")
		}
	default:
		return fmt.Errorf("SESSION_STORE_DRIVER must be one of memory, postgres")
	}
	switch strings.ToLower(strings.TrimSpace(c.Model.StoreDriver)) {
	case "", "memory", "postgres":
	default:
		return fmt.Errorf("MODEL_STORE_DRIVER must be one of memory, postgres")
	}
	if strings.EqualFold(strings.TrimSpace(c.Model.StoreDriver), "postgres") {
		if c.PostgresDSN() == "" {
			return fmt.Errorf("database postgres config is required when MODEL_STORE_DRIVER=postgres")
		}
		if strings.TrimSpace(c.Security.EncryptionKey) == "" {
			return fmt.Errorf("SECURITY_ENCRYPTION_KEY is required when MODEL_STORE_DRIVER=postgres")
		}
	}

	return nil
}

func envStringSlice(name string) []string {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value != "" {
			result = append(result, value)
		}
	}
	return result
}

func overlappingCleanPath(left, right string) bool {
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if left == "" || right == "" {
		return false
	}
	left = filepath.Clean(left)
	right = filepath.Clean(right)
	if left == right {
		return true
	}
	return isPathWithin(left, right) || isPathWithin(right, left)
}

func isPathWithin(child, parent string) bool {
	rel, err := filepath.Rel(parent, child)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// PostgresDSN builds the shared PostgreSQL DSN from the database block.
// PostgresDSN 会从 database 配置块组装共享的 PostgreSQL DSN。
func (c Config) PostgresDSN() string {
	if !strings.EqualFold(strings.TrimSpace(c.Database.DBType), "postgres") {
		return ""
	}
	if strings.TrimSpace(c.Database.DBHost) == "" || c.Database.DBPort <= 0 || strings.TrimSpace(c.Database.DBName) == "" {
		return ""
	}

	queryParts := strings.Fields(strings.TrimSpace(c.Database.DBConfig))
	query := url.Values{}
	for _, item := range queryParts {
		key, value, ok := strings.Cut(item, "=")
		if !ok || strings.TrimSpace(key) == "" {
			continue
		}
		query.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	}

	connURL := &url.URL{
		Scheme:   "postgres",
		Host:     fmt.Sprintf("%s:%d", strings.TrimSpace(c.Database.DBHost), c.Database.DBPort),
		Path:     "/" + strings.TrimSpace(c.Database.DBName),
		RawQuery: query.Encode(),
	}
	if strings.TrimSpace(c.Database.DBUsername) != "" {
		if strings.TrimSpace(c.Database.DBPassword) != "" {
			connURL.User = url.UserPassword(strings.TrimSpace(c.Database.DBUsername), c.Database.DBPassword)
		} else {
			connURL.User = url.User(strings.TrimSpace(c.Database.DBUsername))
		}
	}
	return connURL.String()
}

// DatabaseConnMaxLifetime returns the configured SQL connection max lifetime.
// DatabaseConnMaxLifetime 会返回当前配置的 SQL 连接最大生命周期。
func (c Config) DatabaseConnMaxLifetime() time.Duration {
	return time.Duration(c.Database.ConnMaxLifetime) * time.Second
}

func envInt(key string, fallback int) int {
	raw := strings.TrimSpace(os.Getenv(key))
	if raw == "" {
		return fallback
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return fallback
	}

	return value
}

func envBool(key string, fallback bool) bool {
	raw := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	if raw == "" {
		return fallback
	}
	switch raw {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}

func loadLocalEnvFiles(paths ...string) {
	for _, path := range paths {
		loadLocalEnvFile(path)
	}
}

func loadLocalEnvFile(path string) {
	file, err := os.Open(filepath.Clean(path))
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}

		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		if key == "" || os.Getenv(key) != "" {
			continue
		}

		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		_ = os.Setenv(key, value)
	}
}
