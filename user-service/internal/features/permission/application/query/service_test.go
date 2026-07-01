package query

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

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
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	if listInput.Module != "user" || listInput.HTTPMethod != "POST" || listInput.Limit != 10 {
		t.Fatalf("list input = %#v", listInput)
	}
	if !result.HasNext || result.NextCursor != lastID.String() || result.PageSize != 20 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPermissionQueryServiceGetAndEffectivePermissionsPassThrough(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().GetByPermissionID(gomock.Any(), permissionID).Return(&permissiondomain.Permission{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, nil)
	store.EXPECT().ListEffectiveByUserID(gomock.Any(), userID).Return([]permissiondomain.Permission{{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}, nil)
	service := NewPermissionQueryService(store, NewMockRouteCatalogScanner(gomock.NewController(t)))

	permissionResult, err := service.GetPermission(context.Background(), GetPermissionQuery{PermissionID: permissionID})
	if err != nil {
		t.Fatalf("GetPermission: %v", err)
	}
	if permissionResult.Permission.PermissionID != permissionID {
		t.Fatalf("permission result = %#v", permissionResult.Permission)
	}

	effectiveResult, err := service.ListUserEffectivePermissions(context.Background(), UserEffectivePermissionsQuery{UserID: userID})
	if err != nil {
		t.Fatalf("ListUserEffectivePermissions: %v", err)
	}
	if len(effectiveResult.Items) != 1 || effectiveResult.Items[0].PermissionID != permissionID {
		t.Fatalf("effective result = %#v", effectiveResult.Items)
	}
}
