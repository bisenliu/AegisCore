package main

import (
	"context"

	"go.uber.org/zap"

	commonlogger "github.com/aegiscore/common/runtime/logger"
	rolebootstrap "github.com/aegiscore/user-service/internal/features/role/application/bootstrap"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
)

type rbacSeedService interface {
	Seed(ctx context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error)
}

type rbacSeedDependencies struct {
	service   rbacSeedService
	bootstrap *rolebootstrap.Service
	log       *zap.Logger
}

type rbacSeedDependencyFactory func(context.Context, string) (rbacSeedDependencies, func() error, error)

func bootstrapSuperAdmin(ctx context.Context, deps rbacSeedDependencies, opts rbacBootstrapSuperAdminOptions) (rolebootstrap.BootstrapSuperAdminResult, error) {
	ctx = contextWithRBACLogger(ctx, deps)
	return deps.bootstrap.BootstrapSuperAdmin(ctx, rolebootstrap.Command{
		Username:    opts.username,
		Nickname:    opts.nickname,
		PasswordEnv: opts.passwordEnv,
	})
}

func contextWithRBACLogger(ctx context.Context, deps rbacSeedDependencies) context.Context {
	if deps.log == nil {
		return ctx
	}
	return commonlogger.ToContext(ctx, deps.log)
}
