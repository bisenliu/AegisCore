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
