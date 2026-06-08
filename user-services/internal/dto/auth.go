package dto

// LoginRequest 是用户名密码认证的 JSON 请求体。
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=1,max=255" label:"用户名" example:"alice"`
	Password string `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
}

// RefreshTokenRequest 是换取 refresh token 的 JSON 请求体，兼容裸 token 或 Bearer 值。
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,min=1" label:"Refresh Token（首选裸 token，兼容 Bearer 前缀）" example:"eyJhbGciOi..."`
}

// ChangePasswordRequest 是使用 Authorization token 完成强制改密的请求。
type ChangePasswordRequest struct {
	Token       string `json:"-"`
	NewPassword string `json:"new_password" validate:"required,min=1" label:"新密码" example:"new-secret"`
}

// TokenResponse 是登录和刷新流程返回的认证 token 载荷。
type TokenResponse struct {
	AccessToken            string `json:"access_token" example:"eyJhbGciOi..."`
	RefreshToken           string `json:"refresh_token,omitempty" example:"eyJhbGciOi..."`
	TokenType              string `json:"token_type" example:"Bearer"`
	ExpiresIn              int64  `json:"expires_in" example:"900"`
	PasswordChangeRequired bool   `json:"password_change_required,omitempty" example:"true"`
}

// LogoutResponse 表示登出操作是否完成。
type LogoutResponse struct {
	LoggedOut bool `json:"logged_out" example:"true"`
}

// ChangePasswordResponse 表示改密操作是否完成。
type ChangePasswordResponse struct {
	Changed bool `json:"changed" example:"true"`
}
