package command

import (
	"context"

	"go.uber.org/fx"

	"github.com/aegiscore/common/runtime/config"
	authcredentials "github.com/aegiscore/user-service/internal/features/auth/application/credentials"
	authsessions "github.com/aegiscore/user-service/internal/features/auth/application/sessions"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
)

// UseCaseDepsParams 包含构造 auth command use case 共享依赖所需的 Fx 输入。
type UseCaseDepsParams struct {
	fx.In

	Credentials authcredentials.Verifier
	Tokens      authtokens.Issuer
	Sessions    authsessions.Lifecycle
	Config      *config.Config
}

type UseCaseDeps struct {
	credentials          authcredentials.Verifier
	tokens               authtokens.Issuer
	sessions             authsessions.Lifecycle
	refreshTokenRotation bool
}

// NewUseCaseDeps 组合凭证、token、会话和轮换依赖，供具体 command use case 使用。
func NewUseCaseDeps(params UseCaseDepsParams) *UseCaseDeps {
	return &UseCaseDeps{
		credentials:          params.Credentials,
		tokens:               params.Tokens,
		sessions:             params.Sessions,
		refreshTokenRotation: params.Config.Auth.RefreshTokenRotation,
	}
}

func (d *UseCaseDeps) issueTokenPair(ctx context.Context, userID string, tokenVersion int64, sessionID string) (*authtokens.TokenResult, error) {
	tokens, err := d.tokens.IssueTokenPair(ctx, userID, tokenVersion, sessionID)
	if err != nil {
		return nil, err
	}
	if err := d.sessions.CreateTokenSession(ctx, userID, sessionID, tokenVersion, tokens.RefreshTTL); err != nil {
		return nil, err
	}
	return tokens.Response, nil
}
