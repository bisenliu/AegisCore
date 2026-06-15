package permissionhttp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	commonvalidation "github.com/aegiscore/common/validation"
	permissioncommand "github.com/aegiscore/user-service/internal/features/permission/application/command"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionControllerGetRouteDiff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := commonvalidation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault validator: %v", err)
	}
	controller := NewPermissionController(&stubCommandService{}, &stubQueryService{}, validator)
	engine := gin.New()
	RegisterRoutes(engine.Group("/api/v1/permissions"), controller)

	recorder := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/permissions/route-diff", nil)
	engine.ServeHTTP(recorder, req)

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", recorder.Code, recorder.Body.String())
	}
	if !strings.Contains(recorder.Body.String(), "missing_in_permissions") || !strings.Contains(recorder.Body.String(), "stale_permissions") {
		t.Fatalf("body = %s", recorder.Body.String())
	}
}

type stubCommandService struct{}

func (s *stubCommandService) CreatePermission(context.Context, permissioncommand.CreatePermissionCommand) (*permissioncommand.PermissionResult, error) {
	return nil, nil
}
func (s *stubCommandService) UpdatePermission(context.Context, permissioncommand.UpdatePermissionCommand) (*permissioncommand.PermissionResult, error) {
	return nil, nil
}
func (s *stubCommandService) EnablePermission(context.Context, permissioncommand.SetPermissionActiveCommand) (*permissioncommand.PermissionResult, error) {
	return nil, nil
}
func (s *stubCommandService) DisablePermission(context.Context, permissioncommand.SetPermissionActiveCommand) (*permissioncommand.PermissionResult, error) {
	return nil, nil
}

type stubQueryService struct{}

func (s *stubQueryService) ListPermissions(context.Context, permissionquery.ListPermissionsQuery) (*permissionquery.ListPermissionsResult, error) {
	return nil, nil
}
func (s *stubQueryService) GetPermission(context.Context, permissionquery.GetPermissionQuery) (*permissionquery.PermissionResult, error) {
	return nil, nil
}
func (s *stubQueryService) ListUserEffectivePermissions(context.Context, permissionquery.UserEffectivePermissionsQuery) (*permissionquery.UserEffectivePermissionsResult, error) {
	return nil, nil
}
func (s *stubQueryService) GetRouteDiff(context.Context) (*permissionquery.RouteDiffResult, error) {
	return &permissionquery.RouteDiffResult{MissingInPermissions: nil, StalePermissions: []permissiondomain.Permission{{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"}}}, nil
}
