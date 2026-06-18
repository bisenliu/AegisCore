package command

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// LoginUseCase 处理用户名密码认证。
type LoginUseCase interface {
	Login(ctx context.Context, cmd LoginCommand) (*authtokens.TokenResult, error)
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
func (u *loginUseCase) Login(ctx context.Context, cmd LoginCommand) (*authtokens.TokenResult, error) {
	if err := authvalidators.ValidateLoginCommand(cmd.Username, cmd.Password); err != nil {
		u.deps.metricsRecorder().LoginFailed(ctx, authapplication.MetricsReasonValidationFailed)
		return nil, err
	}

	logger.Info(ctx, "login user", zap.String("username", cmd.Username))
	user, err := u.deps.credentials.VerifyPassword(ctx, cmd.Username, cmd.Password)
	if err != nil {
		u.deps.metricsRecorder().LoginFailed(ctx, loginFailureReason(err))
		return nil, err
	}

	if user.RequiresPasswordChange() {
		// 必须改密用户只认证到可获取受限改密 token 的程度。
		logger.Warn(ctx, "login requires password change", zap.String("username", cmd.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
		tokens, err := u.deps.tokens.IssuePasswordChangeToken(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
		if err != nil {
			u.deps.metricsRecorder().LoginFailed(ctx, authapplication.MetricsReasonPasswordChangeRequiredIssueFailed)
			return nil, err
		}
		u.deps.metricsRecorder().LoginSucceeded(ctx)
		return tokens, nil
	}

	logger.Info(ctx, "login user authenticated", zap.String("username", cmd.Username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	tokens, reason, err := u.deps.issueTokenPair(ctx, user.UserID.String(), user.TokenVersion, uuid.NewString())
	if err != nil {
		u.deps.metricsRecorder().LoginFailed(ctx, reason)
		return nil, err
	}
	u.deps.metricsRecorder().LoginSucceeded(ctx)
	return tokens, nil
}

func loginFailureReason(err error) string {
	switch {
	case errors.Is(err, authdomain.ErrUserStatusRejected):
		return authapplication.MetricsReasonUserStatusRejected
	case errors.Is(err, authdomain.ErrInvalidCredentials):
		return authapplication.MetricsReasonCredentialInvalid
	default:
		return authapplication.MetricsReasonSystemError
	}
}
