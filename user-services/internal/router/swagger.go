package router

import (
	"net/http"
	"os"
	"strconv"
	"strings"

	_ "github.com/aegiscore/user-services/docs"
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// swaggerEnabledEnv 显式开启或关闭 Swagger 路由，并覆盖环境默认行为。
const swaggerEnabledEnv = "SWAGGER_ENABLED"

func RegisterSwagger(engine *gin.Engine, environment string) {
	if !swaggerEnabled(environment) {
		return
	}

	engine.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	engine.GET("/docs", redirectToSwagger)
	engine.GET("/api-docs", redirectToSwagger)
}

func redirectToSwagger(c *gin.Context) {
	c.Redirect(http.StatusMovedPermanently, "/swagger/index.html")
}

func swaggerEnabled(environment string) bool {
	if raw, ok := os.LookupEnv(swaggerEnabledEnv); ok {
		if enabled, err := strconv.ParseBool(raw); err == nil {
			return enabled
		}
	}

	switch strings.ToLower(strings.TrimSpace(environment)) {
	case "prod", "production":
		return false
	default:
		return true
	}
}
