package validators

import (
	"strings"

	platformauth "github.com/aegiscore/platform/security/auth"
	"github.com/aegiscore/platform/security/password"
	authdomain "github.com/aegiscore/user-service/internal/features/auth/domain"
)

// ValidateLoginCommand 校验 transport-neutral 登录输入。
func ValidateLoginCommand(username string, plainPassword string) error {
	if strings.TrimSpace(username) == "" || strings.TrimSpace(plainPassword) == "" {
		return authdomain.ErrInvalidCredentials
	}
	return nil
}

// ValidateRefreshToken 校验 refresh token 输入是否携带了有效 token 值。
func ValidateRefreshToken(token string) error {
	if isBlankToken(token) {
		return authdomain.ErrTokenInvalid
	}
	return nil
}

// ValidateForceChangePasswordCommand 校验强制改密输入。
func ValidateForceChangePasswordCommand(token string, newPassword string) error {
	if isBlankToken(token) {
		return authdomain.ErrTokenInvalid
	}
	if strings.TrimSpace(newPassword) == "" {
		return password.ErrEmptyPassword
	}
	return nil
}

// isBlankToken 检查传入的凭证文本是否为空或全由空格组成。
func isBlankToken(token string) bool {
	token = platformauth.StripBearerPrefix(token)
	return token == "" || strings.EqualFold(token, platformauth.TokenTypeBearer)
}
