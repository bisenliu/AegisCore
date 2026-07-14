//go:build generate

package authorization

//go:generate go tool mockgen -destination=mock_test.go -package=authorization github.com/aegiscore/user-service/internal/features/permission/application/authorization Engine
