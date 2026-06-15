package validators

import "github.com/aegiscore/user-service/internal/shared/identity"

// CreateUserStatus 返回创建用户命令实际使用的用户状态。
func CreateUserStatus(status *identity.UserStatus) identity.UserStatus {
	if status == nil {
		return identity.UserStatusNormal
	}
	return *status
}
