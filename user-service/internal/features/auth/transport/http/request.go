package authhttp

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
	Token       string `json:"-" header:"Authorization" label:"Authorization"`
	NewPassword string `json:"new_password" validate:"required,min=8,max=256" label:"新密码" example:"new-secret"`
}
