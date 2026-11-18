package permission

import (
	"context"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"

	commonmetrics "github.com/aegiscore/common/runtime/observability/metrics"
	permissionapplication "github.com/aegiscore/user-service/internal/features/permission/application"
	permissionauthorization "github.com/aegiscore/user-service/internal/features/permission/application/authorization"
)

const (
	rbacPolicySyncMetricName      = "aegiscore_user_service_rbac_policy_sync_operations_total"
	rbacPolicyMismatchMetricName  = "aegiscore_user_service_rbac_policy_version_mismatches_total"
	rbacPolicyAppliedMetricName   = "aegiscore_user_service_rbac_policy_applied_revision"
	rbacPolicyReloadLagMetricName = "aegiscore_user_service_rbac_policy_reload_lag"
	rbacEnforceMetricName         = "aegiscore_user_service_rbac_enforce_total"
	rbacEnforceLatencyMetricName  = "aegiscore_user_service_rbac_enforce_duration_seconds"
	rbacPolicySyncMetricHelp      = "Total number of RBAC policy sync operation results by fixed operation, result, reason, and source."
	rbacPolicyMismatchMetricHelp  = "Total number of RBAC policy version mismatches by fixed watcher source."
	rbacPolicyAppliedMetricHelp   = "Current database policy revision atomically applied by the local Casbin engine."
	rbacPolicyReloadLagMetricHelp = "Current RBAC policy projection lag measured as max(known latest database revision - local engine applied revision, 0)."
	rbacEnforceMetricHelp         = "Total number of RBAC enforce decisions by fixed result, method, and route template."
	rbacEnforceLatencyMetricHelp  = "RBAC enforce latency in seconds by fixed result, method, and route template."
)

type prometheusMetrics struct {
	policySync      *prometheus.CounterVec
	versionMismatch *prometheus.CounterVec
	policyApplied   prometheus.GaugeFunc
	policyReloadLag prometheus.Gauge
	enforce         *prometheus.CounterVec
	enforceLatency  *prometheus.HistogramVec
}

type permissionMetricsParams struct {
	fx.In

	Provider     *commonmetrics.Provider
	PolicyHealth permissionauthorization.PolicyHealth `name:"permission_policy_health"`
}

// newPermissionMetrics 创建用于统计权限同步、加载和拦截情况的监控指标。
func newPermissionMetrics(params permissionMetricsParams) (permissionapplication.Metrics, error) {
	provider := params.Provider
	policyHealth := params.PolicyHealth
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
		}, []string{"source", "reason"}),
		policyApplied: prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: rbacPolicyAppliedMetricName,
			Help: rbacPolicyAppliedMetricHelp,
		}, func() float64 {
			if policyHealth == nil {
				return 0
			}
			revision := policyHealth.ProjectionStatus().AppliedRevision
			if revision < 0 {
				return 0
			}
			return float64(revision)
		}),
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
	}
	if err := provider.Register(recorder.policySync); err != nil {
		return nil, fmt.Errorf("register rbac policy sync metrics: %w", err)
	}
	if err := provider.Register(recorder.versionMismatch); err != nil {
		return nil, fmt.Errorf("register rbac policy version mismatch metrics: %w", err)
	}
	if err := provider.Register(recorder.policyApplied); err != nil {
		return nil, fmt.Errorf("register rbac policy applied revision metric: %w", err)
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
	return recorder, nil
}

// PolicyReloadSucceeded 记录权限规则重新加载成功的次数。
func (m *prometheusMetrics) PolicyReloadSucceeded(_ context.Context, source string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyReload, commonmetrics.StatusSuccess, permissionapplication.MetricsReasonNone, rbacSource(source)).Inc()
}

// PolicyReloadFailed 记录权限规则重新加载失败的次数及原因。
func (m *prometheusMetrics) PolicyReloadFailed(_ context.Context, source string, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyReload, commonmetrics.StatusFailure, rbacReason(reason), rbacSource(source)).Inc()
}

// PolicyPublishSucceeded 记录广播权限变更通知成功的次数。
func (m *prometheusMetrics) PolicyPublishSucceeded(context.Context) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyPublish, commonmetrics.StatusSuccess, permissionapplication.MetricsReasonNone, permissionapplication.MetricsSourceLocalChange).Inc()
}

// PolicyPublishFailed 记录广播权限变更通知失败的次数。
func (m *prometheusMetrics) PolicyPublishFailed(_ context.Context, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationPolicyPublish, commonmetrics.StatusFailure, rbacReason(reason), permissionapplication.MetricsSourceLocalChange).Inc()
}

// WatcherCheckFailed 记录监听组件执行健康检查或对账检查失败的次数。
func (m *prometheusMetrics) WatcherCheckFailed(_ context.Context, source string, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationWatcherRevisionCheck, commonmetrics.StatusFailure, rbacReason(reason), rbacSource(source)).Inc()
}

// WatcherReloadSucceeded 记录由网络广播触发的权限更新成功次数。
func (m *prometheusMetrics) WatcherReloadSucceeded(_ context.Context, source string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationWatcherReload, commonmetrics.StatusSuccess, permissionapplication.MetricsReasonNone, rbacSource(source)).Inc()
}

// WatcherReloadFailed 记录由网络广播触发的权限更新失败次数。
func (m *prometheusMetrics) WatcherReloadFailed(_ context.Context, source string, reason string) {
	m.policySync.WithLabelValues(permissionapplication.MetricsOperationWatcherReload, commonmetrics.StatusFailure, rbacReason(reason), rbacSource(source)).Inc()
}

// WatcherVersionMismatch 记录本地缓存版本与数据库真实版本出现差异的次数。
func (m *prometheusMetrics) WatcherVersionMismatch(_ context.Context, source string, reason string) {
	m.versionMismatch.WithLabelValues(rbacSource(source), rbacReason(reason)).Inc()
}

// PolicyReloadLagObserved 记录本地节点版本落后数据库最新版本的代数延迟数值。
func (m *prometheusMetrics) PolicyReloadLagObserved(_ context.Context, lag int64) {
	if lag < 0 {
		lag = 0
	}
	m.policyReloadLag.Set(float64(lag))
}

// EnforceObserved 记录单次用户接口访问权限检查的结果（通过还是拦截）以及耗时。
func (m *prometheusMetrics) EnforceObserved(_ context.Context, result string, method string, routeTemplate string, duration time.Duration) {
	result = rbacEnforceResult(result)
	m.enforce.WithLabelValues(result, method, routeTemplate).Inc()
	m.enforceLatency.WithLabelValues(result, method, routeTemplate).Observe(duration.Seconds())
}

// rbacSource 规范化权限更新触发来源的统计标签（如定时检查或广播推送）。
func rbacSource(source string) string {
	switch source {
	case permissionapplication.MetricsSourceLocalChange,
		permissionapplication.MetricsSourceWatcherPubSub,
		permissionapplication.MetricsSourceWatcherRevisionCheck:
		return source
	default:
		return permissionapplication.MetricsSourceLocalChange
	}
}

// rbacReason 规范化权限操作失败原因的统计标签。
func rbacReason(reason string) string {
	switch reason {
	case permissionapplication.MetricsReasonNone,
		permissionapplication.MetricsReasonReloadFailed,
		permissionapplication.MetricsReasonPublishFailed,
		permissionapplication.MetricsReasonRevisionStoreUnavailable,
		permissionapplication.MetricsReasonRevisionMismatch,
		permissionapplication.MetricsReasonSystemError:
		return reason
	default:
		return permissionapplication.MetricsReasonSystemError
	}
}

// rbacEnforceResult 规范化权限核对结果（允许访问或禁止访问）。
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
