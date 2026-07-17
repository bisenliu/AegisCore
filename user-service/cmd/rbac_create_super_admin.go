package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"
	"go.uber.org/zap"

	commonlogger "github.com/aegiscore/common/runtime/logger"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	roleseed "github.com/aegiscore/user-service/internal/features/role/application/seed"
	usercommand "github.com/aegiscore/user-service/internal/features/user/application/command"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

type rbacCreateSuperAdminResult struct {
	userID          uuid.UUID
	created         bool
	passwordUpdated bool
	roleAdded       bool
}

type rbacSeedService interface {
	Seed(ctx context.Context, opts roleseed.SeedOptions) (roleseed.SeedResult, error)
	AssignSuperAdmin(ctx context.Context, userID uuid.UUID) (roleseed.AssignSuperAdminResult, error)
}

type rbacCredentialStore interface {
	GetByUsername(ctx context.Context, username string) (*authdomain.UserCredential, error)
	UpdateCredentials(ctx context.Context, input authdomain.UpdateCredentialsInput) (int64, error)
}

type rbacPasswordHasher interface {
	HashContext(ctx context.Context, plain string) (string, error)
}

type rbacSeedDependencies struct {
	service         rbacSeedService
	users           usercommand.CreateUserService
	credentials     rbacCredentialStore
	passwordService rbacPasswordHasher
	log             *zap.Logger
}

type rbacSeedDependencyFactory func(context.Context, string) (rbacSeedDependencies, func() error, error)

func createSuperAdmin(ctx context.Context, deps rbacSeedDependencies, opts rbacCreateSuperAdminOptions) (rbacCreateSuperAdminResult, error) {
	ctx = contextWithRBACLogger(ctx, deps)
	// 命令保持幂等：不存在则创建用户，存在则默认只补超级管理员角色；显式 reset 时才更新密码和状态。
	normalized, err := normalizeCreateSuperAdminOptions(opts)
	if err != nil {
		return rbacCreateSuperAdminResult{}, err
	}

	credential, err := deps.credentials.GetByUsername(ctx, normalized.username)
	if err != nil && !errors.Is(err, identity.ErrUserNotFound) {
		return rbacCreateSuperAdminResult{}, err
	}

	result := rbacCreateSuperAdminResult{}
	if errors.Is(err, identity.ErrUserNotFound) {
		status := identity.UserStatusNormal
		created, err := deps.users.CreateUser(ctx, usercommand.CreateUserCommand{Nickname: normalized.nickname, Username: normalized.username, Password: normalized.password, Status: &status})
		if err != nil {
			return rbacCreateSuperAdminResult{}, err
		}
		result.userID = created.User.UserID
		result.created = true
	} else {
		result.userID = credential.UserID
		if !normalized.resetPassword {
			assigned, err := deps.service.AssignSuperAdmin(ctx, result.userID)
			if err != nil {
				return rbacCreateSuperAdminResult{}, err
			}
			result.roleAdded = assigned.Added
			return result, nil
		}
		passwordHash, err := deps.passwordService.HashContext(ctx, normalized.password)
		if err != nil {
			return rbacCreateSuperAdminResult{}, fmt.Errorf("hash create super admin password: %w", err)
		}
		if _, err := deps.credentials.UpdateCredentials(ctx, authdomain.UpdateCredentialsInput{UserID: credential.UserID, PasswordHash: passwordHash, Status: identity.UserStatusNormal}); err != nil {
			return rbacCreateSuperAdminResult{}, err
		}
		result.passwordUpdated = true
	}

	assigned, err := deps.service.AssignSuperAdmin(ctx, result.userID)
	if err != nil {
		return rbacCreateSuperAdminResult{}, err
	}
	result.roleAdded = assigned.Added
	return result, nil
}

func contextWithRBACLogger(ctx context.Context, deps rbacSeedDependencies) context.Context {
	if deps.log == nil {
		return ctx
	}
	return commonlogger.ToContext(ctx, deps.log)
}

func normalizeCreateSuperAdminOptions(opts rbacCreateSuperAdminOptions) (rbacCreateSuperAdminOptions, error) {
	passwordEnv := strings.TrimSpace(opts.passwordEnv)
	if passwordEnv == "" {
		passwordEnv = defaultCreateSuperAdminPasswordEnv
	}
	adminPassword, ok := os.LookupEnv(passwordEnv)
	if !ok {
		return rbacCreateSuperAdminOptions{}, fmt.Errorf("%s environment variable is required", passwordEnv)
	}
	normalized := rbacCreateSuperAdminOptions{
		username:      normalizeUsername(opts.username),
		nickname:      strings.TrimSpace(opts.nickname),
		password:      strings.TrimSpace(adminPassword),
		passwordEnv:   passwordEnv,
		resetPassword: opts.resetPassword,
	}
	if normalized.username == "" {
		return rbacCreateSuperAdminOptions{}, fmt.Errorf("admin username is required")
	}
	if normalized.nickname == "" {
		normalized.nickname = normalized.username
	}
	if strings.TrimSpace(adminPassword) == "" {
		return rbacCreateSuperAdminOptions{}, fmt.Errorf("admin password is required")
	}
	return normalized, nil
}

func normalizeUsername(username string) string {
	return strings.ToLower(strings.TrimSpace(username))
}
