//go:build generate

package command

//go:generate go tool mockgen -destination=mock_test.go -package=command github.com/aegiscore/user-service/internal/features/user/application UserProfileStore
