package permission

import (
	"context"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const (
	rbacPolicySyncMetricName      = "aegiscore_user_service_rbac_policy_sync_operations_total"
	rbacPolicyMismatchMetricName  = "aegiscore_user_service_rbac_policy_version_mismatches_total"
	permissionRouteDiffMetricName = "aegiscore_user_service_permission_route_diff"
)

type prometheusMetrics struct {
	policySync      *prometheus.CounterVec
	versionMismatch *prometheus.CounterVec
	routeDiff       *prometheus.GaugeVec
}

func newPermissionMetrics(provider *commonmetrics.Provider) (permissionapplication.Metrics, error) {
	if provider == nil || !provider.Enabled() {
		return permissionapplication.NopMetrics(), nil
	}
	recorder := &prometheusMetrics{
		policySync: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: rbacPolicySyncMetricName,
			Help: "Total number of RBAC policy sync business operation results.",
		}, []string{"operation", "result", "reason", "source"}),
		versionMismatch: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: rbacPolicyMismatchMetricName,
			Help: "Total number of RBAC policy version mismatches detected by watchers.",
		}, []string{"source"}),
		routeDiff: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: permissionRouteDiffMetricName,
			Help: "Latest permission route diff counts by kind.",
		}, []string{"kind"}),
	}
	if err := provider.Register(recorder.policySync); err != nil {
		return nil, fmt.Errorf("register rbac policy sync metrics: %w", err)
	}
	if err := provider.Register(recorder.versionMismatch); err != nil {
		return nil, fmt.Errorf("register rbac policy version mismatch metrics: %w", err)
	}
	if err := provider.Register(recorder.routeDiff); err != nil {
		return nil, fmt.Errorf("register permission route diff metrics: %w", err)
	}
	return recorder, nil
}

func (m *prometheusMetrics) PolicyReloadSucceeded(_ context.Context, source string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyReload, commonmetrics.StatusSuccess, permissionapplication.MetricsReasonNone, rbacSource(source)).Inc()
}

func (m *prometheusMetrics) PolicyReloadFailed(_ context.Context, source string, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyReload, commonmetrics.StatusFailure, rbacReason(reason), rbacSource(source)).Inc()
}

func (m *prometheusMetrics) PolicyPublishSucceeded(context.Context) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyPublish, commonmetrics.StatusSuccess, permissionapplication.MetricsReasonNone, permissionapplication.MetricsSourceLocalChange).Inc()
}

func (m *prometheusMetrics) PolicyPublishFailed(_ context.Context, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyPublish, commonmetrics.StatusFailure, rbacReason(reason), permissionapplication.MetricsSourceLocalChange).Inc()
}

func (m *prometheusMetrics) WatcherCheckFailed(_ context.Context, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationWatcherVersionCheck, commonmetrics.StatusFailure, rbacReason(reason), permissionapplication.MetricsSourceWatcherVersionCheck).Inc()
}

func (m *prometheusMetrics) WatcherReloadSucceeded(_ context.Context, source string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationWatcherReload, commonmetrics.StatusSuccess, permissionapplication.MetricsReasonNone, rbacSource(source)).Inc()
}

func (m *prometheusMetrics) WatcherReloadFailed(_ context.Context, source string, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationWatcherReload, commonmetrics.StatusFailure, rbacReason(reason), rbacSource(source)).Inc()
}

func (m *prometheusMetrics) WatcherVersionMismatch(_ context.Context, source string) {
	m.versionMismatch.WithLabelValues(rbacSource(source)).Inc()
}

func (m *prometheusMetrics) RouteDiffObserved(_ context.Context, missing int, stale int) {
	m.routeDiff.WithLabelValues(permissionapplication.MetricsRouteDiffMissing).Set(float64(missing))
	m.routeDiff.WithLabelValues(permissionapplication.MetricsRouteDiffStale).Set(float64(stale))
}

func rbacSource(source string) string {
	switch source {
	case permissionapplication.MetricsSourceLocalChange,
		permissionapplication.MetricsSourceWatcherPubSub,
		permissionapplication.MetricsSourceWatcherVersionCheck:
		return source
	default:
		return permissionapplication.MetricsSourceLocalChange
	}
}

func rbacReason(reason string) string {
	switch reason {
	case permissionapplication.MetricsReasonNone,
		permissionapplication.MetricsReasonReloadFailed,
		permissionapplication.MetricsReasonPublishFailed,
		permissionapplication.MetricsReasonStoreUnavailable,
		permissionapplication.MetricsReasonSystemError:
		return reason
	default:
		return permissionapplication.MetricsReasonSystemError
	}
}
