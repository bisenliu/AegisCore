package router

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
)

var (
	// ErrInvalidMetricsPath 表示 metrics endpoint 配置路径不能安全挂载。
	ErrInvalidMetricsPath = errors.New("invalid metrics path")
)

// MetricsRouteParams 包含挂载 Prometheus metrics endpoint 所需的服务级依赖。
type MetricsRouteParams struct {
	Config   config.MetricsConfig
	Provider *commonmetrics.Provider
}

func registerMetricsRoute(engine *gin.Engine, params MetricsRouteParams) error {
	if !params.Config.Enabled || params.Provider == nil || !params.Provider.Enabled() {
		return nil
	}
	metricsPath, err := normalizeMetricsPath(params.Config.Path)
	if err != nil {
		return err
	}
	if err := validateMetricsPath(metricsPath); err != nil {
		return err
	}
	handler := params.Provider.HTTPHandler(promhttp.HandlerOpts{})
	if handler == nil {
		return nil
	}
	engine.GET(metricsPath, gin.WrapH(handler))
	return nil
}

// IsMetricsPath 判断 path 是否为当前配置启用的 Prometheus metrics endpoint。
func IsMetricsPath(requestPath string, cfg config.MetricsConfig) bool {
	if !cfg.Enabled {
		return false
	}
	metricsPath, err := normalizeMetricsPath(cfg.Path)
	if err != nil {
		return false
	}
	return requestPath == metricsPath
}

// IsLowNoiseRuntimePath 判断 path 是否属于可跳过成功日志和 tracing 的运行时端点。
func IsLowNoiseRuntimePath(requestPath string, metricsCfg config.MetricsConfig) bool {
	return IsHealthProbePath(requestPath) || IsMetricsPath(requestPath, metricsCfg)
}

func normalizeMetricsPath(rawPath string) (string, error) {
	// metrics endpoint 必须是规范化的绝对静态路径，避免与 Gin 参数、通配符或保留运行时路由产生歧义。
	metricsPath := strings.TrimSpace(rawPath)
	if metricsPath == "" {
		return "", fmt.Errorf("%w: metrics path is required", ErrInvalidMetricsPath)
	}
	if !strings.HasPrefix(metricsPath, "/") {
		return "", fmt.Errorf("%w: metrics path must start with /", ErrInvalidMetricsPath)
	}
	if metricsPath != "/" {
		metricsPath = "/" + strings.Trim(metricsPath, "/")
	}
	cleanPath := path.Clean(metricsPath)
	if cleanPath != metricsPath {
		return "", fmt.Errorf("%w: metrics path must be normalized", ErrInvalidMetricsPath)
	}
	return metricsPath, nil
}

func validateMetricsPath(metricsPath string) error {
	// 启动期阻断与健康检查、OpenAPI 和业务 API 的冲突，避免后注册路由覆盖已知端点。
	if metricsPath == "/" {
		return fmt.Errorf("%w: metrics path must not be root", ErrInvalidMetricsPath)
	}
	if strings.ContainsAny(metricsPath, ":*") {
		return fmt.Errorf("%w: metrics path must not contain route parameters or wildcards", ErrInvalidMetricsPath)
	}
	if IsHealthProbePath(metricsPath) {
		return fmt.Errorf("%w: metrics path conflicts with health probe path", ErrInvalidMetricsPath)
	}
	for _, reserved := range reservedMetricsExactPaths() {
		if metricsPath == reserved {
			return fmt.Errorf("%w: metrics path conflicts with reserved route %s", ErrInvalidMetricsPath, reserved)
		}
	}
	for _, reserved := range reservedMetricsPathPrefixes() {
		if pathHasPrefix(metricsPath, reserved) {
			return fmt.Errorf("%w: metrics path conflicts with reserved route prefix %s", ErrInvalidMetricsPath, reserved)
		}
	}
	return nil
}

func reservedMetricsExactPaths() []string {
	return []string{
		openAPIJSONPath,
		"/docs",
		"/api-docs",
	}
}

func reservedMetricsPathPrefixes() []string {
	return []string{
		"/api/v1",
		"/openapi",
	}
}

func pathHasPrefix(candidate string, prefix string) bool {
	if candidate == prefix {
		return true
	}
	return strings.HasPrefix(candidate, prefix+"/")
}
