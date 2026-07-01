package credentials

//go:generate go tool mockgen -destination=mock_test.go -package=credentials github.com/aegiscore/user-service/internal/features/auth/application UserCredentialStore
