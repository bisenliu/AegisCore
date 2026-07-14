//go:build generate

package credentials

//go:generate go tool mockgen -destination=mock_test.go -package=credentials github.com/aegiscore/user-service/internal/features/auth/application UserCredentialStore
//go:generate go tool mockgen -destination=mock_password_test.go -package=credentials github.com/aegiscore/user-service/internal/features/auth/application/credentials PasswordService
