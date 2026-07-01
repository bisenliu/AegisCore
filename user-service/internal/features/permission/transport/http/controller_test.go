package permissionhttp

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.uber.org/mock/gomock"

	commonvalidation "github.com/aegiscore/common/validation"
	permissionquery "github.com/aegiscore/user-service/internal/features/permission/application/query"
	permissiondomain "github.com/aegiscore/user-service/internal/features/permission/domain"
)

func TestPermissionControllerGetRouteDiff(t *testing.T) {
	gin.SetMode(gin.TestMode)
	validator, err := commonvalidation.NewDefault()
	if err != nil {
		t.Fatalf("NewDefault validator: %v", err)
	}
	commands := NewMockPermissionCommandService(gomock.NewController(t))
	queries := NewMockPermissionQueryService(gomock.NewController(t))
	queries.EXPECT().GetRouteDiff(gomock.Any()).Return(&permissionquery.RouteDiffResult{MissingInPermissions: nil, StalePermissions: []permissiondomain.Permission{{PermissionID: uuid.MustParse("018f0000-0000-7000-8000-000000000001"), HTTPMethod: "GET", PathTemplate: "/api/v1/stale"}}}, nil)
	controller := NewPermissionController(commands, queries, validator)
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
