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
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/user-service/internal/router"
)

const httpServerUnmatchedRouteSpanTarget = "route not found"

// GinParams 包含创建 Gin engine 所需的 Fx 输入。
type GinParams struct {
	fx.In

	Config  *config.Config
	Log     *zap.Logger
	Metrics *commonmetrics.Provider
	Trace   *commontracing.Provider
}

// NewGinEngine 创建 Gin engine，禁用可信代理并安装共享中间件。
// 中间件顺序保持 tracing -> span rename -> request id -> metrics -> recovery -> request log -> CORS，避免 panic 丢失指标和日志上下文。
func NewGinEngine(params GinParams) (*gin.Engine, error) {
	if params.Trace == nil || params.Trace.TracerProvider() == nil {
		return nil, fmt.Errorf("tracing provider is required")
	}
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	// 服务没有可信代理配置契约时不信任转发头，避免客户端伪造来源 IP。
	if err := engine.SetTrustedProxies(nil); err != nil {
		return nil, fmt.Errorf("disable trusted proxies: %w", err)
	}
	engine.Use(
		otelgin.Middleware(params.Config.App.Name,
			otelgin.WithTracerProvider(params.Trace.TracerProvider()),
			otelgin.WithPropagators(params.Trace.TextMapPropagator()),
			otelgin.WithFilter(traceBusinessRequest(params.Config.Observability.Metrics)),
		),
		renameHTTPServerSpan(),
		commonmw.RequestID(),
		commonmw.HTTPServerMetrics(commonmw.HTTPMetricsOptions{
			Provider:   params.Metrics,
			Skip:       skipMetricsScrapeRequest(params.Config.Observability.Metrics),
			SkipResult: skipSuccessfulRuntimeEndpointMetrics(params.Config.Observability.Metrics),
		}),
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

func skipMetricsScrapeRequest(metricsCfg config.MetricsConfig) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		return router.IsMetricsPath(c.Request.URL.Path, metricsCfg)
	}
}

func skipSuccessfulRuntimeEndpointMetrics(metricsCfg config.MetricsConfig) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		return c.Writer.Status() < http.StatusBadRequest && router.IsLowNoiseRuntimePath(c.Request.URL.Path, metricsCfg)
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
		// Gin 只有路由匹配后才能提供 FullPath 模板；请求结束后重命名 span 可避免按原始 URL 产生高基数 trace 名称。
		if span := trace.SpanFromContext(c.Request.Context()); span.SpanContext().IsValid() {
			span.SetName(httpServerSpanName(c))
		}
	}
}

func httpServerSpanName(c *gin.Context) string {
	if path := c.FullPath(); path != "" {
		return c.Request.Method + " " + path
	}
	return c.Request.Method + " " + httpServerUnmatchedRouteSpanTarget
}
