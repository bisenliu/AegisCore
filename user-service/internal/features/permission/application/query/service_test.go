package query

import (
	"context"
	"testing"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionQueryServiceListPermissionsNormalizesFiltersAndCursor(t *testing.T) {
	firstID := uuid.MustParse("018f0000-0000-7000-8000-000000000201")
	lastID := uuid.MustParse("018f0000-0000-7000-8000-000000000202")
	store := &queryPermissionStore{permissions: []permissiondomain.Permission{{PermissionID: firstID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, {PermissionID: lastID, Module: "user", HTTPMethod: "POST", PathTemplate: "/api/v1/users", Active: true}}, hasNext: true}
	service := NewPermissionQueryService(store, &stubRouteScanner{})

	result, err := service.ListPermissions(context.Background(), ListPermissionsQuery{PageSize: 20, Limit: 10, Module: "  user  ", HTTPMethod: "post"})
	if err != nil {
		t.Fatalf("ListPermissions: %v", err)
	}
	if store.listInput.Module != "user" || store.listInput.HTTPMethod != "POST" || store.listInput.Limit != 10 {
		t.Fatalf("list input = %#v", store.listInput)
	}
	if !result.HasNext || result.NextCursor != lastID.String() || result.PageSize != 20 {
		t.Fatalf("result = %#v", result)
	}
}

func TestPermissionQueryServiceGetAndEffectivePermissionsPassThrough(t *testing.T) {
	permissionID := uuid.MustParse("018f0000-0000-7000-8000-000000000203")
	userID := uuid.MustParse("018f0000-0000-7000-8000-000000000204")
	store := &queryPermissionStore{permission: permissiondomain.Permission{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}, permissions: []permissiondomain.Permission{{PermissionID: permissionID, Module: "user", HTTPMethod: "GET", PathTemplate: "/api/v1/users", Active: true}}}
	service := NewPermissionQueryService(store, &stubRouteScanner{})

	permissionResult, err := service.GetPermission(context.Background(), GetPermissionQuery{PermissionID: permissionID})
	if err != nil {
		t.Fatalf("GetPermission: %v", err)
	}
	if permissionResult.Permission.PermissionID != permissionID || store.getPermissionID != permissionID {
		t.Fatalf("permission result = %#v get id = %s", permissionResult.Permission, store.getPermissionID)
	}

	effectiveResult, err := service.ListUserEffectivePermissions(context.Background(), UserEffectivePermissionsQuery{UserID: userID})
	if err != nil {
		t.Fatalf("ListUserEffectivePermissions: %v", err)
	}
	if len(effectiveResult.Items) != 1 || effectiveResult.Items[0].PermissionID != permissionID || store.effectiveUserID != userID {
		t.Fatalf("effective result = %#v user id = %s", effectiveResult.Items, store.effectiveUserID)
	}
}

type queryPermissionStore struct {
	permission      permissiondomain.Permission
	permissions     []permissiondomain.Permission
	hasNext         bool
	listInput       permissionapplication.ListPermissionsInput
	getPermissionID uuid.UUID
	effectiveUserID uuid.UUID
}

func (s *queryPermissionStore) Create(context.Context, permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
	return nil, nil
}

func (s *queryPermissionStore) GetByPermissionID(_ context.Context, permissionID uuid.UUID) (*permissiondomain.Permission, error) {
	s.getPermissionID = permissionID
	return &s.permission, nil
}

func (s *queryPermissionStore) List(_ context.Context, input permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, bool, error) {
	s.listInput = input
	return s.permissions, s.hasNext, nil
}

func (s *queryPermissionStore) ListAll(context.Context) ([]permissiondomain.Permission, error) {
	return s.permissions, nil
}

func (s *queryPermissionStore) ListEffectiveByUserID(_ context.Context, userID uuid.UUID) ([]permissiondomain.Permission, error) {
	s.effectiveUserID = userID
	return s.permissions, nil
}

func (s *queryPermissionStore) Update(context.Context, permissionapplication.UpdatePermissionInput) (*permissiondomain.Permission, error) {
	return nil, nil
}

func (s *queryPermissionStore) SetActive(context.Context, uuid.UUID, bool) (*permissiondomain.Permission, error) {
	return nil, nil
}
