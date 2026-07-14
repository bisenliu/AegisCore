//go:build generate

package seed

//go:generate go tool mockgen -destination=mock_test.go -package=seed github.com/aegiscore/user-service/internal/features/role/application SeedRoleStore,SeedUserRoleStore,SeedRolePermissionStore
//go:generate go tool mockgen -destination=mock_permission_test.go -package=seed github.com/aegiscore/user-service/internal/features/permission/application SeedPermissionStore
