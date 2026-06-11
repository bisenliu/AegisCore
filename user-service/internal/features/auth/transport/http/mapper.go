package authhttp

import (
	"errors"

	contracterrors "github.com/aegiscore/common/contract/errors"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func toTokenResponse(result *authapplication.TokenResult) TokenResponse {
	return TokenResponse{
		AccessToken:            result.AccessToken,
		RefreshToken:           result.RefreshToken,
		TokenType:              result.TokenType,
		ExpiresIn:              result.ExpiresIn,
		PasswordChangeRequired: result.PasswordChangeRequired,
	}
}

func toChangePasswordResponse(result *authapplication.ChangePasswordResult) ChangePasswordResponse {
	return ChangePasswordResponse{Changed: result.Changed}
}

func toLogoutResponse(result *authapplication.LogoutResult) LogoutResponse {
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
	case errors.Is(err, userdomain.ErrUserNotFound):
		return contracterrors.NotFoundError(messages.UserNotFound)
	default:
		return contracterrors.FromError(err)
	}
}
