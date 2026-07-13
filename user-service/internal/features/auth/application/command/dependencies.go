package command

import (
	"context"
	"fmt"

	runtimeid "github.com/aegiscore/common/runtime/id"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
)

// LoginDeps 包含登录 use case 所需的最小依赖。
type LoginDeps struct {
	Credentials authcredentials.Verifier
	Tokens      authtokens.Issuer
	Sessions    authsessions.Lifecycle
	Metrics     authapplication.Metrics
}

// RefreshTokenSettings 包含 refresh token use case 所需的运行时开关。
type RefreshTokenSettings struct {
	RefreshTokenRotation bool
}

// RefreshTokenDeps 包含 refresh token use case 所需的最小依赖。
type RefreshTokenDeps struct {
	Tokens   authtokens.Issuer
	Sessions authsessions.Lifecycle
	Metrics  authapplication.Metrics
	Settings RefreshTokenSettings
}

// ChangePasswordDeps 包含强制改密 use case 所需的最小依赖。
type ChangePasswordDeps struct {
	Credentials authcredentials.Verifier
	Tokens      authtokens.Issuer
	Sessions    authsessions.Lifecycle
	Metrics     authapplication.Metrics
}

// LogoutCurrentSessionDeps 包含当前会话登出 use case 所需的最小依赖。
type LogoutCurrentSessionDeps struct {
	Sessions authsessions.Lifecycle
	Metrics  authapplication.Metrics
}

// LogoutAllSessionsDeps 包含全部会话登出 use case 所需的最小依赖。
type LogoutAllSessionsDeps struct {
	Sessions authsessions.Lifecycle
	Metrics  authapplication.Metrics
}

func metricsOrNop(metrics authapplication.Metrics) authapplication.Metrics {
	if metrics == nil {
		return authapplication.NopMetrics()
	}
	return metrics
}

func issueTokenPair(ctx context.Context, issuer authtokens.Issuer, sessions authsessions.Lifecycle, userID string, tokenVersion int64, sessionID string) (*authtokens.TokenResult, string, error) {
	tokens, err := issuer.IssueTokenPair(ctx, userID, tokenVersion, sessionID)
	if err != nil {
		return nil, authapplication.MetricsReasonTokenIssueFailed, err
	}
	if err := sessions.CreateTokenSession(ctx, userID, sessionID, tokenVersion, tokens.RefreshTTL); err != nil {
		return nil, authapplication.MetricsReasonSessionCreateFailed, err
	}
	return tokens.Response, authapplication.MetricsReasonNone, nil
}

func newAuthSessionID() (string, error) {
	sessionID, err := runtimeid.NewUUIDString()
	if err != nil {
		return "", fmt.Errorf("generate auth session id: %w", err)
	}
	return sessionID, nil
}
