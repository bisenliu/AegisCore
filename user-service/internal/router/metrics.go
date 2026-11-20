package router

import (
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/aegiscore/platform/runtime/config"
	platformmetrics "github.com/aegiscore/platform/runtime/observability/metrics"
)

var (
	// ErrInvalidMetricsPath 表示 metrics endpoint 配置路径不能安全挂载。
	ErrInvalidMetricsPath = errors.New("invalid metrics path")
)

// MetricsRouteParams 包含挂载 Prometheus metrics endpoint 所需的服务级依赖。
type MetricsRouteParams struct {
	Config   config.MetricsConfig
	Provider *platformmetrics.Provider
}

// registerMetricsRoute 开放用于监控系统采集数据的抓取接口。
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

// IsMetricsRoute 判断 route template 是否为当前配置启用的 Prometheus metrics endpoint。
func IsMetricsRoute(route string, cfg config.MetricsConfig) bool {
	if !cfg.Enabled {
		return false
	}
	metricsPath, err := normalizeMetricsPath(cfg.Path)
	if err != nil {
		return false
	}
	return route == metricsPath
}

// IsLowNoiseRuntimeRoute 判断 route template 是否属于可跳过成功日志和指标的运行时端点。
func IsLowNoiseRuntimeRoute(route string, metricsCfg config.MetricsConfig) bool {
	return IsHealthProbePath(route) || IsMetricsRoute(route, metricsCfg)
}

// normalizeMetricsPath 规范化监控数据抓取接口的访问路径，确保以斜杠开头。
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

// validateMetricsPath 检查监控接口路径是否合法，严禁占用健康检查或核心业务的接口地址。
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

// reservedMetricsExactPaths 列出系统保留的核心接口路径（如健康检查接口），防止被监控配置意外覆盖。
func reservedMetricsExactPaths() []string {
	return []string{
		openAPIJSONPath,
		"/docs",
		"/api-docs",
	}
}

// reservedMetricsPathPrefixes 列出系统保留的路径前缀（如 /api/ 业务接口），防止监控路径与业务路径产生冲突。
func reservedMetricsPathPrefixes() []string {
	return []string{
		"/api/v1",
		"/openapi",
	}
}

// pathHasPrefix 检查访问路径是否以某个指定的前缀开头。
func pathHasPrefix(candidate string, prefix string) bool {
	if candidate == prefix {
		return true
	}
	return strings.HasPrefix(candidate, prefix+"/")
}
