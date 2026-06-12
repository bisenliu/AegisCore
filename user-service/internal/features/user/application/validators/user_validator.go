package validators

import userdomain "github.com/aegiscore/user-service/internal/features/user/domain"

// CreateUserStatus 返回创建用户命令实际使用的用户状态。
func CreateUserStatus(status *userdomain.UserStatus) userdomain.UserStatus {
	if status == nil {
		return userdomain.UserStatusNormal
	}
	return *status
}
