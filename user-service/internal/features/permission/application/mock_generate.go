package application

//go:generate go tool mockgen -destination=mock_policy_sync_test.go -package=application github.com/aegiscore/user-service/internal/features/permission/application PolicyReloadEngine,PolicyVersionPublisher,PolicyVersionTracker,Metrics
