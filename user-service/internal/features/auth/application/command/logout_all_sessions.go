package command

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// LogoutAllSessionsUseCase 处理认证用户全部会话撤销。
type LogoutAllSessionsUseCase interface {
	LogoutAllSessions(ctx context.Context) (*LogoutResult, error)
}

type logoutAllSessionsUseCase struct {
	deps *UseCaseDeps
}

// NewLogoutAllSessionsUseCase 构造全部会话登出 use case。
func NewLogoutAllSessionsUseCase(deps *UseCaseDeps) LogoutAllSessionsUseCase {
	return &logoutAllSessionsUseCase{deps: deps}
}

// LogoutAllSessions 递增认证用户的 token version，并移除全部 refresh 会话。
func (u *logoutAllSessionsUseCase) LogoutAllSessions(ctx context.Context) (*LogoutResult, error) {
	userID, _, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout all missing authenticated session", zap.Error(err))
		return nil, err
	}
	parsedUserID, err := uuid.Parse(userID)
	if err != nil {
		logger.Warn(ctx, "logout all user id invalid", zap.String("user_id", userID))
		return nil, authdomain.ErrMissingSession
	}
	if _, err = u.deps.sessions.RevokeAllUserSessions(ctx, parsedUserID); err != nil {
		return nil, err
	}
	return &LogoutResult{LoggedOut: true}, nil
}
