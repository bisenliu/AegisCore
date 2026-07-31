package permission

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
)

const (
	rbacPolicySyncMetricName      = "aegiscore_user_service_rbac_policy_sync_operations_total"
	rbacPolicyMismatchMetricName  = "aegiscore_user_service_rbac_policy_version_mismatches_total"
	rbacPolicyReloadLagMetricName = "aegiscore_user_service_rbac_policy_reload_lag"
	rbacEnforceMetricName         = "aegiscore_user_service_rbac_enforce_total"
	rbacEnforceLatencyMetricName  = "aegiscore_user_service_rbac_enforce_duration_seconds"
	rbacDispatcherMetricName      = "aegiscore_user_service_rbac_outbox_dispatcher_operations_total"
	rbacDispatcherDueMetricName   = "aegiscore_user_service_rbac_outbox_due_events"
	rbacDispatcherAgeMetricName   = "aegiscore_user_service_rbac_outbox_oldest_unfinished_age_seconds"
	rbacDispatcherRunningName     = "aegiscore_user_service_rbac_outbox_dispatcher_running"
	rbacPolicySyncMetricHelp      = "Total number of RBAC policy sync operation results by fixed operation, result, reason, and source."
	rbacPolicyMismatchMetricHelp  = "Total number of RBAC policy version mismatches by fixed watcher source."
	rbacPolicyReloadLagMetricHelp = "Current RBAC policy reload lag measured as the non-negative difference between latest Redis policy version and local applied policy version."
	rbacEnforceMetricHelp         = "Total number of RBAC enforce decisions by fixed result, method, and route template."
	rbacEnforceLatencyMetricHelp  = "RBAC enforce latency in seconds by fixed result, method, and route template."
	rbacDispatcherMetricHelp      = "Total number of RBAC outbox dispatcher operations by fixed operation, result, and reason."
	rbacDispatcherDueMetricHelp   = "Current number of due RBAC policy outbox events."
	rbacDispatcherAgeMetricHelp   = "Current age in seconds of the oldest unfinished RBAC policy outbox event."
	rbacDispatcherRunningHelp     = "Whether the RBAC policy outbox dispatcher is running."
)

type prometheusMetrics struct {
	policySync      *prometheus.CounterVec
	versionMismatch *prometheus.CounterVec
	policyReloadLag prometheus.Gauge
	enforce         *prometheus.CounterVec
	enforceLatency  *prometheus.HistogramVec
	dispatcher      *prometheus.CounterVec
	dispatcherDue   prometheus.Gauge
	dispatcherAge   prometheus.Gauge
	dispatcherRun   prometheus.Gauge
}

