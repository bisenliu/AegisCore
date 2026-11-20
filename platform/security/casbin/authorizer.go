package casbin

import (
	"context"
	"errors"
	"fmt"
	"reflect"
)

var (
	// ErrDenied 表示当前用户或角色没有访问该接口的权限。
	ErrDenied = errors.New("casbin access denied")
	// ErrNotConfigured 表示授权器或底层 enforcer 未正确配置。
	ErrNotConfigured = errors.New("casbin authorizer is not configured")
)

// Enforcer 定义底层权限规则判定引擎所需的最基础能力接口。
type Enforcer interface {
	Enforce(args ...interface{}) (bool, error)
}

// Request 表示权限判定所需的三要素：访问者（Subject，如用户或角色）、被访问的目标资源（Object，如 API 接口路径）和操作方式（Action，如 GET/POST）。
type Request struct {
	Subject string
	Object  string
	Action  string
}

// Authorizer 提供通用的权限检查器，负责对请求方是否有权访问目标接口进行裁决。
type Authorizer struct {
	enforcer Enforcer
}

// NewAuthorizer 创建通用 Casbin 授权器。
func NewAuthorizer(enforcer Enforcer) *Authorizer {
	return &Authorizer{enforcer: enforcer}
}

// Enforce 检查请求是否具备访问权限，返回是否允许的布尔值以及执行过程中是否出错。
func (a *Authorizer) Enforce(ctx context.Context, req Request) (bool, error) {
	if a == nil {
		return false, ErrNotConfigured
	}
	return Enforce(ctx, a.enforcer, req)
}

// Authorize 校验用户是否有权访问目标资源；若没有权限则返回 ErrDenied 拒绝访问。
func (a *Authorizer) Authorize(ctx context.Context, req Request) error {
	if a == nil {
		return ErrNotConfigured
	}
	allowed, err := Enforce(ctx, a.enforcer, req)
	if err != nil {
		return err
	}
	if !allowed {
		return ErrDenied
	}
	return nil
}

// Enforce 使用权限判定引擎核对访问者、目标资源和操作方式三要素，确认是否允许当前访问。
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

// isNilEnforcer 检查权限检查器是否未正确配置；若未配置则拒绝放行请求，防止在无权限防护的状态下暴露接口。
func isNilEnforcer(enforcer Enforcer) bool {
	if enforcer == nil {
		return true
	}
	// 接口中可能装入 typed nil 指针，必须用反射识别，否则会绕过未配置检查并在 Enforce 时 panic。
	value := reflect.ValueOf(enforcer)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
