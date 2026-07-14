//go:build generate

package main

//go:generate go tool mockgen -destination=mock_rbac_test.go -package=main github.com/aegiscore/user-service/cmd rbacSeedService,rbacCredentialStore,rbacPasswordHasher
//go:generate go tool mockgen -destination=mock_user_command_test.go -package=main github.com/aegiscore/user-service/internal/features/user/application/command CreateUserService
