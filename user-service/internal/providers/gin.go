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
	commonroute "github.com/aegiscore/common/http/route"
	"github.com/aegiscore/common/runtime/config"
	commonlogger "github.com/aegiscore/common/runtime/logger"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	commontracing "github.com/aegiscore/common/runtime/observability/tracing"
	"github.com/aegiscore/user-service/internal/router"
)

// GinModeConfigured 表示 Gin 包级进程运行模式已按配置显式设置。
type GinModeConfigured struct{}

// ConfigureGinMode 根据已解析 runtime config 设置 Gin 包级进程运行模式。
func ConfigureGinMode(cfg *config.Config) (GinModeConfigured, error) {
	if cfg == nil {
		return GinModeConfigured{}, fmt.Errorf("config is required")
	}
	gin.SetMode(cfg.Runtime.Gin.Mode)
	return GinModeConfigured{}, nil
}

// GinParams 包含创建 Gin engine 所需的 Fx 输入。
type GinParams struct {
	fx.In

	ModeConfigured GinModeConfigured
	Config         *config.Config
	Log            *zap.Logger
	Metrics        *commonmetrics.Provider
	Trace          *commontracing.Provider
}

// NewGinEngine 创建 Gin engine，配置可信代理并安装共享中间件。
// 中间件顺序保持 tracing -> span rename -> request id -> metrics -> recovery -> request log -> CORS，避免 panic 丢失指标和日志上下文。
func NewGinEngine(params GinParams) (*gin.Engine, error) {
	if params.Trace == nil {
		return nil, fmt.Errorf("tracing provider is required")
	}
	engine := gin.New()
	if err := engine.SetTrustedProxies(params.Config.Server.HTTP.TrustedProxies); err != nil {
		return nil, fmt.Errorf("configure trusted proxies: %w", err)
	}
	engine.Use(
		otelgin.Middleware(params.Config.App.Name,
			otelgin.WithTracerProvider(params.Trace.OTelTracerProvider()),
			otelgin.WithPropagators(params.Trace.TextMapPropagator()),
		),
		renameHTTPServerSpan(),
		commonmw.RequestID(),
		requestLoggerContext(params.Log),
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

func requestLoggerContext(log *zap.Logger) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Request = c.Request.WithContext(commonlogger.ToContext(c.Request.Context(), log))
		c.Next()
	}
}

func skipSuccessfulRuntimeEndpointLog(metricsCfg config.MetricsConfig) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		return c.Writer.Status() < 400 && router.IsLowNoiseRuntimeRoute(commonroute.TemplateOrUnmatched(c), metricsCfg)
	}
}

func skipMetricsScrapeRequest(metricsCfg config.MetricsConfig) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		return router.IsMetricsRoute(commonroute.TemplateOrUnmatched(c), metricsCfg)
	}
}

func skipSuccessfulRuntimeEndpointMetrics(metricsCfg config.MetricsConfig) func(*gin.Context) bool {
	return func(c *gin.Context) bool {
		return c.Writer.Status() < http.StatusBadRequest && router.IsLowNoiseRuntimeRoute(commonroute.TemplateOrUnmatched(c), metricsCfg)
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
	return c.Request.Method + " " + commonroute.TemplateOrUnmatched(c)
}
