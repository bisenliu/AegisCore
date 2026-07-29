package command

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// LoginUseCase 处理用户名密码认证。
type LoginUseCase interface {
	Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error)
}

// LoginCommand 是用户名密码认证的应用层输入。
type LoginCommand struct {
	Username string
	Password string
}

// LoginResult 是登录 use case 的成功业务结果。
type LoginResult struct {
	PasswordChangeRequired bool
	Tokens                 *authtokens.TokenResult
}

type loginUseCase struct {
	credentials authcredentials.Verifier
	tokens      authtokens.Issuer
	sessions    authsessions.Lifecycle
	metrics     authapplication.Metrics
}

// NewLoginUseCase 构造登录 use case。
func NewLoginUseCase(credentials authcredentials.Verifier, tokens authtokens.Issuer, sessions authsessions.Lifecycle, metrics authapplication.Metrics) LoginUseCase {
	return &loginUseCase{
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
		metrics:     metricsOrNop(metrics),
	}
}

// Login 校验凭证，并签发普通 token 或受限改密 token。
func (u *loginUseCase) Login(ctx context.Context, cmd LoginCommand) (*LoginResult, error) {
	if err := authvalidators.ValidateLoginCommand(cmd.Username, cmd.Password); err != nil {
		u.metrics.LoginFailed(ctx, authapplication.MetricsReasonValidationFailed)
		return nil, err
	}

	user, err := u.credentials.VerifyPassword(ctx, cmd.Username, cmd.Password)
	if err != nil {
		u.metrics.LoginFailed(ctx, loginFailureReason(err))
		return nil, err
	}

	if user.RequiresPasswordChange() {
		return u.loginWithPasswordChangeSession(ctx, cmd.Username, user)
	}

	return u.loginWithSession(ctx, cmd.Username, user)
}

func (u *loginUseCase) loginWithSession(ctx context.Context, username string, user *authdomain.UserCredential) (*LoginResult, error) {
	sessionID, err := newAuthSessionID()
	if err != nil {
		logger.Error(ctx, "generate auth session id failed", logger.StackTrace(zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion), zap.Error(err))...)
		u.metrics.LoginFailed(ctx, authapplication.MetricsReasonTokenIssueFailed)
		return nil, err
	}
	// refresh session 是 JWT token pair 的服务端投影；refresh、logout 和 token version 撤销都依赖该投影阻断仅凭 JWT 过期时间继续访问。
	tokens, reason, err := issueTokenPair(ctx, u.tokens, u.sessions, user.UserID, user.TokenVersion, sessionID)
	if err != nil {
		u.metrics.LoginFailed(ctx, reason)
		return nil, err
	}
	u.metrics.LoginSucceeded(ctx)
	return &LoginResult{Tokens: tokens}, nil
}

func (u *loginUseCase) loginWithPasswordChangeSession(ctx context.Context, username string, user *authdomain.UserCredential) (*LoginResult, error) {
	// 必须改密用户只完成凭证确认，不创建普通 refresh session，避免旧密码仍可换取长期会话。
	// 受限 token 的 jti、session_id 和 token_version 必须落到一次性改密会话中，后续改密时据此原子消费并防重放。
	logger.Warn(ctx, "login requires password change", zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion))
	sessionID, err := newAuthSessionID()
	if err != nil {
		logger.Error(ctx, "generate auth session id failed", logger.StackTrace(zap.String("username", username), zap.String("user_id", user.UserID.String()), zap.Int64("token_version", user.TokenVersion), zap.Error(err))...)
		u.metrics.LoginFailed(ctx, authapplication.MetricsReasonPasswordChangeRequiredIssueFailed)
		return nil, err
	}
	tokens, err := u.tokens.IssuePasswordChangeToken(ctx, user.UserID, user.TokenVersion, sessionID)
	if err != nil {
		u.metrics.LoginFailed(ctx, authapplication.MetricsReasonPasswordChangeRequiredIssueFailed)
		return nil, err
	}
	claims, _, err := u.tokens.ParsePasswordChangeToken(ctx, tokens.AccessToken)
	if err != nil {
		u.metrics.LoginFailed(ctx, authapplication.MetricsReasonPasswordChangeRequiredIssueFailed)
		return nil, err
	}
	if err := u.sessions.CreatePasswordChangeSession(ctx, user.UserID, sessionID, claims.ID, user.TokenVersion, time.Duration(tokens.ExpiresIn)*time.Second); err != nil {
		u.metrics.LoginFailed(ctx, authapplication.MetricsReasonPasswordChangeRequiredIssueFailed)
		return nil, err
	}
	u.metrics.LoginSucceeded(ctx)
	return &LoginResult{PasswordChangeRequired: true, Tokens: tokens}, nil
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
