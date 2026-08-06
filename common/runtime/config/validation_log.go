package config

import "strings"

// validateLog 校验共享日志配置的低基数字段，避免运行时初始化时再发现非法 level 或 format。
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

// isValidLogLevel 判断日志级别是否属于共享 logger 支持的稳定枚举。
func isValidLogLevel(value string) bool {
	switch value {
	case "debug", "info", "warn", "error":
		return true
	default:
		return false
	}
}

// isValidLogFormat 判断日志输出格式是否属于共享 logger 支持的稳定枚举。
func isValidLogFormat(value string) bool {
	switch value {
	case "json", "console":
		return true
	default:
		return false
	}
}
