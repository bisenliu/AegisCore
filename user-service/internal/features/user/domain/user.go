package domain

import "github.com/google/uuid"

// User 是 application 端口与 infrastructure adapter 之间传递的标准领域用户模型。
type User struct {
	ID           int64
	UserID       uuid.UUID
	Nickname     string
	Username     string
	PasswordHash string
	Status       UserStatus
	TokenVersion int64
	CreatedAt    int64
	UpdatedAt    int64
}

// CanLogin 返回当前状态是否允许普通认证。
func (u User) CanLogin() bool {
	return u.Status.CanLogin()
}

// RequiresPasswordChange 返回用户是否必须完成强制改密流程。
func (u User) RequiresPasswordChange() bool {
	return u.Status == UserStatusMustChangePassword
}

// CanChangePassword 返回用户是否可通过受限 token 流程修改密码。
func (u User) CanChangePassword() bool {
	return u.RequiresPasswordChange()
}
