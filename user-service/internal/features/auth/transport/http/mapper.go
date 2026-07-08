package authhttp

import (
	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	"github.com/aegiscore/user-service/internal/messages"
)

func toTokenResponse(result *authtokens.TokenResult) TokenResponse {
	return TokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		ExpiresIn:    result.ExpiresIn,
	}
}

func toPasswordChangeRequiredEnvelope(result *authtokens.TokenResult) contractresponse.Envelope {
	return contractresponse.Envelope{
		Success: false,
		Code:    contracterrors.CodePasswordChangeRequired,
		Message: messages.PasswordChangeRequired,
		Data:    toTokenResponse(result),
	}
}

func toChangePasswordResponse(result *authcommand.ChangePasswordResult) ChangePasswordResponse {
	return ChangePasswordResponse{Changed: result.Changed}
}

func toLogoutResponse(result *authcommand.LogoutResult) LogoutResponse {
	return LogoutResponse{LoggedOut: result.LoggedOut}
}
