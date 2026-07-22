package bootstrap

import (
	"context"

	"github.com/google/uuid"

	"github.com/aegiscore/user-service/internal/shared/identity"
)

// BootstrapSuperAdminUserID 是首次超级管理员引导用户的固定外部 ID。
const BootstrapSuperAdminUserID = "00000000-0000-0000-0000-000000000002"

// PasswordHasher 封装 bootstrap 创建临时密码哈希所需的最小能力。
type PasswordHasher interface {
	HashContext(ctx context.Context, plain string) (string, error)
}

// BootstrapStore 持久化一次性超级管理员引导结果。
type BootstrapStore interface {
	BootstrapSuperAdmin(ctx context.Context, input BootstrapSuperAdminInput) (*BootstrapSuperAdminResult, error)
}

// BootstrapSuperAdminInput 是持久化层创建 bootstrap 用户和角色绑定所需的完整输入。
type BootstrapSuperAdminInput struct {
	UserID       uuid.UUID
	RoleID       uuid.UUID
	Username     string
	Nickname     string
	PasswordHash string
	Status       identity.UserStatus
}

// BootstrapSuperAdminResult 返回 bootstrap 创建的稳定业务标识。
type BootstrapSuperAdminResult struct {
	UserID   uuid.UUID
	RoleID   uuid.UUID
	Username string
	Nickname string
}
