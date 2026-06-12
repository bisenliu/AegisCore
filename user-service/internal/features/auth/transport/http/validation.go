package authhttp

import (
	"strings"

	contracterrors "github.com/aegiscore/common/contract/errors"
	commonauth "github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-service/internal/messages"
)

// NormalizeLogin 裁剪凭证，并将空登录字段映射为 unauthenticated 错误。
func NormalizeLogin(req *LoginRequest) error {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		return contracterrors.UnauthenticatedError(messages.InvalidCredentials)
	}
	return nil
}

// NormalizeChangePassword 规范化受限 token，并校验新密码字段。
func NormalizeChangePassword(req *ChangePasswordRequest) error {
	req.Token = commonauth.StripBearerPrefix(req.Token)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.Token == "" || strings.EqualFold(req.Token, commonauth.TokenTypeBearer) {
		return contracterrors.TokenInvalidError(messages.MissingSession)
	}
	if req.NewPassword == "" {
		return contracterrors.ValidationFailedError(messages.InvalidPassword)
	}
	return nil
}

// NormalizeRefresh 规范化 refresh token 输入，并拒绝空值或仅 Bearer 的值。
func NormalizeRefresh(req *RefreshTokenRequest) error {
	req.RefreshToken = commonauth.StripBearerPrefix(req.RefreshToken)
	if req.RefreshToken == "" || strings.EqualFold(req.RefreshToken, commonauth.TokenTypeBearer) {
		return contracterrors.TokenInvalidError(messages.MissingSession)
	}
	return nil
}
