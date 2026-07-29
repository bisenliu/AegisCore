package command

import (
	"context"
	"fmt"

	"github.com/google/uuid"

	runtimeid "github.com/aegiscore/common/runtime/id"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
)

// RefreshTokenSettings 包含 refresh token use case 所需的运行时开关。
type RefreshTokenSettings struct {
	RefreshTokenRotation bool
}

func metricsOrNop(metrics authapplication.Metrics) authapplication.Metrics {
	if metrics == nil {
		return authapplication.NopMetrics()
	}
	return metrics
}

func issueTokenPair(ctx context.Context, issuer authtokens.Issuer, sessions authsessions.Lifecycle, userID uuid.UUID, tokenVersion int64, sessionID string) (*authtokens.TokenResult, string, error) {
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
