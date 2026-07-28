package config

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"
)

const (
	minPort = 1
	maxPort = 65535
)

// ValidationError 聚合配置校验失败，使启动阶段能一次性报告全部非法字段。
type ValidationError struct {
	errs []error
}

// NewValidationError 聚合调用方扩展配置校验错误。
func NewValidationError(errs []error) *ValidationError {
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{errs: errs}
}

func (e *ValidationError) Error() string {
	if e == nil || len(e.errs) == 0 {
		return "config validation failed"
	}
	parts := make([]string, 0, len(e.errs))
	for _, err := range e.errs {
		parts = append(parts, err.Error())
	}
	return "config validation failed: " + strings.Join(parts, "; ")
}

func (e *ValidationError) Unwrap() []error {
	if e == nil {
		return nil
	}
	return e.errs
}

// Validate 在服务启动前拒绝结构非法的运行时配置。
// 它只检查服务无关配置；服务特定的必需命名资源和业务策略属于服务模块。
func (c Config) Validate() error {
	var errs []error

	errs = append(errs, c.validateApp()...)
	errs = append(errs, c.validateRuntime()...)
	errs = append(errs, c.validateServer()...)
	errs = append(errs, c.validateLog()...)
	errs = append(errs, c.validateObservability()...)

	if len(errs) == 0 {
		return nil
	}
	return NewValidationError(errs)
}

func (c Config) validateApp() []error {
	var errs []error
	if strings.TrimSpace(c.App.Name) == "" {
		errs = append(errs, FieldError("app.name", "is required"))
	}
	if strings.TrimSpace(c.App.Environment) == "" {
		errs = append(errs, FieldError("app.environment", "is required"))
	}
	return errs
}

func (c Config) validateRuntime() []error {
	var errs []error
	errs = append(errs, ValidatePositiveDuration("runtime.lifecycle.start_timeout", c.Runtime.Lifecycle.StartTimeout)...)
	errs = append(errs, ValidatePositiveDuration("runtime.lifecycle.stop_timeout", c.Runtime.Lifecycle.StopTimeout)...)
	if !isValidGinMode(c.Runtime.Gin.Mode) {
		errs = append(errs, FieldError("runtime.gin.mode", "must be one of debug, release, test"))
	}
	if strings.TrimSpace(c.Runtime.Timezone) == "" {
		errs = append(errs, FieldError("runtime.timezone", "is required"))
	} else if _, err := time.LoadLocation(c.Runtime.Timezone); err != nil {
		errs = append(errs, FieldError("runtime.timezone", "must be a valid IANA timezone"))
	}
	if c.Runtime.Lifecycle.StopTimeout > 0 {
		// lifecycle stop 是 Fx app 总预算，不能短于任一协议 server 的组件级关闭预算。
		if c.Server.HTTP.ShutdownTimeout > 0 && c.Runtime.Lifecycle.StopTimeout < c.Server.HTTP.ShutdownTimeout {
			errs = append(errs, FieldError("runtime.lifecycle.stop_timeout", "must be >= server.http.shutdown_timeout"))
		}
		if c.Server.GRPC.ShutdownTimeout > 0 && c.Runtime.Lifecycle.StopTimeout < c.Server.GRPC.ShutdownTimeout {
			errs = append(errs, FieldError("runtime.lifecycle.stop_timeout", "must be >= server.grpc.shutdown_timeout"))
		}
		if minimumStopBudget := c.minimumLifecycleStopBudget(); minimumStopBudget > 0 && c.Runtime.Lifecycle.StopTimeout < minimumStopBudget {
			errs = append(errs, FieldError("runtime.lifecycle.stop_timeout", fmt.Sprintf("must be at least %s to cover shutdown budget", minimumStopBudget)))
		}
	}
	return errs
}

func (c Config) minimumLifecycleStopBudget() time.Duration {
	protocolShutdown := c.Server.HTTP.ShutdownTimeout
	if c.Server.GRPC.ShutdownTimeout > protocolShutdown {
		protocolShutdown = c.Server.GRPC.ShutdownTimeout
	}
	if protocolShutdown <= 0 {
		return 0
	}
	return protocolShutdown +
		DefaultLifecycleWorkerDrainAllowance +
		DefaultLifecycleTracingFlushAllowance +
		DefaultLifecycleShutdownSafetyMargin
}

