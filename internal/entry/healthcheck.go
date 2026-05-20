// healthcheck.go provides the process-level liveness probe used by container healthchecks.
// healthcheck.go 提供容器健康检查所需的进程级活性探针能力。
package entry

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"moss/internal/config"
)

const healthcheckTimeout = 2 * time.Second

// CheckHealthEndpoint probes one HTTP health endpoint and fails on non-200 responses.
// CheckHealthEndpoint 会探测单个 HTTP health endpoint，并在非 200 响应时返回错误。
func CheckHealthEndpoint(ctx context.Context, endpoint string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build health request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("probe health endpoint: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health endpoint returned status %d", resp.StatusCode)
	}
	return nil
}

// RunHealthcheck probes the configured local HTTP health endpoint for container liveness checks.
// RunHealthcheck 会探测当前配置对应的本地 HTTP health endpoint，供容器活性检查使用。
func RunHealthcheck(cfg config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), healthcheckTimeout)
	defer cancel()

	endpoint := fmt.Sprintf("http://127.0.0.1:%d/healthz", cfg.Server.HTTPPort)
	return CheckHealthEndpoint(ctx, endpoint)
}
