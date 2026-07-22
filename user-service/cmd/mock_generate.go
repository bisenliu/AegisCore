//go:build generate

package main

//go:generate go tool mockgen -destination=mock_rbac_test.go -package=main github.com/aegiscore/user-service/cmd rbacSeedService
