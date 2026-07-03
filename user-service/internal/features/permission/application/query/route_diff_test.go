package query

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

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
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.MissingInPermissions, 1)
	assert.Equal(t, "POST", result.MissingInPermissions[0].Method)
	assert.Equal(t, "/api/v1/users", result.MissingInPermissions[0].Path)
	require.Len(t, result.StalePermissions, 1)
	assert.Equal(t, "/api/v1/stale", result.StalePermissions[0].PathTemplate)
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
	require.NoError(t, err)
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
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Len(t, result.MissingInPermissions, 2)
	assert.Equal(t, "DELETE", result.MissingInPermissions[0].Method)
	assert.Equal(t, "/api/v1/users/:user_id", result.MissingInPermissions[0].Path)
	assert.Equal(t, "POST", result.MissingInPermissions[1].Method)
	assert.Equal(t, "/api/v1/users", result.MissingInPermissions[1].Path)
	require.Len(t, result.StalePermissions, 2)
	assert.Equal(t, "/api/v1/stale-b", result.StalePermissions[0].PathTemplate)
	assert.Equal(t, "/api/v1/stale-a", result.StalePermissions[1].PathTemplate)
}

func TestRouteDiffPropagatesScannerAndStoreErrors(t *testing.T) {
	scannerErr := errors.New("scan failed")
	metrics := NewMockMetrics(gomock.NewController(t))
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return(nil, scannerErr)
	service := NewPermissionQueryService(NewMockPermissionStore(gomock.NewController(t)), scanner, metrics)
	_, err := service.GetRouteDiff(context.Background())
	require.ErrorIs(t, err, scannerErr)

	storeErr := errors.New("list all failed")
	scanner = NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return(nil, nil)
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return(nil, storeErr)
	service = NewPermissionQueryService(store, scanner, metrics)
	_, err = service.GetRouteDiff(context.Background())
	require.ErrorIs(t, err, storeErr)
}

func TestRouteDiffRejectsInvalidDiscoveredRoute(t *testing.T) {
	scanner := NewMockRouteCatalogScanner(gomock.NewController(t))
	scanner.EXPECT().DiscoverRoutes(gomock.Any()).Return([]permissionapplication.DiscoveredRoute{{Method: "", Path: "/api/v1/users"}}, nil)
	store := NewMockPermissionStore(gomock.NewController(t))
	store.EXPECT().ListAll(gomock.Any()).Return(nil, nil)
	service := NewPermissionQueryService(store, scanner)
	_, err := service.GetRouteDiff(context.Background())
	require.Error(t, err)
}
