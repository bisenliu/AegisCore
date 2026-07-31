package command

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// ForceChangePasswordUseCase 处理强制改密。
type ForceChangePasswordUseCase interface {
	ForceChangePassword(ctx context.Context, cmd ForceChangePasswordCommand) (*ForceChangePasswordResult, error)
}

// ForceChangePasswordCommand 是使用受限 token 完成强制改密的应用层输入。
type ForceChangePasswordCommand struct {
	Token       string
	NewPassword string
}

// ForceChangePasswordResult 表示强制改密 use case 是否完成。
type ForceChangePasswordResult struct {
	Changed bool
}

type forceChangePasswordUseCase struct {
	credentials authcredentials.Verifier
	tokens      authtokens.Issuer
	sessions    authsessions.Lifecycle
	metrics     authapplication.Metrics
}

// NewForceChangePasswordUseCase 构造强制改密 use case。
func NewForceChangePasswordUseCase(credentials authcredentials.Verifier, tokens authtokens.Issuer, sessions authsessions.Lifecycle, metrics authapplication.Metrics) ForceChangePasswordUseCase {
	return &forceChangePasswordUseCase{
		credentials: credentials,
		tokens:      tokens,
		sessions:    sessions,
		metrics:     metricsOrNop(metrics),
	}
}

// ForceChangePassword 校验受限 token，更新凭证并撤销现有会话。
func (u *forceChangePasswordUseCase) ForceChangePassword(ctx context.Context, cmd ForceChangePasswordCommand) (*ForceChangePasswordResult, error) {
	if err := authvalidators.ValidateForceChangePasswordCommand(cmd.Token, cmd.NewPassword); err != nil {
		return nil, err
	}

	parsedUserID, tokenVersion, err := u.verifyPasswordChangeToken(ctx, cmd.Token)
	if err != nil {
		return nil, err
	}
	updated, err := u.credentials.ForceChangePassword(ctx, parsedUserID, tokenVersion, cmd.NewPassword)
	if err != nil {
		return nil, err
	}
	// 密码已经写入后再刷新撤销投影；这里返回的错误表示“凭证已变更但会话撤销投影不完整”，调用方不能当作密码未变更处理。
	if err := u.sessions.RevokeUserSessionsAtVersion(ctx, updated.UserID, updated.TokenVersion); err != nil {
		logger.Error(ctx, "password change session revocation projection failed", logger.StackTrace(zap.String("user_id", updated.UserID.String()), zap.Int64("token_version", updated.TokenVersion), zap.Error(err))...)
		u.metrics.PasswordChangeRevocationProjectionFailed(ctx, authapplication.MetricsPasswordChangeRevocationProjection)
		return nil, errors.Join(authdomain.ErrSessionRevocationIncomplete, err)
	}
	return &ForceChangePasswordResult{Changed: true}, nil
}

func (u *forceChangePasswordUseCase) verifyPasswordChangeToken(ctx context.Context, token string) (uuid.UUID, int64, error) {
	claims, parsedUserID, err := u.tokens.ParsePasswordChangeToken(ctx, token)
	if err != nil {
		return uuid.Nil, 0, err
	}
	if err := u.sessions.ConsumePasswordChangeClaims(ctx, claims); err != nil {
		return uuid.Nil, 0, err
	}
	return parsedUserID, claims.TokenVersion, nil
}
