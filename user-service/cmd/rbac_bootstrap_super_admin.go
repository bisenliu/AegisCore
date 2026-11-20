package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	platformlogger "github.com/aegiscore/platform/runtime/logger"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

// defaultAdminBootstrapPasswordEnv 是初始化超级管理员密码的环境变量名称。
const defaultAdminBootstrapPasswordEnv = "ADMIN_BOOTSTRAP_PASSWORD"

type rbacSeedService interface {
	Seed(ctx context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error)
}

type rbacSeedDependencies struct {
	service   rbacSeedService
	bootstrap *rolebootstrap.Service
	cfg       *serviceconfig.Config
	log       *zap.Logger
}

type rbacSeedDependencyFactory func(context.Context) (rbacSeedDependencies, func() error, error)

// bootstrapSuperAdmin 创建系统首位超级管理员；如果账号已存在或权限未预置则停止操作，避免覆盖现有账号。
func bootstrapSuperAdmin(ctx context.Context, deps rbacSeedDependencies, opts rbacBootstrapSuperAdminOptions) (rolebootstrap.BootstrapSuperAdminResult, error) {
	ctx = contextWithRBACLogger(ctx, deps)
	passwordEnv := strings.TrimSpace(opts.passwordEnv)
	if passwordEnv == "" {
		passwordEnv = defaultAdminBootstrapPasswordEnv
	}
	password, ok := os.LookupEnv(passwordEnv)
	if !ok {
		return rolebootstrap.BootstrapSuperAdminResult{}, fmt.Errorf("bootstrap password environment variable %s is required", passwordEnv)
	}
	// 密码只从环境变量读取且不写入命令输出，后续明文校验和哈希由 bootstrap application 负责。
	return deps.bootstrap.BootstrapSuperAdmin(ctx, rolebootstrap.Command{
		Username: opts.username,
		Nickname: opts.nickname,
		Password: password,
	})
}

// contextWithRBACLogger 将权限初始化操作的日志工具传递到上下文中，保证整套初始化操作打印的日志风格统一。
func contextWithRBACLogger(ctx context.Context, deps rbacSeedDependencies) context.Context {
	if deps.log == nil {
		return ctx
	}
	return platformlogger.ToContext(ctx, deps.log)
}
