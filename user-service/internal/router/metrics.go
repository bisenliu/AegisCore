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
	Pprof    config.PprofConfig
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
	if err := validateMetricsPath(metricsPath, params.Pprof); err != nil {
		return err
	}
	gatherer := params.Provider.Gatherer()
	if gatherer == nil {
		return nil
	}
	engine.GET(metricsPath, gin.WrapH(promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{})))
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

func validateMetricsPath(metricsPath string, pprofCfg config.PprofConfig) error {
	if metricsPath == "/" {
		return fmt.Errorf("%w: metrics path must not be root", ErrInvalidMetricsPath)
	}
	if strings.ContainsAny(metricsPath, ":*") {
		return fmt.Errorf("%w: metrics path must not contain route parameters or wildcards", ErrInvalidMetricsPath)
	}
	if IsHealthProbePath(metricsPath) {
		return fmt.Errorf("%w: metrics path conflicts with health probe path", ErrInvalidMetricsPath)
	}
	for _, reserved := range reservedMetricsPathPrefixes(pprofCfg) {
		if pathHasPrefix(metricsPath, reserved) {
			return fmt.Errorf("%w: metrics path conflicts with reserved route prefix %s", ErrInvalidMetricsPath, reserved)
		}
	}
	return nil
}

func reservedMetricsPathPrefixes(pprofCfg config.PprofConfig) []string {
	prefixes := []string{
		"/api/v1",
		openAPIJSONPath,
		"/openapi",
		"/docs",
		"/api-docs",
		"/debug/pprof",
	}
	if pprofCfg.Enabled {
		prefixes = append(prefixes, normalizePprofBasePath(pprofCfg.BasePath))
	}
	return prefixes
}

func normalizePprofBasePath(basePath string) string {
	trimmed := strings.TrimSpace(basePath)
	if trimmed == "" || trimmed == "/" {
		return "/debug/pprof"
	}
	return "/" + strings.Trim(trimmed, "/")
}

func pathHasPrefix(candidate string, prefix string) bool {
	if candidate == prefix {
		return true
	}
	return strings.HasPrefix(candidate, prefix+"/")
}
