package command

import (
	"context"
	"errors"
	"time"

	"go.uber.org/zap"

	"github.com/aegiscore/platform/runtime/logger"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	"github.com/aegiscore/user-service/internal/features/auth/application/authctx"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
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
	denylist authapplication.AccessTokenDenylist
	metrics  authapplication.Metrics
}

// NewLogoutCurrentSessionUseCase 构造当前会话登出 use case。
func NewLogoutCurrentSessionUseCase(sessions authsessions.Lifecycle, denylist authapplication.AccessTokenDenylist, metrics authapplication.Metrics) LogoutCurrentSessionUseCase {
	return &logoutCurrentSessionUseCase{
		sessions: sessions,
		denylist: denylist,
		metrics:  metricsOrNop(metrics),
	}
}

// LogoutCurrentSession 撤销当前 refresh token 会话，并把当前 access token 加入剩余有效期拒绝名单。
// 不递增 token version，避免同一用户的其他会话被一并踢下线。
func (u *logoutCurrentSessionUseCase) LogoutCurrentSession(ctx context.Context) (*LogoutResult, error) {
	userID, sessionID, err := authctx.AuthenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing authenticated session", zap.Error(err))
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonAuthContextMissing)
		return nil, err
	}
	token, err := authctx.AuthenticatedAccessToken(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing access token identity", zap.String("user_id", userID.String()))
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonAuthContextMissing)
		return nil, err
	}
	if err := u.sessions.DeleteSession(ctx, userID, sessionID); err != nil && !errors.Is(err, authdomain.ErrAuthSessionNotFound) {
		u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonSessionDeleteFailed)
		return nil, err
	}
	// 会话已删除或不存在时仍必须写入拒绝名单。写入失败不能报成功，调用方重试时要能补写同一 token。
	// 若 Token 已自然过期，原生 JWT 校验会直接拦截，无需且不可写入非正 TTL 的拒绝名单。
	ttl := time.Until(token.ExpiresAt)
	if ttl > 0 {
		if err := u.denylist.DenyAccessToken(ctx, token.ID, ttl); err != nil {
			logger.Warn(ctx, "logout access denylist write failed", zap.String("user_id", userID.String()), zap.Error(err))
			u.metrics.LogoutFailed(ctx, authapplication.MetricsOperationLogoutCurrent, authapplication.MetricsReasonAccessDenylistFailed)
			return nil, authdomain.ErrSessionRevocationIncomplete
		}
	}
	u.metrics.LogoutSucceeded(ctx, authapplication.MetricsOperationLogoutCurrent)
	return &LogoutResult{LoggedOut: true}, nil
}
