package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
)

// LogoutAllSessionsUseCase 处理认证用户全部会话撤销。
type LogoutAllSessionsUseCase interface {
	LogoutAllSessions(ctx context.Context) (*LogoutResult, error)
}

type logoutAllSessionsUseCase struct {
	sessions authsessions.Lifecycle
	metrics  authapplication.Metrics
}

// NewLogoutAllSessionsUseCase 构造全部会话登出 use case。
func NewLogoutAllSessionsUseCase(deps LogoutAllSessionsDeps) LogoutAllSessionsUseCase {
	return &logoutAllSessionsUseCase{
		sessions: deps.Sessions,
		metrics:  metricsOrNop(deps.Metrics),
	}
}

// LogoutAllSessions 递增认证用户的 token version，并移除全部 refresh 会话。
func (u *logoutAllSessionsUseCase) LogoutAllSessions(ctx context.Context) (*LogoutResult, error) {
	userID, err := authctx.AuthenticatedUserID(ctx)
	if err != nil {
		logger.Warn(ctx, "logout all missing authenticated session", zap.Error(err))
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutAll, authapplication.MetricsReasonAuthContextMissing)
		return nil, err
	}
	if _, err = u.sessions.RevokeAllUserSessions(ctx, userID); err != nil {
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutAll, authapplication.MetricsReasonSessionRevokeFailed)
		return nil, err
	}
	u.metrics.LogoutSucceeded(ctx, authapplication.MetricsOperationLogoutAll)
	return &LogoutResult{LoggedOut: true}, nil
}
