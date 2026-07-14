//go:build generate

package casbin

//go:generate go tool mockgen -destination=mock_test.go -package=casbin github.com/aegiscore/user-service/internal/features/permission/infrastructure/casbin Loader,UserRoleResolver
//go:generate go tool mockgen -destination=mock_metrics_test.go -package=casbin github.com/aegiscore/common/runtime/observability/metrics ReloadMetrics
