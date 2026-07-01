package sessions

//go:generate go tool mockgen -destination=mock_test.go -package=sessions github.com/aegiscore/user-service/internal/features/auth/application UserTokenVersionStore,TokenVersionCache,RefreshSessionStore
//go:generate go tool mockgen -destination=mock_validators_test.go -package=sessions github.com/aegiscore/user-service/internal/features/auth/application/validators TokenVersionLocalInvalidator
