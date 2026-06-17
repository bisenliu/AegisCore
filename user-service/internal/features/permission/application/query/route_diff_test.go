package query

import (
	"context"
	"errors"
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

func TestRouteDiffRecordsMetrics(t *testing.T) {
	store := &stubPermissionStore{permissions: []permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000021"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000022"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"},
	}}
	scanner := &stubRouteScanner{routes: []permissionapplication.DiscoveredRoute{
		{Method: "GET", Path: "/api/v1/users"},
		{Method: "POST", Path: "/api/v1/users"},
	}}
	metrics := &routeDiffMetricsSpy{}
	service := NewPermissionQueryService(store, scanner, metrics)

	_, err := service.GetRouteDiff(context.Background())
	if err != nil {
		t.Fatalf("GetRouteDiff: %v", err)
	}
	if metrics.missing != 1 || metrics.stale != 1 {
		t.Fatalf("route diff metrics missing=%d stale=%d, want 1/1", metrics.missing, metrics.stale)
	}
}

func TestRouteDiffNormalizesSortsAndStaysReadOnly(t *testing.T) {
	store := &stubPermissionStore{permissions: []permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000011"), HTTPMethod: "DELETE", PathTemplate: "/api/v1/stale-b"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000012"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000013"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale-a"},
	}}
	scanner := &stubRouteScanner{routes: []permissionapplication.DiscoveredRoute{
		{Method: "post", Path: "/api/v1/users"},
		{Method: "get", Path: "/api/v1/users"},
		{Method: "delete", Path: "/api/v1/users/:user_id"},
	}}
	service := NewPermissionQueryService(store, scanner)

	result, err := service.GetRouteDiff(context.Background())
	if err != nil {
		t.Fatalf("GetRouteDiff: %v", err)
	}
	if got := result.MissingInPermissions; len(got) != 2 || got[0].Method != "DELETE" || got[0].Path != "/api/v1/users/:user_id" || got[1].Method != "POST" || got[1].Path != "/api/v1/users" {
		t.Fatalf("missing = %#v", got)
	}
	if got := result.StalePermissions; len(got) != 2 || got[0].PathTemplate != "/api/v1/stale-b" || got[1].PathTemplate != "/api/v1/stale-a" {
		t.Fatalf("stale = %#v", got)
	}
	if store.createCalled || store.updateCalled || store.setActiveCalled {
		t.Fatalf("route diff mutated permissions")
	}
}

func TestRouteDiffPropagatesScannerAndStoreErrors(t *testing.T) {
	scannerErr := errors.New("scan failed")
	metrics := &routeDiffMetricsSpy{}
	service := NewPermissionQueryService(&stubPermissionStore{}, &stubRouteScanner{err: scannerErr}, metrics)
	if _, err := service.GetRouteDiff(context.Background()); !errors.Is(err, scannerErr) {
		t.Fatalf("scanner err = %v", err)
	}
	if metrics.called {
		t.Fatal("metrics updated after scanner error")
	}

	storeErr := errors.New("list all failed")
	service = NewPermissionQueryService(&stubPermissionStore{listAllErr: storeErr}, &stubRouteScanner{}, metrics)
	if _, err := service.GetRouteDiff(context.Background()); !errors.Is(err, storeErr) {
		t.Fatalf("store err = %v", err)
	}
	if metrics.called {
		t.Fatal("metrics updated after store error")
	}
}

func TestRouteDiffRejectsInvalidDiscoveredRoute(t *testing.T) {
	service := NewPermissionQueryService(&stubPermissionStore{}, &stubRouteScanner{routes: []permissionapplication.DiscoveredRoute{{Method: "", Path: "/api/v1/users"}}})
	if _, err := service.GetRouteDiff(context.Background()); err == nil {
		t.Fatalf("err is nil for invalid discovered route")
	}
}

type stubPermissionStore struct {
	permissions     []permissiondomain.Permission
	listAllErr      error
	createCalled    bool
	updateCalled    bool
	setActiveCalled bool
}

func (s *stubPermissionStore) Create(context.Context, permissionapplication.CreatePermissionInput) (*permissiondomain.Permission, error) {
	s.createCalled = true
	return nil, nil
}
func (s *stubPermissionStore) GetByPermissionID(context.Context, uuid.UUID) (*permissiondomain.Permission, error) {
	return nil, nil
}
func (s *stubPermissionStore) List(context.Context, permissionapplication.ListPermissionsInput) ([]permissiondomain.Permission, bool, error) {
	return s.permissions, false, nil
}
func (s *stubPermissionStore) ListAll(context.Context) ([]permissiondomain.Permission, error) {
	if s.listAllErr != nil {
		return nil, s.listAllErr
	}
	return s.permissions, nil
}
func (s *stubPermissionStore) ListEffectiveByUserID(context.Context, uuid.UUID) ([]permissiondomain.Permission, error) {
	return s.permissions, nil
}
func (s *stubPermissionStore) Update(context.Context, permissionapplication.UpdatePermissionInput) (*permissiondomain.Permission, error) {
	s.updateCalled = true
	return nil, nil
}
func (s *stubPermissionStore) SetActive(context.Context, uuid.UUID, bool) (*permissiondomain.Permission, error) {
	s.setActiveCalled = true
	return nil, nil
}

type stubRouteScanner struct {
	routes []permissionapplication.DiscoveredRoute
	err    error
}

func (s *stubRouteScanner) DiscoverRoutes(context.Context) ([]permissionapplication.DiscoveredRoute, error) {
	if s.err != nil {
		return nil, s.err
	}
	return s.routes, nil
}

type routeDiffMetricsSpy struct {
	permissionapplication.Metrics
	called  bool
	missing int
	stale   int
}

func (m *routeDiffMetricsSpy) RouteDiffObserved(_ context.Context, missing int, stale int) {
	m.called = true
	m.missing = missing
	m.stale = stale
}
