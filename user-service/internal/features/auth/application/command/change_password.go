package command

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authvalidators "github.com/aegiscore/user-service/internal/features/auth/application/validators"
)

// ChangePasswordUseCase 处理强制改密。
type ChangePasswordUseCase interface {
	ChangePassword(ctx context.Context, cmd ChangePasswordCommand) (*ChangePasswordResult, error)
}

// ChangePasswordCommand 是使用受限 token 完成强制改密的应用层输入。
type ChangePasswordCommand struct {
	Token       string
	NewPassword string
}

// ChangePasswordResult 表示改密 use case 是否完成。
type ChangePasswordResult struct {
	Changed bool
}

type changePasswordUseCase struct {
	deps *UseCaseDeps
}

// NewChangePasswordUseCase 构造强制改密 use case。
func NewChangePasswordUseCase(deps *UseCaseDeps) ChangePasswordUseCase {
	return &changePasswordUseCase{deps: deps}
}

// ChangePassword 校验受限 token，更新凭证并撤销现有会话。
func (u *changePasswordUseCase) ChangePassword(ctx context.Context, cmd ChangePasswordCommand) (*ChangePasswordResult, error) {
	if err := authvalidators.ValidateChangePasswordCommand(cmd.Token, cmd.NewPassword); err != nil {
		return nil, err
	}

	parsedUserID, err := u.verifyPasswordChangeToken(ctx, cmd.Token)
	if err != nil {
		return nil, err
	}
	updated, err := u.deps.credentials.ChangePassword(ctx, parsedUserID, cmd.NewPassword)
	if err != nil {
		return nil, err
	}
	if err := u.deps.sessions.RevokeUserSessionsAtVersion(ctx, updated.UserID, updated.TokenVersion); err != nil {
		logger.Error(ctx, "password change session revocation projection failed", logger.StackTrace(zap.String("user_id", updated.UserID.String()), zap.Int64("token_version", updated.TokenVersion), zap.Error(err))...)
	}
	return &ChangePasswordResult{Changed: true}, nil
}

func (u *changePasswordUseCase) verifyPasswordChangeToken(ctx context.Context, token string) (uuid.UUID, error) {
	claims, parsedUserID, err := u.deps.tokens.ParsePasswordChangeToken(ctx, token)
	if err != nil {
		return uuid.Nil, err
	}
	if err := u.deps.sessions.ValidatePasswordChangeClaims(ctx, claims); err != nil {
		return uuid.Nil, err
	}
	return parsedUserID, nil
}
