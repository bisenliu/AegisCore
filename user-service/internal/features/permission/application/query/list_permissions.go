package query

import (
	"context"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/aegiscore/common/runtime/logger"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	"github.com/aegiscore/user-service/internal/features/permission/application/validators"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

// ListPermissionsQuery 包含权限列表查询使用的规范化分页和过滤条件。
type ListPermissionsQuery struct {
	Cursor     *uuid.UUID
	PageSize   int
	Limit      int
	Module     string
	HTTPMethod string
}

// ListPermissionsResult 是权限列表查询用例的 transport-neutral 分页输出。
type ListPermissionsResult struct {
	Items      []permissiondomain.Permission
	PageSize   int
	NextCursor string
	HasNext    bool
}

// ListPermissions 使用规范化过滤条件返回分页权限目录列表。
func (s *permissionQueryService) ListPermissions(ctx context.Context, query ListPermissionsQuery) (*ListPermissionsResult, error) {
	method, err := validators.NormalizeOptionalHTTPMethod(query.HTTPMethod)
	if err != nil {
		return nil, err
	}
	items, hasNext, err := s.store.List(ctx, permissionapplication.ListPermissionsInput{AfterPermissionID: query.Cursor, Limit: query.Limit, Module: validators.NormalizeOptionalModule(query.Module), HTTPMethod: method})
	if err != nil {
		logger.Error(ctx, "list permissions failed", logger.StackTrace(zap.Error(err))...)
		return nil, err
	}
	nextCursor := ""
	if hasNext && len(items) > 0 {
		nextCursor = items[len(items)-1].PermissionID.String()
	}
	return &ListPermissionsResult{Items: items, PageSize: query.PageSize, NextCursor: nextCursor, HasNext: hasNext}, nil
}
