package command

import (
	"context"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
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
	deps *UseCaseDeps
}

// NewLogoutCurrentSessionUseCase 构造当前会话登出 use case。
func NewLogoutCurrentSessionUseCase(deps *UseCaseDeps) LogoutCurrentSessionUseCase {
	return &logoutCurrentSessionUseCase{deps: deps}
}

// LogoutCurrentSession 撤销当前 refresh token 会话，但不修改用户 token version。
func (u *logoutCurrentSessionUseCase) LogoutCurrentSession(ctx context.Context) (*LogoutResult, error) {
	userID, sessionID, err := authenticatedSession(ctx)
	if err != nil {
		logger.Warn(ctx, "logout missing authenticated session", zap.Error(err))
		return nil, err
	}
	if err := u.deps.sessions.DeleteSession(ctx, userID, sessionID); err != nil {
		return nil, err
	}
	return &LogoutResult{LoggedOut: true}, nil
}
