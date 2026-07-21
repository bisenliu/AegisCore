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

func TestPermissionQueryServiceListPermissionsNormalizesFilters(t *testing.T) {
	firstID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	lastID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	store := NewMockPermissionStore(gomock.NewController(t))
	var listInput permissionapplication.ListPermissionsInput
	store.EXPECT().List(gomock.Any(), gomock.Any()).DoAndReturn(func(_ context.Context, input permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, error) {
		listInput = input
		return []permissiondomain.Permission{{PermissionID: firstID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"}, {PermissionID: lastID, Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users"}}, nil
	})
	service := NewPermissionQueryService(store)

	result, err := service.ListPermissions(context.Background(), ListPermissionsQuery{Module: "  user  ", HTTPMethod: "post"})
	require.NoError(t, err)
	require.Equal(t, "user", listInput.Module)
	require.Equal(t, "POST", listInput.HTTPMethod)
	require.Len(t, result.Items, 2)
	require.Equal(t, firstID, result.Items[0].PermissionID)
	require.Equal(t, lastID, result.Items[1].PermissionID)
}

func TestPermissionQueryServiceListPermissionsRejectsInvalidHTTPMethod(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	service := NewPermissionQueryService(store)

	_, err := service.ListPermissions(context.Background(), ListPermissionsQuery{HTTPMethod: "not-a-method"})
	require.Error(t, err)
	require.ErrorIs(t, err, permissiondomain.ErrPermissionInvalid)
}

func TestPermissionQueryServiceEffectivePermissionsPassThrough(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListEffectiveByUserID(gomock.Any(), userID).Return([]permissiondomain.Permission{{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users"}}, nil)
	service := NewPermissionQueryService(store)

	effectiveResult, err := service.ListUserEffectivePermissions(context.Background(), UserEffectivePermissionsQuery{UserID: userID})
	require.NoError(t, err)
	require.Len(t, effectiveResult.Items, 1)
	require.Equal(t, permissionID, effectiveResult.Items[0].PermissionID)
}
