package command

import (
	"context"

	"github.com/aegiscore/common/runtime/logger"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// LoginUseCase 处理用户名密码认证。
type LoginUseCase interface {
	Login(ctx context.Context, cmd LoginCommand) (*TokenResult, error)
}

// LoginCommand 是用户名密码认证的应用层输入。
type LoginCommand struct {
	Username string
	Password string
}

type loginUseCase struct {
	deps *UseCaseDeps
}

// NewLoginUseCase 构造登录 use case。
func NewLoginUseCase(deps *UseCaseDeps) LoginUseCase {
	return &loginUseCase{deps: deps}
}

// Login 校验凭证，并签发普通 token 或受限改密 token。
func (u *loginUseCase) Login(ctx context.Context, cmd LoginCommand) (*TokenResult, error) {
	if err := authvalidators.ValidateLoginCommand(cmd.Username, cmd.Password); err != nil {
		return nil, err
	}

	logger.Info(ctx, "login user", zap.String("username", cmd.Username))
	user, err := u.deps.credentials.VerifyPassword(ctx, cmd.Username, cmd.Password)
	if err != nil {
		return nil, err
	}

	if user.RequiresPasswordChange() {
		// 必须改密用户只认证到可获取受限改密 token 的程度。
		logger.Warn(ctx, "login requires password change", zap.String("username", cmd.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
		return u.deps.tokens.IssuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	}

	logger.Info(ctx, "login user authenticated", zap.String("username", cmd.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	return u.deps.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
}
