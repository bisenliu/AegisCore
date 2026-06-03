package dto

type LoginRequest struct {
	Username string `json:"username" validate:"required,min=1,max=255" label:"用户名" example:"alice"`
	Password string `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required,min=1" label:"Refresh Token（首选裸 token，兼容 Bearer 前缀）" example:"eyJhbGciOi..."`
}

type ChangePasswordRequest struct {
	Token       string `json:"-"`
	NewPassword string `json:"new_password" validate:"required,min=1" label:"新密码" example:"new-secret"`
}

type TokenResponse struct {
	AccessToken            string `json:"access_token" example:"eyJhbGciOi..."`
	RefreshToken           string `json:"refresh_token,omitempty" example:"eyJhbGciOi..."`
	TokenType              string `json:"token_type" example:"Bearer"`
	ExpiresIn              int64  `json:"expires_in" example:"900"`
	PasswordChangeRequired bool   `json:"password_change_required,omitempty" example:"true"`
}

type LogoutResponse struct {
	LoggedOut bool `json:"logged_out" example:"true"`
}

type ChangePasswordResponse struct {
	Changed bool `json:"changed" example:"true"`
}
