package query

import (
	"context"

	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/features/permission/application/validators"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// ListPermissionsQuery 包含权限列表查询使用的过滤条件。
type ListPermissionsQuery struct {
	Module     string
	HTTPMethod string
}

// ListPermissionsResult 是权限列表查询用例的 transport-neutral 输出。
type ListPermissionsResult struct {
	Items []permissiondomain.Permission
}

// ListPermissions 使用规范化过滤条件返回完整权限目录列表。
func (s *permissionQueryService) ListPermissions(ctx context.Context, query ListPermissionsQuery) (*ListPermissionsResult, error) {
	method, err := validators.NormalizeOptionalHTTPMethod(query.HTTPMethod)
	if err != nil {
		return nil, err
	}
	items, err := s.store.List(ctx, permissionapplication.ListPermissionsInput{Module: validators.NormalizeOptionalModule(query.Module), HTTPMethod: method})
	if err != nil {
		logger.Error(ctx, "list permissions failed", logger.StackTrace(zap.Error(err))...)
		return nil, err
	}
	return &ListPermissionsResult{Items: items}, nil
}
