package authhttp

import (
	contracterrors "github.com/aegiscore/platform/contract/errors"
	contractresponse "github.com/aegiscore/platform/contract/response"
	authcommand "github.com/aegiscore/user-service/internal/features/auth/application/command"
	authtokens "github.com/aegiscore/user-service/internal/features/auth/application/tokens"
	"github.com/aegiscore/user-service/internal/messages"
)

// toTokenResponse 将签发成功的通行证信息打包成接口返回的数据格式。
func toTokenResponse(result *authtokens.TokenResult) TokenResponse {
	return TokenResponse{
		AccessToken:  result.AccessToken,
		RefreshToken: result.RefreshToken,
		TokenType:    result.TokenType,
		ExpiresIn:    result.ExpiresIn,
	}
}

// toPasswordChangeRequiredEnvelope 当用户被要求必须修改密码时，包装专属的拦截提示并下发临时改密凭证。
func toPasswordChangeRequiredEnvelope(result *authtokens.TokenResult) contractresponse.Envelope {
	return contractresponse.Envelope{
		Success: false,
		Code:    contracterrors.CodePasswordChangeRequired,
		Message: messages.PasswordChangeRequired,
		Data:    toTokenResponse(result),
	}
}

// toForceChangePasswordResponse 将密码修改成功的结果包装为对外返回的数据格式。
func toForceChangePasswordResponse(result *authcommand.ForceChangePasswordResult) ForceChangePasswordResponse {
	return ForceChangePasswordResponse{Changed: result.Changed}
}

// toLogoutResponse 将退出登录的结果包装为接口响应。
func toLogoutResponse(result *authcommand.LogoutResult) LogoutResponse {
	return LogoutResponse{LoggedOut: result.LoggedOut}
}
