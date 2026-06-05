package validation

import (
	"strings"

	"github.com/aegiscore/common/contract/response"
	"github.com/aegiscore/common/security/auth"
	"github.com/aegiscore/user-services/internal/dto"
	"github.com/aegiscore/user-services/internal/messages"
	"github.com/google/uuid"
)

func NormalizeCreateUser(req *dto.CreateUserRequest) error {
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Username = strings.ToLower(strings.TrimSpace(req.Username))
	req.Password = strings.TrimSpace(req.Password)
	if req.Nickname == "" || req.Username == "" {
		return response.ValidationFailedError(messages.InvalidUsername)
	}
	if req.Password == "" {
		return response.ValidationFailedError(messages.InvalidPassword)
	}
	return nil
}

func NormalizeListUsers(req *dto.ListUsersRequest) {
	paging := response.NormalizePagination(req.Page, req.PageSize)
	req.Page = paging.Page
	req.PageSize = paging.PageSize
	req.Offset = paging.Offset
	req.Limit = paging.Limit
	req.Nickname = strings.TrimSpace(req.Nickname)
	req.Username = strings.TrimSpace(req.Username)
}

func ParseUserID(req dto.GetUserRequest) (uuid.UUID, error) {
	userID, err := uuid.Parse(req.UserID)
	if err != nil {
		return uuid.Nil, response.BadRequestError(messages.InvalidUserID)
	}
	return userID, nil
}

func NormalizeLogin(req *dto.LoginRequest) error {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		return response.UnauthenticatedError(messages.InvalidCredentials)
	}
	return nil
}

func NormalizeChangePassword(req *dto.ChangePasswordRequest) error {
	req.Token = auth.StripBearerPrefix(req.Token)
	req.NewPassword = strings.TrimSpace(req.NewPassword)
	if req.Token == "" || strings.EqualFold(req.Token, auth.TokenTypeBearer) {
		return response.TokenInvalidError(messages.MissingSession)
	}
	if req.NewPassword == "" {
		return response.ValidationFailedError(messages.InvalidPassword)
	}
	return nil
}

func NormalizeRefresh(req *dto.RefreshTokenRequest) error {
	req.RefreshToken = auth.StripBearerPrefix(req.RefreshToken)
	if req.RefreshToken == "" || strings.EqualFold(req.RefreshToken, auth.TokenTypeBearer) {
		return response.TokenInvalidError(messages.MissingSession)
	}
	return nil
}
