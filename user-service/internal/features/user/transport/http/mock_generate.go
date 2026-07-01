package userhttp

//go:generate go tool mockgen -destination=mock_test.go -package=userhttp github.com/aegiscore/user-service/internal/features/user/application/command CreateUserService
//go:generate go tool mockgen -destination=mock_query_test.go -package=userhttp github.com/aegiscore/user-service/internal/features/user/application/query UserQueryService
