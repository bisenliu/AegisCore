package bootstrap

import (
	"fmt"

	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/config"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

// GinParams 包含创建 Gin engine 所需的 Fx 输入。
type GinParams struct {
	fx.In

	Config *config.Config
	Log    *zap.Logger
}

// NewGinEngine 创建 Gin engine，应用可信代理配置并安装共享中间件。
func NewGinEngine(params GinParams) (*gin.Engine, error) {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	if len(params.Config.HTTP.TrustedProxies) > 0 {
		if err := engine.SetTrustedProxies(params.Config.HTTP.TrustedProxies); err != nil {
			return nil, fmt.Errorf("set trusted proxies: %w", err)
		}
	}
	engine.Use(
		commonmw.TraceID(),
		commonmw.Recovery(params.Log),
		commonmw.RequestLogger(params.Log),
		commonmw.CORS(),
	)
	return engine, nil
}
