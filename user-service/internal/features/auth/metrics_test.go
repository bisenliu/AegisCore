package auth

import (
	"context"
	"strconv"
	"strings"
	"testing"

	"github.com/aegiscore/common/runtime/config"
	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	authapplication "github.com/aegiscore/user-service/internal/features/auth/application"
)

func TestAuthPrometheusMetrics(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: true},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	recorder, err := newAuthMetrics(provider)
	if err != nil {
		t.Fatalf("newAuthMetrics: %v", err)
	}
	recorder.LoginSucceeded(context.Background())
	recorder.RefreshFailed(context.Background(), authapplication.MetricsReasonTokenVersionMismatch)
	recorder.TokenVersionMismatch(context.Background(), authapplication.MetricsSourceAccessToken)
	recorder.SessionPurgeSubmitFailed(context.Background())

	text := gatherAuthMetricText(t, provider)
	for _, want := range []string{
		`aegiscore_user_service_auth_operations_total{environment="test",operation="login",reason="none",result="success",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_auth_operations_total{environment="test",operation="refresh",reason="token_version_mismatch",result="failure",service="aegiscore-user-service-test"} 1`,
		`aegiscore_user_service_auth_token_version_mismatches_total{environment="test",service="aegiscore-user-service-test",source="access_token"} 1`,
		`aegiscore_user_service_auth_session_purge_submit_failures_total{environment="test",service="aegiscore-user-service-test"} 1`,
	} {
		if !strings.Contains(text, want) {
			t.Fatalf("metrics missing %q:\n%s", want, text)
		}
	}
}

func TestAuthMetricsDisabledUsesNoop(t *testing.T) {
	provider, err := commonmetrics.NewProvider(commonmetrics.Options{
		Config:      config.MetricsConfig{Enabled: false},
		ServiceName: "aegiscore-user-service-test",
		Environment: "test",
	})
	if err != nil {
		t.Fatalf("NewProvider: %v", err)
	}
	recorder, err := newAuthMetrics(provider)
	if err != nil {
		t.Fatalf("newAuthMetrics: %v", err)
	}
	if _, ok := recorder.(interface{ LoginSucceeded(context.Context) }); !ok {
		t.Fatalf("recorder does not implement auth metrics")
	}
}

func gatherAuthMetricText(t *testing.T, provider *commonmetrics.Provider) string {
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
				builder.WriteString(strings.TrimRight(strings.TrimRight(formatAuthMetricFloat(metric.GetCounter().GetValue()), "0"), "."))
			case metric.GetGauge() != nil:
				builder.WriteString(strings.TrimRight(strings.TrimRight(formatAuthMetricFloat(metric.GetGauge().GetValue()), "0"), "."))
			}
			builder.WriteString("\n")
		}
	}
	return builder.String()
}

func formatAuthMetricFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', 6, 64)
}
