package config

import (
	"errors"
	"strings"
	"time"
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

// Error 将聚合校验错误渲染为稳定、可读的启动失败文本。
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

// Unwrap 返回全部字段错误，使调用方可以通过 errors.Is/As 检查底层错误。
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

// validateApp 校验所有服务都必须声明的应用身份字段。
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
