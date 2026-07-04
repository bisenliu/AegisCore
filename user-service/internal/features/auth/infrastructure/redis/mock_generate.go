package redis

//go:generate go tool mockgen -destination=mock_test.go -package=redis github.com/aegiscore/user-service/internal/features/auth/application UserTokenVersionStore,Metrics
