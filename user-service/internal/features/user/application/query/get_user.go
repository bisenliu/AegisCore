package query

import (
	"context"
	"errors"

	"github.com/aegiscore/common/runtime/logger"
	userdomain "github.com/aegiscore/user-service/internal/features/user/domain"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

// GetUserByIDQuery 包含按外部用户 ID 查询用户资料所需的应用层输入。
type GetUserByIDQuery struct {
	UserID uuid.UUID
}

// GetUserResult 是按 ID 查询用户用例的 transport-neutral 输出。
type GetUserResult struct {
	User userdomain.User
}

// GetUserByID 按外部 UUID 返回公开用户资料。
func (s *userQueryService) GetUserByID(ctx context.Context, query GetUserByIDQuery) (*GetUserResult, error) {
	logger.Info(ctx, "query user profile", zap.String("user_id", query.UserID.String()))
	user, err := s.store.GetByUserID(ctx, query.UserID)
	if err != nil {
		if errors.Is(err, userdomain.ErrUserNotFound) {
			logger.Warn(ctx, "query user profile not found", zap.String("user_id", query.UserID.String()))
			return nil, userdomain.ErrUserNotFound
		}
		logger.Error(ctx, "query user profile failed", logger.StackTrace(zap.String("user_id", query.UserID.String()), zap.Error(err))...)
		return nil, err
	}
	return &GetUserResult{User: *user}, nil
}