func (c Config) validateServer() []error {
	var errs []error
	if !c.Server.HTTP.Enabled && !c.Server.GRPC.Enabled {
		errs = append(errs, FieldError("server", "must enable at least one of server.http or server.grpc"))
	}
	if c.Server.HTTP.Enabled {
		errs = append(errs, validateServerAddress("server.http", c.Server.HTTP.Host, c.Server.HTTP.Port)...)
		errs = append(errs, ValidatePositiveDuration("server.http.read_timeout", c.Server.HTTP.ReadTimeout)...)
		errs = append(errs, ValidatePositiveDuration("server.http.write_timeout", c.Server.HTTP.WriteTimeout)...)
		errs = append(errs, ValidatePositiveDuration("server.http.idle_timeout", c.Server.HTTP.IdleTimeout)...)
		errs = append(errs, ValidatePositiveDuration("server.http.shutdown_timeout", c.Server.HTTP.ShutdownTimeout)...)
		errs = append(errs, validateTrustedProxies(c.Server.HTTP.TrustedProxies)...)
	}
	if c.Server.GRPC.Enabled {
		errs = append(errs, validateServerAddress("server.grpc", c.Server.GRPC.Host, c.Server.GRPC.Port)...)
		errs = append(errs, ValidatePositiveDuration("server.grpc.shutdown_timeout", c.Server.GRPC.ShutdownTimeout)...)
	}
	return errs
}

func validateServerAddress(base string, host string, port int) []error {
	var errs []error
	if strings.TrimSpace(host) == "" {
		errs = append(errs, FieldError(base+".host", "is required"))
	}
	return append(errs, validatePort(base+".port", port)...)
}

func validateTrustedProxies(values []string) []error {
	var errs []error
	for index, value := range values {
		path := fmt.Sprintf("server.http.trusted_proxies[%d]", index)
		proxy := strings.TrimSpace(value)
		if proxy == "" {
			errs = append(errs, FieldError(path, "must be an IP address or CIDR"))
			continue
		}
		if net.ParseIP(proxy) != nil {
			continue
		}
		if _, _, err := net.ParseCIDR(proxy); err == nil {
			continue
		}
		errs = append(errs, FieldError(path, "must be an IP address or CIDR"))
	}
	return errs
}

func (c Config) validateLog() []error {
	var errs []error
	level := strings.ToLower(strings.TrimSpace(c.Log.Level))
	if level == "" {
		errs = append(errs, FieldError("log.level", "is required"))
	} else if !isValidLogLevel(level) {
		errs = append(errs, FieldError("log.level", "must be one of debug, info, warn, error"))
	}
	format := strings.ToLower(strings.TrimSpace(c.Log.Format))
	if format == "" {
		errs = append(errs, FieldError("log.format", "is required"))
	} else if !isValidLogFormat(format) {
		errs = append(errs, FieldError("log.format", "must be one of json, console"))
	}
	return errs
}

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

func (c Config) isProductionLike() bool {
	switch strings.ToLower(strings.TrimSpace(c.App.Environment)) {
	case "prod", "production", "staging":
		return true
	default:
		return false
	}
}

func isValidLogLevel(value string) bool {
	switch value {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

func isValidLogFormat(value string) bool {
	switch value {
	case "json", "console":
		return true
	default:
		return false
	}
}

func isValidGinMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "release", "test":
		return true
	default:
		return false
	}
}

func isLoopbackHost(host string) bool {
	host = strings.TrimSpace(host)
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func validatePort(path string, value int) []error {
	if value < minPort || value > maxPort {
		return []error{FieldError(path, fmt.Sprintf("must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

func validatePortText(path string, value string) []error {
	port, err := strconv.Atoi(value)
	if err != nil || port < minPort || port > maxPort {
		return []error{FieldError(path, fmt.Sprintf("port must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

// ValidatePositiveDuration 校验 duration 必须为正数。
func ValidatePositiveDuration(path string, value time.Duration) []error {
	if value <= 0 {
		return []error{FieldError(path, "must be > 0")}
	}
	return nil
}

// ValidateNonNegativeInt 校验 int 必须为非负数。
func ValidateNonNegativeInt(path string, value int) []error {
	if value < 0 {
		return []error{FieldError(path, "must be >= 0")}
	}
	return nil
}

// FieldError 创建与共享配置校验一致的字段错误。
func FieldError(path string, message string) error {
	return errors.New(path + " " + message)
}
