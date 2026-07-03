package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/require"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionQueryServiceListPermissionsNormalizesFiltersAndCursor(t *testing.T) {
	firstID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	lastID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	store := NewMockPermissionStore(gomock.NewController(t))
	var listInput permissionapplication.ListPermissionsInput
	store.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, bool, error) {
		listInput = input
		return []permissiondomain.Permission{{PermissionID: firstID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, {PermissionID: lastID, Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true}}, true, nil
	})
	service := NewPermissionQueryService(store, NewMockRouteCatalogScanner(gomock.NewController(t)))

	result, err := service.ListPermissions(context.Background(), ListPermissionsQuery{PageSize: 20, Limit: 10, Module: "  user  ", HTTPMethod: "post"})
	require.NoError(t, err)
	require.Equal(t, "user", listInput.Module)
	require.Equal(t, "POST", listInput.HTTPMethod)
	require.Equal(t, 10, listInput.Limit)
	require.True(t, result.HasNext)
	require.Equal(t, lastID.String(), result.NextCursor)
	require.Equal(t, 20, result.PageSize)
}

func TestPermissionQueryServiceGetAndEffectivePermissionsPassThrough(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	store.EXPECT().ListEffectiveByUserID(gomock.Any(), userID).Return([]permissiondomain.Permission{{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}, nil)
	service := NewPermissionQueryService(store, NewMockRouteCatalogScanner(gomock.NewController(t)))

	permissionResult, err := service.GetPermission(context.Background(), GetPermissionQuery{PermissionID: permissionID})
	require.NoError(t, err)
	require.Equal(t, permissionID, permissionResult.Permission.PermissionID)

	effectiveResult, err := service.ListUserEffectivePermissions(context.Background(), UserEffectivePermissionsQuery{UserID: userID})
	require.NoError(t, err)
	require.Len(t, effectiveResult.Items, 1)
	require.Equal(t, permissionID, effectiveResult.Items[0].PermissionID)
}
