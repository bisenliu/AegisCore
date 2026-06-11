package authhttp

import (
	"errors"

	"github.com/aegiscore/common/contract/response"
	authapi "github.com/aegiscore/user-service/internal/features/auth/api"
	authapp "github.com/aegiscore/user-service/internal/features/auth/app"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/aegiscore/user-service/internal/messages"
)

func toTokenResponse(result *authapp.TokenResult) authapi.TokenResponse {
	return authapi.TokenResponse{
		AccessToken:            result.AccessToken,
		RefreshToken:           result.RefreshToken,
		TokenType:              result.TokenType,
		ExpiresIn:              result.ExpiresIn,
		PasswordChangeRequired: result.PasswordChangeRequired,
	}
}

func toChangePasswordResponse(result *authapp.ChangePasswordResult) authapi.ChangePasswordResponse {
	return authapi.ChangePasswordResponse{Changed: result.Changed}
}

func toLogoutResponse(result *authapp.LogoutResult) authapi.LogoutResponse {
	return authapi.LogoutResponse{LoggedOut: result.LoggedOut}
}

func toAuthHTTPError(err error) error {
	switch {
	case errors.Is(err, authdomain.ErrInvalidCredentials):
		return response.UnauthenticatedError(messages.InvalidCredentials)
	case errors.Is(err, authdomain.ErrMissingSession):
		return response.UnauthenticatedError(messages.MissingSession)
	case errors.Is(err, authdomain.ErrTokenInvalid):
		return response.TokenInvalidError(messages.MissingSession)
	case errors.Is(err, userdomain.ErrUserNotFound):
		return response.NotFoundError(messages.UserNotFound)
	default:
		return response.FromError(err)
	}
}
