package dto

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email,max=255" label:"邮箱" example:"alice@example.com"`
	Password string `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,min=1" label:"Refresh Token（首选裸 token，兼容 Bearer 前缀）" example:"eyJhbGciOi..."`
}

type TokenResponse struct {
	AccessToken  string `json:"access_token" example:"eyJhbGciOi..."`
	RefreshToken string `json:"refresh_token,omitempty" example:"eyJhbGciOi..."`
	TokenType    string `json:"token_type" example:"Bearer"`
	ExpiresIn    int64  `json:"expires_in" example:"900"`
}

type LogoutResponse struct {
	LoggedOut bool `json:"logged_out" example:"true"`
}
