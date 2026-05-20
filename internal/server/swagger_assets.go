// swagger_assets.go exposes the embedded Swagger UI assets through the HTTP server.
// swagger_assets.go 负责通过 HTTP 服务暴露嵌入式 Swagger UI 静态资源。
package server

import (
	"context"
	_ "embed"

	hertzapp "github.com/cloudwego/hertz/pkg/app"
	"github.com/cloudwego/hertz/pkg/protocol/consts"
)

var (
	//go:embed swaggerui/swagger-ui.css
	swaggerUICSS []byte

	//go:embed swaggerui/swagger-ui-bundle.js
	swaggerUIBundleJS []byte
)

func handleSwaggerCSS() hertzapp.HandlerFunc {
	return func(_ context.Context, c *hertzapp.RequestContext) {
		c.Data(consts.StatusOK, "text/css; charset=utf-8", swaggerUICSS)
	}
}

func handleSwaggerBundleJS() hertzapp.HandlerFunc {
	return func(_ context.Context, c *hertzapp.RequestContext) {
		c.Data(consts.StatusOK, "application/javascript; charset=utf-8", swaggerUIBundleJS)
	}
}
