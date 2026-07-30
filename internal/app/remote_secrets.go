// remote_secrets.go resolves outbound remote-tool credentials from runtime-only providers.
// remote_secrets.go 从仅运行时可见的 provider 解析远程工具出站凭据。
package app

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"moss/internal/tools"
)

var remoteSecretEnvNamePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

func validateRemoteToolSecretProvider(auth *tools.RemoteAuth) error {
	if auth == nil {
		return nil
	}
	if !strings.HasPrefix(strings.TrimSpace(auth.SecretRef), "env://") {
		return fmt.Errorf("remote tool secret_ref provider is not installed; current runtime supports env://")
	}
	return nil
}

// resolveRemoteToolSecret reads env-backed secrets on every invocation so rotation and revocation do not require catalog writes.
// resolveRemoteToolSecret 在每次调用时读取环境变量，使轮换和撤销无需改写工具目录。
func resolveRemoteToolSecret(_ context.Context, secretRef string) (tools.RemoteResolvedSecret, error) {
	const prefix = "env://"
	if !strings.HasPrefix(secretRef, prefix) {
		return tools.RemoteResolvedSecret{}, fmt.Errorf("unsupported remote secret reference provider")
	}
	name := strings.TrimPrefix(secretRef, prefix)
	if !remoteSecretEnvNamePattern.MatchString(name) {
		return tools.RemoteResolvedSecret{}, fmt.Errorf("invalid remote secret environment reference")
	}
	value, exists := os.LookupEnv(name)
	if !exists || strings.TrimSpace(value) == "" {
		return tools.RemoteResolvedSecret{}, fmt.Errorf("remote secret environment value is unavailable")
	}
	secret := tools.RemoteResolvedSecret{Value: value}
	if raw := strings.TrimSpace(os.Getenv(name + "_REVOKED")); raw != "" {
		revoked, err := strconv.ParseBool(raw)
		if err != nil {
			return tools.RemoteResolvedSecret{}, fmt.Errorf("invalid remote secret revocation state")
		}
		secret.Revoked = revoked
	}
	if raw := strings.TrimSpace(os.Getenv(name + "_EXPIRES_AT")); raw != "" {
		expiresAt, err := time.Parse(time.RFC3339, raw)
		if err != nil {
			return tools.RemoteResolvedSecret{}, fmt.Errorf("invalid remote secret expiration")
		}
		secret.ExpiresAt = expiresAt
	}
	return secret, nil
}
