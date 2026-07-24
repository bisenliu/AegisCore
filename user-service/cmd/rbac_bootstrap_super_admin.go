package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"go.uber.org/zap"

	commonlogger "github.com/aegiscore/common/runtime/logger"
	serviceconfig "github.com/aegiscore/user-service/internal/config"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

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

type rbacSeedDependencyFactory func(context.Context, string) (rbacSeedDependencies, func() error, error)

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
	return deps.bootstrap.BootstrapSuperAdmin(ctx, rolebootstrap.Command{
		Username: opts.username,
		Nickname: opts.nickname,
		Password: password,
	})
}

func contextWithRBACLogger(ctx context.Context, deps rbacSeedDependencies) context.Context {
	if deps.log == nil {
		return ctx
	}
	return commonlogger.ToContext(ctx, deps.log)
}
