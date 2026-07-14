//go:build generate

package query

//go:generate go tool mockgen -destination=mock_test.go -package=query github.com/aegiscore/user-service/internal/features/user/application UserProfileStore
