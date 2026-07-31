//go:build generate

package authhttp

//go:generate go tool mockgen -destination=mock_test.go -package=authhttp github.com/aegiscore/user-service/internal/features/auth/application/command LoginUseCase,RefreshTokenUseCase,ForceChangePasswordUseCase,LogoutCurrentSessionUseCase,LogoutAllSessionsUseCase
