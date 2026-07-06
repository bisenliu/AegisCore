package command

//go:generate go tool mockgen -destination=mock_test.go -package=command github.com/aegiscore/user-service/internal/features/auth/application UserCredentialStore,UserTokenVersionStore,TokenVersionCache,RefreshSessionStore,PasswordChangeSessionStore,Metrics
//go:generate go tool mockgen -destination=mock_credentials_test.go -package=command github.com/aegiscore/user-service/internal/features/auth/application/credentials Verifier
//go:generate go tool mockgen -destination=mock_tokens_test.go -package=command github.com/aegiscore/user-service/internal/features/auth/application/tokens Issuer
//go:generate go tool mockgen -destination=mock_sessions_test.go -package=command github.com/aegiscore/user-service/internal/features/auth/application/sessions Lifecycle
