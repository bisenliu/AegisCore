package command

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	runtimeid "github.com/aegiscore/common/runtime/id"
	"github.com/aegiscore/common/runtime/logger"
	"github.com/aegiscore/common/security/password"
	userapplication "github.com/aegiscore/user-service/internal/features/user/application"
	"github.com/aegiscore/user-service/internal/features/user/application/validators"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

// CreateUserCommand 包含创建用户所需的应用层输入。
type CreateUserCommand struct {
	Nickname string
	Username string
	Password string
	Status   *identity.UserStatus
}

// CreateUserResult 是创建用户用例的 transport-neutral 输出。
type CreateUserResult struct {
	User userdomain.User
}

// CreateUserService 定义用户资料写侧用例。
type CreateUserService interface {
	CreateUser(ctx context.Context, cmd CreateUserCommand) (*CreateUserResult, error)
}

type createUserService struct {
	store           userapplication.UserProfileStore
	passwordService *password.Service
}

// NewCreateUserService 根据仓储依赖构造用户资料写侧服务。
func NewCreateUserService(store userapplication.UserProfileStore, passwordService *password.Service) CreateUserService {
	return &createUserService{store: store, passwordService: passwordService}
}

// CreateUser 创建新用户资料，哈希密码，并将 username 冲突映射为领域错误。
func (s *createUserService) CreateUser(ctx context.Context, cmd CreateUserCommand) (*CreateUserResult, error) {
	status := validators.CreateUserStatus(cmd.Status)

	passwordHash, err := s.passwordService.HashContext(ctx, cmd.Password)
	if err != nil {
		logger.Error(ctx, "hash user password failed", logger.StackTrace(zap.String("username", cmd.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, fmt.Errorf("hash user password: %w", err)
	}

	userID, err := runtimeid.NewUUID()
	if err != nil {
		logger.Error(ctx, "generate user id failed", logger.StackTrace(zap.String("username", cmd.Username), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, fmt.Errorf("generate user id: %w", err)
	}

	user, err := s.store.Create(ctx, userapplication.CreateUserInput{Nickname: cmd.Nickname, UserID: userID, Username: cmd.Username, PasswordHash: passwordHash, Status: status})
	if err != nil {
		if errors.Is(err, identity.ErrUserAlreadyExists) {
			logger.Warn(ctx, "create user conflict", zap.String("username", cmd.Username), zap.Int64("status", int64(status)))
			return nil, identity.ErrUserAlreadyExists
		}
		logger.Error(ctx, "create user failed", logger.StackTrace(zap.String("username", cmd.Username), zap.String("user_id", userID.String()), zap.Int64("status", int64(status)), zap.Error(err))...)
		return nil, err
	}
	return &CreateUserResult{User: *user}, nil
}
