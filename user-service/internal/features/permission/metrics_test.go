package permission

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

func TestPermissionPrometheusMetrics(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: true},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	require.NoError(t, err)
	health := &metricPolicyHealth{status: permissionapplication.PolicyProjectionStatus{
		Initialized:     true,
		ReloadSucceeded: false,
		AppliedRevision: 11,
		TargetRevision:  12,
		LastError:       errors.New("reload failed"),
	}}
	recorder, err := newPermissionMetrics(permissionMetricsParams{Provider: provider, PolicyHealth: health})
	require.NoError(t, err)
	recorder.PolicyReloadSucceeded(context.Background(), permissionapplication.MetricsSourceLocalChange)
	recorder.PolicyPublishFailed(context.Background(), permissionapplication.MetricsReasonPublishFailed)
	recorder.WatcherVersionMismatch(context.Background(), permissionapplication.MetricsSourceWatcherRevisionCheck, permissionapplication.MetricsReasonRevisionMismatch)
	recorder.WatcherCheckFailed(context.Background(), permissionapplication.MetricsSourceWatcherPubSub, permissionapplication.MetricsReasonRevisionStoreUnavailable)
	recorder.PolicyReloadFailed(context.Background(), permissionapplication.MetricsSourceLocalChange, permissionapplication.MetricsReasonReloadFailed)
	recorder.PolicyReloadLagObserved(context.Background(), 4)
	recorder.PolicyReloadLagObserved(context.Background(), -1)
	recorder.PolicyReloadLagObserved(context.Background(), 2)
	recorder.EnforceObserved(context.Background(), permissionapplication.MetricsEnforceResultAllow, "GET", "/api/v1/users/:user_id", 10*time.Millisecond)
	recorder.EnforceObserved(context.Background(), permissionapplication.MetricsEnforceResultDeny, "DELETE", "/api/v1/users/:user_id", 20*time.Millisecond)
	recorder.EnforceObserved(context.Background(), "unexpected", "PATCH", "/api/v1/users/:user_id", 30*time.Millisecond)
	recorder.DispatcherOperationObserved(context.Background(), permissionapplication.MetricsOperationDispatcherClaim, permissionapplication.MetricsResultSuccess, permissionapplication.MetricsReasonNone, permissionapplication.MetricsKindNone)
	recorder.DispatcherOperationObserved(context.Background(), permissionapplication.MetricsOperationDispatcherPublish, permissionapplication.MetricsResultFailure, permissionapplication.MetricsReasonPublishFailed, permissionapplication.MetricsKindPolicyChanged)
	recorder.DispatcherBacklogObserved(context.Background(), 7, 3*time.Minute)
	recorder.DispatcherRunningObserved(context.Background(), true)

	text := gatherPermissionMetricText(t, provider)
	for _, want := range []string{
		`aegiscore_user_service_rbac_policy_sync_operations_total{environment="test",operation="policy_reload",reason="none",result="success",service="aegiscore-user-service-test",source="local_change"} 1`,
		`aegiscore_user_service_rbac_policy_sync_operations_total{environment="test",operation="policy_publish",reason="publish_failed",result="failure",service="aegiscore-user-service-test",source="local_change"} 1`,
		`aegiscore_user_service_rbac_policy_version_mismatches_total{environment="test",reason="revision_mismatch",service="aegiscore-user-service-test",source="watcher_revision_check"} 1`,
		`aegiscore_user_service_rbac_policy_sync_operations_total{environment="test",operation="watcher_revision_check",reason="revision_store_unavailable",result="failure",service="aegiscore-user-service-test",source="watcher_pubsub"} 1`,
		`aegiscore_user_service_rbac_policy_applied_revision{environment="test",service="aegiscore-user-service-test"} 11`,
		`aegiscore_user_service_rbac_policy_reload_lag{environment="test",service="aegiscore-user-service-test"} 2`,
		`aegiscore_user_service_rbac_enforce_total{environment="test",method="GET",result="allow",route_template="/api/v1/users/:user_id",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_enforce_total{environment="test",method="DELETE",result="deny",route_template="/api/v1/users/:user_id",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_enforce_total{environment="test",method="PATCH",result="error",route_template="/api/v1/users/:user_id",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_enforce_duration_seconds{environment="test",method="GET",result="allow",route_template="/api/v1/users/:user_id",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_outbox_dispatcher_operations_total{environment="test",kind="none",operation="dispatcher_claim",reason="none",result="success",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_outbox_dispatcher_operations_total{environment="test",kind="policy_changed",operation="dispatcher_publish",reason="publish_failed",result="failure",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_rbac_outbox_due_events{environment="test",service="aegiscore-user-service-test"} 7`,
		`aegiscore_user_service_rbac_outbox_oldest_unfinished_age_seconds{environment="test",service="aegiscore-user-service-test"} 180`,
		`aegiscore_user_service_rbac_outbox_dispatcher_running{environment="test",service="aegiscore-user-service-test"} 1`,
	} {
		require.Contains(t, text, want)
	}
	require.NotContains(t, text, "user_id=\"")
	require.NotContains(t, text, "role_id=\"")
	require.NotContains(t, text, "permission_id=\"")
	require.NotContains(t, text, "revision=\"")
	require.NotContains(t, text, "raw_path=")
	require.NotContains(t, text, "reason=\"reload_lag\"")
	require.NotContains(t, text, "error=")
	require.Equal(t, "Current RBAC policy projection lag measured as max(known latest database revision - local engine applied revision, 0).", rbacPolicyReloadLagMetricHelp)
}

