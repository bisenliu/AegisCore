package authhttp

import (
	"errors"

	contracterrors "github.com/aegiscore/common/contract/errors"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	"github.com/aegiscore/user-service/internal/messages"
	"github.com/aegiscore/user-service/internal/shared/identity"
)

func toTokenResponse(result *authtokens.TokenResult) TokenResponse {
	return TokenResponse{
		AccessToken:            result.AccessToken,
		RefreshToken:           result.RefreshToken,
		TokenType:              result.TokenType,
		ExpiresIn:              result.ExpiresIn,
		PasswordChangeRequired: result.PasswordChangeRequired,
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
