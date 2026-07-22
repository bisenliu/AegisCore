package bootstrap

import "errors"

var (
	// ErrSuperAdminAlreadyBootstrapped 表示固定 bootstrap 用户已存在，首次引导已经完成。
	ErrSuperAdminAlreadyBootstrapped = errors.New("super admin bootstrap has already been completed")
	// ErrBootstrapUsernameAlreadyExists 表示 bootstrap username 已被正常或软删除用户占用。
	ErrBootstrapUsernameAlreadyExists = errors.New("bootstrap username already exists")
	// ErrBootstrapInvalidInput 表示 bootstrap 命令输入不满足业务规则。
	ErrBootstrapInvalidInput = errors.New("bootstrap super admin input is invalid")
)
