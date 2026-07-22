package bootstrap

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/aegiscore/user-service/internal/shared/identity"
	"github.com/aegiscore/user-service/internal/shared/rbacbaseline"
)

const (
	defaultPasswordEnv = "ADMIN_BOOTSTRAP_PASSWORD"
	minPasswordBytes   = 12
	maxPasswordBytes   = 72
)

// Command 是 bootstrap-super-admin CLI 传入应用服务的原始输入。
type Command struct {
	Username    string
	Nickname    string
	PasswordEnv string
}

// Service 编排一次性超级管理员 bootstrap。
type Service struct {
	store  BootstrapStore
	hasher PasswordHasher
}

// NewService 构造 bootstrap 应用服务。
func NewService(store BootstrapStore, hasher PasswordHasher) *Service {
	return &Service{store: store, hasher: hasher}
}

// BootstrapSuperAdmin 校验输入、哈希临时密码并创建固定 bootstrap 用户。
func (s *Service) BootstrapSuperAdmin(ctx context.Context, cmd Command) (BootstrapSuperAdminResult, error) {
	if s == nil || s.store == nil || s.hasher == nil {
		return BootstrapSuperAdminResult{}, fmt.Errorf("%w: bootstrap service dependencies are required", ErrBootstrapInvalidInput)
	}
	normalized, password, err := normalizeCommand(cmd)
	if err != nil {
		return BootstrapSuperAdminResult{}, err
	}
	passwordHash, err := s.hasher.HashContext(ctx, password)
	if err != nil {
		return BootstrapSuperAdminResult{}, fmt.Errorf("hash bootstrap super admin password: %w", err)
	}
	userID, err := uuid.Parse(BootstrapSuperAdminUserID)
	if err != nil {
		return BootstrapSuperAdminResult{}, fmt.Errorf("parse bootstrap super admin user id: %w", err)
	}
	roleID, err := uuid.Parse(rbacbaseline.SuperAdminRoleID)
	if err != nil {
		return BootstrapSuperAdminResult{}, fmt.Errorf("parse super admin role id: %w", err)
	}
	result, err := s.store.BootstrapSuperAdmin(ctx, BootstrapSuperAdminInput{
		UserID:       userID,
		RoleID:       roleID,
		Username:     normalized.Username,
		Nickname:     normalized.Nickname,
		PasswordHash: passwordHash,
		Status:       identity.UserStatusMustChangePassword,
	})
	if err != nil {
		return BootstrapSuperAdminResult{}, err
	}
	if result == nil {
		return BootstrapSuperAdminResult{}, fmt.Errorf("%w: bootstrap store returned nil result", ErrBootstrapInvalidInput)
	}
	return *result, nil
}

type normalizedCommand struct {
	Username    string
	Nickname    string
	PasswordEnv string
}

func normalizeCommand(cmd Command) (normalizedCommand, string, error) {
	passwordEnv := strings.TrimSpace(cmd.PasswordEnv)
	if passwordEnv == "" {
		passwordEnv = defaultPasswordEnv
	}
	password, ok := os.LookupEnv(passwordEnv)
	if !ok {
		return normalizedCommand{}, "", fmt.Errorf("%w: %s environment variable is required", ErrBootstrapInvalidInput, passwordEnv)
	}
	if len(password) < minPasswordBytes {
		return normalizedCommand{}, "", fmt.Errorf("%w: bootstrap password must be at least %d bytes", ErrBootstrapInvalidInput, minPasswordBytes)
	}
	if len(password) > maxPasswordBytes {
		return normalizedCommand{}, "", fmt.Errorf("%w: bootstrap password must be at most %d bytes", ErrBootstrapInvalidInput, maxPasswordBytes)
	}
	username := strings.ToLower(strings.TrimSpace(cmd.Username))
	if username == "" {
		return normalizedCommand{}, "", fmt.Errorf("%w: bootstrap username is required", ErrBootstrapInvalidInput)
	}
	nickname := strings.TrimSpace(cmd.Nickname)
	if nickname == "" {
		nickname = username
	}
	return normalizedCommand{Username: username, Nickname: nickname, PasswordEnv: passwordEnv}, password, nil
}
