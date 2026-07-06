package authhttp

import (
	"errors"

	contracterrors "github.com/aegiscore/common/contract/errors"
	contractresponse "github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/password"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/aegiscore/user-service/internal/shared/identity"
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

func toAuthHTTPError(err error) error {
	switch {
	case errors.Is(err, password.ErrPasswordKDFBusy):
		return contracterrors.WrapServiceUnavailable(err, messages.AuthServiceBusy)
	case errors.Is(err, authdomain.ErrSessionRevocationIncomplete):
		return contracterrors.WrapServiceUnavailable(err, messages.AuthRevocationIncomplete)
	case errors.Is(err, authdomain.ErrInvalidCredentials):
		return contracterrors.UnauthenticatedError(messages.InvalidCredentials)
	case errors.Is(err, authdomain.ErrMissingSession):
		return contracterrors.UnauthenticatedError(messages.MissingSession)
	case errors.Is(err, authdomain.ErrTokenInvalid):
		return contracterrors.TokenInvalidError(messages.MissingSession)
	case errors.Is(err, identity.ErrUserNotFound):
		return contracterrors.NotFoundError(messages.UserNotFound)
	default:
		return contracterrors.FromError(err)
	}
}
