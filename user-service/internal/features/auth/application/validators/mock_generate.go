package validators

//go:generate go tool mockgen -destination=mock_test.go -package=validators github.com/aegiscore/user-service/internal/features/auth/application UserTokenVersionStore,TokenVersionCache
