package rolehttp

//go:generate go tool mockgen -destination=mock_test.go -package=rolehttp github.com/aegiscore/user-service/internal/features/role/application/command RoleCommandService
//go:generate go tool mockgen -destination=mock_query_test.go -package=rolehttp github.com/aegiscore/user-service/internal/features/role/application/query RoleQueryService
