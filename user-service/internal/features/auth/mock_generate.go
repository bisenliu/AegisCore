//go:build generate

package auth

//go:generate go tool mockgen -destination=mock_test.go -package=auth github.com/aegiscore/user-service/internal/features/auth/application UserTokenVersionStore,TokenVersionCache
