package authhttp

import (
	"strings"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonauth "github.com/aegiscore/common/security/auth"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	"github.com/aegiscore/user-service/internal/messages"
)

// prepareLoginCommand 将登录 HTTP 请求转换为应用层命令。
func prepareLoginCommand(req LoginRequest) (authcommand.LoginCommand, error) {
	username := strings.TrimSpace(req.Username)
	password := strings.TrimSpace(req.Password)
	if username == "" || password == "" {
		return authcommand.LoginCommand{}, contracterrors.UnauthenticatedError(messages.InvalidCredentials)
	}
	return authcommand.LoginCommand{Username: username, Password: password}, nil
}

// prepareRefreshTokenCommand 将刷新 token HTTP 请求转换为应用层命令。
func prepareRefreshTokenCommand(req RefreshTokenRequest) (authcommand.RefreshTokenCommand, error) {
	refreshToken := commonauth.StripBearerPrefix(req.RefreshToken)
	if refreshToken == "" || strings.EqualFold(refreshToken, commonauth.TokenTypeBearer) {
		return authcommand.RefreshTokenCommand{}, contracterrors.TokenInvalidError(messages.MissingSession)
	}
	return authcommand.RefreshTokenCommand{RefreshToken: refreshToken}, nil
}

// prepareChangePasswordCommand 将强制改密 HTTP 请求转换为应用层命令。
func prepareChangePasswordCommand(req ChangePasswordRequest) (authcommand.ChangePasswordCommand, error) {
	token := commonauth.StripBearerPrefix(req.Token)
	newPassword := strings.TrimSpace(req.NewPassword)
	if token == "" || strings.EqualFold(token, commonauth.TokenTypeBearer) {
		return authcommand.ChangePasswordCommand{}, contracterrors.TokenInvalidError(messages.MissingSession)
	}
	if newPassword == "" {
		return authcommand.ChangePasswordCommand{}, contracterrors.ValidationFailedError(messages.InvalidPassword)
	}
	return authcommand.ChangePasswordCommand{Token: token, NewPassword: newPassword}, nil
}
