package domain

import "github.com/google/uuid"

type User struct {
	ID           int64
	UserID       uuid.UUID
	Nickname     string
	Username     string
	PasswordHash string
	Status       UserStatus
	TokenVersion int64
	CreatedAt    int64
	UpdatedAt    int64
}

func (u User) CanLogin() bool {
	return u.Status.CanLogin()
}

func (u User) RequiresPasswordChange() bool {
	return u.Status == UserStatusMustChangePassword
}

func (u User) CanChangePassword() bool {
	return u.RequiresPasswordChange()
}
