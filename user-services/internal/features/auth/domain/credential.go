package domain

import (
	userdomain "github.com/aegiscore/user-services/internal/features/user/domain"
	"github.com/google/uuid"
)

// UserCredential 是认证能力需要的最小用户凭据模型。
type UserCredential struct {
	UserID       uuid.UUID
	Username     string
	PasswordHash string
	Status       userdomain.UserStatus
	TokenVersion int64
}

// CanLogin 返回当前状态是否允许普通认证。
func (u UserCredential) CanLogin() bool {
	return u.Status.CanLogin()
}

// RequiresPasswordChange 返回用户是否必须完成强制改密流程。
func (u UserCredential) RequiresPasswordChange() bool {
	return u.Status == userdomain.UserStatusMustChangePassword
}

// CanChangePassword 返回用户是否可通过受限 token 流程修改密码。
func (u UserCredential) CanChangePassword() bool {
	return u.RequiresPasswordChange()
}

// CredentialUpdateResult 返回凭证替换后的用户和 token version。
type CredentialUpdateResult struct {
	UserID       uuid.UUID
	TokenVersion int64
}

// UpdateCredentialsInput 包含改密时使用的新凭证和目标状态。
type UpdateCredentialsInput struct {
	UserID       uuid.UUID
	PasswordHash string
	Status       userdomain.UserStatus
}
