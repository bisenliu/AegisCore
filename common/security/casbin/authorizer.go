package casbin

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrDenied 表示 Casbin 策略拒绝本次访问。
	ErrDenied = errors.New("casbin access denied")
	// ErrNotConfigured 表示授权器或底层 enforcer 未正确配置。
	ErrNotConfigured = errors.New("casbin authorizer is not configured")
)

// Enforcer 是 Casbin enforcer 的最小能力接口。
type Enforcer interface {
	Enforce(args ...interface{}) (bool, error)
}

// Request 表示一次 Casbin 授权校验请求。
type Request struct {
	Subject string
	Object  string
	Action  string
}

// Authorizer 封装无业务语义的 Casbin 授权校验。
type Authorizer struct {
	enforcer Enforcer
}

// NewAuthorizer 创建通用 Casbin 授权器。
func NewAuthorizer(enforcer Enforcer) *Authorizer {
	return &Authorizer{enforcer: enforcer}
}

// Enforce 返回 Casbin 对本次授权请求的原始允许结果。
func (a *Authorizer) Enforce(ctx context.Context, req Request) (bool, error) {
	if a == nil {
		return false, ErrNotConfigured
	}
	return Enforce(ctx, a.enforcer, req)
}

// Authorize 校验请求，策略拒绝时返回 ErrDenied。
func (a *Authorizer) Authorize(ctx context.Context, req Request) error {
	if a == nil {
		return ErrNotConfigured
	}
	return Authorize(ctx, a.enforcer, req)
}

// Enforce 使用传入的 enforcer 执行 Casbin 三元组校验。
func Enforce(ctx context.Context, enforcer Enforcer, req Request) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if isNilEnforcer(enforcer) {
		return false, ErrNotConfigured
	}
	allowed, err := enforcer.Enforce(req.Subject, req.Object, req.Action)
	if err != nil {
		return false, fmt.Errorf("casbin enforce: %w", err)
	}
	return allowed, nil
}

// Authorize 使用传入的 enforcer 执行 Casbin 三元组校验，拒绝时返回 ErrDenied。
func Authorize(ctx context.Context, enforcer Enforcer, req Request) error {
	allowed, err := Enforce(ctx, enforcer, req)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}

func isNilEnforcer(enforcer Enforcer) bool {
	if enforcer == nil {
		return true
	}
	value := reflect.ValueOf(enforcer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
