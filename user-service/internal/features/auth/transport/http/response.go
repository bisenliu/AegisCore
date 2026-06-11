package authhttp

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
