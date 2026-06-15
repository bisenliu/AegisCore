package domain

import "github.com/google/uuid"

// Permission 是权限目录 feature 的标准领域模型。
type Permission struct {
	ID           int64
	PermissionID uuid.UUID
	Name         string
	Description  string
	Module       string
	HTTPMethod   string
	PathTemplate string
	Active       bool
	IsSystem     bool
	CreatedAt    int64
	UpdatedAt    int64
}

// Identity 返回权限的 HTTP 路由身份。
func (p Permission) Identity() RouteIdentity {
	return RouteIdentity{Method: p.HTTPMethod, PathTemplate: p.PathTemplate}
}

// ProtectSystemIdentity 校验系统权限是否发生受保护身份字段变更。
func (p Permission) ProtectSystemIdentity(next RouteIdentity) error {
	if !p.IsSystem {
		return nil
	}
	if p.HTTPMethod != next.Method || p.PathTemplate != next.PathTemplate {
		return ErrSystemPermissionProtected
	}
	return nil
}
