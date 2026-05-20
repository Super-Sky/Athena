// postgres_logger.go adapts PostgreSQL logging into the repository's entry and observability expectations.
// postgres_logger.go 负责把 PostgreSQL 日志适配到仓库的入口装配和可观测约定中。
package entry

import (
	"log"
	"os"
	"strings"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func postgresGORMConfig(dbLogMode bool, logLevel string) func(*gorm.Config) {
	return func(cfg *gorm.Config) {
		if cfg == nil {
			return
		}
		if !dbLogMode {
			cfg.Logger = gormlogger.Default.LogMode(gormlogger.Silent)
			return
		}

		level := gormlogger.Info
		switch strings.ToLower(strings.TrimSpace(logLevel)) {
		case "silent":
			level = gormlogger.Silent
		case "error":
			level = gormlogger.Error
		case "warn", "warning":
			level = gormlogger.Warn
		case "", "info", "debug":
			level = gormlogger.Info
		default:
			level = gormlogger.Info
		}

		cfg.Logger = gormlogger.New(
			log.New(os.Stdout, "[gorm] ", log.LstdFlags),
			gormlogger.Config{
				SlowThreshold:             200 * time.Millisecond,
				LogLevel:                  level,
				IgnoreRecordNotFoundError: true,
				Colorful:                  false,
			},
		)
	}
}
