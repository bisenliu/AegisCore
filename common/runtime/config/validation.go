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

func newValidationError(errs []error) *ValidationError {
	if len(errs) == 0 {
		return nil
	}
	return &ValidationError{errs: errs}
}

// NewValidationError 聚合调用方扩展配置校验错误。
func NewValidationError(errs []error) *ValidationError {
	return newValidationError(errs)
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
	return newValidationError(errs)
}

func (c Config) validateApp() []error {
	var errs []error
	if strings.TrimSpace(c.App.Name) == "" {
		errs = append(errs, configFieldError("app.name", "is required"))
	}
	if strings.TrimSpace(c.App.Environment) == "" {
		errs = append(errs, configFieldError("app.environment", "is required"))
	}
	return errs
}

func (c Config) validateRuntime() []error {
	var errs []error
	errs = append(errs, validatePositiveDuration("runtime.lifecycle.start_timeout", c.Runtime.Lifecycle.StartTimeout)...)
	errs = append(errs, validatePositiveDuration("runtime.lifecycle.stop_timeout", c.Runtime.Lifecycle.StopTimeout)...)
	if !isValidGinMode(c.Runtime.Gin.Mode) {
		errs = append(errs, configFieldError("runtime.gin.mode", "must be one of debug, release, test"))
	}
	if strings.TrimSpace(c.Runtime.Timezone) == "" {
		errs = append(errs, configFieldError("runtime.timezone", "is required"))
	} else if _, err := time.LoadLocation(c.Runtime.Timezone); err != nil {
		errs = append(errs, configFieldError("runtime.timezone", "must be a valid IANA timezone"))
	}
	if c.Runtime.Lifecycle.StopTimeout > 0 {
		// lifecycle stop 是 Fx app 总预算，不能短于任一协议 server 的组件级关闭预算。
		if c.Server.HTTP.ShutdownTimeout > 0 && c.Runtime.Lifecycle.StopTimeout < c.Server.HTTP.ShutdownTimeout {
			errs = append(errs, configFieldError("runtime.lifecycle.stop_timeout", "must be >= server.http.shutdown_timeout"))
		}
		if c.Server.GRPC.ShutdownTimeout > 0 && c.Runtime.Lifecycle.StopTimeout < c.Server.GRPC.ShutdownTimeout {
			errs = append(errs, configFieldError("runtime.lifecycle.stop_timeout", "must be >= server.grpc.shutdown_timeout"))
		}
		if minimumStopBudget := c.minimumLifecycleStopBudget(); minimumStopBudget > 0 && c.Runtime.Lifecycle.StopTimeout < minimumStopBudget {
			errs = append(errs, configFieldError("runtime.lifecycle.stop_timeout", fmt.Sprintf("must be at least %s to cover shutdown budget", minimumStopBudget)))
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
		errs = append(errs, configFieldError("server", "must enable at least one of server.http or server.grpc"))
	}
	if c.Server.HTTP.Enabled {
		errs = append(errs, validateServerAddress("server.http", c.Server.HTTP.Host, c.Server.HTTP.Port)...)
		errs = append(errs, validatePositiveDuration("server.http.read_timeout", c.Server.HTTP.ReadTimeout)...)
		errs = append(errs, validatePositiveDuration("server.http.write_timeout", c.Server.HTTP.WriteTimeout)...)
		errs = append(errs, validatePositiveDuration("server.http.idle_timeout", c.Server.HTTP.IdleTimeout)...)
		errs = append(errs, validatePositiveDuration("server.http.shutdown_timeout", c.Server.HTTP.ShutdownTimeout)...)
	}
	if c.Server.GRPC.Enabled {
		errs = append(errs, validateServerAddress("server.grpc", c.Server.GRPC.Host, c.Server.GRPC.Port)...)
		errs = append(errs, validatePositiveDuration("server.grpc.shutdown_timeout", c.Server.GRPC.ShutdownTimeout)...)
	}
	return errs
}

func validateServerAddress(base string, host string, port int) []error {
	var errs []error
	if strings.TrimSpace(host) == "" {
		errs = append(errs, configFieldError(base+".host", "is required"))
	}
	return append(errs, validatePort(base+".port", port)...)
}

func (c Config) validateLog() []error {
	var errs []error
	level := strings.ToLower(strings.TrimSpace(c.Log.Level))
	if level == "" {
		errs = append(errs, configFieldError("log.level", "is required"))
	} else if !isValidLogLevel(level) {
		errs = append(errs, configFieldError("log.level", "must be one of debug, info, warn, error"))
	}
	format := strings.ToLower(strings.TrimSpace(c.Log.Format))
	if format == "" {
		errs = append(errs, configFieldError("log.format", "is required"))
	} else if !isValidLogFormat(format) {
		errs = append(errs, configFieldError("log.format", "must be one of json, console"))
	}
	return errs
}

func (c Config) validateObservability() []error {
	var errs []error
	metricsPath := strings.TrimSpace(c.Observability.Metrics.Path)
	if metricsPath == "" {
		errs = append(errs, configFieldError("observability.metrics.path", "is required"))
	} else if !strings.HasPrefix(metricsPath, "/") {
		errs = append(errs, configFieldError("observability.metrics.path", "must start with /"))
	}
	if c.Observability.Tracing.SampleRatio < 0 || c.Observability.Tracing.SampleRatio > 1 {
		errs = append(errs, configFieldError("observability.tracing.sample_ratio", "must be between 0 and 1"))
	}
	if c.Observability.Tracing.Enabled {
		if strings.TrimSpace(c.Observability.Tracing.OTLPEndpoint) == "" {
			errs = append(errs, configFieldError("observability.tracing.otlp_endpoint", "is required when tracing is enabled"))
		}
		if c.isProductionLike() && c.Observability.Tracing.Insecure {
			errs = append(errs, configFieldError("observability.tracing.insecure", "must not be true when tracing is enabled in production-like environments"))
		}
	}
	errs = append(errs, c.validatePprof()...)
	return errs
}

func (c Config) validatePprof() []error {
	var errs []error
	host, portText, err := net.SplitHostPort(c.Observability.Pprof.Addr)
	if err != nil {
		return []error{configFieldError("observability.pprof.addr", "must be a host:port address")}
	}
	if strings.TrimSpace(host) == "" {
		errs = append(errs, configFieldError("observability.pprof.addr", "host is required"))
	}
	if portErrs := validatePortText("observability.pprof.addr", portText); len(portErrs) > 0 {
		errs = append(errs, portErrs...)
	}
	if c.Observability.Pprof.Enabled && c.isProductionLike() && !isLoopbackHost(host) {
		errs = append(errs, configFieldError("observability.pprof.addr", "must use a loopback address in production-like environments"))
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
		return []error{configFieldError(path, fmt.Sprintf("must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

func validatePortText(path string, value string) []error {
	port, err := strconv.Atoi(value)
	if err != nil || port < minPort || port > maxPort {
		return []error{configFieldError(path, fmt.Sprintf("port must be between %d and %d", minPort, maxPort))}
	}
	return nil
}

func validatePositiveDuration(path string, value time.Duration) []error {
	if value <= 0 {
		return []error{configFieldError(path, "must be > 0")}
	}
	return nil
}

// ValidatePositiveDuration 校验 duration 必须为正数。
func ValidatePositiveDuration(path string, value time.Duration) []error {
	return validatePositiveDuration(path, value)
}

func validateNonNegativeInt(path string, value int) []error {
	if value < 0 {
		return []error{configFieldError(path, "must be >= 0")}
	}
	return nil
}

// ValidateNonNegativeInt 校验 int 必须为非负数。
func ValidateNonNegativeInt(path string, value int) []error {
	return validateNonNegativeInt(path, value)
}

func configFieldError(path string, message string) error {
	return errors.New(path + " " + message)
}

// FieldError 创建与共享配置校验一致的字段错误。
func FieldError(path string, message string) error {
	return configFieldError(path, message)
}
