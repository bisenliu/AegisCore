package router

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"github.com/aegiscore/user-service/docs"
)

// openAPIEnabledEnv 显式开启或关闭 OpenAPI 文档路由，并覆盖环境默认行为。
const openAPIEnabledEnv = "OPENAPI_ENABLED"

const (
	openAPIJSONPath = "/openapi.json"
	openAPIUIPath   = "/openapi/index.html"
)

// RegisterOpenAPI 按条件挂载 OpenAPI UI、JSON 文档和文档重定向路由。
func RegisterOpenAPI(engine *gin.Engine, environment string) {
	if !openAPIEnabled(environment) {
		return
	}

	engine.GET(openAPIJSONPath, serveOpenAPI)
	engine.GET("/openapi/*any", ginSwagger.WrapHandler(swaggerFiles.Handler, ginSwagger.URL(openAPIJSONPath)))
	engine.GET("/docs", redirectToOpenAPI)
	engine.GET("/api-docs", redirectToOpenAPI)
}

func serveOpenAPI(c *gin.Context) {
	c.Data(http.StatusOK, "application/json; charset=utf-8", docs.ReadOpenAPI())
}

func redirectToOpenAPI(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, openAPIUIPath)
}

func openAPIEnabled(environment string) bool {
	if raw, ok := os.LookupEnv(openAPIEnabledEnv); ok {
		// 显式环境变量覆盖优先于部署环境默认行为。
		if enabled, err := strconv.ParseBool(raw); err == nil {
			return enabled
		}
	}

	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production":
		// 生产环境默认关闭 OpenAPI 文档路由，除非通过 OPENAPI_ENABLED 显式开启。
		return false
	default:
		return true
	}
}
