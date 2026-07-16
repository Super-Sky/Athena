// app_auth.go enforces scoped application identity for app-facing Agent Run APIs.
// app_auth.go 为面向应用的 Agent Run API 强制执行作用域身份认证。
package server

import (
	"context"
	"crypto/subtle"
	"errors"
	"strings"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
	appcore "moss/internal/app"
	"moss/internal/config"
)

const (
	appAuthHeaderAppID         = "X-Athena-App-ID"
	appAuthHeaderToken         = "X-Athena-App-Token"
	appAuthHeaderWorkspaceID   = "X-Athena-Workspace-ID"
	appAuthHeaderAppInstanceID = "X-Athena-App-Instance-ID"
)

var errAgentRunNotFound = errors.New("agent run not found")

type appAuthContextKey struct{}

// AppRequestIdentity is the authenticated and scope-checked application request identity.
// AppRequestIdentity 是完成认证与 scope 校验后的应用请求身份。
type AppRequestIdentity struct {
	AppID         string
	WorkspaceID   string
	AppInstanceID string
}

type appFacingHandler func(context.Context, *hertzapp.RequestContext, config.Config, *appcore.Service)

type appAuthError struct {
	status int
	code   string
}

func (e *appAuthError) Error() string { return e.code }

func withAppAuth(next appFacingHandler) appFacingHandler {
	return func(ctx context.Context, c *hertzapp.RequestContext, cfg config.Config, application *appcore.Service) {
		if !appAuthEnabled(cfg.AppAuth) {
			next(ctx, c, cfg, application)
			return
		}
		identity, err := authenticateAppRequest(c, cfg.AppAuth)
		if err != nil {
			writeAppAuthError(c, err)
			return
		}
		next(context.WithValue(ctx, appAuthContextKey{}, identity), c, cfg, application)
	}
}

func appAuthEnabled(cfg config.AppAuthConfig) bool {
	return cfg.Required
}

func authenticateAppRequest(c *hertzapp.RequestContext, cfg config.AppAuthConfig) (AppRequestIdentity, error) {
	identity := AppRequestIdentity{
		AppID:         strings.TrimSpace(string(c.Request.Header.Peek(appAuthHeaderAppID))),
		WorkspaceID:   strings.TrimSpace(string(c.Request.Header.Peek(appAuthHeaderWorkspaceID))),
		AppInstanceID: strings.TrimSpace(string(c.Request.Header.Peek(appAuthHeaderAppInstanceID))),
	}
	token := strings.TrimSpace(string(c.Request.Header.Peek(appAuthHeaderToken)))
	if identity.AppID == "" || identity.WorkspaceID == "" || identity.AppInstanceID == "" || token == "" {
		return AppRequestIdentity{}, &appAuthError{status: consts.StatusUnauthorized, code: "app_auth_required"}
	}
	for _, configured := range cfg.Identities {
		if strings.TrimSpace(configured.AppID) != identity.AppID || !constantTimeTokenEqual(configured.Token, token) {
			continue
		}
		if !configuredAppScopeAllows(configured, identity.WorkspaceID, identity.AppInstanceID) {
			return AppRequestIdentity{}, &appAuthError{status: consts.StatusNotFound, code: "resource_not_found"}
		}
		return identity, nil
	}
	return AppRequestIdentity{}, &appAuthError{status: consts.StatusUnauthorized, code: "invalid_app_identity"}
}

func constantTimeTokenEqual(expected, actual string) bool {
	expected = strings.TrimSpace(expected)
	actual = strings.TrimSpace(actual)
	if len(expected) == 0 || len(expected) != len(actual) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(actual)) == 1
}

func configuredAppScopeAllows(identity config.AppAuthIdentityConfig, workspaceID, appInstanceID string) bool {
	for _, scope := range identity.Scopes {
		if strings.TrimSpace(scope.WorkspaceID) != workspaceID {
			continue
		}
		for _, configuredInstanceID := range scope.AppInstanceIDs {
			if strings.TrimSpace(configuredInstanceID) == appInstanceID {
				return true
			}
		}
	}
	return false
}

func appRequestIdentity(ctx context.Context) (AppRequestIdentity, bool) {
	identity, ok := ctx.Value(appAuthContextKey{}).(AppRequestIdentity)
	return identity, ok
}

func applyAgentRunIdentityScope(ctx context.Context, req *agentRunStartRequest) error {
	identity, ok := appRequestIdentity(ctx)
	if !ok || req == nil {
		return nil
	}
	if value := strings.TrimSpace(req.WorkspaceID); value != "" && value != identity.WorkspaceID {
		return &appAuthError{status: consts.StatusForbidden, code: "app_scope_mismatch"}
	}
	if value := strings.TrimSpace(req.AppInstanceID); value != "" && value != identity.AppInstanceID {
		return &appAuthError{status: consts.StatusForbidden, code: "app_scope_mismatch"}
	}
	req.WorkspaceID = identity.WorkspaceID
	req.AppInstanceID = identity.AppInstanceID
	return nil
}

func authorizeAgentRunForRequest(ctx context.Context, run runtimeRunDTO) error {
	identity, ok := appRequestIdentity(ctx)
	if !ok {
		return nil
	}
	if strings.TrimSpace(run.WorkspaceID) != identity.WorkspaceID || strings.TrimSpace(run.AppInstanceID) != identity.AppInstanceID {
		return errAgentRunNotFound
	}
	return nil
}

func writeAppAuthError(c *hertzapp.RequestContext, err error) {
	var authErr *appAuthError
	if !errors.As(err, &authErr) {
		c.JSON(consts.StatusInternalServerError, map[string]string{"error": "app_auth_failed"})
		return
	}
	message := authErr.code
	if authErr.status == consts.StatusNotFound {
		message = "resource_not_found"
	}
	c.JSON(authErr.status, map[string]string{"error": message})
}