func TestPermissionMetricsDisabledUsesNoop(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: false},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	require.NoError(t, err)
	_, err = newPermissionMetrics(permissionMetricsParams{Provider: provider})
	require.NoError(t, err)
}

func TestCasbinPolicyReloadMetricsRecordsStatus(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: true},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	require.NoError(t, err)
	recorder := newCasbinPolicyReloadMetrics(provider)
	recorder.ReloadSucceeded()
	recorder.SetLastStatus(true)
	recorder.ReloadFailed()
	recorder.SetLastStatus(false)

	text := gatherPermissionMetricText(t, provider)
	for _, want := range []string{
		`aegiscore_casbin_policy_reloads_total{environment="test",service="aegiscore-user-service-test",status="success"} 1`,
		`aegiscore_casbin_policy_reloads_total{environment="test",service="aegiscore-user-service-test",status="failure"} 1`,
		`aegiscore_casbin_policy_reload_last_success{environment="test",service="aegiscore-user-service-test"} 0`,
	} {
		require.Contains(t, text, want)
	}
}

func TestCasbinPolicyReloadMetricsDisabledUsesNoop(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: false},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	require.NoError(t, err)
	recorder := newCasbinPolicyReloadMetrics(provider)
	recorder.ReloadSucceeded()
	recorder.ReloadFailed()
	recorder.SetLastStatus(false)
}

type metricPolicyHealth struct {
	status permissionapplication.PolicyProjectionStatus
}

func (h *metricPolicyHealth) ProjectionStatus() permissionapplication.PolicyProjectionStatus {
	return h.status
}

func gatherPermissionMetricText(t *testing.T, provider *commonmetrics.Provider) string {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	require.NoError(t, err)
	var builder strings.Builder
	for _, family := range families {
		for _, metric := range family.GetMetric() {
			builder.WriteString(family.GetName())
			builder.WriteString("{")
			for i, label := range metric.GetLabel() {
				if i > 0 {
					builder.WriteString(",")
				}
				builder.WriteString(label.GetName())
				builder.WriteString(`="`)
				builder.WriteString(label.GetValue())
				builder.WriteString(`"`)
			}
			builder.WriteString("} ")
			switch {
			case metric.GetCounter() != nil:
				builder.WriteString(strings.TrimRight(strings.TrimRight(strconv.FormatFloat(metric.GetCounter().GetValue(), 'f', 6, 64), "0"), "."))
			case metric.GetGauge() != nil:
				builder.WriteString(strings.TrimRight(strings.TrimRight(strconv.FormatFloat(metric.GetGauge().GetValue(), 'f', 6, 64), "0"), "."))
			case metric.GetHistogram() != nil:
				builder.WriteString(strconv.FormatUint(metric.GetHistogram().GetSampleCount(), 10))
			}
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
