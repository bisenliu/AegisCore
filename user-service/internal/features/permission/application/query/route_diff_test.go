package query

import (
	"context"
	"testing"

	"github.com/google/uuid"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestRouteDiff(t *testing.T) {
	store := &stubPermissionStore{permissions: []permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"},
	}}
	scanner := &stubRouteScanner{routes: []permissionapplication.DiscoveredRoute{
		{Method: "GET", Path: "/api/v1/users"},
		{Method: "POST", Path: "/api/v1/users"},
	}}
	service := NewPermissionQueryService(store, scanner)

	result, err := service.GetRouteDiff(context.Background())
	if err != nil {
		t.Fatalf("GetRouteDiff: %v", err)
	}
	if len(result.MissingInPermissions) != 1 || result.MissingInPermissions[0].Method != "POST" || result.MissingInPermissions[0].Path != "/api/v1/users" {
		t.Fatalf("missing = %#v", result.MissingInPermissions)
	}
	if len(result.StalePermissions) != 1 || result.StalePermissions[0].PathTemplate != "/api/v1/stale" {
		t.Fatalf("stale = %#v", result.StalePermissions)
	}
}

type stubPermissionStore struct {
	permissions []permissiondomain.Permission
}

func (s *stubPermissionStore) Create(context.Context, permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
	return nil, nil
}
func (s *stubPermissionStore) GetByPermissionID(context.Context, uuid.UUID) (*permissiondomain.Permission, error) {
	return nil, nil
}
func (s *stubPermissionStore) List(context.Context, permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, bool, error) {
	return s.permissions, false, nil
}
func (s *stubPermissionStore) ListAll(context.Context) ([]permissiondomain.Permission, error) {
	return s.permissions, nil
}
func (s *stubPermissionStore) ListEffectiveByUserID(context.Context, uuid.UUID) ([]permissiondomain.Permission, error) {
	return s.permissions, nil
}
func (s *stubPermissionStore) Update(context.Context, permissionapplication.UpdatePermissionInput) (*permissiondomain.Permission, error) {
	return nil, nil
}
func (s *stubPermissionStore) SetActive(context.Context, uuid.UUID, bool) (*permissiondomain.Permission, error) {
	return nil, nil
}

type stubRouteScanner struct {
	routes []permissionapplication.DiscoveredRoute
}

func (s *stubRouteScanner) DiscoverRoutes(context.Context) ([]permissionapplication.DiscoveredRoute, error) {
	return s.routes, nil
}
