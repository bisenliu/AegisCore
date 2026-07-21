//go:build generate

package permissionhttp

//go:generate go tool mockgen -destination=mock_query_test.go -package=permissionhttp github.com/aegiscore/user-service/internal/features/permission/application/query PermissionQueryService
