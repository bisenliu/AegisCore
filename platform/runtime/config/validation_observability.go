package config

import (
	"net"
	"strings"
)

// validateObservability 校验 metrics、tracing 和 pprof 的业务中立观测配置。
func (c Config) validateObservability() []error {
	var errs []error
	metricsPath := strings.TrimSpace(c.Observability.Metrics.Path)
	if metricsPath == "" {
		errs = append(errs, FieldError("observability.metrics.path", "is required"))
	} else if !strings.HasPrefix(metricsPath, "/") {
		errs = append(errs, FieldError("observability.metrics.path", "must start with /"))
	}
	if c.Observability.Tracing.SampleRatio < 0 || c.Observability.Tracing.SampleRatio > 1 {
		errs = append(errs, FieldError("observability.tracing.sample_ratio", "must be between 0 and 1"))
	}
	if c.Observability.Tracing.Enabled {
		if strings.TrimSpace(c.Observability.Tracing.OTLPEndpoint) == "" {
			errs = append(errs, FieldError("observability.tracing.otlp_endpoint", "is required when tracing is enabled"))
		}
		if c.isProductionLike() && c.Observability.Tracing.Insecure {
			errs = append(errs, FieldError("observability.tracing.insecure", "must not be true when tracing is enabled in production-like environments"))
		}
	}
	errs = append(errs, c.validatePprof()...)
	return errs
}

// validatePprof 校验独立 pprof listener 地址，并在生产类环境强制使用 loopback。
func (c Config) validatePprof() []error {
	var errs []error
	host, portText, err := net.SplitHostPort(c.Observability.Pprof.Addr)
	if err != nil {
		return []error{FieldError("observability.pprof.addr", "must be a host:port address")}
	}
	if strings.TrimSpace(host) == "" {
		errs = append(errs, FieldError("observability.pprof.addr", "host is required"))
	}
	if portErrs := validatePortText("observability.pprof.addr", portText); len(portErrs) > 0 {
		errs = append(errs, portErrs...)
	}
	if c.Observability.Pprof.Enabled && c.isProductionLike() && !isLoopbackHost(host) {
		errs = append(errs, FieldError("observability.pprof.addr", "must use a loopback address in production-like environments"))
	}
	return errs
}

// isProductionLike 判断当前 app environment 是否需要生产级安全约束。
func (c Config) isProductionLike() bool {
	switch strings.ToLower(strings.TrimSpace(c.App.Environment)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

// isLoopbackHost 判断 host 是否解析为 loopback 地址，localhost 作为稳定别名接受。
func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
