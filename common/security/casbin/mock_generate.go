//go:build generate

package casbin

//go:generate go tool mockgen -destination=mock_test.go -package=casbin github.com/aegiscore/common/security/casbin Enforcer
