package query

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestRouteDiff(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return([]permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000002"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"},
	}, nil)
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return([]permissionapplication.DiscoveredRoute{
		{Method: "GET", Path: "/api/v1/users"},
		{Method: "POST", Path: "/api/v1/users"},
	}, nil)
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
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return([]permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000021"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000022"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"},
	}, nil)
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return([]permissionapplication.DiscoveredRoute{
		{Method: "GET", Path: "/api/v1/users"},
		{Method: "POST", Path: "/api/v1/users"},
	}, nil)
	metrics := NewMockMetrics(gomock.NewController(t))
	metrics.EXPECT().RouteDiffObserved(gomock.Any(), 1, 1)
	service := NewPermissionQueryService(store, scanner, metrics)

	_, err := service.GetRouteDiff(context.Background())
	if err != nil {
		t.Fatalf("GetRouteDiff: %v", err)
	}
}

func TestRouteDiffNormalizesSortsAndStaysReadOnly(t *testing.T) {
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return([]permissiondomain.Permission{
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000011"), HTTPMethod: "DELETE", PathTemplate: "/api/v1/stale-b"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000012"), HTTPMethod: "GET", PathTemplate: "/api/v1/users"},
		{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000013"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale-a"},
	}, nil)
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return([]permissionapplication.DiscoveredRoute{
		{Method: "post", Path: "/api/v1/users"},
		{Method: "get", Path: "/api/v1/users"},
		{Method: "delete", Path: "/api/v1/users/:user_id"},
	}, nil)
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
}

func TestRouteDiffPropagatesScannerAndStoreErrors(t *testing.T) {
	scannerErr := errors.New("scan failed")
	metrics := NewMockMetrics(gomock.NewController(t))
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return(nil, scannerErr)
	service := NewPermissionQueryService(NewMockPermissionStore(gomock.NewController(t)), scanner, metrics)
	if _, err := service.GetRouteDiff(context.Background()); !errors.Is(err, scannerErr) {
		t.Fatalf("scanner err = %v", err)
	}

	storeErr := errors.New("list all failed")
	scanner = NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return(nil, nil)
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return(nil, storeErr)
	service = NewPermissionQueryService(store, scanner, metrics)
	if _, err := service.GetRouteDiff(context.Background()); !errors.Is(err, storeErr) {
		t.Fatalf("store err = %v", err)
	}
}

func TestRouteDiffRejectsInvalidDiscoveredRoute(t *testing.T) {
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return([]permissionapplication.DiscoveredRoute{{Method: "", Path: "/api/v1/users"}}, nil)
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return(nil, nil)
	service := NewPermissionQueryService(store, scanner)
	if _, err := service.GetRouteDiff(context.Background()); err == nil {
		t.Fatalf("err is nil for invalid discovered route")
	}
}