func newPermissionMetrics(provider *commonmetrics.Provider) (permissionapplication.Metrics, error) {
	if provider == nil || !provider.Enabled() {
		return permissionapplication.NopMetrics(), nil
	}
	recorder := &prometheusMetrics{
		policySync: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: rbacPolicySyncMetricName,
			Help: rbacPolicySyncMetricHelp,
		}, []string{"operation", "result", "reason", "source"}),
		versionMismatch: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: rbacPolicyMismatchMetricName,
			Help: rbacPolicyMismatchMetricHelp,
		}, []string{"source"}),
		policyReloadLag: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: rbacPolicyReloadLagMetricName,
			Help: rbacPolicyReloadLagMetricHelp,
		}),
		enforce: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: rbacEnforceMetricName,
			Help: rbacEnforceMetricHelp,
		}, []string{"result", "method", "route_template"}),
		enforceLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    rbacEnforceLatencyMetricName,
			Help:    rbacEnforceLatencyMetricHelp,
			Buckets: prometheus.DefBuckets,
		}, []string{"result", "method", "route_template"}),
		dispatcher: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: rbacDispatcherMetricName,
			Help: rbacDispatcherMetricHelp,
		}, []string{"operation", "result", "reason", "kind"}),
		dispatcherDue: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: rbacDispatcherDueMetricName,
			Help: rbacDispatcherDueMetricHelp,
		}),
		dispatcherAge: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: rbacDispatcherAgeMetricName,
			Help: rbacDispatcherAgeMetricHelp,
		}),
		dispatcherRun: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: rbacDispatcherRunningName,
			Help: rbacDispatcherRunningHelp,
		}),
	}
	if err := provider.Register(recorder.policySync); err != nil {
		return nil, fmt.Errorf("register rbac policy sync metrics: %w", err)
	}
	if err := provider.Register(recorder.versionMismatch); err != nil {
		return nil, fmt.Errorf("register rbac policy version mismatch metrics: %w", err)
	}
	if err := provider.Register(recorder.policyReloadLag); err != nil {
		return nil, fmt.Errorf("register rbac policy reload lag metrics: %w", err)
	}
	if err := provider.Register(recorder.enforce); err != nil {
		return nil, fmt.Errorf("register rbac enforce metrics: %w", err)
	}
	if err := provider.Register(recorder.enforceLatency); err != nil {
		return nil, fmt.Errorf("register rbac enforce latency metrics: %w", err)
	}
	if err := provider.Register(recorder.dispatcher); err != nil {
		return nil, fmt.Errorf("register rbac outbox dispatcher metrics: %w", err)
	}
	if err := provider.Register(recorder.dispatcherDue); err != nil {
		return nil, fmt.Errorf("register rbac outbox due events metric: %w", err)
	}
	if err := provider.Register(recorder.dispatcherAge); err != nil {
		return nil, fmt.Errorf("register rbac outbox oldest unfinished age metric: %w", err)
	}
	if err := provider.Register(recorder.dispatcherRun); err != nil {
		return nil, fmt.Errorf("register rbac outbox dispatcher running metric: %w", err)
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

func (m *prometheusMetrics) PolicyReloadLagObserved(_ context.Context, lag int64) {
	if lag < 0 {
		lag = 0
	}
	m.policyReloadLag.Set(float64(lag))
}

func (m *prometheusMetrics) EnforceObserved(_ context.Context, result string, method string, routeTemplate string, duration time.Duration) {
	result = rbacEnforceResult(result)
	m.enforce.WithLabelValues(result, method, routeTemplate).Inc()
	m.enforceLatency.WithLabelValues(result, method, routeTemplate).Observe(duration.Seconds())
}

func (m *prometheusMetrics) DispatcherOperationObserved(_ context.Context, operation string, result string, reason string, kind string) {
	m.dispatcher.WithLabelValues(rbacDispatcherOperation(operation), rbacDispatcherResult(result), rbacDispatcherReason(reason), rbacDispatcherKind(kind)).Inc()
}

func rbacDispatcherKind(kind string) string {
	switch kind {
	case permissionapplication.MetricsKindNone,
		permissionapplication.MetricsKindPolicyChanged,
		permissionapplication.MetricsKindUserRoleChanged:
		return kind
	default:
		return permissionapplication.MetricsKindNone
	}
}

func (m *prometheusMetrics) DispatcherBacklogObserved(_ context.Context, dueCount int, oldestUnfinishedAge time.Duration) {
	if dueCount < 0 {
		dueCount = 0
	}
	if oldestUnfinishedAge < 0 {
		oldestUnfinishedAge = 0
	}
	m.dispatcherDue.Set(float64(dueCount))
	m.dispatcherAge.Set(oldestUnfinishedAge.Seconds())
}

func (m *prometheusMetrics) DispatcherRunningObserved(_ context.Context, running bool) {
	if running {
		m.dispatcherRun.Set(1)
		return
	}
	m.dispatcherRun.Set(0)
}

func rbacDispatcherOperation(operation string) string {
	switch operation {
	case permissionapplication.MetricsOperationDispatcherClaim,
		permissionapplication.MetricsOperationDispatcherPublish,
		permissionapplication.MetricsOperationDispatcherAck,
		permissionapplication.MetricsOperationDispatcherFailure,
		permissionapplication.MetricsOperationDispatcherRetry:
		return operation
	default:
		return permissionapplication.MetricsOperationDispatcherFailure
	}
}

func rbacDispatcherResult(result string) string {
	switch result {
	case permissionapplication.MetricsResultSuccess, permissionapplication.MetricsResultFailure:
		return result
	default:
		return permissionapplication.MetricsResultFailure
	}
}

func rbacDispatcherReason(reason string) string {
	switch reason {
	case permissionapplication.MetricsReasonNone,
		permissionapplication.MetricsReasonClaimFailed,
		permissionapplication.MetricsReasonPublishFailed,
		permissionapplication.MetricsReasonAckFailed,
		permissionapplication.MetricsReasonFailureRecordFailed,
		permissionapplication.MetricsReasonClaimLost:
		return reason
	default:
		return permissionapplication.MetricsReasonSystemError
	}
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

func rbacEnforceResult(result string) string {
	switch result {
	case permissionapplication.MetricsEnforceResultAllow,
		permissionapplication.MetricsEnforceResultDeny,
		permissionapplication.MetricsEnforceResultError:
		return result
	default:
		return permissionapplication.MetricsEnforceResultError
	}
}
