//go:build generate

package middleware

//go:generate go tool mockgen -destination=mock_casbin_test.go -package=middleware github.com/aegiscore/common/http/middleware CasbinAuthorizer
//go:generate go tool mockgen -destination=mock_auth_test.go -package=middleware github.com/aegiscore/common/security/auth TokenVersionValidator
