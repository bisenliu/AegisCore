package command

import (
	"context"
	"fmt"
	"uuid"

	runtimeid "github.com/aegiscore/common/runtime/id"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
)

// RefreshTokenSettings 包含 refresh token use case 所需的运行时开关。
type RefreshTokenSettings struct {
	RefreshTokenRotation bool
}

// metricsOrNop 如果未提供专门的统计记录器，则使用空操作对象兜底，避免业务流程中出现空指针异常。
func metricsOrNop(metrics authapplication.Metrics) authapplication.Metrics {
	if metrics == nil {
		return authapplication.NopMetrics()
	}
	return metrics
}

// issueTokenPair 创建用户登录会话并签发通行凭证（访问通行证与续期通行证）；任何一步出错均不返回可用凭证。
func issueTokenPair(ctx context.Context, issuer authtokens.Issuer, sessions authsessions.Lifecycle, userID uuid.UUID, tokenVersion int64, sessionID string) (*authtokens.TokenResult, string, error) {
	issued, err := issuer.IssueTokenPair(ctx, userID, tokenVersion, sessionID)
	if err != nil {
		return nil, authapplication.MetricsReasonTokenIssueFailed, err
	}
	if err := sessions.CreateTokenSession(ctx, userID, sessionID, tokenVersion, issued.RefreshTTL); err != nil {
		return nil, authapplication.MetricsReasonSessionCreateFailed, err
	}
	return issued.Result, authapplication.MetricsReasonNone, nil
}

// newAuthSessionID 生成一个全局唯一的登录会话编号。
func newAuthSessionID() (string, error) {
	sessionID, err := runtimeid.NewUUIDString()
	if err != nil {
		return "", fmt.Errorf("generate auth session id: %w", err)
	}
	return sessionID, nil
}
