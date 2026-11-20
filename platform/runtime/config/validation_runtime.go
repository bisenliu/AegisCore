package config

import (
	"fmt"
	"strings"
	"time"
)

// validateRuntime 校验 runtime lifecycle、Gin mode 和进程时区配置。
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

// minimumLifecycleStopBudget 计算 Fx app 停止时必须覆盖的最小组合预算。
// 预算包含最长协议 server shutdown、worker drain、tracing flush 和安全余量。
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

// isValidGinMode 判断 Gin mode 是否属于 Gin 支持且配置契约允许的枚举。
func isValidGinMode(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "debug", "release", "test":
		return true
	default:
		return false
	}
}
