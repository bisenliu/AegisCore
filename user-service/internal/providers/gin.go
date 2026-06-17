package providers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel/trace"
	"go.uber.org/fx"
	"go.uber.org/zap"

	commonmw "github.com/aegiscore/common/http/middleware"
	"github.com/aegiscore/common/runtime/config"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/user-service/internal/router"
)

// GinParams 包含创建 Gin engine 所需的 Fx 输入。
type GinParams struct {
	fx.In

	Config *config.Config
	Log    *zap.Logger
	Trace  *commontracing.Provider
}

// NewGinEngine 创建 Gin engine，应用可信代理配置并安装共享中间件。
func NewGinEngine(params GinParams) (*gin.Engine, error) {
	if params.Trace == nil || params.Trace.TracerProvider() == nil {
		return nil, fmt.Errorf("tracing provider is required")
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	if len(params.Config.HTTP.TrustedProxies) > 0 {
		if err := engine.SetTrustedProxies(params.Config.HTTP.TrustedProxies); err != nil {
			return nil, fmt.Errorf("set trusted proxies: %w", err)
		}
	}
	engine.Use(
		otelgin.Middleware(params.Config.App.Name,
			otelgin.WithTracerProvider(params.Trace.TracerProvider()),
			otelgin.WithPropagators(params.Trace.TextMapPropagator()),
			otelgin.WithFilter(traceBusinessRequest(params.Config.Observability.Metrics)),
		),
		renameHTTPServerSpan(),
		commonmw.Recovery(params.Log),
		commonmw.RequestLoggerWithOptions(params.Log, commonmw.RequestLoggerOptions{Skip: skipSuccessfulRuntimeEndpointLog(params.Config.Observability.Metrics)}),
		commonmw.CORS(),
	)
	return engine, nil
}

func skipSuccessfulRuntimeEndpointLog(metricsCfg config.MetricsConfig) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		return c.Writer.Status() < 400 && router.IsLowNoiseRuntimePath(c.Request.URL.Path, metricsCfg)
	}
}

func traceBusinessRequest(metricsCfg config.MetricsConfig) func(*http.Request) bool {
	return func(request *http.Request) bool {
		return !router.IsLowNoiseRuntimePath(request.URL.Path, metricsCfg)
	}
}

func renameHTTPServerSpan() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().IsValid() {
			span.SetName(httpServerSpanName(c))
		}
	}
}

func httpServerSpanName(c *gin.Context) string {
	path := c.FullPath()
	if path == "" {
		path = c.Request.URL.Path
	}
	return c.Request.Method + " " + path
}
