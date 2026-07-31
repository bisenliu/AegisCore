package authhttp

// TokenResponse 是认证流程返回的 token 载荷。
type TokenResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOi..."`
	RefreshToken string `json:"refresh_token,omitempty" example:"eyJhbGciOi..."`
	TokenType    string `json:"token_type" example:"Bearer"`
	ExpiresIn    int64  `json:"expires_in" example:"900"`
}

// LogoutResponse 表示登出操作是否完成。
type LogoutResponse struct {
	LoggedOut bool `json:"logged_out" example:"true"`
}

// ForceChangePasswordResponse 表示强制改密操作是否完成。
type ForceChangePasswordResponse struct {
	Changed bool `json:"changed" example:"true"`
}
