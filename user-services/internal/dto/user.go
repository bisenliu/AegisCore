package dto

type GetUserRequest struct {
	ID int64 `uri:"id" validate:"required,gt=0" label:"用户ID" example:"123"`
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=1,max=128" label:"用户名" example:"Alice"`
	Email    string `json:"email" validate:"required,email,max=255" label:"邮箱" example:"alice@example.com"`
	Password string `json:"password" validate:"required,min=1" label:"密码" example:"secret"`
	Active   *bool  `json:"active,omitempty" validate:"omitempty" label:"是否启用" example:"true"`
}

func (r *CreateUserRequest) SetDefaults() {
	if r.Active == nil {
		active := true
		r.Active = &active
	}
}

type UserResponse struct {
	ID        int64  `json:"id" example:"123"`
	Name      string `json:"name" example:"Alice"`
	Email     string `json:"email" example:"alice@example.com"`
	Active    bool   `json:"active" example:"true"`
	CreatedAt int64  `json:"created_at" example:"1780288800000"`
	UpdatedAt int64  `json:"updated_at" example:"1780288800000"`
}
