//go:build generate

package redis

//go:generate go tool mockgen -destination=mock_policy_watcher_test.go -package=redis github.com/aegiscore/user-service/internal/features/permission/application PolicyReloadEngine,Metrics
