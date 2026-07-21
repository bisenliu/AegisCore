package authorization

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

func TestAuthorizerEnforceDelegatesValidUserID(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	var gotUserID uuid.UUID
	var gotPathTemplate string
	var gotMethod string
	engine.EXPECT().Enforce(gomock.Any(), gomock.Any(), "/api/v1/users/:user_id", "GET").DoAndReturn(func(_ context.Context, userID uuid.UUID, pathTemplate string, method string) (bool, error) {
		gotUserID = userID
		gotPathTemplate = pathTemplate
		gotMethod = method
		return true, nil
	})
	metrics := &spyMetrics{}
	authz := NewAuthorizer(engine, metrics)
	userID := "018f0000-0000-7000-8000-000000000701"
	allowed, err := authz.Enforce(context.Background(), userID, "/api/v1/users/:user_id", "GET")
	require.NoError(t, err)
	require.True(t, allowed)
	require.Equal(t, userID, gotUserID.String())
	require.Equal(t, "/api/v1/users/:user_id", gotPathTemplate)
	require.Equal(t, "GET", gotMethod)
	require.Len(t, metrics.enforce, 1)
	require.Equal(t, permissionapplication.MetricsEnforceResultAllow, metrics.enforce[0].result)
	require.Equal(t, "GET", metrics.enforce[0].method)
	require.Equal(t, "/api/v1/users/:user_id", metrics.enforce[0].routeTemplate)
	require.Positive(t, metrics.enforce[0].duration)
}

func TestAuthorizerEnforceInvalidUserIDFailsClosed(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	metrics := &spyMetrics{}
	authz := NewAuthorizer(engine, metrics)
	allowed, err := authz.Enforce(context.Background(), "not-a-uuid", "/api/v1/users", "GET")
	require.ErrorIs(t, err, ErrInvalidSubjectUserID)
	require.False(t, allowed)
	require.Len(t, metrics.enforce, 1)
	require.Equal(t, permissionapplication.MetricsEnforceResultError, metrics.enforce[0].result)
	require.Equal(t, "GET", metrics.enforce[0].method)
	require.Equal(t, "/api/v1/users", metrics.enforce[0].routeTemplate)
}

func TestAuthorizerEnforceRecordsDeny(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	engine.EXPECT().Enforce(gomock.Any(), gomock.Any(), "/api/v1/users/:user_id", "DELETE").Return(false, nil)
	metrics := &spyMetrics{}
	authz := NewAuthorizer(engine, metrics)

	allowed, err := authz.Enforce(context.Background(), "018f0000-0000-7000-8000-000000000701", "/api/v1/users/:user_id", "DELETE")
	require.NoError(t, err)
	require.False(t, allowed)
	require.Len(t, metrics.enforce, 1)
	require.Equal(t, permissionapplication.MetricsEnforceResultDeny, metrics.enforce[0].result)
	require.Equal(t, "DELETE", metrics.enforce[0].method)
	require.Equal(t, "/api/v1/users/:user_id", metrics.enforce[0].routeTemplate)
}

func TestAuthorizerEnforceRecordsEngineError(t *testing.T) {
	engine := NewMockEngine(gomock.NewController(t))
	engineErr := errors.New("role resolver unavailable")
	engine.EXPECT().Enforce(gomock.Any(), gomock.Any(), "/api/v1/users/:user_id", "PATCH").Return(false, engineErr)
	metrics := &spyMetrics{}
	authz := NewAuthorizer(engine, metrics)

	allowed, err := authz.Enforce(context.Background(), "018f0000-0000-7000-8000-000000000701", "/api/v1/users/:user_id", "PATCH")
	require.ErrorIs(t, err, engineErr)
	require.False(t, allowed)
	require.Len(t, metrics.enforce, 1)
	require.Equal(t, permissionapplication.MetricsEnforceResultError, metrics.enforce[0].result)
	require.Equal(t, "PATCH", metrics.enforce[0].method)
	require.Equal(t, "/api/v1/users/:user_id", metrics.enforce[0].routeTemplate)
}

type enforceObservation struct {
	result        string
	method        string
	routeTemplate string
	duration      time.Duration
}

type spyMetrics struct {
	enforce []enforceObservation
}

func (m *spyMetrics) PolicyReloadSucceeded(context.Context, string)       {}
func (m *spyMetrics) PolicyReloadFailed(context.Context, string, string)  {}
func (m *spyMetrics) PolicyPublishSucceeded(context.Context)              {}
func (m *spyMetrics) PolicyPublishFailed(context.Context, string)         {}
func (m *spyMetrics) WatcherCheckFailed(context.Context, string)          {}
func (m *spyMetrics) WatcherReloadSucceeded(context.Context, string)      {}
func (m *spyMetrics) WatcherReloadFailed(context.Context, string, string) {}
func (m *spyMetrics) WatcherVersionMismatch(context.Context, string)      {}
func (m *spyMetrics) EnforceObserved(_ context.Context, result string, method string, routeTemplate string, duration time.Duration) {
	m.enforce = append(m.enforce, enforceObservation{result: result, method: method, routeTemplate: routeTemplate, duration: duration})
}
