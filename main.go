// main.go dispatches the process-level Athena commands and keeps the repository entry thin.
// main.go 负责分发 Athena 的进程级命令，并保持仓库入口层足够薄。
package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"gitee.com/super_sky/mkh_utils"
	"moss/internal/config"
	"moss/internal/entry"
)

// main dispatches the lightweight athena process commands without changing the repository layering.
// main 会在不破坏仓库分层的前提下分发 athena 的轻量进程命令。
func main() {
	mkh_utils.CheckVersion()

	command := "api-server"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}

	switch command {
	case "version":
		fmt.Print(entry.VersionString())
		return
	case "healthcheck":
		cfg, err := config.LoadFromEnv()
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		runHealthcheck(cfg)
	case "api-server":
		cfg, err := config.LoadFromEnv()
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		runAPIServer(cfg)
	case "migrate":
		cfg, err := config.LoadFromEnv()
		if err != nil {
			log.Fatalf("failed to load config: %v", err)
		}
		runMigrate(cfg)
	default:
		log.Fatalf("unsupported command %q, expected one of: api-server, migrate, healthcheck, version", command)
	}
}

// runAPIServer starts the HTTP service using the configured entry graph.
// runAPIServer 会通过当前配置好的 entry 依赖图启动 HTTP 服务。
func runAPIServer(cfg config.Config) {
	appEntry, err := entry.New(cfg)
	if err != nil {
		log.Fatalf("failed to build entry: %v", err)
	}

	appEntry.Startup(context.Background())

	log.Printf("Starter service listening on http://localhost:%d", cfg.Server.HTTPPort)
	log.Printf("Health check: curl http://localhost:%d/healthz", cfg.Server.HTTPPort)
	log.Printf("Stream chat: curl -N -X POST http://localhost:%d/api/chat/stream -H 'Content-Type: application/json' -d '{\"query\":\"hello\"}'", cfg.Server.HTTPPort)
	appEntry.Server.Spin()
}

// runMigrate executes the configured session-store migration path.
// runMigrate 会执行当前 session store 的 migrate 路径。
func runMigrate(cfg config.Config) {
	if err := entry.MigrateStores(context.Background(), cfg); err != nil {
		log.Fatalf("migrate failed: %v", err)
	}
	log.Printf("storage migrate finished successfully")
}

// runHealthcheck probes the local health endpoint so container runtimes do not need curl inside the image.
// runHealthcheck 会探测本地 health endpoint，避免容器运行时必须内置 curl。
func runHealthcheck(cfg config.Config) {
	if err := entry.RunHealthcheck(cfg); err != nil {
		log.Fatalf("healthcheck failed: %v", err)
	}
}
