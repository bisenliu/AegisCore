package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
)

// LogoutCurrentSessionUseCase 处理当前认证会话登出。
type LogoutCurrentSessionUseCase interface {
	LogoutCurrentSession(ctx context.Context) (*LogoutResult, error)
}

// LogoutResult 表示登出 use case 是否完成。
type LogoutResult struct {
	LoggedOut bool
}

type logoutCurrentSessionUseCase struct {
	sessions authsessions.Lifecycle
	metrics  authapplication.Metrics
}

// NewLogoutCurrentSessionUseCase 构造当前会话登出 use case。
func NewLogoutCurrentSessionUseCase(sessions authsessions.Lifecycle, metrics authapplication.Metrics) LogoutCurrentSessionUseCase {
	return &logoutCurrentSessionUseCase{
		sessions: sessions,
		metrics:  metricsOrNop(metrics),
	}
}

// LogoutCurrentSession 撤销当前 refresh token 会话，但不修改用户 token version。
func (u *logoutCurrentSessionUseCase) LogoutCurrentSession(ctx context.Context) (*LogoutResult, error) {
	userID, sessionID, err := authctx.AuthenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing authenticated session", zap.Error(err))
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonAuthContextMissing)
		return nil, err
	}
	if err := u.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonSessionDeleteFailed)
		return nil, err
	}
	u.metrics.LogoutSucceeded(ctx, authapplication.MetricsOperationLogoutCurrent)
	return &LogoutResult{LoggedOut: true}, nil
}
