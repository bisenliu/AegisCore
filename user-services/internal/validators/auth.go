package validators

import (
	"strings"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/api/auth"
	"github.com/aegiscore/user-services/internal/messages"
)

// NormalizeLogin 裁剪凭证，并将空登录字段映射为 unauthenticated 错误。
func NormalizeLogin(req *authapi.LoginRequest) error {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		return response.UnauthenticatedError(messages.InvalidCredentials)
	}
	return nil
}

// NormalizeChangePassword 规范化受限 token，并校验新密码字段。
func NormalizeChangePassword(req *authapi.ChangePasswordRequest) error {
	req.Token = auth.StripBearerPrefix(req.Token)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.Token == "" || strings.EqualFold(req.Token, auth.TokenTypeBearer) {
		return response.TokenInvalidError(messages.MissingSession)
	}
	if req.NewPassword == "" {
		return response.ValidationFailedError(messages.InvalidPassword)
	}
	return nil
}

// NormalizeRefresh 规范化 refresh token 输入，并拒绝空值或仅 Bearer 的值。
func NormalizeRefresh(req *authapi.RefreshTokenRequest) error {
	req.RefreshToken = auth.StripBearerPrefix(req.RefreshToken)
	if req.RefreshToken == "" || strings.EqualFold(req.RefreshToken, auth.TokenTypeBearer) {
		return response.TokenInvalidError(messages.MissingSession)
	}
	return nil
}
