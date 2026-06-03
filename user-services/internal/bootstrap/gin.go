package bootstrap

import (
	"fmt"

	"github.com/aegiscore/common/config"
	commonmw "github.com/aegiscore/common/middleware"
	"github.com/gin-gonic/gin"
	"go.uber.org/fx"
	"go.uber.org/zap"
)

type GinParams struct {
	fx.In

	Config *config.Config
	Log    *zap.Logger
}

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
