package permission

import (
	"context"
	"strconv"
	"strings"
	"testing"

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
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	recorder, err := newPermissionMetrics(provider)
	if err != nil {
		t.Fatalf("newPermissionMetrics: %v", err)
	}
	recorder.PolicyReloadSucceeded(context.Background(), permissionapplication.MetricsSourceLocalChange)
	recorder.PolicyPublishFailed(context.Background(), permissionapplication.MetricsReasonPublishFailed)
	recorder.WatcherVersionMismatch(context.Background(), permissionapplication.MetricsSourceWatcherVersionCheck)
	recorder.RouteDiffObserved(context.Background(), 2, 1)

	text := gatherPermissionMetricText(t, provider)
	for _, want := range []string{
		`aegiscore_user_service_rbac_policy_sync_operations_total{environment="test",operation="policy_reload",reason="none",result="success",service="aegiscore-user-service-test",source="local_change"} 1`,
		`aegiscore_user_service_rbac_policy_sync_operations_total{environment="test",operation="policy_publish",reason="publish_failed",result="failure",service="aegiscore-user-service-test",source="local_change"} 1`,
		`aegiscore_user_service_rbac_policy_version_mismatches_total{environment="test",service="aegiscore-user-service-test",source="watcher_version_check"} 1`,
		`aegiscore_user_service_permission_route_diff{environment="test",kind="missing",service="aegiscore-user-service-test"} 2`,
		`aegiscore_user_service_permission_route_diff{environment="test",kind="stale",service="aegiscore-user-service-test"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q:\n%s", want, text)
		}
	}
}

func TestPermissionMetricsDisabledUsesNoop(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: false},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	if _, err := newPermissionMetrics(provider); err != nil {
		t.Fatalf("newPermissionMetrics disabled: %v", err)
	}
}

func gatherPermissionMetricText(t *testing.T, provider *commonmetrics.Provider) string {
	t.Helper()
	families, err := provider.Gatherer().Gather()
	if err != nil {
		t.Fatalf("Gather: %v", err)
	}
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
			}
			builder.WriteString("\n")
		}
	}
	return builder.String()
}
