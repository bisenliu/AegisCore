package command

//go:generate go tool mockgen -destination=mock_test.go -package=command github.com/aegiscore/user-service/internal/features/role/application RoleStore,UserRoleStore,RolePermissionStore,PermissionLookup
//go:generate go tool mockgen -destination=mock_permission_test.go -package=command github.com/aegiscore/user-service/internal/features/permission/application PolicyChangeNotifier
