package domain

import "github.com/google/uuid"

// Role 是角色管理 feature 的标准领域模型。
type Role struct {
	ID          int64
	RoleID      uuid.UUID
	Name        string
	Description string
	Active      bool
	IsSystem    bool
	CreatedAt   int64
	UpdatedAt   int64
}

// RoleMutation 是系统角色保护规则关注的目标角色状态。
type RoleMutation struct {
	Name   string
	Active bool
}

// ProtectSystemMutation 校验系统角色是否允许修改为目标状态。
func (r Role) ProtectSystemMutation(next RoleMutation) error {
	if !r.IsSystem {
		return nil
	}
	if r.Name != next.Name || r.Active != next.Active {
		return ErrSystemRoleProtected
	}
	return nil
}
