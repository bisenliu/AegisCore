package validators

import userdomain "github.com/aegiscore/user-service/internal/features/user/domain"

// CreateUserStatus returns the effective user status for create-user commands.
func CreateUserStatus(status *userdomain.UserStatus) userdomain.UserStatus {
	if status == nil {
		return userdomain.UserStatusNormal
	}
	return *status
}
